package discord

import (
	"net/url"
	"strconv"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/nicholas-fedor/shoutrrr/pkg/format"
	"github.com/nicholas-fedor/shoutrrr/pkg/types"
	"github.com/nicholas-fedor/shoutrrr/pkg/util"
)

// Webhook API URLs for the test webhook ID and token.
const (
	// testHooksBaseURL is the webhook base URL the tests expect, written out
	// so that the tests pin the API version.
	testHooksBaseURL = "https://discord.com/api/v10/webhooks"
	// testWebhookPath is the API URL for webhook 123456789 with token
	// test-token, without its query.
	testWebhookPath = testHooksBaseURL + "/123456789/test-token"
	// testWebhookURL is the URL a send posts to for that webhook, which asks
	// Discord to confirm the message with wait=true.
	testWebhookURL = testWebhookPath + "?wait=true"
	// testWebhookURLWithThread is testWebhookURL posting to thread 987654321.
	testWebhookURLWithThread = testWebhookPath + "?thread_id=987654321&wait=true"
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
			Mentions:   []string{"here", "role:123456789012345678", "user:876543210987654321"},
		}

		config := newDefaultConfig(t)
		require.NoError(t, config.SetURL(original.GetURL()))

		assert.Equal(t, original, *config)
	})

	t.Run("thread name", func(t *testing.T) {
		t.Parallel()

		serviceURL := (&Config{WebhookID: "123456789", Token: "test-token", ThreadName: "Deploy 42"}).GetURL()

		config := &Config{}
		require.NoError(t, config.SetURL(serviceURL))
		assert.Equal(t, "Deploy 42", config.ThreadName)
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
// that is a snowflake and a token that is one path segment, and rejects extra
// path segments and unknown query keys.
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
		{name: "thread ID not a number", rawURL: "discord://token@123?thread_id=thread-123", wantErr: ErrInvalidThreadID},
		{name: "thread ID with a thread name", rawURL: "discord://token@123?thread_id=987654321&thread_name=Deploy", wantErr: ErrThreadIDAndName},
		{name: "mention that cannot be read", rawURL: "discord://token@123?mentions=channel", wantErr: ErrInvalidMention},
		{name: "no token", rawURL: "discord://@123", wantErr: ErrMissingToken},
		{name: "blank token", rawURL: "discord://%20%20@123", wantErr: ErrMissingToken},
		{name: "dot-dot token", rawURL: "discord://..@123", wantErr: util.ErrInvalidPathSegment},
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

// TestConfigValidate verifies the webhook ID, token, and thread ID rules. The
// webhook ID must be a number of up to 64 bits. The token must be one path
// segment. The thread ID must be empty or a number of up to 64 bits.
// Surrounding spaces are ignored.
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
		{name: "dot token", config: Config{WebhookID: "123456789", Token: " . "}, wantErr: util.ErrInvalidPathSegment},
		{name: "dot-dot token", config: Config{WebhookID: "123456789", Token: ".."}, wantErr: util.ErrInvalidPathSegment},
		{name: "token with a slash", config: Config{WebhookID: "123456789", Token: "a/b"}, wantErr: util.ErrInvalidPathSegment},
		{name: "thread ID", config: Config{WebhookID: "123456789", Token: "test-token", ThreadID: " 987654321 "}},
		{name: "blank thread ID", config: Config{WebhookID: "123456789", Token: "test-token", ThreadID: "  "}},
		{name: "thread ID not a number", config: Config{WebhookID: "123456789", Token: "test-token", ThreadID: "thread-1"}, wantErr: ErrInvalidThreadID},
		{name: "thread ID past 64 bits", config: Config{WebhookID: "123456789", Token: "test-token", ThreadID: "18446744073709551616"}, wantErr: ErrInvalidThreadID},
		{name: "thread name", config: Config{WebhookID: "123456789", Token: "test-token", ThreadName: " Deploy "}},
		{name: "blank thread name", config: Config{WebhookID: "123456789", Token: "test-token", ThreadName: "  "}},
		{name: "thread name of 100 characters", config: Config{WebhookID: "123456789", Token: "test-token", ThreadName: strings.Repeat("é", 100)}},
		{name: "thread name over 100 characters", config: Config{WebhookID: "123456789", Token: "test-token", ThreadName: strings.Repeat("é", 101)}, wantErr: ErrInvalidThreadName},
		{
			name:    "thread name with a thread ID",
			config:  Config{WebhookID: "123456789", Token: "test-token", ThreadID: "987654321", ThreadName: "Deploy"},
			wantErr: ErrThreadIDAndName,
		},
		{name: "blank thread name with a thread ID", config: Config{WebhookID: "123456789", Token: "test-token", ThreadID: "987654321", ThreadName: " "}},
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

// TestConfigMentions verifies how the mentions key reads into the text in
// front of the message and the allowed mentions object.
//
// Keywords and prefixes match in any case. Blank and repeated mentions are
// left out. A mention that is not everyone, here, user:ID, or role:ID with a
// snowflake ID fails, and so do more than 100 user or role mentions.
func TestConfigMentions(t *testing.T) {
	t.Parallel()

	tooManyUsers := make([]string, maxMentionIDs+1)
	for i := range tooManyUsers {
		tooManyUsers[i] = "user:" + strconv.Itoa(1000+i)
	}

	// tests lists mentions with the text and allowed mentions they read
	// into, or the error.
	tests := []struct {
		name        string
		mentions    []string
		wantText    string
		wantAllowed allowedMentions
		wantErr     bool
	}{
		{name: "none"},
		{name: "everyone", mentions: []string{"everyone"}, wantText: "@everyone", wantAllowed: allowedMentions{Parse: []string{"everyone"}}},
		{name: "here", mentions: []string{"here"}, wantText: "@here", wantAllowed: allowedMentions{Parse: []string{"everyone"}}},
		{name: "everyone and here share one type", mentions: []string{"everyone", "here"}, wantText: "@everyone @here", wantAllowed: allowedMentions{Parse: []string{"everyone"}}},
		{name: "user", mentions: []string{"user:123"}, wantText: "<@123>", wantAllowed: allowedMentions{Users: []string{"123"}}},
		{name: "role", mentions: []string{"role:456"}, wantText: "<@&456>", wantAllowed: allowedMentions{Roles: []string{"456"}}},
		{
			name:        "mixed in any case with blanks and repeats",
			mentions:    []string{" HERE ", "", "Role:456", "USER:123", "role:456", "here", "user:123"},
			wantText:    "@here <@&456> <@123>",
			wantAllowed: allowedMentions{Parse: []string{"everyone"}, Users: []string{"123"}, Roles: []string{"456"}},
		},
		{name: "unknown keyword", mentions: []string{"channel"}, wantErr: true},
		{name: "Discord's own syntax", mentions: []string{"<@123>"}, wantErr: true},
		{name: "user without an ID", mentions: []string{"user:"}, wantErr: true},
		{name: "role with a name", mentions: []string{"role:admins"}, wantErr: true},
		{name: "signed ID", mentions: []string{"user:+123"}, wantErr: true},
		{name: "more than 100 users", mentions: tooManyUsers, wantErr: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			config := &Config{WebhookID: "123456789", Token: "test-token", Mentions: tt.mentions}

			set, err := config.mentions()
			if tt.wantErr {
				require.ErrorIs(t, err, ErrInvalidMention)
				require.ErrorIs(t, config.validate(), ErrInvalidMention, "the config's checks reject it")

				return
			}

			require.NoError(t, err)
			assert.Equal(t, tt.wantText, set.text)
			assert.Equal(t, tt.wantAllowed, set.allowed)
		})
	}
}

// TestCreatePostURLFromConfig verifies the webhook API URL with its thread
// ID and wait=true, so Discord confirms each message.
//
// Surrounding spaces are trimmed and the token is path-escaped. An invalid
// config gives no URL. The cases include a thread ID that is not a number.
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
		{name: "thread ID not a number", config: Config{WebhookID: "123456789", Token: "test-token", ThreadID: "a b&c"}, want: ""},
		{name: "blank thread", config: Config{WebhookID: "123456789", Token: "test-token", ThreadID: "  "}, want: testWebhookURL},
		{name: "token with reserved characters", config: Config{WebhookID: "123456789", Token: "a?c#d%"}, want: testHooksBaseURL + "/123456789/a%3Fc%23d%25?wait=true"},
		{name: "dot-dot token", config: Config{WebhookID: "123456789", Token: ".."}, want: ""},
		{name: "webhook ID not a number", config: Config{WebhookID: "webhook", Token: "test-token"}, want: ""},
		{name: "blank token", config: Config{WebhookID: "123456789", Token: "  "}, want: ""},
		{name: "longest webhook ID and a long token", config: Config{WebhookID: longID, Token: longToken}, want: testHooksBaseURL + "/" + longID + "/" + longToken + "?wait=true"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			assert.Equal(t, tt.want, CreatePostURLFromConfig(&tt.config))
		})
	}
}
