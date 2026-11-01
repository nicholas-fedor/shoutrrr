package rocketchat

import (
	"net/url"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/nicholas-fedor/shoutrrr/pkg/format"
	"github.com/nicholas-fedor/shoutrrr/pkg/util"
)

// TestConfigGetURL verifies the URL built from a config with and without a
// port. The username is the userinfo and the channel the path segment after
// the tokens. The TLS keys appear in the query when they are set.
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
		{
			name:   "channel written without its # prefix",
			config: &Config{Host: "rocketchat.my-domain.com", TokenA: testTokenA, TokenB: testTokenB, Channel: "#general"},
			want:   "rocketchat://rocketchat.my-domain.com/" + testTokenA + "/" + testTokenB + "/general",
		},
		{
			name:   "user recipient keeps the @ prefix",
			config: &Config{Host: "rocketchat.my-domain.com", TokenA: testTokenA, TokenB: testTokenB, Channel: "@alice"},
			want:   "rocketchat://rocketchat.my-domain.com/" + testTokenA + "/" + testTokenB + "/@alice",
		},
		{
			name:   "channel starting with @ written as a fragment",
			config: &Config{Host: "rocketchat.my-domain.com", TokenA: testTokenA, TokenB: testTokenB, Channel: "#@team"},
			want:   "rocketchat://rocketchat.my-domain.com/" + testTokenA + "/" + testTokenB + "/#@team",
		},
		{
			name:   "username",
			config: &Config{UserName: "bot", Host: "rocketchat.my-domain.com", TokenA: testTokenA, TokenB: testTokenB},
			want:   "rocketchat://bot@rocketchat.my-domain.com/" + testTokenA + "/" + testTokenB,
		},
		{
			name: "TLS keys",
			config: &Config{
				Host: "rocketchat.my-domain.com", TokenA: testTokenA, TokenB: testTokenB,
				DisableTLS: true, SkipTLSVerify: true,
			},
			want: "rocketchat://rocketchat.my-domain.com/" + testTokenA + "/" + testTokenB + "?disabletls=Yes&skiptlsverify=Yes",
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
// webhook URL needs both. A config that already holds tokens rejects them too
// and keeps its values.
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

			config := &Config{}
			require.NoError(t, config.SetURL(parseURL(t, "rocketchat://kept.example.com/"+testTokenA+"/"+testTokenB)))

			before := *config

			require.ErrorIs(t, config.SetURL(serviceURL), ErrNotEnoughArguments)
			assert.Equal(t, before, *config)
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

// TestConfigURLRoundTrip verifies that a config restored from its own URL
// keeps the username, the TLS keys, and the channel. The channel may be given
// as a path segment, a user recipient, or a URL fragment.
func TestConfigURLRoundTrip(t *testing.T) {
	t.Parallel()

	base := "rocketchat.my-domain.com/" + testTokenA + "/" + testTokenB

	// tests lists service URLs whose config must survive a round trip.
	tests := []struct {
		name   string
		rawURL string
	}{
		{name: "channel", rawURL: "rocketchat://" + base + "/general"},
		{name: "user recipient", rawURL: "rocketchat://" + base + "/@alice"},
		{name: "channel as a fragment", rawURL: "rocketchat://" + base + "/#general"},
		{name: "channel starting with @ as a fragment", rawURL: "rocketchat://" + base + "/#@team"},
		{name: "username and port", rawURL: "rocketchat://bot@rocketchat.my-domain.com:5055/" + testTokenA + "/" + testTokenB + "/general"},
		{name: "disabletls", rawURL: "rocketchat://" + base + "/general?disabletls=yes"},
		{name: "skiptlsverify", rawURL: "rocketchat://" + base + "?skiptlsverify=yes"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			config := &Config{}
			require.NoError(t, config.SetURL(parseURL(t, tt.rawURL)))

			restored := &Config{}
			require.NoError(t, restored.SetURL(config.GetURL()))
			assert.Equal(t, *config, *restored)
		})
	}
}

// parseURL parses rawURL and fails the test if it is invalid.
//
// Parameters:
//   - t: the test.
//   - rawURL: the URL to parse.
//
// Returns:
//   - *url.URL: the parsed URL.
func parseURL(t *testing.T, rawURL string) *url.URL {
	t.Helper()

	parsed, err := url.Parse(rawURL)
	require.NoError(t, err)

	return parsed
}

// TestConfigSetURLInvalidChannel verifies that SetURL rejects a channel given
// as a URL fragment with a slash and path segments after the channel, neither
// of which the channel path segment GetURL writes can hold, while a trailing
// slash after the channel is accepted.
func TestConfigSetURLInvalidChannel(t *testing.T) {
	t.Parallel()

	base := "rocketchat://rocketchat.my-domain.com/" + testTokenA + "/" + testTokenB

	for _, rawURL := range []string{base + "/#/", base + "/#team/general", base + "/general/other", base + "//other"} {
		require.ErrorIs(t, (&Config{}).SetURL(parseURL(t, rawURL)), ErrInvalidChannel, rawURL)
	}

	config := &Config{}
	require.NoError(t, config.SetURL(parseURL(t, base+"/general/")))
	assert.Equal(t, "#general", config.Channel)

	// A rejected URL leaves the config as it was.
	before := *config
	require.ErrorIs(t, config.SetURL(parseURL(t, "rocketchat://bot@other.example.com/a/b/general/other")), ErrInvalidChannel)
	assert.Equal(t, before, *config)
}

// TestConfigSetURLQuery verifies that SetURL reads the TLS keys from the
// query and resets them when a later URL leaves them out. An unknown key, a
// malformed query, or an invalid value is rejected without changing the
// config.
func TestConfigSetURLQuery(t *testing.T) {
	t.Parallel()

	base := "rocketchat://rocketchat.my-domain.com/" + testTokenA + "/" + testTokenB

	config := &Config{}
	require.NoError(t, config.SetURL(parseURL(t, base+"?disabletls=yes&skiptlsverify=yes")))
	assert.True(t, config.DisableTLS)
	assert.True(t, config.SkipTLSVerify)

	before := *config
	require.ErrorIs(t, config.SetURL(parseURL(t, base+"?disabeltls=yes")), format.ErrInvalidConfigKey)
	require.ErrorIs(t, config.SetURL(parseURL(t, base+"?disabletls=%zz")), util.ErrInvalidQuery)
	require.Error(t, config.SetURL(parseURL(t, base+"?disabletls=maybe")))
	assert.Equal(t, before, *config, "a rejected URL leaves the config as it was")

	require.NoError(t, config.SetURL(parseURL(t, base)))
	assert.False(t, config.DisableTLS)
	assert.False(t, config.SkipTLSVerify)
}
