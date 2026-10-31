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
		require.NoError(t, config.SetURL(parseTestURL(t, "discord://token@123?thread_id=%20%20123456789%20%20")))

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
// that is a snowflake and a token that is not blank, and rejects extra path
// segments and unknown query keys.
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
		{name: "webhook ID not a number", rawURL: "discord://token@webhook", wantErr: ErrInvalidWebhookID},
		{name: "webhook ID past 64 bits", rawURL: "discord://token@18446744073709551616", wantErr: ErrInvalidWebhookID},
		{name: "webhook ID with a port", rawURL: "discord://token@123:80", wantErr: ErrInvalidWebhookID},
		{name: "no token", rawURL: "discord://@123", wantErr: ErrMissingToken},
		{name: "blank token", rawURL: "discord://%20%20@123", wantErr: ErrMissingToken},
		{name: "extra path", rawURL: "discord://token@123/invalid", wantErr: ErrIllegalURLArgument},
		{name: "unknown query key", rawURL: "discord://token@123?invalid_key=value"},
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

// TestConfigValidate verifies the webhook ID and token rules: a webhook ID
// that is a number of up to 64 bits and a token that is not blank, with
// surrounding spaces ignored.
func TestConfigValidate(t *testing.T) {
	t.Parallel()

	// tests lists configs with the error each should produce, or nil for a
	// valid one.
	tests := []struct {
		wantErr error
		name    string
		config  Config
	}{
		{name: "valid", config: Config{WebhookID: "123456789", Token: "test-token"}},
		{name: "longest webhook ID", config: Config{WebhookID: "18446744073709551615", Token: "test-token"}},
		{name: "surrounding spaces", config: Config{WebhookID: " 123456789 ", Token: " test-token "}},
		{name: "no webhook ID", config: Config{Token: "test-token"}, wantErr: ErrMissingWebhookID},
		{name: "blank webhook ID", config: Config{WebhookID: "  ", Token: "test-token"}, wantErr: ErrMissingWebhookID},
		{name: "webhook ID not a number", config: Config{WebhookID: "webhook", Token: "test-token"}, wantErr: ErrInvalidWebhookID},
		{name: "negative webhook ID", config: Config{WebhookID: "-1", Token: "test-token"}, wantErr: ErrInvalidWebhookID},
		{name: "webhook ID past 64 bits", config: Config{WebhookID: "18446744073709551616", Token: "test-token"}, wantErr: ErrInvalidWebhookID},
		{name: "no token", config: Config{WebhookID: "123456789"}, wantErr: ErrMissingToken},
		{name: "blank token", config: Config{WebhookID: "123456789", Token: "  "}, wantErr: ErrMissingToken},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			err := tt.config.validate()
			if tt.wantErr == nil {
				require.NoError(t, err)
			} else {
				require.ErrorIs(t, err, tt.wantErr)
			}
		})
	}
}

// TestCreatePostURLFromConfig verifies the webhook API URL, including the
// thread ID, with surrounding spaces trimmed and the token path-escaped, and
// that an invalid config gives no URL.
func TestCreatePostURLFromConfig(t *testing.T) {
	t.Parallel()

	longID, longToken := "18446744073709551615", strings.Repeat("a", 100)

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
		{name: "thread with reserved characters", config: Config{WebhookID: "123456789", Token: "test-token", ThreadID: "a b&c"}, want: testWebhookURL + "?thread_id=a+b%26c"},
		{name: "blank thread", config: Config{WebhookID: "123456789", Token: "test-token", ThreadID: "  "}, want: testWebhookURL},
		{name: "token with reserved characters", config: Config{WebhookID: "123456789", Token: "a/b?c#d%"}, want: testHooksBaseURL + "/123456789/a%2Fb%3Fc%23d%25"},
		{name: "webhook ID not a number", config: Config{WebhookID: "webhook", Token: "test-token"}, want: ""},
		{name: "blank token", config: Config{WebhookID: "123456789", Token: "  "}, want: ""},
		{name: "longest webhook ID and a long token", config: Config{WebhookID: longID, Token: longToken}, want: testHooksBaseURL + "/" + longID + "/" + longToken},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			assert.Equal(t, tt.want, CreatePostURLFromConfig(&tt.config))
		})
	}
}
