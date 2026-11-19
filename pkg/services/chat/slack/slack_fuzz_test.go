package slack

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
)

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
