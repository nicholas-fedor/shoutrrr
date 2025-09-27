package slack

import (
	"fmt"
	"net/url"
	"regexp"
	"strconv"
	"strings"

	"github.com/nicholas-fedor/shoutrrr/pkg/types"
)

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

// Token is a Slack API token or a Slack webhook token.
type Token struct {
	raw string
}

// SetFromProp sets the token from a property value, implementing the types.ConfigProp interface.
func (token *Token) SetFromProp(propValue string) error {
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

	token.raw = fmt.Sprintf("%s:%s-%s-%s",
		typeIdentifier, match[tokenMatchPart1], match[tokenMatchPart2], match[tokenMatchPart3])

	if match[tokenMatchSep1] != match[tokenMatchSep2] {
		return ErrMismatchedTokenSeparators
	}

	return nil
}

// GetPropValue returns the token as a property value, implementing the types.ConfigProp interface.
func (token *Token) GetPropValue() (string, error) {
	if token == nil {
		return "", nil
	}

	return token.raw, nil
}

// TypeIdentifier returns the type identifier of the token.
func (token *Token) TypeIdentifier() string {
	return token.raw[:TypeIdentifierLength]
}

// ParseToken parses and normalizes a token string.
func ParseToken(str string) (*Token, error) {
	token := &Token{}
	if err := token.SetFromProp(str); err != nil {
		return nil, err
	}

	return token, nil
}

// String returns the token in normalized format with dashes (-) as separator.
func (token *Token) String() string {
	return token.raw
}

// UserInfo returns a url.Userinfo struct populated from the token.
func (token *Token) UserInfo() *url.Userinfo {
	return url.UserPassword(token.raw[:TypeIdentifierLength], token.raw[TypeIdentifierOffset:])
}

// IsAPIToken returns whether the identifier is set to anything else but the webhook identifier (`hook`).
func (token *Token) IsAPIToken() bool {
	return token.TypeIdentifier() != HookTokenIdentifier
}

// WebhookURL returns the corresponding Webhook URL for the token.
func (token *Token) WebhookURL() string {
	stringBuilder := strings.Builder{}
	stringBuilder.WriteString(webhookBase)
	stringBuilder.Grow(len(token.raw) - TypeIdentifierOffset)

	for i := TypeIdentifierOffset; i < len(token.raw); i++ {
		c := token.raw[i]
		if c == '-' {
			c = '/'
		}

		stringBuilder.WriteByte(c)
	}

	return stringBuilder.String()
}

// Authorization returns the corresponding `Authorization` HTTP header value for the token.
func (token *Token) Authorization() string {
	stringBuilder := strings.Builder{}
	stringBuilder.WriteString("Bearer ")
	stringBuilder.Grow(len(token.raw))
	stringBuilder.WriteString(token.raw[:TypeIdentifierLength])
	stringBuilder.WriteRune('-')
	stringBuilder.WriteString(token.raw[TypeIdentifierOffset:])

	return stringBuilder.String()
}
