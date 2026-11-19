package chaticon

import (
	"regexp"
	"testing"

	"github.com/stretchr/testify/assert"
)

// iconURLPattern matches an icon URL. It lists each letter's ASCII cases
// because (?i) would also read ſ as s.
var iconURLPattern = regexp.MustCompile(`^[Hh][Tt]{2}[Pp][Ss]?://`)

// FuzzIsURL checks that IsURL accepts exactly the icons that iconURLPattern
// matches. Seeds hold an https URL, an http URL, and an emoji. Others hold a
// bare "http", an upper-case scheme, and a scheme that only starts with https.
// More hold an http scheme without slashes, a URL after other text, and an ftp
// URL. The long s and "(ttp" seeds hold bytes that only a wrong case fold would
// read as s or h.
func FuzzIsURL(f *testing.F) {
	for _, seed := range []string{
		"https://example.com/logo.png", "http://example.com/logo.png", ":tanabata_tree:", "http",
		"HTTPS://x", "httpss://x", "http:logo", "logo at https://x", "ftp://x", "httpſ://x", "(ttp://x",
	} {
		f.Add(seed)
	}

	f.Fuzz(func(t *testing.T, icon string) {
		assert.Equal(t, iconURLPattern.MatchString(icon), IsURL(icon), "%q", icon)
	})
}
