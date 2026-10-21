package rocketchat

import (
	"net/url"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/nicholas-fedor/shoutrrr/pkg/util"
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
			name: "trailing slash after the tokens sets no channel",
			url:  "rocketchat://rocketchat.my-domain.com/" + testTokenA + "/" + testTokenB + "/",
			want: Config{Host: "rocketchat.my-domain.com", TokenA: testTokenA, TokenB: testTokenB},
		},
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
// without both tokens, including an empty integration ID or token, since the
// webhook URL needs both.
func TestConfigSetURLNotEnoughArguments(t *testing.T) {
	t.Parallel()

	// tests lists URLs that lack the tokens.
	tests := []struct {
		name string
		url  string
	}{
		{name: "no path", url: "rocketchat://rocketchat.my-domain.com"},
		{name: "one token", url: "rocketchat://rocketchat.my-domain.com/" + testTokenA},
		{name: "empty first token", url: "rocketchat://rocketchat.my-domain.com//" + testTokenB},
		{name: "empty second token", url: "rocketchat://rocketchat.my-domain.com/" + testTokenA + "/"},
		{name: "both tokens empty", url: "rocketchat://rocketchat.my-domain.com//"},
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

// TestConfigSetURLInvalidHost verifies that SetURL rejects an unbracketed
// IPv6 host, which url.Parse splits at its last colon.
func TestConfigSetURLInvalidHost(t *testing.T) {
	t.Parallel()

	serviceURL, err := url.Parse("rocketchat://2001:db8::1/tokenA/tokenB")
	require.NoError(t, err)

	require.ErrorIs(t, (&Config{}).SetURL(serviceURL), util.ErrInvalidHost)
}

// TestConfigSetURLMissingHost verifies that SetURL rejects a URL without a
// host, since a Rocket.Chat server has no default host.
func TestConfigSetURLMissingHost(t *testing.T) {
	t.Parallel()

	for _, rawURL := range []string{"rocketchat:///tokenA/tokenB", "rocketchat://:5055/tokenA/tokenB"} {
		serviceURL, err := url.Parse(rawURL)
		require.NoError(t, err)

		require.ErrorIs(t, (&Config{}).SetURL(serviceURL), ErrHostMissing, rawURL)
	}
}

// TestConfigSetURLClearsChannel verifies that a config reused for a URL without
// a channel, with or without a trailing slash, drops the channel an earlier URL
// set.
func TestConfigSetURLClearsChannel(t *testing.T) {
	t.Parallel()

	base := "rocketchat://rocketchat.my-domain.com/" + testTokenA + "/" + testTokenB

	for _, rawURL := range []string{base, base + "/"} {
		config := &Config{}

		withChannel, err := url.Parse(base + "/general")
		require.NoError(t, err)
		require.NoError(t, config.SetURL(withChannel))
		require.Equal(t, "#general", config.Channel)

		withoutChannel, err := url.Parse(rawURL)
		require.NoError(t, err)
		require.NoError(t, config.SetURL(withoutChannel))
		assert.Empty(t, config.Channel, rawURL)
	}
}
