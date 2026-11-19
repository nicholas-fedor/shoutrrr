package slack

import (
	"regexp"
	"testing"

	"github.com/stretchr/testify/assert"
)

// iconURLPattern defines an icon URL as a regular expression, the reference
// that isIconURL must agree with.
var iconURLPattern = regexp.MustCompile(`https?://`)

// FuzzIsIconURL checks that isIconURL accepts exactly the icons that
// iconURLPattern matches. Seeds hold an https and an http URL and an emoji.
// Others hold a URL after other text, an upper-case scheme, another scheme, and
// a scheme that only starts with https. The rest hold a separator without a
// scheme and a URL after an earlier separator.
func FuzzIsIconURL(f *testing.F) {
	for _, seed := range []string{
		"https://example.com/logo.png", "http://example.com/logo.png", ":tanabata_tree:",
		"logo at https://x", "HTTPS://x", "ftp://x", "://", "x://https://y", "httpss://x",
	} {
		f.Add(seed)
	}

	f.Fuzz(func(t *testing.T, icon string) {
		assert.Equal(t, iconURLPattern.MatchString(icon), isIconURL(icon), "%q", icon)
	})
}
