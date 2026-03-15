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
	raw string
}

const webhookBase = "https://hooks.slack.com/services/"

// Token type identifiers.
const (
	HookTokenIdentifier = "hook"
	UserTokenIdentifier = "xoxp"
	BotTokenIdentifier  = "xoxb"
)

// Token length and offset constants.
const (
	MinTokenLength       = 3  // Minimum length for a valid token string
	TypeIdentifierLength = 4  // Length of the type identifier (e.g., "xoxb", "hook")
	TypeIdentifierOffset = 5  // Offset to skip type identifier and separator (e.g., "xoxb:")
	Part1Length          = 9  // Expected length of part 1 in token
	Part2Length          = 9  // Expected length of part 2 in token
	Part3Length          = 24 // Expected length of part 3 in token
)

// Token match group indices.
const (
	tokenMatchFull  = iota // Full match
	tokenMatchType         // Type identifier (e.g., "xoxb", "hook")
	tokenMatchPart1        // First part of the token
	tokenMatchSep1         // First separator
	tokenMatchPart2        // Second part of the token
	tokenMatchSep2         // Second separator
	tokenMatchPart3        // Third part of the token
	tokenMatchCount        // Total number of match groups
)

var tokenPattern = regexp.MustCompile(
	`(?:(?P<type>xox.|hook)[-:]|:?)(?P<p1>[A-Z0-9]{` + strconv.Itoa(
		Part1Length,
	) + `,})(?P<s1>[-/,])(?P<p2>[A-Z0-9]{` + strconv.Itoa(
		Part2Length,
	) + `,})(?P<s2>[-/,])(?P<p3>[A-Za-z0-9]{` + strconv.Itoa(
		Part3Length,
	) + `,})`,
)

var _ types.ConfigProp = &Token{}

// Authorization returns the corresponding `Authorization` HTTP header value for the token.
func (t *Token) Authorization() string {
	stringBuilder := strings.Builder{}
	stringBuilder.WriteString("Bearer ")
	stringBuilder.Grow(len(t.raw))
	stringBuilder.WriteString(t.raw[:TypeIdentifierLength])
	stringBuilder.WriteRune('-')
	stringBuilder.WriteString(t.raw[TypeIdentifierOffset:])

	return stringBuilder.String()
}

// GetPropValue returns the token as a property value, implementing the types.ConfigProp interface.
func (t *Token) GetPropValue() (string, error) {
	if t == nil {
		return "", nil
	}

	return t.raw, nil
}

// IsAPIToken returns whether the identifier is set to anything else but the webhook identifier (`hook`).
func (t *Token) IsAPIToken() bool {
	return t.TypeIdentifier() != HookTokenIdentifier
}

// SetFromProp sets the token from a property value, implementing the types.ConfigProp interface.
func (t *Token) SetFromProp(propValue string) error {
	if len(propValue) < MinTokenLength {
		return ErrInvalidToken
	}

	match := tokenPattern.FindStringSubmatch(propValue)
	if match == nil || len(match) != tokenMatchCount {
		return ErrInvalidToken
	}

	typeIdentifier := match[tokenMatchType]
	if typeIdentifier == "" {
		typeIdentifier = HookTokenIdentifier
	}

	t.raw = fmt.Sprintf("%s:%s-%s-%s",
		typeIdentifier, match[tokenMatchPart1], match[tokenMatchPart2], match[tokenMatchPart3])

	if match[tokenMatchSep1] != match[tokenMatchSep2] {
		return ErrMismatchedTokenSeparators
	}

	return nil
}

// String returns the token in normalized format with dashes (-) as separator.
func (t *Token) String() string {
	return t.raw
}

// TypeIdentifier returns the type identifier of the token.
func (t *Token) TypeIdentifier() string {
	return t.raw[:TypeIdentifierLength]
}

// UserInfo returns a url.Userinfo struct populated from the token.
func (t *Token) UserInfo() *url.Userinfo {
	return url.UserPassword(t.raw[:TypeIdentifierLength], t.raw[TypeIdentifierOffset:])
}

// WebhookURL returns the corresponding Webhook URL for the token.
func (t *Token) WebhookURL() string {
	stringBuilder := strings.Builder{}
	stringBuilder.WriteString(webhookBase)
	stringBuilder.Grow(len(t.raw) - TypeIdentifierOffset)

	for i := TypeIdentifierOffset; i < len(t.raw); i++ {
		c := t.raw[i]
		if c == '-' {
			c = '/'
		}

		stringBuilder.WriteByte(c)
	}

	return stringBuilder.String()
}

// ParseToken parses and normalizes a token string.
func ParseToken(str string) (*Token, error) {
	token := &Token{}
	if err := token.SetFromProp(str); err != nil {
		return nil, err
	}

	return token, nil
}
