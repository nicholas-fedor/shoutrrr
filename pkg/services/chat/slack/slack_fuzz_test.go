package slack

import (
	"regexp"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
)

// iconURLPattern matches an icon URL. It lists each letter's ASCII cases
// because (?i) would also read ſ as s.
var iconURLPattern = regexp.MustCompile(`^[Hh][Tt]{2}[Pp][Ss]?://`)

// FuzzIsIconURL checks that isIconURL accepts exactly the icons that
// iconURLPattern matches. Seeds hold an https URL, an http URL, and an emoji.
// Others hold a bare "http", an upper-case scheme, and a scheme that only
// starts with https. More hold an http scheme without slashes, a URL after
// other text, and an ftp URL. The long s and "(ttp" seeds hold bytes that only
// a wrong case fold would read as s or h.
func FuzzIsIconURL(f *testing.F) {
	for _, seed := range []string{
		"https://example.com/logo.png", "http://example.com/logo.png", ":tanabata_tree:", "http",
		"HTTPS://x", "httpss://x", "http:logo", "logo at https://x", "ftp://x", "httpſ://x", "(ttp://x",
	} {
		f.Add(seed)
	}

	f.Fuzz(func(t *testing.T, icon string) {
		assert.Equal(t, iconURLPattern.MatchString(icon), isIconURL(icon), "%q", icon)
	})
}

// FuzzCreateJSONPayload checks that the attachments of any message are its
// first 100 lines, with the last attachment keeping the rest of the message.
// A trailing empty line adds no attachment. Seeds hold an empty message, a
// trailing line break, and blank lines. The rest hold messages at and past the
// limit, with and without a trailing line break.
func FuzzCreateJSONPayload(f *testing.F) {
	for _, lineCount := range []int{99, 100, 101} {
		lines := strings.Repeat("line\n", lineCount)
		f.Add(lines)
		f.Add(strings.TrimSuffix(lines, "\n"))
	}

	for _, seed := range []string{"", "one\ntwo\n", "\n\n", "\r\n"} {
		f.Add(seed)
	}

	f.Fuzz(func(t *testing.T, message string) {
		want := strings.SplitN(message, "\n", MaxAttachments)
		if want[len(want)-1] == "" {
			want = want[:len(want)-1]
		}

		atts := CreateJSONPayload(&Config{}, message).Attachments

		texts := make([]string, 0, len(atts))
		for _, att := range atts {
			texts = append(texts, att.Text)
		}

		assert.Equal(t, want, texts, "%q", message)
	})
}
