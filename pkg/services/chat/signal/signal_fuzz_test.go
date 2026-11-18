package signal

import (
	"regexp"
	"testing"

	"github.com/stretchr/testify/assert"
)

// phonePattern defines a phone number as a regular expression, the reference
// that isValidPhoneNumber must agree with.
var phonePattern = regexp.MustCompile(`^\+?[0-9\s)(+-]+$`)

// groupPattern defines a group ID as a regular expression, the reference that
// isValidGroupID must agree with.
var groupPattern = regexp.MustCompile(`^group\.[a-zA-Z0-9_+/=-]+$`)

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

// FuzzIsValidGroupID checks that isValidGroupID accepts exactly the strings
// that groupPattern matches. Seeds hold group IDs in the standard and the
// URL-safe base64 alphabets. Others hold the prefix alone, another prefix, and
// IDs with a dot, a trailing newline, or a character outside ASCII.
func FuzzIsValidGroupID(f *testing.F) {
	for _, seed := range []string{"group.YWJj+/=", "group.YWJj-_", "group.", "Group.YWJj", "group.a.b", "group.abc\n", "group.é"} {
		f.Add(seed)
	}

	f.Fuzz(func(t *testing.T, groupID string) {
		assert.Equal(t, groupPattern.MatchString(groupID), isValidGroupID(groupID), "%q", groupID)
	})
}
