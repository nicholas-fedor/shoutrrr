package bark

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

// TestConfigGetAPIURL verifies that GetAPIURL joins the configured path and the
// endpoint with exactly one slash between each part.
func TestConfigGetAPIURL(t *testing.T) {
	t.Parallel()

	// tests lists configured paths with the API URL each should produce.
	tests := []struct {
		name string
		path string
		want string
	}{
		{name: "bare path", path: "path", want: "https://host/path/endpoint"},
		{name: "leading slash", path: "/path", want: "https://host/path/endpoint"},
		{name: "leading and trailing slashes", path: "/path/", want: "https://host/path/endpoint"},
		{name: "trailing slash", path: "path/", want: "https://host/path/endpoint"},
		{name: "root path", path: "/", want: "https://host/endpoint"},
		{name: "empty path", path: "", want: "https://host/endpoint"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			config := &Config{Host: "host", Path: tt.path, Scheme: "https"}

			assert.Equal(t, tt.want, config.GetAPIURL("endpoint"))
		})
	}

	t.Run("TLS disabled", func(t *testing.T) {
		t.Parallel()

		config := &Config{Host: "host", Path: "/", Scheme: "https", DisableTLS: true}

		assert.Equal(t, "http://host/endpoint", config.GetAPIURL("endpoint"))
	})

	t.Run("deprecated http scheme", func(t *testing.T) {
		t.Parallel()

		config := &Config{Host: "host", Path: "/", Scheme: "HTTP"}

		assert.Equal(t, "http://host/endpoint", config.GetAPIURL("endpoint"))
	})
}

// TestConfigSetURLScheme verifies how the disabletls key and the deprecated
// scheme key choose the protocol. A scheme of http, in any case, turns TLS
// off. disabletls wins when the query sets both, whatever their order. A
// scheme other than http or https is rejected.
func TestConfigSetURLScheme(t *testing.T) {
	t.Parallel()

	// tests lists queries with the protocol the API URL should use.
	tests := []struct {
		name    string
		query   string
		wantAPI string
	}{
		{name: "default", query: "", wantAPI: "https://hostname/push"},
		{name: "disabletls", query: "?disabletls=yes", wantAPI: "http://hostname/push"},
		{name: "http scheme", query: "?scheme=http", wantAPI: "http://hostname/push"},
		{name: "uppercase http scheme", query: "?scheme=HTTP", wantAPI: "http://hostname/push"},
		{name: "https scheme", query: "?scheme=https", wantAPI: "https://hostname/push"},
		{name: "disabletls after scheme", query: "?scheme=http&disabletls=no", wantAPI: "https://hostname/push"},
		{name: "disabletls before scheme", query: "?disabletls=no&scheme=http", wantAPI: "https://hostname/push"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			config := &Config{}
			require.NoError(t, config.SetURL(parseTestURL(t, "bark://:devicekey@hostname"+tt.query)))

			assert.Equal(t, tt.wantAPI, config.GetAPIURL("push"))
		})
	}

	t.Run("invalid scheme", func(t *testing.T) {
		t.Parallel()

		err := (&Config{}).SetURL(parseTestURL(t, "bark://:devicekey@hostname?scheme=ftp"))

		require.ErrorIs(t, err, ErrInvalidScheme)
	})
}

// TestConfigSetURL verifies that SetURL reads the device key, host, path, and
// query props from a service URL.
func TestConfigSetURL(t *testing.T) {
	t.Parallel()

	// tests lists service URLs with the config each should produce.
	tests := []struct {
		name   string
		rawURL string
		want   Config
	}{
		{
			name:   "required fields only",
			rawURL: "bark://:devicekey@hostname",
			want:   Config{Host: "hostname", DeviceKey: "devicekey"},
		},
		{
			name:   "path and port",
			rawURL: "bark://:devicekey@hostname:8080/bark/",
			want:   Config{Host: "hostname:8080", Path: "/bark/", DeviceKey: "devicekey"},
		},
		{
			name: "all props",
			rawURL: "bark://:device-key@example.com:2225/?badge=5&category=CAT&copy=COPY&group=GROUP" +
				"&icon=https%3A%2F%2Fexample.com%2Ficon.png&scheme=http&sound=bell&title=TITLE&url=URL",
			want: Config{
				Title:      "TITLE",
				Host:       "example.com:2225",
				Path:       "/",
				DeviceKey:  "device-key",
				Scheme:     "http",
				DisableTLS: true,
				Sound:      "bell",
				Badge:      5,
				Icon:       "https://example.com/icon.png",
				Group:      "GROUP",
				URL:        "URL",
				Category:   "CAT",
				Copy:       "COPY",
			},
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

// TestConfigSetURLErrors verifies that SetURL rejects a URL without a device
// key or host, and a URL with an unknown or invalid query prop.
func TestConfigSetURLErrors(t *testing.T) {
	t.Parallel()

	// tests lists invalid service URLs with the errors each should match.
	tests := []struct {
		wantErrs []error
		name     string
		rawURL   string
	}{
		{
			name:     "missing device key",
			rawURL:   "bark://hostname",
			wantErrs: []error{ErrMissingDeviceKey},
		},
		{
			name:     "missing host",
			rawURL:   "bark://:devicekey@",
			wantErrs: []error{ErrMissingHost},
		},
		{
			name:     "unknown prop",
			rawURL:   "bark://:mock-device@host/?foo=bar",
			wantErrs: []error{ErrSetQueryFailed, format.ErrInvalidConfigKey},
		},
		{
			name:     "invalid badge",
			rawURL:   "bark://:mock-device@host/?badge=many",
			wantErrs: []error{ErrSetQueryFailed},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			err := (&Config{}).SetURL(parseTestURL(t, tt.rawURL))

			for _, wantErr := range tt.wantErrs {
				require.ErrorIs(t, err, wantErr)
			}
		})
	}
}

// TestConfigGetURL verifies that GetURL serializes a config back to the
// service URL that SetURL read.
func TestConfigGetURL(t *testing.T) {
	t.Parallel()

	rawURL := "bark://:device-key@example.com:2225/?badge=5&category=CAT&disabletls=Yes&group=GROUP&skiptlsverify=Yes&title=TITLE&url=URL"
	config := &Config{}
	require.NoError(t, config.SetURL(parseTestURL(t, rawURL)))

	assert.Equal(t, rawURL, config.GetURL().String())
}

// TestConfigGetURLScheme verifies that GetURL writes an http scheme as
// disabletls, without changing the config, so a config URL never carries the
// deprecated scheme key. An invalid scheme is kept, so reading the URL
// reports it.
func TestConfigGetURLScheme(t *testing.T) {
	t.Parallel()

	// tests lists schemes with the query GetURL should write for them.
	tests := []struct {
		name      string
		scheme    string
		wantQuery string
	}{
		{name: "http from the URL", scheme: "http", wantQuery: "disabletls=Yes"},
		{name: "uppercase https", scheme: "HTTPS", wantQuery: ""},
		{name: "invalid", scheme: "ftp", wantQuery: "scheme=ftp"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			config := &Config{Host: "hostname", DeviceKey: "devicekey", Path: "/", Scheme: tt.scheme}
			original := *config

			assert.Equal(t, tt.wantQuery, config.GetURL().RawQuery)
			assert.Equal(t, original, *config)
		})
	}
}

// TestConfigQueryResolver verifies the config's query props: an unknown key
// is rejected, the defaults apply cleanly, and there are ten keys and no
// enums.
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
		assert.Equal(t, Config{Scheme: "https"}, *config)
	})

	t.Run("counts", func(t *testing.T) {
		t.Parallel()

		config := &Config{}

		assert.Empty(t, config.Enums())
		assert.Len(t, format.GetConfigQueryResolver(config).QueryFields(), 11)
	})
}
