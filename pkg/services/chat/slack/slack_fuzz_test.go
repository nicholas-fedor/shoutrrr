package slack

import (
	"regexp"
	"strings"
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
