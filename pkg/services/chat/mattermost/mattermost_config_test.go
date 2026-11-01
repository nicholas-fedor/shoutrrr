package mattermost

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

// TestConfigSetURL verifies that SetURL reads the host, token, channel, user
// name, and query props from a service URL.
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
			rawURL: "mattermost://mattermost.my-domain.com/thisshouldbeanapitoken",
			want:   Config{Host: "mattermost.my-domain.com", Token: "thisshouldbeanapitoken"},
		},
		{
			name:   "user name",
			rawURL: "mattermost://testUserName@mattermost.my-domain.com/thisshouldbeanapitoken",
			want:   Config{UserName: "testUserName", Host: "mattermost.my-domain.com", Token: "thisshouldbeanapitoken"},
		},
		{
			name:   "channel",
			rawURL: "mattermost://mattermost.my-domain.com/thisshouldbeanapitoken/testChannel",
			want:   Config{Channel: "testChannel", Host: "mattermost.my-domain.com", Token: "thisshouldbeanapitoken"},
		},
		{
			name:   "user name and channel",
			rawURL: "mattermost://testUserName@mattermost.my-domain.com/thisshouldbeanapitoken/testChannel",
			want: Config{
				UserName: "testUserName",
				Channel:  "testChannel",
				Host:     "mattermost.my-domain.com",
				Token:    "thisshouldbeanapitoken",
			},
		},
		{
			name:   "port and disabletls",
			rawURL: "mattermost://watchtower@home.lan:8065/token/channel?disabletls=yes",
			want: Config{
				UserName:   "watchtower",
				Channel:    "channel",
				Host:       "home.lan:8065",
				Token:      "token",
				DisableTLS: true,
			},
		},
		{
			name:   "skiptlsverify",
			rawURL: "mattermost://home.lan:8065/token?skiptlsverify=yes",
			want:   Config{Host: "home.lan:8065", Token: "token", SkipTLSVerify: true},
		},
		{
			name:   "icon emoji",
			rawURL: "mattermost://AAAAAAAAA/BBBBBBBBB?icon=test",
			want:   Config{Icon: "test", Host: "AAAAAAAAA", Token: "BBBBBBBBB"},
		},
		{
			name:   "escaped icon URL",
			rawURL: "mattermost://user@mockserver/atoken?icon=https%3A%2F%2Fexample%2Fsomething.png",
			want:   Config{UserName: "user", Icon: "https://example/something.png", Host: "mockserver", Token: "atoken"},
		},
		{
			name:   "all fields and props",
			rawURL: "mattermost://user@mockserver/atoken/achannel?icon=https%3A%2F%2Fexample%2Fsomething.png",
			want: Config{
				UserName: "user",
				Icon:     "https://example/something.png",
				Channel:  "achannel",
				Host:     "mockserver",
				Token:    "atoken",
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

// TestConfigSetURLErrors verifies that SetURL rejects a URL without a token
// and a URL with an unknown query prop.
func TestConfigSetURLErrors(t *testing.T) {
	t.Parallel()

	// tests lists invalid service URLs with the error each should produce.
	tests := []struct {
		wantErr error
		name    string
		rawURL  string
	}{
		{name: "missing token", rawURL: "mattermost://mattermost.my-domain.com", wantErr: ErrNotEnoughArguments},
		{name: "missing host", rawURL: "mattermost:///atoken", wantErr: ErrHostMissing},
		{name: "port without a host", rawURL: "mattermost://:8065/atoken", wantErr: ErrHostMissing},
		{name: "unknown prop", rawURL: "mattermost://user@mockserver/atoken?foo=bar", wantErr: format.ErrInvalidConfigKey},
		{name: "invalid bool prop", rawURL: "mattermost://mockserver/atoken?disabletls=maybe", wantErr: format.ErrInvalidBoolValue},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			require.ErrorIs(t, (&Config{}).SetURL(parseTestURL(t, tt.rawURL)), tt.wantErr)
		})
	}
}

// TestConfigSetURLRejectedLeavesConfig verifies that a URL without a host or
// a token leaves the config as it was, since both are checked before anything
// is read.
func TestConfigSetURLRejectedLeavesConfig(t *testing.T) {
	t.Parallel()

	// tests lists rejected URLs with the error each produces.
	tests := []struct {
		wantErr error
		name    string
		rawURL  string
	}{
		{name: "no host", rawURL: "mattermost://changed@:8065/changed/channel?title=Changed", wantErr: ErrHostMissing},
		{name: "no token", rawURL: "mattermost://changed@other.example.com?title=Changed", wantErr: ErrNotEnoughArguments},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			config := &Config{}
			require.NoError(t, config.SetURL(parseTestURL(t, "mattermost://kept@mattermost.example.com/kept/channel?title=Kept")))

			before := *config

			require.ErrorIs(t, config.SetURL(parseTestURL(t, tt.rawURL)), tt.wantErr)
			assert.Equal(t, before, *config)
		})
	}
}

// TestConfigSetURLClearsChannel verifies that a URL without a channel clears
// the channel an earlier URL set, so the webhook's own channel is used again.
func TestConfigSetURLClearsChannel(t *testing.T) {
	t.Parallel()

	config := &Config{}
	require.NoError(t, config.SetURL(parseTestURL(t, "mattermost://mattermost.example.com/token/channel")))
	require.Equal(t, "channel", config.Channel)

	require.NoError(t, config.SetURL(parseTestURL(t, "mattermost://mattermost.example.com/token")))
	assert.Empty(t, config.Channel)
}

// TestCreateConfigFromURL verifies that CreateConfigFromURL returns the parsed
// config, or no config and the parse error.
func TestCreateConfigFromURL(t *testing.T) {
	t.Parallel()

	t.Run("valid URL", func(t *testing.T) {
		t.Parallel()

		config, err := CreateConfigFromURL(parseTestURL(t, "mattermost://AAAAAAAAA/BBBBBBBBB?icon=test"))
		require.NoError(t, err)

		assert.Equal(t, &Config{Icon: "test", Host: "AAAAAAAAA", Token: "BBBBBBBBB"}, config)
	})

	t.Run("missing token", func(t *testing.T) {
		t.Parallel()

		config, err := CreateConfigFromURL(parseTestURL(t, "mattermost://AAAAAAAAA"))

		require.ErrorIs(t, err, ErrNotEnoughArguments)
		assert.Nil(t, config)
	})
}

// TestConfigGetURL verifies that GetURL serializes a config back to its
// service URL, normalizing prop values.
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
			name:   "user name and channel",
			rawURL: "mattermost://bot@mattermost.host/token/channel",
			want:   "mattermost://bot@mattermost.host/token/channel",
		},
		{
			name:   "icon prop",
			rawURL: "mattermost://user@mockserver/atoken/achannel?icon=something",
			want:   "mattermost://user@mockserver/atoken/achannel?icon=something",
		},
		{
			name:   "disabletls is normalized",
			rawURL: "mattermost://watchtower@home.lan:8065/token/channel?disabletls=yes",
			want:   "mattermost://watchtower@home.lan:8065/token/channel?disabletls=Yes",
		},
		{
			name:   "skiptlsverify is normalized",
			rawURL: "mattermost://home.lan:8065/token?skiptlsverify=yes",
			want:   "mattermost://home.lan:8065/token?skiptlsverify=Yes",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			config := &Config{}
			require.NoError(t, config.SetURL(parseTestURL(t, tt.rawURL)))

			assert.Equal(t, tt.want, config.GetURL().String())
		})
	}

	t.Run("host and token only", func(t *testing.T) {
		t.Parallel()

		config := &Config{Host: "mattermost.host", Token: "token"}

		assert.Equal(t, "mattermost://mattermost.host/token", config.GetURL().String())
	})
}

// TestConfigQueryResolver verifies the config's query props: an unknown key
// is rejected, the defaults apply cleanly, and there are six keys and no
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
		assert.Equal(t, Config{}, *config)
	})

	t.Run("counts", func(t *testing.T) {
		t.Parallel()

		config := &Config{}

		assert.Empty(t, config.Enums())
		assert.Len(t, format.GetConfigQueryResolver(config).QueryFields(), 6)
	})
}
