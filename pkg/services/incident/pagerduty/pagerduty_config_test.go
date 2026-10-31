package pagerduty

import (
	"net/url"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/nicholas-fedor/shoutrrr/pkg/format"
	"github.com/nicholas-fedor/shoutrrr/pkg/util"
)

// testIntegrationKey is a 32-character hexadecimal integration key.
const testIntegrationKey = "a1b2c3d4e5f678901234567890abcdef"

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

// TestConfigSetURL verifies that SetURL reads the integration key from the
// path, the host and port with the PagerDuty events host and port 443 as
// defaults, and the event props from the query.
func TestConfigSetURL(t *testing.T) {
	t.Parallel()

	// tests lists service URLs with the config each should produce.
	tests := []struct {
		name   string
		rawURL string
		want   Config
	}{
		{
			name:   "default host and port",
			rawURL: "pagerduty:///" + testIntegrationKey,
			want:   Config{Host: "events.pagerduty.com", Port: 443, IntegrationKey: testIntegrationKey},
		},
		{
			name:   "host without a port",
			rawURL: "pagerduty://events.eu.pagerduty.com/" + testIntegrationKey,
			want:   Config{Host: "events.eu.pagerduty.com", Port: 443, IntegrationKey: testIntegrationKey},
		},
		{
			name:   "host, port, and integration key",
			rawURL: "pagerduty://events.pagerduty.com:443/" + testIntegrationKey,
			want:   Config{Host: "events.pagerduty.com", Port: 443, IntegrationKey: testIntegrationKey},
		},
		{
			name:   "uppercase integration key and port",
			rawURL: "pagerduty://events.pagerduty.com:12345/A1B2C3D4E5F678901234567890ABCDEF",
			want:   Config{Host: "events.pagerduty.com", Port: 12345, IntegrationKey: "A1B2C3D4E5F678901234567890ABCDEF"},
		},
		{
			name: "event props",
			rawURL: "pagerduty://events.pagerduty.com:443/" + testIntegrationKey +
				"?severity=critical&source=my-app&action=resolve&dedup_key=disk-db1&client=my-monitor" +
				"&client_url=https://example.com&details=%7B%22key%22%3A%22value%22%7D&contexts=link:http://example.com",
			want: Config{
				Host:           "events.pagerduty.com",
				Port:           443,
				IntegrationKey: testIntegrationKey,
				Severity:       "critical",
				Source:         "my-app",
				Action:         "resolve",
				DedupKey:       "disk-db1",
				Details:        `{"key":"value"}`,
				Contexts:       "link:http://example.com",
				Client:         "my-monitor",
				ClientURL:      "https://example.com",
			},
		},
		{
			name:   "unescaped space",
			rawURL: "pagerduty://events.pagerduty.com:443/" + testIntegrationKey + "?source=my app",
			want:   Config{Host: "events.pagerduty.com", Port: 443, IntegrationKey: testIntegrationKey, Source: "my app"},
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

// TestConfigSetURLErrors verifies that SetURL rejects a URL without an
// integration key, a key that is not 32 hexadecimal characters, a port above
// 65535, and an unknown query prop.
func TestConfigSetURLErrors(t *testing.T) {
	t.Parallel()

	// tests lists invalid service URLs with the error each should match, or
	// only an error when wantErr is nil.
	tests := []struct {
		wantErr error
		name    string
		rawURL  string
	}{
		{name: "missing key", rawURL: "pagerduty://events.pagerduty.com", wantErr: errMissingIntegrationKey},
		{name: "empty key", rawURL: "pagerduty://events.pagerduty.com/", wantErr: errMissingIntegrationKey},
		{name: "short key", rawURL: "pagerduty://events.pagerduty.com/invalid-key", wantErr: errInvalidIntegrationKey},
		{name: "non-hex key", rawURL: "pagerduty://events.pagerduty.com/" + "z1b2c3d4e5f678901234567890abcdef", wantErr: errInvalidIntegrationKey},
		{name: "port out of range", rawURL: "pagerduty://events.pagerduty.com:70000/" + testIntegrationKey},
		{name: "unknown prop", rawURL: "pagerduty://events.pagerduty.com/" + testIntegrationKey + "?foo=bar", wantErr: format.ErrInvalidConfigKey},
		{name: "unbracketed IPv6 host", rawURL: "pagerduty://2001:db8::1/" + testIntegrationKey, wantErr: util.ErrInvalidHost},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			err := (&Config{}).SetURL(parseTestURL(t, tt.rawURL))

			require.Error(t, err)

			if tt.wantErr != nil {
				require.ErrorIs(t, err, tt.wantErr)
			}
		})
	}
}

// TestConfigGetURL verifies that GetURL writes the integration key as the
// path, the port unless it is unset or the default 443, and the event props
// that differ from their defaults as the query, and round-trips through
// SetURL.
func TestConfigGetURL(t *testing.T) {
	t.Parallel()

	// portTests lists ports with the URL GetURL should write for them.
	portTests := []struct {
		name string
		want string
		port uint16
	}{
		{name: "unset port", port: 0, want: "pagerduty://events.pagerduty.com/" + testIntegrationKey},
		{name: "default port", port: 443, want: "pagerduty://events.pagerduty.com/" + testIntegrationKey},
		{name: "custom port", port: 12345, want: "pagerduty://events.pagerduty.com:12345/" + testIntegrationKey},
	}

	for _, tt := range portTests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			config := &Config{
				Host:           "events.pagerduty.com",
				Port:           tt.port,
				IntegrationKey: testIntegrationKey,
				Severity:       "error",
				Source:         "default",
				Action:         "trigger",
			}

			assert.Equal(t, tt.want, config.GetURL().String())
		})
	}

	// roundTrips lists service URLs with the URL GetURL should write after
	// SetURL reads them into a config with the default props.
	roundTrips := []struct {
		name   string
		rawURL string
		want   string
	}{
		{
			name: "event props",
			rawURL: "pagerduty://events.pagerduty.com/" + testIntegrationKey +
				"?action=resolve&client=my-monitor&client_url=https%3A%2F%2Fexample.com&dedup_key=disk-db1&severity=critical&source=db1",
			want: "pagerduty://events.pagerduty.com/" + testIntegrationKey +
				"?action=resolve&client=my-monitor&client_url=https%3A%2F%2Fexample.com&dedup_key=disk-db1&severity=critical&source=db1",
		},
		{
			name:   "default host and port",
			rawURL: "pagerduty:///" + testIntegrationKey + "?severity=critical",
			want:   "pagerduty://events.pagerduty.com/" + testIntegrationKey + "?severity=critical",
		},
		{
			name:   "explicit default port",
			rawURL: "pagerduty://events.pagerduty.com:443/" + testIntegrationKey + "?severity=critical",
			want:   "pagerduty://events.pagerduty.com/" + testIntegrationKey + "?severity=critical",
		},
	}

	for _, tt := range roundTrips {
		t.Run("round trip "+tt.name, func(t *testing.T) {
			t.Parallel()

			config := &Config{}
			resolver := format.NewPropKeyResolver(config)
			require.NoError(t, resolver.SetDefaultProps(config))
			require.NoError(t, config.SetURL(parseTestURL(t, tt.rawURL)))

			assert.Equal(t, tt.want, config.GetURL().String())
		})
	}
}

// TestConfigQueryResolver verifies the config's query props: an unknown key
// is rejected, the defaults describe a triggered error event, and there are
// no enums.
func TestConfigQueryResolver(t *testing.T) {
	t.Parallel()

	t.Run("unknown key", func(t *testing.T) {
		t.Parallel()

		value, err := format.GetConfigQueryResolver(&Config{}).Get("invalid query var")

		require.ErrorIs(t, err, format.ErrInvalidConfigKey)
		assert.Empty(t, value)
	})

	t.Run("default values", func(t *testing.T) {
		t.Parallel()

		config := &Config{}
		resolver := format.NewPropKeyResolver(config)

		require.NoError(t, resolver.SetDefaultProps(config))
		assert.Equal(t, "error", config.Severity)
		assert.Equal(t, "default", config.Source)
		assert.Equal(t, "trigger", config.Action)
	})

	t.Run("fields and enums", func(t *testing.T) {
		t.Parallel()

		config := &Config{}

		assert.Empty(t, config.Enums())
		assert.Equal(t,
			[]string{"action", "client", "client_url", "contexts", "dedup_key", "details", "severity", "source"},
			format.GetConfigQueryResolver(config).QueryFields())
	})
}
