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
				Title:     "TITLE",
				Host:      "example.com:2225",
				Path:      "/",
				DeviceKey: "device-key",
				Scheme:    "http",
				Sound:     "bell",
				Badge:     5,
				Icon:      "https://example.com/icon.png",
				Group:     "GROUP",
				URL:       "URL",
				Category:  "CAT",
				Copy:      "COPY",
			},
		},
		{
			name:   "dummy URL without a device key",
			rawURL: "bark://dummy.com",
			want:   Config{Host: "dummy.com"},
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

	rawURL := "bark://:device-key@example.com:2225/?badge=5&category=CAT&group=GROUP&scheme=http&title=TITLE&url=URL"
	config := &Config{}
	require.NoError(t, config.SetURL(parseTestURL(t, rawURL)))

	assert.Equal(t, rawURL, config.GetURL().String())
}

// TestConfigQueryResolver verifies the config's query props: an unknown key
// is rejected, the defaults apply cleanly, and there are nine keys and no
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
		assert.Len(t, format.GetConfigQueryResolver(config).QueryFields(), 9)
	})
}
