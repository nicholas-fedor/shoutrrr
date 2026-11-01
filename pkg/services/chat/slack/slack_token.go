package slack

import (
	"fmt"
	"net/url"
	"regexp"
	"strconv"
	"strings"

	"github.com/nicholas-fedor/shoutrrr/pkg/types"
)

// Token is a Slack API token or a Slack webhook token.
type Token struct {
	// raw is the normalized token, the type identifier and a colon followed by
	// the three parts joined by dashes, such as "xoxb:123-456-abc".
	raw string
}

// webhookBase is the URL that a webhook token's parts are appended to.
const webhookBase = "https://hooks.slack.com/services/"

// Token type identifiers.
const (
	// HookTokenIdentifier identifies a webhook token.
	HookTokenIdentifier = "hook"
	// UserTokenIdentifier identifies a user API token.
	UserTokenIdentifier = "xoxp"
	// BotTokenIdentifier identifies a bot API token.
	BotTokenIdentifier = "xoxb"
)

// Token length and offset constants.
const (
	// MinTokenLength is the minimum length for a valid token string.
	MinTokenLength = 3
	// TypeIdentifierLength is the length of the type identifier (e.g., "xoxb", "hook").
	TypeIdentifierLength = 4
	// TypeIdentifierOffset is the offset that skips the type identifier and
	// its separator (e.g., "xoxb:").
	TypeIdentifierOffset = 5
	// Part1Length is the minimum length of the token's first part.
	Part1Length = 9
	// Part2Length is the minimum length of the token's second part.
	Part2Length = 9
	// Part3Length is the minimum length of the token's third part.
	Part3Length = 24
)

// Token match group indices.
const (
	// tokenMatchFull is the full match. It anchors the indices at 0, so no
	// code reads it.
	tokenMatchFull = iota //nolint:unused // Anchors the match group indices at 0.
	// tokenMatchType is the type identifier (e.g., "xoxb", "hook").
	tokenMatchType
	// tokenMatchPart1 is the first part of the token.
	tokenMatchPart1
	// tokenMatchSep1 is the first separator.
	tokenMatchSep1
	// tokenMatchPart2 is the second part of the token.
	tokenMatchPart2
	// tokenMatchSep2 is the second separator.
	tokenMatchSep2
	// tokenMatchPart3 is the third part of the token.
	tokenMatchPart3
	// tokenMatchCount is the total number of match groups.
	tokenMatchCount
)

// tokenPattern matches a token in any accepted form: an optional type
// identifier followed by a dash or colon, then three parts separated by
// dashes, slashes, or commas.
var tokenPattern = regexp.MustCompile(
	`(?:(?P<type>xox.|hook)[-:]|:?)(?P<p1>[A-Z0-9]{` + strconv.Itoa(
		Part1Length,
	) + `,})(?P<s1>[-/,])(?P<p2>[A-Z0-9]{` + strconv.Itoa(
		Part2Length,
	) + `,})(?P<s2>[-/,])(?P<p3>[A-Za-z0-9]{` + strconv.Itoa(
		Part3Length,
	) + `,})`,
)

// Compile-time check that Token is read and written as a config prop.
var _ types.ConfigProp = &Token{}

// Authorization returns the corresponding `Authorization` HTTP header value for the token.
//
// Returns:
//   - string: "Bearer " followed by the token with a dash after the type
//     identifier, such as "Bearer xoxb-123-456-abc".
func (t *Token) Authorization() string {
	typeIdentifier, parts := t.split()

	stringBuilder := strings.Builder{}
	stringBuilder.WriteString("Bearer ")
	stringBuilder.Grow(len(t.raw))
	stringBuilder.WriteString(typeIdentifier)
	stringBuilder.WriteRune('-')
	stringBuilder.WriteString(parts)

	return stringBuilder.String()
}

// GetPropValue returns the token as a property value, implementing the types.ConfigProp interface.
//
// Returns:
//   - string: the normalized token, or empty for a nil token.
//   - error: always nil.
func (t *Token) GetPropValue() (string, error) {
	if t == nil {
		return "", nil
	}

	return t.raw, nil
}

// IsAPIToken returns whether the identifier is set to anything else but the webhook identifier (`hook`).
//
// Returns:
//   - bool: true for an API token, false for a webhook token.
func (t *Token) IsAPIToken() bool {
	return t.TypeIdentifier() != HookTokenIdentifier
}

// SetFromProp sets the token from a property value, implementing the types.ConfigProp interface.
//
// A token without a type identifier is read as a webhook token.
//
// Parameters:
//   - propValue: the token in any form [tokenPattern] accepts.
//
// Returns:
//   - error: [ErrInvalidToken] for a value that is not a token, or
//     [ErrMismatchedTokenSeparators] when its two separators differ.
func (t *Token) SetFromProp(propValue string) error {
	if len(propValue) < MinTokenLength {
		return ErrInvalidToken
	}

	match := tokenPattern.FindStringSubmatch(propValue)
	if match == nil || len(match) != tokenMatchCount {
		return ErrInvalidToken
	}

	if match[tokenMatchSep1] != match[tokenMatchSep2] {
		return ErrMismatchedTokenSeparators
	}

	typeIdentifier := match[tokenMatchType]
	if typeIdentifier == "" {
		typeIdentifier = HookTokenIdentifier
	}

	t.raw = fmt.Sprintf("%s:%s-%s-%s",
		typeIdentifier, match[tokenMatchPart1], match[tokenMatchPart2], match[tokenMatchPart3])

	return nil
}

// String returns the token in normalized format with dashes (-) as separator.
//
// Returns:
//   - string: the normalized token.
func (t *Token) String() string {
	return t.raw
}

// TypeIdentifier returns the type identifier of the token.
//
// Returns:
//   - string: [HookTokenIdentifier], or the API token type such as
//     [BotTokenIdentifier].
func (t *Token) TypeIdentifier() string {
	typeIdentifier, _ := t.split()

	return typeIdentifier
}

// UserInfo returns a url.Userinfo struct populated from the token.
//
// Returns:
//   - *url.Userinfo: the type identifier as the username and the three parts
//     as the password, or nil for a token that was never set.
func (t *Token) UserInfo() *url.Userinfo {
	if t.raw == "" {
		return nil
	}

	typeIdentifier, parts := t.split()

	return url.UserPassword(typeIdentifier, parts)
}

// WebhookURL returns the corresponding Webhook URL for the token.
//
// Returns:
//   - string: [webhookBase] followed by the three parts joined by slashes.
func (t *Token) WebhookURL() string {
	_, parts := t.split()

	stringBuilder := strings.Builder{}
	stringBuilder.WriteString(webhookBase)
	stringBuilder.Grow(len(parts))

	for i := range len(parts) {
		c := parts[i]
		if c == '-' {
			c = '/'
		}

		stringBuilder.WriteByte(c)
	}

	return stringBuilder.String()
}

// split returns the type identifier of the token and its three parts joined
// by dashes. A token that was never set gives two empty strings.
//
// Returns:
//   - string: the type identifier.
//   - string: the three parts.
func (t *Token) split() (string, string) {
	if len(t.raw) < TypeIdentifierOffset {
		return "", ""
	}

	return t.raw[:TypeIdentifierLength], t.raw[TypeIdentifierOffset:]
}

// ParseToken parses and normalizes a token string.
//
// Parameters:
//   - str: the token in any form [Token.SetFromProp] accepts.
//
// Returns:
//   - *Token: the token.
//   - error: [ErrInvalidToken] or [ErrMismatchedTokenSeparators].
func ParseToken(str string) (*Token, error) {
	token := &Token{}
	if err := token.SetFromProp(str); err != nil {
		return nil, err
	}

	return token, nil
}
