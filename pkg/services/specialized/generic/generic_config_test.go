package generic

import (
	"net/url"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/nicholas-fedor/shoutrrr/pkg/format"
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

// TestDefaultConfig verifies the default config: a JSON content type and POST
// requests, with title and message under their own keys.
func TestDefaultConfig(t *testing.T) {
	t.Parallel()

	config, _ := DefaultConfig()

	assert.Equal(t, &Config{
		ContentType:   "application/json",
		TitleKey:      "title",
		MessageKey:    "message",
		RequestMethod: "POST",
	}, config)
}

// TestConfigFromWebhookURL verifies that a webhook URL's query sets the
// config props, headers, and extra data, the remaining query stays on the
// webhook URL, and an http scheme disables TLS.
func TestConfigFromWebhookURL(t *testing.T) {
	t.Parallel()

	t.Run("props, headers, and extra data", func(t *testing.T) {
		t.Parallel()

		config, _, err := ConfigFromWebhookURL(parseTestURL(t,
			"https://example.com/webhook?template=json&contenttype=application/json&method=PUT&titlekey=customtitle"+
				"&messagekey=custommessage&@Authorization=Bearer+token&$extraKey=extraValue&extra=param"))
		require.NoError(t, err)

		assert.Equal(t, "json", config.Template)
		assert.Equal(t, "application/json", config.ContentType)
		assert.Equal(t, "PUT", config.RequestMethod)
		assert.Equal(t, "customtitle", config.TitleKey)
		assert.Equal(t, "custommessage", config.MessageKey)
		assert.Equal(t, map[string]string{"Authorization": "Bearer token"}, config.headers)
		assert.Equal(t, map[string]string{"extraKey": "extraValue"}, config.extraData)
		assert.False(t, config.DisableTLS)
		assert.Equal(t, "https://example.com/webhook?extra=param", config.WebhookURL().String())
	})

	t.Run("http scheme", func(t *testing.T) {
		t.Parallel()

		config, _, err := ConfigFromWebhookURL(parseTestURL(t, "http://example.com/webhook"))
		require.NoError(t, err)

		assert.True(t, config.DisableTLS)
		assert.Equal(t, "http://example.com/webhook", config.WebhookURL().String())
	})

	t.Run("invalid prop value", func(t *testing.T) {
		t.Parallel()

		_, _, err := ConfigFromWebhookURL(parseTestURL(t, "https://example.com/webhook?disabletls=maybe"))

		require.ErrorIs(t, err, format.ErrInvalidBoolValue)
	})
}

// TestConfigSetURL verifies that SetURL reads the config props, headers, and
// extra data from the query and keeps the rest of the URL as the webhook URL,
// with escaped keys passed through to the webhook.
func TestConfigSetURL(t *testing.T) {
	t.Parallel()

	// tests lists service URLs with the webhook URL and template each should
	// produce.
	tests := []struct {
		name         string
		rawURL       string
		wantWebhook  string
		wantTemplate string
	}{
		{name: "path and query", rawURL: "generic://example.com/path?foo=bar", wantWebhook: "https://example.com/path?foo=bar"},
		{name: "disabletls", rawURL: "generic://example.com?disabletls=yes", wantWebhook: "http://example.com"},
		{name: "escaped prop key", rawURL: "generic://example.com/?__template=passed", wantWebhook: "https://example.com/?template=passed"},
		{
			name:         "escaped and plain prop keys",
			rawURL:       "generic://example.com/?__template=passed&template=captured",
			wantWebhook:  "https://example.com/?template=passed",
			wantTemplate: "captured",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			config, _ := DefaultConfig()
			require.NoError(t, config.SetURL(parseTestURL(t, tt.rawURL)))

			assert.Equal(t, tt.wantWebhook, config.WebhookURL().String())
			assert.Equal(t, tt.wantTemplate, config.Template)
		})
	}

	t.Run("invalid prop value", func(t *testing.T) {
		t.Parallel()

		config, _ := DefaultConfig()

		require.ErrorIs(t, config.SetURL(parseTestURL(t, "generic://example.com?disabletls=maybe")), format.ErrInvalidBoolValue)
	})
}

// TestConfigGetURL verifies that GetURL serializes the config back to a
// service URL with its props, headers, and extra data, escaping keys that
// collide with props.
func TestConfigGetURL(t *testing.T) {
	t.Parallel()

	// tests lists service URLs with the URL GetURL should produce after SetURL
	// reads them.
	tests := []struct {
		name   string
		rawURL string
		want   string
	}{
		{
			name: "everything",
			rawURL: "generic://user:pass@host.tld/api/v1/webhook?$context=inside-joke&@Authorization=frend&__title=w" +
				"&contenttype=a%2Fb&template=f&title=t",
			want: "generic://user:pass@host.tld/api/v1/webhook?%24context=inside-joke&%40Authorization=frend&__title=w" +
				"&contenttype=a%2Fb&template=f&title=t",
		},
		{name: "escaped prop key", rawURL: "generic://example.com/?__template=passed", want: "generic://example.com/?__template=passed"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			config, _ := DefaultConfig()
			require.NoError(t, config.SetURL(parseTestURL(t, tt.rawURL)))

			assert.Equal(t, tt.want, config.GetURL().String())
		})
	}
}

// TestConfigQueryResolver verifies the config's query props and that there
// are no enums.
func TestConfigQueryResolver(t *testing.T) {
	t.Parallel()

	config := &Config{}

	assert.Empty(t, config.Enums())
	assert.Equal(t,
		[]string{"contenttype", "disabletls", "messagekey", "method", "template", "title", "titlekey"},
		format.GetConfigQueryResolver(config).QueryFields())
}
