package join

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

// TestConfigSetURL verifies that SetURL reads the API key from the password and
// the devices, title, and icon from the query.
func TestConfigSetURL(t *testing.T) {
	t.Parallel()

	// tests lists service URLs with the config each should produce.
	tests := []struct {
		name   string
		rawURL string
		want   Config
	}{
		{
			name:   "one device",
			rawURL: "join://Token:apikey@join?devices=dev1",
			want:   Config{APIKey: "apikey", Devices: []string{"dev1"}},
		},
		{
			name:   "devices split by commas",
			rawURL: "join://Token:apikey@join?devices=a,b,c,d",
			want:   Config{APIKey: "apikey", Devices: []string{"a", "b", "c", "d"}},
		},
		{
			name:   "title and icon",
			rawURL: "join://Token:apikey@join?devices=dev1&title=new+title&icon=https%3A%2F%2Fexample.com%2Ficon.png",
			want:   Config{APIKey: "apikey", Devices: []string{"dev1"}, Title: "new title", Icon: "https://example.com/icon.png"},
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

// TestConfigSetURLErrors verifies that SetURL rejects a URL without devices or
// an API key, and a URL with an unknown query prop.
func TestConfigSetURLErrors(t *testing.T) {
	t.Parallel()

	// tests lists invalid service URLs with the error each should produce.
	tests := []struct {
		wantErr error
		name    string
		rawURL  string
	}{
		{name: "missing devices", rawURL: "join://Token:apikey@join", wantErr: ErrDevicesMissing},
		{name: "missing API key", rawURL: "join://Token:@join?devices=dev1", wantErr: ErrAPIKeyMissing},
		{name: "unknown prop", rawURL: "join://Token:apikey@join?devices=dev1&devicey=a", wantErr: format.ErrInvalidConfigKey},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			require.ErrorIs(t, (&Config{}).SetURL(parseTestURL(t, tt.rawURL)), tt.wantErr)
		})
	}
}

// TestConfigGetURL verifies that GetURL serializes a config back to the
// service URL that SetURL read, with the API key as the password.
func TestConfigGetURL(t *testing.T) {
	t.Parallel()

	t.Run("round trip", func(t *testing.T) {
		t.Parallel()

		rawURL := "join://Token:apikey@join?devices=dev1%2Cdev2&icon=warning&title=hey"
		config := &Config{}
		require.NoError(t, config.SetURL(parseTestURL(t, rawURL)))

		assert.Equal(t, rawURL, config.GetURL().String())
	})

	t.Run("API key only", func(t *testing.T) {
		t.Parallel()

		serviceURL := (&Config{APIKey: "test-token"}).GetURL()

		password, ok := serviceURL.User.Password()
		require.True(t, ok)
		assert.Equal(t, "test-token", password)
		assert.Equal(t, Scheme, serviceURL.Scheme)
	})
}

// TestConfigQueryResolver verifies the config's query props: devices join with
// commas, unknown keys are rejected, and the keys are devices, icon, and title
// with no enums.
func TestConfigQueryResolver(t *testing.T) {
	t.Parallel()

	t.Run("devices joined by commas", func(t *testing.T) {
		t.Parallel()

		value, err := format.GetConfigQueryResolver(&Config{Devices: []string{"a", "b", "c"}}).Get("devices")

		require.NoError(t, err)
		assert.Equal(t, "a,b,c", value)
	})

	t.Run("unknown key", func(t *testing.T) {
		t.Parallel()

		config := &Config{}
		resolver := format.NewPropKeyResolver(config)

		value, err := resolver.Get("devicey")
		require.ErrorIs(t, err, format.ErrInvalidConfigKey)
		assert.Empty(t, value)

		require.ErrorIs(t, resolver.Set("devicey", "a,b,c,d"), format.ErrInvalidConfigKey)
	})

	t.Run("default values", func(t *testing.T) {
		t.Parallel()

		config := &Config{}
		resolver := format.NewPropKeyResolver(config)

		require.NoError(t, resolver.SetDefaultProps(config))
	})

	t.Run("fields and enums", func(t *testing.T) {
		t.Parallel()

		config := &Config{}

		assert.Empty(t, config.Enums())
		assert.Equal(t, []string{"devices", "icon", "title"}, format.GetConfigQueryResolver(config).QueryFields())
	})
}
