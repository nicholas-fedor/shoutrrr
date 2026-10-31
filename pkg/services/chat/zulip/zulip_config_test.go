package zulip

import (
	"net/url"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/nicholas-fedor/shoutrrr/pkg/format"
)

// Values shared by the tests.
const (
	// testBotMail is the bot e-mail address in the test URLs.
	testBotMail = "bot@example.com"
	// testBotKey is the bot API key in the test URLs.
	testBotKey = "SECRETbotKey"
	// testHost is the Zulip server in the test URLs.
	testHost = "zulip.example.com"
	// testServiceURL is a service URL for a channel message.
	testServiceURL = "zulip://" + testBotMail + ":" + testBotKey + "@" + testHost + "?stream=general&topic=alerts"
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

// TestConfigClone verifies that Clone copies every field into a new config.
func TestConfigClone(t *testing.T) {
	t.Parallel()

	config := &Config{
		BotMail:       testBotMail,
		BotKey:        testBotKey,
		Host:          testHost,
		Type:          MessageTypeDirect,
		Stream:        "general",
		Topic:         "alerts",
		Title:         "Disk",
		To:            "alice@example.com",
		ReadBySender:  true,
		SkipTLSVerify: true,
	}

	clone := config.Clone()

	assert.Equal(t, config, clone)
	assert.NotSame(t, config, clone)
	assert.Equal(t, &Config{}, (&Config{}).Clone())
}

// TestConfigSetURL verifies that SetURL reads the bot credentials and host
// from the URL authority and the message settings from the query, replacing
// any earlier values.
func TestConfigSetURL(t *testing.T) {
	t.Parallel()

	// tests lists service URLs with the config each should produce.
	tests := []struct {
		name   string
		rawURL string
		want   Config
	}{
		{
			name:   "credentials and host only",
			rawURL: "zulip://" + testBotMail + ":" + testBotKey + "@" + testHost,
			want:   Config{BotMail: testBotMail, BotKey: testBotKey, Host: testHost},
		},
		{
			name:   "channel message",
			rawURL: testServiceURL + "&title=Disk",
			want:   Config{BotMail: testBotMail, BotKey: testBotKey, Host: testHost, Stream: "general", Topic: "alerts", Title: "Disk"},
		},
		{
			name:   "direct message",
			rawURL: "zulip://" + testBotMail + ":" + testBotKey + "@" + testHost + "?type=direct&to=alice@example.com,bob@example.com&read_by_sender=true",
			want: Config{
				BotMail: testBotMail, BotKey: testBotKey, Host: testHost,
				Type: MessageTypeDirect, To: "alice@example.com,bob@example.com", ReadBySender: true,
			},
		},
		{
			name:   "read_by_sender yes",
			rawURL: "zulip://" + testBotMail + ":" + testBotKey + "@" + testHost + "?read_by_sender=yes",
			want:   Config{BotMail: testBotMail, BotKey: testBotKey, Host: testHost, ReadBySender: true},
		},
		{
			name:   "read_by_sender 1",
			rawURL: "zulip://" + testBotMail + ":" + testBotKey + "@" + testHost + "?read_by_sender=1",
			want:   Config{BotMail: testBotMail, BotKey: testBotKey, Host: testHost, ReadBySender: true},
		},
		{
			name:   "read_by_sender false",
			rawURL: "zulip://" + testBotMail + ":" + testBotKey + "@" + testHost + "?read_by_sender=false",
			want:   Config{BotMail: testBotMail, BotKey: testBotKey, Host: testHost},
		},
		{
			name:   "keys in another case",
			rawURL: "zulip://" + testBotMail + ":" + testBotKey + "@" + testHost + "?Stream=alerts&READ_BY_SENDER=yes",
			want:   Config{BotMail: testBotMail, BotKey: testBotKey, Host: testHost, Stream: "alerts", ReadBySender: true},
		},
		{
			name:   "host with a port",
			rawURL: "zulip://" + testBotMail + ":" + testBotKey + "@" + testHost + ":8443",
			want:   Config{BotMail: testBotMail, BotKey: testBotKey, Host: testHost + ":8443"},
		},
		{
			name:   "empty API key",
			rawURL: "zulip://" + testBotMail + ":@" + testHost,
			want:   Config{BotMail: testBotMail, Host: testHost},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			config := &Config{
				Type: MessageTypeDirect, Stream: "old", Topic: "old", Title: "old", To: "old", ReadBySender: true,
			}
			require.NoError(t, config.SetURL(parseTestURL(t, tt.rawURL)))

			assert.Equal(t, tt.want, *config)
		})
	}
}

// TestConfigSetURLErrors verifies that SetURL requires the bot e-mail address,
// an API key, and a host. It also rejects a read_by_sender value that is not a
// boolean and an unknown key.
func TestConfigSetURLErrors(t *testing.T) {
	t.Parallel()

	// tests lists invalid service URLs with the error each should produce.
	tests := []struct {
		wantErr error
		name    string
		rawURL  string
	}{
		{name: "no bot e-mail address", rawURL: "zulip://:" + testBotKey + "@" + testHost, wantErr: ErrMissingBotMail},
		{name: "no API key", rawURL: "zulip://" + testBotMail + "@" + testHost, wantErr: ErrMissingAPIKey},
		{name: "no host", rawURL: "zulip://" + testBotMail + ":" + testBotKey + "@", wantErr: ErrMissingHost},
		{
			name:    "invalid read_by_sender",
			rawURL:  "zulip://" + testBotMail + ":" + testBotKey + "@" + testHost + "?read_by_sender=maybe",
			wantErr: format.ErrInvalidBoolValue,
		},
		{
			name:    "unknown key",
			rawURL:  "zulip://" + testBotMail + ":" + testBotKey + "@" + testHost + "?strem=alerts",
			wantErr: format.ErrInvalidConfigKey,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			require.ErrorIs(t, (&Config{}).SetURL(parseTestURL(t, tt.rawURL)), tt.wantErr)
		})
	}
}

// TestConfigGetURL verifies that GetURL writes the credentials and host into
// the URL authority and the non-empty settings into the query, and
// round-trips through SetURL.
func TestConfigGetURL(t *testing.T) {
	t.Parallel()

	// tests lists configs with the URL each should produce.
	tests := []struct {
		name   string
		config Config
		want   string
	}{
		{
			name:   "credentials and host only",
			config: Config{BotMail: testBotMail, BotKey: testBotKey, Host: testHost},
			want:   "zulip://bot%40example.com:" + testBotKey + "@" + testHost,
		},
		{
			name: "every setting",
			config: Config{
				BotMail: testBotMail, BotKey: testBotKey, Host: testHost + ":8443",
				Type: MessageTypeDirect, Stream: "general", Topic: "alerts", Title: "Disk",
				To: "alice@example.com,bob@example.com", ReadBySender: true, SkipTLSVerify: true,
			},
			want: "zulip://bot%40example.com:" + testBotKey + "@" + testHost + ":8443" +
				"?read_by_sender=Yes&skiptlsverify=Yes&stream=general&title=Disk&to=alice%40example.com%2Cbob%40example.com&topic=alerts&type=direct",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			serviceURL := tt.config.GetURL()
			assert.Equal(t, tt.want, serviceURL.String())

			roundTrip := &Config{}
			require.NoError(t, roundTrip.SetURL(serviceURL))
			assert.Equal(t, tt.config, *roundTrip)
		})
	}
}

// TestCreateConfigFromURL verifies that CreateConfigFromURL returns the
// config a URL describes, or the URL's validation error.
func TestCreateConfigFromURL(t *testing.T) {
	t.Parallel()

	config, err := CreateConfigFromURL(parseTestURL(t, testServiceURL))
	require.NoError(t, err)
	assert.Equal(t, &Config{BotMail: testBotMail, BotKey: testBotKey, Host: testHost, Stream: "general", Topic: "alerts"}, config)

	_, err = CreateConfigFromURL(parseTestURL(t, "zulip://"+testBotMail+"@"+testHost))
	require.ErrorIs(t, err, ErrMissingAPIKey)
}

// TestConfigQueryResolver verifies the config's query keys and that it has
// no enums.
func TestConfigQueryResolver(t *testing.T) {
	t.Parallel()

	config := &Config{}

	assert.Empty(t, config.Enums())
	assert.Equal(t,
		[]string{"read_by_sender", "skiptlsverify", "stream", "title", "to", "topic", "type"},
		format.GetConfigQueryResolver(config).QueryFields())
}
