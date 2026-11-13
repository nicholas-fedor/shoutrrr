package gotify

import (
	"strings"

	"github.com/nicholas-fedor/shoutrrr/pkg/types"
)

// DefaultValidator provides the default implementation of Validator.
type DefaultValidator struct{}

// Validator handles input validation for the Gotify service.
type Validator interface {
	// ValidateMessage rejects an empty message.
	ValidateMessage(message string) error
	// ValidateServiceInitialized rejects a send before Initialize.
	ValidateServiceInitialized(config *Config) error
	// ValidatePriority rejects a priority outside -2 to 10.
	ValidatePriority(priority int) error
	// ValidateToken reports whether a token is a Gotify application token.
	ValidateToken(token string) bool
}

// Token formats.
const (
	// TokenLength is the length of a token from Gotify before 3.0: a prefix
	// letter and 14 characters from [TokenChars]. Gotify 3 still accepts
	// these tokens.
	TokenLength = 15
	// TokenChars lists the characters of a token from Gotify before 3.0.
	TokenChars = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789.-_"
	// MinEnhancedTokenLength is the minimum length of a token from Gotify 3.0
	// or later. Such a token starts with gtfy, an identifier letter, and a
	// dot, followed by 43 base64url characters that encode a 32-byte key.
	MinEnhancedTokenLength = 49
)

// Token prefixes.
const (
	// applicationTokenPrefix starts an application token from Gotify 3.0 or
	// later.
	applicationTokenPrefix = "gtfya."
	// clientTokenPrefix starts a client token from Gotify 3.0 or later.
	clientTokenPrefix = "gtfyc."
	// legacyApplicationPrefix starts an application token from Gotify before
	// 3.0.
	legacyApplicationPrefix = 'A'
	// legacyClientPrefix starts a client token from Gotify before 3.0.
	legacyClientPrefix = 'C'
)

// legacyTokenBytes marks the bytes of [TokenChars], so a token check reads one
// table entry per byte. A byte of a multibyte character is never marked.
var legacyTokenBytes = byteSet(TokenChars)

// ValidateMessage checks if the message is not empty.
//
// Parameters:
//   - message: the message.
//
// Returns:
//   - error: [ErrEmptyMessage] for an empty message.
func (v *DefaultValidator) ValidateMessage(message string) error {
	if message == "" {
		return ErrEmptyMessage
	}

	return nil
}

// ValidatePriority checks if the priority is within the valid range (-2 to 10).
//
// Parameters:
//   - priority: the priority.
//
// Returns:
//   - error: [ErrInvalidPriority] for a priority outside the range.
func (v *DefaultValidator) ValidatePriority(priority int) error {
	if priority < -2 || priority > 10 {
		return ErrInvalidPriority
	}

	return nil
}

// ValidateServiceInitialized checks if the service configuration is initialized.
//
// Parameters:
//   - config: the service configuration, nil before Initialize.
//
// Returns:
//   - error: [types.ErrServiceNotInitialized] for a nil configuration.
func (v *DefaultValidator) ValidateServiceInitialized(config *Config) error {
	if config == nil {
		return types.ErrServiceNotInitialized
	}

	return nil
}

// ValidateToken reports whether token is a Gotify application token.
//
// An application token is the only kind that sends a message without an
// application ID. Gotify 3.0 and later issue application tokens that start
// with gtfya. and have at least [MinEnhancedTokenLength] characters. Earlier
// versions issued tokens of [TokenLength] characters from [TokenChars] that
// start with A, which Gotify 3 still accepts. Client tokens are rejected in
// both forms.
//
// Parameters:
//   - token: the token.
//
// Returns:
//   - bool: true for an application token in either form.
func (v *DefaultValidator) ValidateToken(token string) bool {
	return hasTokenForm(token, applicationTokenPrefix, legacyApplicationPrefix)
}

// hasTokenForm reports whether token is one kind of Gotify token, in the form
// of Gotify 3.0 and later or of earlier versions.
//
// Parameters:
//   - token: the token.
//   - prefix: the prefix of the kind's Gotify 3 tokens, such as gtfya.
//   - legacyPrefix: the letter that starts the kind's earlier tokens, such as
//     A.
//
// Returns:
//   - bool: true for a token that starts with prefix and has at least
//     [MinEnhancedTokenLength] characters, or a token of [TokenLength]
//     characters from [TokenChars] that starts with legacyPrefix.
func hasTokenForm(token, prefix string, legacyPrefix byte) bool {
	if strings.HasPrefix(token, prefix) {
		return len(token) >= MinEnhancedTokenLength
	}

	if len(token) != TokenLength || token[0] != legacyPrefix {
		return false
	}

	for i := 1; i < len(token); i++ {
		if !legacyTokenBytes[token[i]] {
			return false
		}
	}

	return true
}

// byteSet returns a table that marks each byte of chars.
//
// Parameters:
//   - chars: the bytes to mark.
//
// Returns:
//   - [256]bool: true at the index of each byte of chars.
func byteSet(chars string) [256]bool {
	var set [256]bool

	for i := range len(chars) {
		set[chars[i]] = true
	}

	return set
}
