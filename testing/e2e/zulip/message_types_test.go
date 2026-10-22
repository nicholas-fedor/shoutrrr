package e2e_test

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

// TestZulipMessageTypes verifies that a real Zulip server accepts long,
// unicode, special character, and Markdown messages, and that an empty message
// fails.
//
//nolint:paralleltest // Sends to a live server one message at a time.
func TestZulipMessageTypes(t *testing.T) {
	// messages lists the messages the server accepts, by name.
	messages := []struct {
		name, message string
	}{
		{name: "long message", message: strings.Repeat("This is a long message. ", 100)},
		{name: "unicode characters", message: "E2E Test: Unicode — 日本語 🎉 émojis ñ"},
		{name: "special characters", message: "E2E Test: Special <>&\"' \t\n\r characters"},
		{
			name: "markdown formatting",
			message: `**Bold text** and *italic text* and ~~strikethrough~~` + "\n" +
				"```go\npackage main\n```" + "\n" +
				"- List item 1\n- List item 2\n" + "\n" +
				"[Link](https://example.com)",
		},
	}

	for _, tt := range messages {
		t.Run(tt.name, func(t *testing.T) {
			service := newService(t, requireServiceURL(t))

			require.NoError(t, service.Send(tt.message, nil))
		})
	}

	t.Run("empty message fails", func(t *testing.T) {
		service := newService(t, requireServiceURL(t))

		require.Error(t, service.Send("", nil))
	})
}
