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
// webhook URL, an escaped key is forwarded to the webhook, and the scheme
// alone decides whether TLS is disabled.
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

	t.Run("skiptlsverify", func(t *testing.T) {
		t.Parallel()

		config, _, err := ConfigFromWebhookURL(parseTestURL(t,
			"https://example.com/webhook?skiptlsverify=yes&__skiptlsverify=passed"))
		require.NoError(t, err)

		assert.True(t, config.SkipTLSVerify)
		assert.Equal(t, "https://example.com/webhook?skiptlsverify=passed", config.WebhookURL().String())
	})

	// The scheme decides TLS for a webhook URL, so disabletls has no effect.
	t.Run("disabletls with https scheme", func(t *testing.T) {
		t.Parallel()

		config, _, err := ConfigFromWebhookURL(parseTestURL(t, "https://example.com/webhook?disabletls=yes"))
		require.NoError(t, err)

		assert.False(t, config.DisableTLS)
		assert.Equal(t, "https://example.com/webhook", config.WebhookURL().String())
	})

	t.Run("invalid prop value", func(t *testing.T) {
		t.Parallel()

		_, _, err := ConfigFromWebhookURL(parseTestURL(t, "https://example.com/webhook?disabletls=maybe"))

		require.ErrorIs(t, err, format.ErrInvalidBoolValue)
	})
}

// TestConfigSetURL verifies that SetURL reads the config props, headers, and
// extra data from the query and keeps the rest of the URL as the webhook URL,
// with escaped keys passed through to the webhook. The skiptlsverify key is a
// config prop, so the webhook receives it only when escaped.
func TestConfigSetURL(t *testing.T) {
	t.Parallel()

	// tests lists service URLs with the webhook URL and template each should
	// produce.
	tests := []struct {
		name           string
		rawURL         string
		wantWebhook    string
		wantTemplate   string
		wantSkipVerify bool
	}{
		{name: "path and query", rawURL: "generic://example.com/path?foo=bar", wantWebhook: "https://example.com/path?foo=bar"},
		{name: "disabletls", rawURL: "generic://example.com?disabletls=yes", wantWebhook: "http://example.com"},
		{name: "skiptlsverify", rawURL: "generic://example.com/hook?skiptlsverify=yes", wantWebhook: "https://example.com/hook", wantSkipVerify: true},
		{name: "escaped skiptlsverify", rawURL: "generic://example.com/hook?__skiptlsverify=yes", wantWebhook: "https://example.com/hook?skiptlsverify=yes"},
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
			assert.Equal(t, tt.wantSkipVerify, config.SkipTLSVerify)
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
		{name: "skiptlsverify", rawURL: "generic://example.com/hook?skiptlsverify=yes", want: "generic://example.com/hook?skiptlsverify=Yes"},
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
		[]string{"contenttype", "disabletls", "messagekey", "method", "skiptlsverify", "template", "title", "titlekey"},
		format.GetConfigQueryResolver(config).QueryFields())
}

// TestConfigSetURLMissingHost verifies that SetURL rejects a URL without a
// host, since a generic webhook has no default host. A rejected URL leaves the
// config as it was, query props included.
func TestConfigSetURLMissingHost(t *testing.T) {
	t.Parallel()

	for _, rawURL := range []string{"generic:///webhook?title=Changed", "generic://:8080/webhook", "generic:/%2F%20"} {
		config, resolver := DefaultConfig()
		require.NoError(t, config.setURL(&resolver, parseTestURL(t, "generic://example.com/webhook?title=Kept")))

		before := config.GetURL().String()

		require.ErrorIs(t, config.setURL(&resolver, parseTestURL(t, rawURL)), ErrHostMissing, rawURL)
		assert.Equal(t, before, config.GetURL().String(), rawURL)
	}
}

// TestConfigZeroValue verifies that a config built in code without a webhook
// reads back without panicking. Its URL is rejected for the missing host, as a
// send with that config is.
func TestConfigZeroValue(t *testing.T) {
	t.Parallel()

	config := &Config{}

	assert.Equal(t, "https:", config.WebhookURL().String())
	assert.NotNil(t, config.MaskedURL())

	configURL := config.GetURL()
	require.ErrorIs(t, (&Config{}).SetURL(configURL), ErrHostMissing)
}

// TestConfigMaskedURL verifies that the masked URL replaces the userinfo, a
// path other than the root, and the values of the webhook query, custom
// headers, and extra data, keeps the host and the config props, and leaves
// the config unchanged.
func TestConfigMaskedURL(t *testing.T) {
	t.Parallel()

	tests := map[string]struct {
		rawURL string
		want   string
	}{
		"every part": {
			rawURL: "generic://user:pass@hooks.example.com/services/T00/B00/XXXX?token=abc&@Authorization=Bearer%20x&$apikey=k&title=Hello",
			want:   "generic://REDACTED@hooks.example.com/REDACTED?%24apikey=REDACTED&%40Authorization=REDACTED&title=Hello&token=REDACTED",
		},
		"root path without secrets": {
			rawURL: "generic://hooks.example.com/?title=Hello",
			want:   "generic://hooks.example.com/?title=Hello",
		},
	}

	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			config, _ := DefaultConfig()
			require.NoError(t, config.SetURL(parseTestURL(t, tc.rawURL)))

			before := config.GetURL().String()

			assert.Equal(t, tc.want, config.MaskedURL().String())
			assert.Equal(t, tc.want, format.MaskedURL(config).String())
			assert.Equal(t, before, config.GetURL().String(), "masking must not change the config")
		})
	}
}
