package notifiarr

import (
	"net/url"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/nicholas-fedor/shoutrrr/pkg/format"
	"github.com/nicholas-fedor/shoutrrr/pkg/util"
)

// testAPIKey is the API key in the test service URLs.
const testAPIKey = "apikey123"

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

// TestDefaultConfig verifies that the default config names the app Shoutrrr.
func TestDefaultConfig(t *testing.T) {
	t.Parallel()

	config, _ := DefaultConfig()

	assert.Equal(t, &Config{Name: "Shoutrrr"}, config)
}

// TestConfigSetURL verifies that SetURL reads the API key from the host and
// the Discord props from the query, decoding escaped values.
func TestConfigSetURL(t *testing.T) {
	t.Parallel()

	// tests lists service URLs with the config each should produce.
	tests := []struct {
		name   string
		rawURL string
		want   Config
	}{
		{name: "API key only", rawURL: "notifiarr://" + testAPIKey, want: Config{APIKey: testAPIKey}},
		{
			name:   "Discord props",
			rawURL: "notifiarr://" + testAPIKey + "?channel=123456789&thumbnail=https://example.com/thumb.png&image=https://example.com/img.png&color=FF0000&name=Backup",
			want: Config{
				APIKey: testAPIKey, Name: "Backup", Channel: "123456789",
				Thumbnail: "https://example.com/thumb.png", Image: "https://example.com/img.png", Color: "FF0000",
			},
		},
		{
			name:   "escaped values",
			rawURL: "notifiarr://" + testAPIKey + "?thumbnail=https%3A//example.com/image%20with%20spaces.png&color=%23FF0000",
			want:   Config{APIKey: testAPIKey, Thumbnail: "https://example.com/image with spaces.png", Color: "#FF0000"},
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
}

// TestConfigFromWebhookURL verifies that a webhook URL sets the API key from
// a notifiarr host or a passthrough path, sets the Discord props from the
// query, and keeps the other query values for the service URL.
func TestConfigFromWebhookURL(t *testing.T) {
	t.Parallel()

	// tests lists webhook URLs with the API key, channel, and service URL each
	// should produce.
	tests := []struct {
		name        string
		webhookURL  string
		wantAPIKey  string
		wantChannel string
		wantURL     string
	}{
		{
			name:        "passthrough URL",
			webhookURL:  "https://notifiarr.com/api/v1/notification/passthrough/" + testAPIKey + "?channel=123456789",
			wantAPIKey:  testAPIKey,
			wantChannel: "123456789",
			wantURL:     "notifiarr://" + testAPIKey + "?channel=123456789",
		},
		{
			name:       "passthrough URL with trailing segments",
			webhookURL: "https://notifiarr.com/api/v1/notification/passthrough/" + testAPIKey + "/extra",
			wantAPIKey: testAPIKey,
			wantURL:    "notifiarr://" + testAPIKey,
		},
		{
			name:       "notifiarr scheme",
			webhookURL: "notifiarr://" + testAPIKey,
			wantAPIKey: testAPIKey,
			wantURL:    "notifiarr://" + testAPIKey,
		},
		{
			name:       "API key in the query",
			webhookURL: "https://notifiarr.com/api/v1/notification/passthrough?apikey=" + testAPIKey,
			wantAPIKey: testAPIKey,
			wantURL:    "notifiarr://" + testAPIKey,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			config, _, err := ConfigFromWebhookURL(parseTestURL(t, tt.webhookURL))
			require.NoError(t, err)

			assert.Equal(t, tt.wantAPIKey, config.APIKey)
			assert.Equal(t, tt.wantChannel, config.Channel)
			assert.Equal(t, tt.wantURL, config.GetURL().String())
		})
	}
}

// TestConfigFromWebhookURLRejectsUnknownKeys verifies that a webhook URL with a
// query key that is not a Notifiarr setting fails, instead of the key being
// dropped.
//
// The passthrough API reads no query parameters. A key given in two spellings
// fails too, since neither spelling is clearly the one meant.
func TestConfigFromWebhookURLRejectsUnknownKeys(t *testing.T) {
	t.Parallel()

	base := "https://notifiarr.com/api/v1/notification/passthrough/" + testAPIKey

	// tests lists webhook query strings with the error each should produce.
	tests := []struct {
		wantErr error
		name    string
		query   string
	}{
		{name: "unknown key", query: "?custom=1", wantErr: format.ErrInvalidConfigKey},
		{name: "misspelled key", query: "?chanel=123456789", wantErr: format.ErrInvalidConfigKey},
		{name: "payload param", query: "?update=yes", wantErr: format.ErrInvalidConfigKey},
		{name: "key in two spellings", query: "?channel=1&Channel=2", wantErr: format.ErrConflictingKeys},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			_, err := (&Service{}).GetServiceURLFromCustom(parseTestURL(t, "notifiarr+"+base+tt.query))
			require.ErrorIs(t, err, tt.wantErr)

			require.ErrorIs(t, (&Config{}).SetURL(parseTestURL(t, "notifiarr://"+testAPIKey+tt.query)), tt.wantErr)
		})
	}
}

// TestConfigGetURL verifies that GetURL writes the API key as the host and
// the props that differ from their defaults as the query, and round-trips
// through SetURL.
func TestConfigGetURL(t *testing.T) {
	t.Parallel()

	config, _ := DefaultConfig()
	require.NoError(t, config.SetURL(parseTestURL(t, "notifiarr://"+testAPIKey+"?channel=123456789&color=FF0000")))

	serviceURL := config.GetURL()
	assert.Equal(t, "notifiarr://"+testAPIKey+"?channel=123456789&color=FF0000", serviceURL.String())

	roundTrip, _ := DefaultConfig()
	require.NoError(t, roundTrip.SetURL(serviceURL))
	assert.Equal(t, config, roundTrip)
}

// TestConfigQueryResolver verifies the config's query props and that there
// are no enums.
func TestConfigQueryResolver(t *testing.T) {
	t.Parallel()

	config := &Config{}

	assert.Empty(t, config.Enums())
	assert.Equal(t,
		[]string{"apikey", "channel", "color", "image", "name", "thumbnail"},
		format.GetConfigQueryResolver(config).QueryFields())
}

// TestConfigSetURLMissingAPIKey verifies that SetURL rejects a URL without an
// API key in the host or the apikey query parameter, including one whose
// authority is only a port, and accepts the query parameter alone.
func TestConfigSetURLMissingAPIKey(t *testing.T) {
	t.Parallel()

	for _, rawURL := range []string{"notifiarr:", "notifiarr://", "notifiarr://:8080", "notifiarr://?apikey="} {
		config, resolver := DefaultConfig()

		require.ErrorIs(t, config.setURL(&resolver, parseTestURL(t, rawURL)), ErrAPIKeyMissing, rawURL)
	}

	config, resolver := DefaultConfig()
	require.NoError(t, config.setURL(&resolver, parseTestURL(t, "notifiarr://?apikey="+testAPIKey)))
	assert.Equal(t, testAPIKey, config.APIKey)
}

// TestConfigSetURLDotKey verifies that SetURL rejects an API key of "." or
// "..", from the host or the apikey query parameter, since the key is one
// segment of the request path.
func TestConfigSetURLDotKey(t *testing.T) {
	t.Parallel()

	for _, rawURL := range []string{"notifiarr://..", "notifiarr://.", "notifiarr://?apikey=.."} {
		config, resolver := DefaultConfig()

		require.ErrorIs(t, config.setURL(&resolver, parseTestURL(t, rawURL)), util.ErrInvalidPathSegment, rawURL)
	}
}

// TestConfigSetURLKeyNotAHost verifies that SetURL rejects an API key that the
// URL GetURL writes would not read back unchanged. A colon or brackets would
// read back as a host and port. A slash or a question mark from the apikey
// query parameter would make that URL fail to parse.
func TestConfigSetURLKeyNotAHost(t *testing.T) {
	t.Parallel()

	for _, rawURL := range []string{
		"notifiarr://key:443", "notifiarr://[::1]", "notifiarr://?apikey=key:443", "notifiarr://?apikey=a%2F..%2Fx%3Fq",
	} {
		config, resolver := DefaultConfig()

		require.ErrorIs(t, config.setURL(&resolver, parseTestURL(t, rawURL)), util.ErrInvalidHost, rawURL)
	}
}
