package pushbullet

import (
	"net/url"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/nicholas-fedor/shoutrrr/pkg/format"
	"github.com/nicholas-fedor/shoutrrr/pkg/util"
)

// testToken is an access token of the length Pushbullet issues.
const testToken = "tokentokentokentokentokentokentoke"

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

// TestConfigSetURL verifies that SetURL reads the token from the host, the
// targets from the path, a channel target from the fragment, and the title
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
			name:   "token only",
			rawURL: "pushbullet://" + testToken,
			want:   Config{Token: testToken, Targets: []string{""}},
		},
		{
			name:   "device",
			rawURL: "pushbullet://" + testToken + "/test",
			want:   Config{Token: testToken, Targets: []string{"test"}},
		},
		{
			name:   "device and channel",
			rawURL: "pushbullet://" + testToken + "/foo#bar",
			want:   Config{Token: testToken, Targets: []string{"foo", "#bar"}},
		},
		{
			name:   "email, device, and title",
			rawURL: "pushbullet://" + testToken + "/iam@email.com/device?title=Great+News",
			want:   Config{Token: testToken, Targets: []string{"iam@email.com", "device"}, Title: "Great News"},
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

// TestConfigSetURLErrors verifies that SetURL rejects a token of the wrong
// length, a token with colons or brackets, and an unknown query prop.
func TestConfigSetURLErrors(t *testing.T) {
	t.Parallel()

	// tests lists invalid service URLs with the error each should produce.
	tests := []struct {
		wantErr error
		name    string
		rawURL  string
	}{
		{name: "short token", rawURL: "pushbullet://shorttoken/device", wantErr: ErrTokenIncorrectSize},
		{name: "token with colons", rawURL: "pushbullet://:SECRETtokentokentokentokentokent:/device", wantErr: util.ErrInvalidHost},
		{name: "token in brackets", rawURL: "pushbullet://[::1]/device", wantErr: util.ErrInvalidHost},
		{name: "long token", rawURL: "pushbullet://" + testToken + "x/device", wantErr: ErrTokenIncorrectSize},
		{name: "unknown prop", rawURL: "pushbullet://" + testToken + "/device?foo=bar", wantErr: format.ErrInvalidConfigKey},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			require.ErrorIs(t, (&Config{}).SetURL(parseTestURL(t, tt.rawURL)), tt.wantErr)
		})
	}
}

// TestConfigGetURL verifies that GetURL serializes a config back to the
// service URL that SetURL read.
func TestConfigGetURL(t *testing.T) {
	t.Parallel()

	rawURL := "pushbullet://" + testToken + "/device?title=Great+News"
	config := &Config{}
	require.NoError(t, config.SetURL(parseTestURL(t, rawURL)))

	assert.Equal(t, rawURL, config.GetURL().String())
}

// TestConfigQueryResolver verifies the config's query props: an unknown key
// is rejected, the default title applies, and title is the only key.
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
		assert.Equal(t, Config{Title: "Shoutrrr notification"}, *config)
	})

	t.Run("fields and enums", func(t *testing.T) {
		t.Parallel()

		config := &Config{}

		assert.Empty(t, config.Enums())
		assert.Equal(t, []string{"title"}, format.GetConfigQueryResolver(config).QueryFields())
	})
}
