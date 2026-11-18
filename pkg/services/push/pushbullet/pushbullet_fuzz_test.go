package pushbullet

import (
	"regexp"
	"testing"

	"github.com/stretchr/testify/assert"
)

// emailPattern defines an e-mail target as a regular expression, the
// reference that isEmailTarget must agree with.
var emailPattern = regexp.MustCompile(`.*@.*\..*`)

// FuzzIsEmailTarget checks that isEmailTarget accepts exactly the targets that
// emailPattern matches. Seeds hold an address, a target with a "." only before
// its "@", and an empty target. Others hold a line break between the "@" and the
// ".", or a second "@" after a line break. The rest hold bytes outside ASCII.
func FuzzIsEmailTarget(f *testing.F) {
	for _, seed := range []string{"iam@email.com", "first.last@host", "", "a@b\n.c", "a@\n@b.c", "é@ü.x", "\xff@\xfe.\xfd"} {
		f.Add(seed)
	}

	f.Fuzz(func(t *testing.T, target string) {
		assert.Equal(t, emailPattern.MatchString(target), isEmailTarget(target), "%q", target)
	})
}
