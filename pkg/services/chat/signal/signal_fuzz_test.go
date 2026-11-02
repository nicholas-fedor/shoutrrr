package signal

import (
	"regexp"
	"testing"

	"github.com/stretchr/testify/assert"
)

// phonePattern defines a phone number as a regular expression, the reference
// that isValidPhoneNumber must agree with.
var phonePattern = regexp.MustCompile(`^\+?[0-9\s)(+-]+$`)

// FuzzIsValidPhoneNumber checks that isValidPhoneNumber accepts exactly the
// strings that phonePattern matches. Seeds hold phone numbers with and without
// a plus sign, spaces, and parentheses. Others hold whitespace that RE2's \s
// leaves out, non-ASCII digits, and text.
func FuzzIsValidPhoneNumber(f *testing.F) {
	for _, seed := range []string{"", "+", "+15551234567", "+1 (555) 123-4567", "\t5\r\n", "5\v", "５", "alice", "u:alice"} {
		f.Add(seed)
	}

	f.Fuzz(func(t *testing.T, phone string) {
		assert.Equal(t, phonePattern.MatchString(phone), isValidPhoneNumber(phone), "%q", phone)
	})
}
