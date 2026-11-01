package slack

import (
	"net/url"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestParseToken verifies that webhook and API tokens in each accepted
// notation normalize to "type:part-part-part", with webhook tokens typed
// "hook" when no type is given.
func TestParseToken(t *testing.T) {
	t.Parallel()

	const webhook = "hook:AAAAAAAAA-BBBBBBBBB-123456789123456789123456"

	// tests lists raw tokens with the normalized token each should produce.
	tests := []struct {
		raw  string
		want string
	}{
		{raw: "AAAAAAAAA/BBBBBBBBB/123456789123456789123456", want: webhook},
		{raw: "AAAAAAAAA-BBBBBBBBB-123456789123456789123456", want: webhook},
		{raw: "AAAAAAAAA,BBBBBBBBB,123456789123456789123456", want: webhook},
		{raw: ":AAAAAAAAA-BBBBBBBBB-123456789123456789123456", want: webhook},
		{raw: "hook:AAAAAAAAA-BBBBBBBBB-123456789123456789123456", want: webhook},
		{raw: "hook-AAAAAAAAA-BBBBBBBBB-123456789123456789123456", want: webhook},
		{raw: "xoxb:123456789012-1234567890123-4mt0t4l1YL3g1T5L4cK70k3N", want: "xoxb:123456789012-1234567890123-4mt0t4l1YL3g1T5L4cK70k3N"},
		{raw: "xoxb-123456789012-1234567890123-4mt0t4l1YL3g1T5L4cK70k3N", want: "xoxb:123456789012-1234567890123-4mt0t4l1YL3g1T5L4cK70k3N"},
		{raw: "xoxp-123456789012-1234567890123-4mt0t4l1YL3g1T5L4cK70k3N", want: "xoxp:123456789012-1234567890123-4mt0t4l1YL3g1T5L4cK70k3N"},
	}

	for _, tt := range tests {
		t.Run(tt.raw, func(t *testing.T) {
			t.Parallel()

			token, err := ParseToken(tt.raw)
			require.NoError(t, err)

			assert.Equal(t, tt.want, token.String())
		})
	}
}

// TestParseTokenErrors verifies that a token that is too short, has a part
// that is too short or missing, or mixes separators fails to parse.
func TestParseTokenErrors(t *testing.T) {
	t.Parallel()

	// tests lists invalid raw tokens with the error each should produce.
	tests := []struct {
		wantErr error
		raw     string
	}{
		{raw: "ab", wantErr: ErrInvalidToken},
		{raw: "AAAAAAAA/BBBBBBBBB/123456789123456789123456", wantErr: ErrInvalidToken},
		{raw: "AAAAAAAAA/BBBBBBBB/123456789123456789123456", wantErr: ErrInvalidToken},
		{raw: "AAAAAAAAA/BBBBBBBBB/12345678912345678912345", wantErr: ErrInvalidToken},
		{raw: "AAAAAAAAA/BBBBBBBBB/", wantErr: ErrInvalidToken},
		{raw: "AAAAAAAAA/BBBBBBBBB-123456789123456789123456", wantErr: ErrMismatchedTokenSeparators},
	}

	for _, tt := range tests {
		t.Run(tt.raw, func(t *testing.T) {
			t.Parallel()

			token, err := ParseToken(tt.raw)

			require.ErrorIs(t, err, tt.wantErr)
			assert.Nil(t, token)
		})
	}
}

// TestTokenAccessors verifies the forms a webhook and an API token take: the
// type identifier, the webhook URL, the Authorization header value, and the
// URL user info.
func TestTokenAccessors(t *testing.T) {
	t.Parallel()

	hook, err := ParseToken("AAAAAAAAA/BBBBBBBBB/123456789123456789123456")
	require.NoError(t, err)

	assert.Equal(t, "hook", hook.TypeIdentifier())
	assert.False(t, hook.IsAPIToken())
	assert.Equal(t, "https://hooks.slack.com/services/AAAAAAAAA/BBBBBBBBB/123456789123456789123456", hook.WebhookURL())
	assert.Equal(t, url.UserPassword("hook", "AAAAAAAAA-BBBBBBBBB-123456789123456789123456"), hook.UserInfo())

	bot, err := ParseToken("xoxb:AAAAAAAAA-BBBBBBBBB-123456789123456789123456")
	require.NoError(t, err)

	assert.Equal(t, "xoxb", bot.TypeIdentifier())
	assert.True(t, bot.IsAPIToken())
	assert.Equal(t, "Bearer xoxb-AAAAAAAAA-BBBBBBBBB-123456789123456789123456", bot.Authorization())
}

// TestTokenZeroValue verifies that a token that was never set gives empty forms
// instead of panicking. A config built in code without a token holds one.
func TestTokenZeroValue(t *testing.T) {
	t.Parallel()

	var token Token

	assert.Empty(t, token.TypeIdentifier())
	assert.Equal(t, "https://hooks.slack.com/services/", token.WebhookURL())
	assert.Equal(t, "Bearer -", token.Authorization())
	assert.Nil(t, token.UserInfo())
}

// TestTokenProp verifies that a token reads and writes its normalized form as
// a config property and keeps its value when a new one is rejected. A nil
// token reads as empty.
func TestTokenProp(t *testing.T) {
	t.Parallel()

	token := &Token{}
	require.NoError(t, token.SetFromProp("xoxb-123456789012-1234567890123-4mt0t4l1YL3g1T5L4cK70k3N"))

	value, err := token.GetPropValue()
	require.NoError(t, err)
	assert.Equal(t, "xoxb:123456789012-1234567890123-4mt0t4l1YL3g1T5L4cK70k3N", value)

	require.ErrorIs(t, token.SetFromProp("invalid"), ErrInvalidToken)
	require.ErrorIs(t, token.SetFromProp("AAAAAAAAA/BBBBBBBBB-123456789123456789123456"), ErrMismatchedTokenSeparators)
	assert.Equal(t, "xoxb:123456789012-1234567890123-4mt0t4l1YL3g1T5L4cK70k3N", token.String(), "a rejected value must keep the token")

	var empty *Token

	value, err = empty.GetPropValue()
	require.NoError(t, err)
	assert.Empty(t, value)
}
