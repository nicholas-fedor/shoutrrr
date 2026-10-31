package xmpp

import (
	"net/url"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/nicholas-fedor/shoutrrr/pkg/format"
	"github.com/nicholas-fedor/shoutrrr/pkg/util"
)

// parseTestURL parses rawURL and fails the test if it is invalid.
//
// Parameters:
//   - t: the test.
//   - rawURL: the URL to parse.
//
// Returns:
//   - *url.URL: the parsed URL.
func parseTestURL(t *testing.T, rawURL string) *url.URL {
	t.Helper()

	parsed, err := url.Parse(rawURL)
	require.NoError(t, err)

	return parsed
}

// TestConfigSetURL verifies that SetURL reads the credentials, host, port,
// and targets, defaults the port by scheme, and reads the MUC and TLS props
// from the query.
func TestConfigSetURL(t *testing.T) {
	t.Parallel()

	// tests lists service URLs with the config each should produce.
	tests := []struct {
		name   string
		rawURL string
		want   Config
	}{
		{
			name:   "localpart user and chat recipient",
			rawURL: "xmpp://alice:secret@xmpp.example.com/?to=bob@example.com",
			want:   Config{Host: "xmpp.example.com", Port: DefaultPort, User: "alice", Password: "secret", To: []string{"bob@example.com"}, scheme: Scheme},
		},
		{
			name:   "full auth JID distinct from the dial host",
			rawURL: "xmpp://alice%40jabber.example.com:secret@xmpp.example.com/?to=bob@jabber.example.com",
			want: Config{
				Host: "xmpp.example.com", Port: DefaultPort, User: "alice@jabber.example.com", Password: "secret",
				To: []string{"bob@jabber.example.com"}, scheme: Scheme,
			},
		},
		{
			name:   "xmpps default port",
			rawURL: "xmpps://alice:secret@xmpp.example.com/?to=bob@example.com",
			want:   Config{Host: "xmpp.example.com", Port: DefaultTLSPort, User: "alice", Password: "secret", To: []string{"bob@example.com"}, scheme: SchemeTLS},
		},
		{
			name: "rooms, nick, room password, and custom port",
			rawURL: "xmpp://alice:secret@xmpp.example.com:5225/" +
				"?to=bob@example.com,carol@example.com&rooms=a@conf.example.com,b@conf.example.com&nick=bot&roompassword=s3cret",
			want: Config{
				Host: "xmpp.example.com", Port: 5225, User: "alice", Password: "secret",
				To:    []string{"bob@example.com", "carol@example.com"},
				Rooms: []string{"a@conf.example.com", "b@conf.example.com"},
				Nick:  "bot", RoomPassword: "s3cret", scheme: Scheme,
			},
		},
		{
			name:   "disabletls on xmpp, skiptlsverify, and title",
			rawURL: "xmpp://alice:secret@xmpp.example.com/?to=bob@example.com&disabletls=yes&skiptlsverify=yes&title=Alert",
			want: Config{
				Host: "xmpp.example.com", Port: DefaultPort, User: "alice", Password: "secret",
				To: []string{"bob@example.com"}, Title: "Alert", DisableTLS: true, SkipTLSVerify: true, scheme: Scheme,
			},
		},
		{
			name:   "empty targets dropped",
			rawURL: "xmpp://alice:secret@xmpp.example.com/?to=,bob@example.com,&rooms=",
			want:   Config{Host: "xmpp.example.com", Port: DefaultPort, User: "alice", Password: "secret", To: []string{"bob@example.com"}, scheme: Scheme},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			config := &Config{}
			require.NoError(t, config.SetURL(parseTestURL(t, tt.rawURL)))

			assert.Equal(t, tt.want, *config)
		})
	}

	t.Run("empty scheme defaults to xmpp", func(t *testing.T) {
		t.Parallel()

		serviceURL := parseTestURL(t, "xmpp://alice:secret@xmpp.example.com/?to=bob@example.com")
		serviceURL.Scheme = ""

		config := &Config{}
		require.NoError(t, config.SetURL(serviceURL))
		assert.Equal(t, Scheme, config.scheme)
	})
}

// TestConfigSetURLErrors verifies that SetURL rejects URLs with missing or
// invalid parts, and leaves the receiver unchanged when it fails.
func TestConfigSetURLErrors(t *testing.T) {
	t.Parallel()

	// tests lists invalid service URLs with the error each should match, or
	// the text it should contain when wantErr is nil.
	tests := []struct {
		wantErr  error
		name     string
		rawURL   string
		wantText string
	}{
		{name: "missing targets", rawURL: "xmpp://alice:secret@xmpp.example.com/", wantErr: ErrMissingTargets},
		{name: "missing password", rawURL: "xmpp://alice@xmpp.example.com/?to=bob@example.com", wantErr: ErrMissingPassword},
		{name: "missing user", rawURL: "xmpp://:secret@xmpp.example.com/?to=bob@example.com", wantErr: ErrMissingUser},
		{name: "missing host", rawURL: "xmpp://alice:secret@/?to=bob@example.com", wantErr: ErrMissingHost},
		{name: "unbracketed IPv6 host", rawURL: "xmpp://alice:secret@2001:db8::1/?to=bob@example.com", wantErr: util.ErrInvalidHost},
		{name: "disabletls on xmpps", rawURL: "xmpps://alice:secret@xmpp.example.com/?to=bob@example.com&disabletls=yes", wantErr: ErrDisableTLSOnXMPPS},
		{name: "invalid recipient JID", rawURL: "xmpp://alice:secret@xmpp.example.com/?to=not-a-jid", wantErr: ErrInvalidRecipientJID},
		{name: "invalid room JID", rawURL: "xmpp://alice:secret@xmpp.example.com/?rooms=not-a-jid", wantErr: ErrInvalidRoomJID},
		{name: "invalid auth JID", rawURL: "xmpp://alice%40:secret@xmpp.example.com/?to=bob@example.com", wantErr: ErrInvalidUserJID},
		{name: "unsupported scheme", rawURL: "http://alice:secret@xmpp.example.com/?to=bob@example.com", wantErr: ErrUnsupportedScheme},
		{name: "unknown prop", rawURL: "xmpp://alice:secret@xmpp.example.com/?to=bob@example.com&notakey=1", wantErr: format.ErrInvalidConfigKey},
		{name: "port out of range", rawURL: "xmpp://alice:secret@xmpp.example.com:70000/?to=bob@example.com", wantText: "parsing port"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			err := (&Config{}).SetURL(parseTestURL(t, tt.rawURL))

			if tt.wantErr != nil {
				require.ErrorIs(t, err, tt.wantErr)
			} else {
				require.Error(t, err)
				assert.Contains(t, err.Error(), tt.wantText)
			}
		})
	}

	t.Run("receiver unchanged on failure", func(t *testing.T) {
		t.Parallel()

		config := &Config{}
		require.NoError(t, config.SetURL(parseTestURL(t, "xmpp://alice:secret@xmpp.example.com/?to=bob@example.com")))
		want := config.Clone()

		require.ErrorIs(t, config.SetURL(parseTestURL(t, "xmpp://alice:secret@xmpp.example.com/")), ErrMissingTargets)
		assert.Equal(t, want, *config)
	})

	t.Run("omitted props reset", func(t *testing.T) {
		t.Parallel()

		config := &Config{}
		require.NoError(t, config.SetURL(parseTestURL(t,
			"xmpp://alice:secret@xmpp.example.com/?to=bob@example.com&rooms=alerts@conference.example.com&nick=bot")))
		require.NoError(t, config.SetURL(parseTestURL(t, "xmpp://carol:other@xmpp.example.com/?to=dave@example.com")))

		assert.Equal(t, Config{
			Host: "xmpp.example.com", Port: DefaultPort, User: "carol", Password: "other",
			To: []string{"dave@example.com"}, scheme: Scheme,
		}, *config)
	})
}

// TestConfigGetURL verifies that GetURL round-trips through SetURL, keeps the
// xmpps scheme, defaults the scheme and port, and leaves out empty user info.
func TestConfigGetURL(t *testing.T) {
	t.Parallel()

	// roundTrips lists service URLs that should round-trip unchanged in
	// config.
	roundTrips := []string{
		"xmpp://alice:secret@xmpp.example.com:5222/?to=bob@example.com",
		"xmpps://alice:secret@xmpp.example.com/?rooms=alerts@conference.example.com&nick=bot",
		"xmpp://alice%40jabber.example.com:p%40ss@xmpp.example.com:5225/?to=bob@example.com&disabletls=yes",
	}

	for _, rawURL := range roundTrips {
		t.Run(rawURL, func(t *testing.T) {
			t.Parallel()

			config := &Config{}
			require.NoError(t, config.SetURL(parseTestURL(t, rawURL)))

			roundTrip := &Config{}
			require.NoError(t, roundTrip.SetURL(config.GetURL()))
			assert.Equal(t, config, roundTrip)
		})
	}

	t.Run("default scheme and port", func(t *testing.T) {
		t.Parallel()

		serviceURL := (&Config{Host: "xmpp.example.com", User: "alice", Password: "secret"}).GetURL()

		assert.Equal(t, Scheme, serviceURL.Scheme)
		assert.Equal(t, "5222", serviceURL.Port())
		assert.Equal(t, "alice", serviceURL.User.Username())
	})

	t.Run("default xmpps port", func(t *testing.T) {
		t.Parallel()

		assert.Equal(t, "5223", (&Config{Host: "xmpp.example.com", scheme: SchemeTLS}).GetURL().Port())
	})

	t.Run("no user", func(t *testing.T) {
		t.Parallel()

		assert.Nil(t, (&Config{Host: "xmpp.example.com", scheme: Scheme, Port: DefaultPort}).GetURL().User)
	})
}

// TestConfigClone verifies that a clone has its own target lists.
func TestConfigClone(t *testing.T) {
	t.Parallel()

	config := &Config{To: []string{"bob@example.com"}, Rooms: []string{"alerts@conference.example.com"}}
	clone := config.Clone()

	clone.To[0] = "changed@example.com"
	clone.Rooms[0] = "changed@conference.example.com"

	assert.Equal(t, []string{"bob@example.com"}, config.To)
	assert.Equal(t, []string{"alerts@conference.example.com"}, config.Rooms)
}

// TestConfigHelpers verifies the auth JID, MUC nick, and TLS helpers.
func TestConfigHelpers(t *testing.T) {
	t.Parallel()

	assert.Equal(t, "alice@xmpp.example.com", (&Config{User: "alice", Host: "xmpp.example.com"}).authJIDString())
	assert.Equal(t, "alice@jabber.example.com", (&Config{User: "alice@jabber.example.com", Host: "xmpp.example.com"}).authJIDString())

	assert.Equal(t, "bot", (&Config{Nick: "bot", User: "alice", Host: "xmpp.example.com"}).mucNick())
	assert.Equal(t, "alice", (&Config{User: "alice", Host: "xmpp.example.com"}).mucNick())
	assert.Equal(t, Resource, (&Config{User: "alice"}).mucNick())

	assert.True(t, (&Config{scheme: SchemeTLS}).implicitTLS())
	assert.False(t, (&Config{scheme: Scheme}).implicitTLS())
	assert.True(t, (&Config{}).useTLS())
	assert.False(t, (&Config{DisableTLS: true}).useTLS())
}

// TestSplitJID verifies that splitJID drops a resource and rejects JIDs
// without a localpart or domain, or with a second @.
func TestSplitJID(t *testing.T) {
	t.Parallel()

	local, domain, err := splitJID("alice@jabber.example.com/phone")
	require.NoError(t, err)
	assert.Equal(t, "alice", local)
	assert.Equal(t, "jabber.example.com", domain)

	for _, value := range []string{"alice", "alice@", "@jabber.example.com", "alice@/phone", "alice@jabber@example.com"} {
		_, _, err := splitJID(value)
		require.ErrorIs(t, err, ErrInvalidUserJID, value)
	}
}

// TestComposeMessage verifies that the title goes on its own line before the
// message, and that either part alone is sent as is.
func TestComposeMessage(t *testing.T) {
	t.Parallel()

	assert.Equal(t, "hello", composeMessage("", "hello"))
	assert.Equal(t, "Alert\nhello", composeMessage("Alert", "hello"))
	assert.Equal(t, "Alert", composeMessage("Alert", ""))
}

// TestCompactStrings verifies that empty entries are dropped and that a list
// with none left becomes nil.
func TestCompactStrings(t *testing.T) {
	t.Parallel()

	assert.Equal(t, []string{"a", "b"}, compactStrings([]string{"", "a", "", "b"}))
	assert.Nil(t, compactStrings([]string{"", ""}))
	assert.Nil(t, compactStrings(nil))
}
