package slack

import (
	"net/url"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/nicholas-fedor/shoutrrr/pkg/format"
)

// parseURL parses rawURL, failing the test when it does not parse.
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

// TestConfigSetURL verifies that SetURL reads the token, channel, and query
// settings of a current URL, and the token and bot name of a legacy webhook
// URL, which sends to the webhook's own channel.
func TestConfigSetURL(t *testing.T) {
	t.Parallel()

	// tests lists service URLs with the config each should produce.
	tests := []struct {
		name   string
		rawURL string
		want   Config
	}{
		{
			name:   "webhook",
			rawURL: "slack://hook:AAAAAAAAA-BBBBBBBBB-123456789123456789123456@webhook?botname=testbot&color=3f00fe&title=Test+title",
			want: Config{
				Token: Token{raw: "hook:AAAAAAAAA-BBBBBBBBB-123456789123456789123456"}, Channel: "webhook",
				BotName: "testbot", Color: "3f00fe", Title: "Test title",
			},
		},
		{
			name:   "bot API",
			rawURL: "slack://xoxb:123456789012-1234567890123-4mt0t4l1YL3g1T5L4cK70k3N@C0123456789?icon=https://example.com/logo.png&thread_ts=1503435956.000247",
			want: Config{
				Token: Token{raw: "xoxb:123456789012-1234567890123-4mt0t4l1YL3g1T5L4cK70k3N"}, Channel: "C0123456789",
				Icon: "https://example.com/logo.png", ThreadTS: "1503435956.000247",
			},
		},
		{
			name:   "aliases",
			rawURL: "slack://hook:AAAAAAAAA-BBBBBBBBB-123456789123456789123456@webhook?username=testbot&icon_emoji=ghost",
			want: Config{
				Token: Token{raw: "hook:AAAAAAAAA-BBBBBBBBB-123456789123456789123456"}, Channel: "webhook",
				BotName: "testbot", Icon: "ghost",
			},
		},
		{
			name:   "legacy webhook",
			rawURL: "slack://AAAAAAAAA/BBBBBBBBB/123456789123456789123456",
			want:   Config{Token: Token{raw: "hook:AAAAAAAAA-BBBBBBBBB-123456789123456789123456"}, Channel: "webhook"},
		},
		{
			name:   "legacy webhook with a bot name",
			rawURL: "slack://testbot@AAAAAAAAA/BBBBBBBBB/123456789123456789123456?color=3f00fe",
			want: Config{
				Token: Token{raw: "hook:AAAAAAAAA-BBBBBBBBB-123456789123456789123456"}, Channel: "webhook",
				BotName: "testbot", Color: "3f00fe",
			},
		},
		{
			name:   "docs command placeholder",
			rawURL: "slack://dummy@dummy.com",
			want:   Config{Token: Token{raw: "dummy"}, Channel: "dummy.com"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			config := &Config{}
			require.NoError(t, config.SetURL(parseURL(t, tt.rawURL)))

			assert.Equal(t, tt.want, *config)
		})
	}
}

// TestConfigSetURLErrors verifies that SetURL rejects an invalid token, a
// token with mismatched separators, and an unknown query key.
func TestConfigSetURLErrors(t *testing.T) {
	t.Parallel()

	// tests lists invalid service URLs with the error each should produce.
	tests := []struct {
		wantErr error
		name    string
		rawURL  string
	}{
		{name: "short token", rawURL: "slack://AAAAAAAA", wantErr: ErrInvalidToken},
		{name: "mismatched separators", rawURL: "slack://hook:AAAAAAAAA-BBBBBBBBB,123456789123456789123456@webhook", wantErr: ErrMismatchedTokenSeparators},
		{name: "unknown key", rawURL: "slack://hook:AAAAAAAAA-BBBBBBBBB-123456789123456789123456@webhook?bass=dirty", wantErr: format.ErrInvalidConfigKey},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			require.ErrorIs(t, (&Config{}).SetURL(parseURL(t, tt.rawURL)), tt.wantErr)
		})
	}
}

// TestConfigGetURL verifies that GetURL writes the current format, so a
// current URL round-trips and a legacy URL comes back in the current format.
func TestConfigGetURL(t *testing.T) {
	t.Parallel()

	const current = "slack://hook:AAAAAAAAA-BBBBBBBBB-123456789123456789123456@webhook?botname=testbot&color=3f00fe&title=Test+title"

	// tests lists service URLs with the URL each config should write.
	tests := []struct {
		name   string
		rawURL string
		want   string
	}{
		{name: "current", rawURL: current, want: current},
		{name: "legacy", rawURL: "slack://testbot@AAAAAAAAA/BBBBBBBBB/123456789123456789123456?color=3f00fe&title=Test+title", want: current},
		{
			name:   "bot API",
			rawURL: "slack://xoxb:123456789012-1234567890123-4mt0t4l1YL3g1T5L4cK70k3N@C0123456789",
			want:   "slack://xoxb:123456789012-1234567890123-4mt0t4l1YL3g1T5L4cK70k3N@C0123456789",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			config := &Config{}
			require.NoError(t, config.SetURL(parseURL(t, tt.rawURL)))

			assert.Equal(t, tt.want, config.GetURL().String())
		})
	}
}

// TestCreateConfigFromURL verifies that CreateConfigFromURL returns the config
// a URL describes, and the error of a URL that does not describe one.
func TestCreateConfigFromURL(t *testing.T) {
	t.Parallel()

	config, err := CreateConfigFromURL(parseURL(t, "slack://testbot@AAAAAAAAA/BBBBBBBBB/123456789123456789123456"))
	require.NoError(t, err)
	assert.Equal(t, "testbot", config.BotName)
	assert.Equal(t, "webhook", config.Channel)

	_, err = CreateConfigFromURL(parseURL(t, "slack://AAAAAAAA"))
	require.ErrorIs(t, err, ErrInvalidToken)
}
