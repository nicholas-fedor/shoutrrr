package rocketchat

import (
	"net/url"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestConfigGetURL verifies the URL built from a config with and without a
// port.
func TestConfigGetURL(t *testing.T) {
	t.Parallel()

	// tests lists configs with the URL they should produce.
	tests := []struct {
		name   string
		config *Config
		want   string
	}{
		{
			name:   "with port",
			config: &Config{Host: "rocketchat.my-domain.com", Port: "5055", TokenA: testTokenA, TokenB: testTokenB},
			want:   "rocketchat://rocketchat.my-domain.com:5055/" + testTokenA + "/" + testTokenB,
		},
		{
			name:   "without port",
			config: &Config{Host: "rocketchat.my-domain.com", TokenA: testTokenA, TokenB: testTokenB},
			want:   "rocketchat://rocketchat.my-domain.com/" + testTokenA + "/" + testTokenB,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			assert.Equal(t, tt.want, tt.config.GetURL().String())
		})
	}
}

// TestConfigSetURL verifies how SetURL fills the config from the user, host,
// port, tokens, and channel path segment or fragment.
func TestConfigSetURL(t *testing.T) {
	t.Parallel()

	// tests lists URLs with the config SetURL should produce.
	tests := []struct {
		name string
		url  string
		want Config
	}{
		{
			name: "user channel keeps the @ prefix",
			url:  "rocketchat://rocketchat.my-domain.com/" + testTokenA + "/" + testTokenB + "/@user",
			want: Config{Host: "rocketchat.my-domain.com", TokenA: testTokenA, TokenB: testTokenB, Channel: "@user"},
		},
		{
			name: "plain channel gets a # prefix",
			url:  "rocketchat://rocketchat.my-domain.com/" + testTokenA + "/" + testTokenB + "/general",
			want: Config{Host: "rocketchat.my-domain.com", TokenA: testTokenA, TokenB: testTokenB, Channel: "#general"},
		},
		{
			name: "fragment sets the channel",
			url:  "rocketchat://rocketchat.my-domain.com/" + testTokenA + "/" + testTokenB + "/#news",
			want: Config{Host: "rocketchat.my-domain.com", TokenA: testTokenA, TokenB: testTokenB, Channel: "#news"},
		},
		{
			name: "user, port, and no channel",
			url:  "rocketchat://bot@rocketchat.my-domain.com:5055/" + testTokenA + "/" + testTokenB,
			want: Config{UserName: "bot", Host: "rocketchat.my-domain.com", Port: "5055", TokenA: testTokenA, TokenB: testTokenB},
		},
		{
			name: "dummy URL needs no tokens",
			url:  "rocketchat://dummy@dummy.com",
			want: Config{UserName: "dummy", Host: "dummy.com"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			serviceURL, err := url.Parse(tt.url)
			require.NoError(t, err)

			config := &Config{}
			require.NoError(t, config.SetURL(serviceURL))
			assert.Equal(t, tt.want, *config)
		})
	}
}

// TestConfigSetURLNotEnoughArguments verifies that SetURL rejects URLs
// without both tokens.
func TestConfigSetURLNotEnoughArguments(t *testing.T) {
	t.Parallel()

	// tests lists URLs that lack the tokens.
	tests := []struct {
		name string
		url  string
	}{
		{name: "no path", url: "rocketchat://rocketchat.my-domain.com"},
		{name: "one token", url: "rocketchat://rocketchat.my-domain.com/" + testTokenA},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			serviceURL, err := url.Parse(tt.url)
			require.NoError(t, err)

			require.ErrorIs(t, (&Config{}).SetURL(serviceURL), ErrNotEnoughArguments)
		})
	}
}
