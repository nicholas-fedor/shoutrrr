package xmpp_test

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/nicholas-fedor/shoutrrr/pkg/services/chat/xmpp"
)

func TestConfigValidURLParsing(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name           string
		url            string
		wantHost       string
		wantPort       uint16
		wantUser       string
		wantTo         []string
		wantRooms      []string
		wantNick       string
		wantTLS        bool
		wantSkipVerify bool
	}{
		{
			name:     "xmpp scheme with chat target",
			url:      chatURL,
			wantHost: "xmpp.example.com",
			wantPort: xmpp.DefaultPort,
			wantUser: "alice",
			wantTo:   []string{"bob@example.com"},
			wantTLS:  true,
		},
		{
			name:     "xmpps scheme defaults to 5223",
			url:      "xmpps://alice:secret@xmpp.example.com/?to=bob@example.com",
			wantHost: "xmpp.example.com",
			wantPort: xmpp.DefaultTLSPort,
			wantUser: "alice",
			wantTo:   []string{"bob@example.com"},
			wantTLS:  true,
		},
		{
			name:      "full auth JID distinct from dial host",
			url:       "xmpp://alice%40jabber.example.com:secret@xmpp.example.com/?rooms=alerts@conference.example.com",
			wantHost:  "xmpp.example.com",
			wantPort:  xmpp.DefaultPort,
			wantUser:  "alice@jabber.example.com",
			wantRooms: []string{"alerts@conference.example.com"},
			wantTLS:   true,
		},
		{
			name:     "disabletls on xmpp",
			url:      "xmpp://alice:secret@xmpp.example.com/?to=bob@example.com&disabletls=yes",
			wantHost: "xmpp.example.com",
			wantPort: xmpp.DefaultPort,
			wantUser: "alice",
			wantTo:   []string{"bob@example.com"},
			wantTLS:  false,
		},
		{
			name:           "skiptlsverify and custom port",
			url:            "xmpps://alice:secret@xmpp.example.com:5224/?to=bob@example.com&skiptlsverify=yes",
			wantHost:       "xmpp.example.com",
			wantPort:       5224,
			wantUser:       "alice",
			wantTo:         []string{"bob@example.com"},
			wantTLS:        true,
			wantSkipVerify: true,
		},
		{
			name:      "nick and roompassword",
			url:       bothURL,
			wantHost:  "xmpp.example.com",
			wantPort:  xmpp.DefaultPort,
			wantUser:  "alice",
			wantTo:    []string{"bob@example.com"},
			wantRooms: []string{"alerts@conference.example.com"},
			wantNick:  "bot",
			wantTLS:   true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			service := createTestService(t, tt.url, &mockSession{})

			require.Equal(t, tt.wantHost, service.Config.Host)
			require.Equal(t, tt.wantPort, service.Config.Port)
			require.Equal(t, tt.wantUser, service.Config.User)
			require.Equal(t, tt.wantTo, service.Config.To)
			require.Equal(t, tt.wantRooms, service.Config.Rooms)
			require.Equal(t, tt.wantNick, service.Config.Nick)
			require.Equal(t, tt.wantTLS, !service.Config.DisableTLS)
			require.Equal(t, tt.wantSkipVerify, service.Config.SkipTLSVerify)
			require.Equal(t, xmpp.Scheme, service.GetID())
		})
	}
}

func TestConfigURLRoundTrip(t *testing.T) {
	t.Parallel()

	service := createTestService(t, chatURL, &mockSession{})
	parsed := &xmpp.Config{}
	require.NoError(t, parsed.SetURL(service.Config.GetURL()))
	require.Equal(t, service.Config.Host, parsed.Host)
	require.Equal(t, service.Config.User, parsed.User)
	require.Equal(t, service.Config.To, parsed.To)
	require.Equal(t, xmpp.Scheme, service.Config.GetURL().Scheme)
}

func TestConfigInvalidURLs(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		url     string
		wantErr error
	}{
		{
			name:    "disabletls on xmpps",
			url:     "xmpps://alice:secret@xmpp.example.com/?to=bob@example.com&disabletls=yes",
			wantErr: xmpp.ErrDisableTLSOnXMPPS,
		},
		{
			name:    "missing targets",
			url:     "xmpp://alice:secret@xmpp.example.com/",
			wantErr: xmpp.ErrMissingTargets,
		},
		{
			name:    "missing password",
			url:     "xmpp://alice@xmpp.example.com/?to=bob@example.com",
			wantErr: xmpp.ErrMissingPassword,
		},
		{
			name:    "invalid recipient",
			url:     "xmpp://alice:secret@xmpp.example.com/?to=not-a-jid",
			wantErr: xmpp.ErrInvalidRecipientJID,
		},
		{
			name:    "invalid room",
			url:     "xmpp://alice:secret@xmpp.example.com/?rooms=not-a-jid",
			wantErr: xmpp.ErrInvalidRoomJID,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			service := &xmpp.Service{}
			err := service.Initialize(mustParse(t, tt.url), &mockLogger{})
			require.ErrorIs(t, err, tt.wantErr)
		})
	}
}
