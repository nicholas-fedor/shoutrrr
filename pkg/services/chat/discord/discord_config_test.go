package discord

import (
	"net/url"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/nicholas-fedor/shoutrrr/pkg/format"
	"github.com/nicholas-fedor/shoutrrr/pkg/types"
)

// Webhook API URLs for the test webhook ID and token.
const (
	// testHooksBaseURL is the webhook base URL the tests expect, written out
	// so that the tests pin the API version.
	testHooksBaseURL = "https://discord.com/api/v10/webhooks"
	// testWebhookURL is the API URL for webhook 123456789 with token test-token.
	testWebhookURL = testHooksBaseURL + "/123456789/test-token"
	// testWebhookURLWithThread is testWebhookURL posting to thread 987654321.
	testWebhookURLWithThread = testWebhookURL + "?thread_id=987654321"
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

// newDefaultConfig returns a config with the struct tag defaults applied.
//
// Parameters:
//   - t: the test.
//
// Returns:
//   - *Config: the config.
func newDefaultConfig(t *testing.T) *Config {
	t.Helper()

	config := &Config{}
	resolver := format.NewPropKeyResolver(config)
	require.NoError(t, resolver.SetDefaultProps(config))

	return config
}

// TestConfigLevelColors verifies that each message level maps to its
// configured color.
func TestConfigLevelColors(t *testing.T) {
	t.Parallel()

	// tests lists configs with the level colors each should produce.
	tests := []struct {
		name   string
		config Config
	}{
		{name: "default colors", config: Config{Color: 0x50d9ff, ColorError: 0xd60510, ColorWarn: 0xffc441, ColorInfo: 0x2488ff, ColorDebug: 0x7b00ab}},
		{name: "custom colors", config: Config{Color: 0x123456, ColorError: 0x654321, ColorWarn: 0xabcdef, ColorInfo: 0xfedcba, ColorDebug: 0x987654}},
		{name: "zero values", config: Config{}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			colors := tt.config.LevelColors()

			assert.Equal(t, tt.config.Color, colors[types.Unknown])
			assert.Equal(t, tt.config.ColorError, colors[types.Error])
			assert.Equal(t, tt.config.ColorWarn, colors[types.Warning])
			assert.Equal(t, tt.config.ColorInfo, colors[types.Info])
			assert.Equal(t, tt.config.ColorDebug, colors[types.Debug])
		})
	}
}

// TestConfigGetURLDefaults verifies that GetURL leaves settings at their
// defaults out of the query.
func TestConfigGetURLDefaults(t *testing.T) {
	t.Parallel()

	config := newDefaultConfig(t)
	config.WebhookID, config.Token = "123456789", "test-token"

	assert.Equal(t, "discord://test-token@123456789", config.GetURL().String())
}

// TestConfigURLRoundTrip verifies that a config survives GetURL and SetURL,
// with only the required fields and with every query parameter set.
func TestConfigURLRoundTrip(t *testing.T) {
	t.Parallel()

	t.Run("required fields", func(t *testing.T) {
		t.Parallel()

		serviceURL := (&Config{WebhookID: "123456789", Token: "test-token"}).GetURL()
		assert.Equal(t, "discord", serviceURL.Scheme)
		assert.Equal(t, "123456789", serviceURL.Host)
		assert.Equal(t, "test-token", serviceURL.User.Username())

		config := &Config{}
		require.NoError(t, config.SetURL(serviceURL))
		assert.Equal(t, "123456789", config.WebhookID)
		assert.Equal(t, "test-token", config.Token)
	})

	t.Run("every query parameter", func(t *testing.T) {
		t.Parallel()

		original := Config{
			WebhookID:  "123456789",
			Token:      "test-token",
			Title:      "Test Title",
			Username:   "TestBot",
			Avatar:     "https://example.com/avatar.png",
			Color:      0x50d9ff,
			ColorError: 0xd60510,
			ColorWarn:  0xffc441,
			ColorInfo:  0x2488ff,
			ColorDebug: 0x7b00ab,
			SplitLines: false,
			JSON:       false,
			ThreadID:   "987654321",
		}

		config := newDefaultConfig(t)
		require.NoError(t, config.SetURL(original.GetURL()))

		assert.Equal(t, original, *config)
	})

	t.Run("JSON mode", func(t *testing.T) {
		t.Parallel()

		serviceURL := (&Config{WebhookID: "123456789", Token: "test-token", JSON: true}).GetURL()
		assert.Equal(t, "/raw", serviceURL.Path)

		config := &Config{}
		require.NoError(t, config.SetURL(serviceURL))
		assert.True(t, config.JSON)
	})
}

// TestConfigThreadID verifies that the thread ID is trimmed when parsed and
// left out of the URL when empty.
func TestConfigThreadID(t *testing.T) {
	t.Parallel()

	t.Run("surrounding spaces", func(t *testing.T) {
		t.Parallel()

		config := newDefaultConfig(t)
		require.NoError(t, config.SetURL(parseTestURL(t, "discord://token@channel?thread_id=%20%20123456789%20%20")))

		assert.Equal(t, "123456789", config.ThreadID)
		assert.Equal(t, "123456789", config.GetURL().Query().Get("thread_id"))
	})

	t.Run("empty", func(t *testing.T) {
		t.Parallel()

		serviceURL := (&Config{WebhookID: "123456789", Token: "test-token"}).GetURL()
		assert.False(t, serviceURL.Query().Has("thread_id"))
	})
}

// TestConfigGetURLWithoutValues verifies that an empty config builds a URL
// without a webhook ID or token.
func TestConfigGetURLWithoutValues(t *testing.T) {
	t.Parallel()

	serviceURL := (&Config{}).GetURL()

	assert.Empty(t, serviceURL.Host)
	assert.Empty(t, serviceURL.User.Username())
}

// TestConfigSetURLRejectsInvalidURLs verifies that SetURL requires a webhook ID
// and token and rejects extra path segments and unknown query keys.
func TestConfigSetURLRejectsInvalidURLs(t *testing.T) {
	t.Parallel()

	// tests lists invalid URLs with the error each should produce, or nil when
	// any error will do.
	tests := []struct {
		name    string
		rawURL  string
		wantErr error
	}{
		{name: "no webhook ID", rawURL: "discord://token@", wantErr: ErrMissingWebhookID},
		{name: "no token", rawURL: "discord://@webhook", wantErr: ErrMissingToken},
		{name: "extra path", rawURL: "discord://token@webhook/invalid", wantErr: ErrIllegalURLArgument},
		{name: "unknown query key", rawURL: "discord://token@webhook?invalid_key=value"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			err := newDefaultConfig(t).SetURL(parseTestURL(t, tt.rawURL))
			require.Error(t, err)

			if tt.wantErr != nil {
				require.ErrorIs(t, err, tt.wantErr)
			}
		})
	}
}

// TestCreatePostURLFromConfig verifies the webhook API URL, including the
// thread ID, with surrounding spaces trimmed, and that an incomplete config
// gives no URL.
func TestCreatePostURLFromConfig(t *testing.T) {
	t.Parallel()

	longID, longToken := strings.Repeat("1", 100), strings.Repeat("a", 100)

	// tests lists configs with the URL each should produce.
	tests := []struct {
		name   string
		config Config
		want   string
	}{
		{name: "no thread", config: Config{WebhookID: "123456789", Token: "test-token"}, want: testWebhookURL},
		{name: "thread", config: Config{WebhookID: "123456789", Token: "test-token", ThreadID: "987654321"}, want: testWebhookURLWithThread},
		{name: "spaces around webhook ID and token", config: Config{WebhookID: "  123456789  ", Token: "  test-token  "}, want: testWebhookURL},
		{name: "spaces around thread", config: Config{WebhookID: "123456789", Token: "test-token", ThreadID: "  987654321  "}, want: testWebhookURLWithThread},
		{name: "no webhook ID or token", config: Config{}, want: ""},
		{name: "long webhook ID and token", config: Config{WebhookID: longID, Token: longToken}, want: testHooksBaseURL + "/" + longID + "/" + longToken},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			assert.Equal(t, tt.want, CreatePostURLFromConfig(&tt.config))
		})
	}
}
