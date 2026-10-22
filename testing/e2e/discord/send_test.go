package e2e_test

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

// avatarURL is the image the username and avatar test sets as the webhook's avatar.
const avatarURL = "https://raw.githubusercontent.com/nicholas-fedor/shoutrrr/master/docs/assets/media/shoutrrr-180px.png"

// TestDiscordSend verifies that Discord accepts plain text messages, with
// special characters, a custom username and avatar, a raw JSON payload, and a
// long message split into chunks.
//
//nolint:paralleltest // Sends to a live webhook one message at a time.
func TestDiscordSend(t *testing.T) {
	// sends lists the messages with the query values set on the URL for each.
	sends := []struct {
		query         map[string]string
		name, message string
	}{
		{name: "basic text content", message: "E2E Test: Basic text content notification"},
		{
			name:    "special characters",
			message: "E2E Test: Special characters - éñüñ 中文 🚀 @everyone #channel <@123456> ||spoiler|| `code`",
		},
		{
			name:    "custom username and avatar",
			query:   map[string]string{"username": "ShoutrrrDemo", "avatar": avatarURL},
			message: "E2E Test: Custom username and avatar",
		},
		{
			name:    "JSON mode",
			query:   map[string]string{"json": "true"},
			message: `{"content": "E2E Test: Raw JSON message demonstrating JSON mode"}`,
		},
		{
			// With splitLines off, the message is split into chunks.
			name:  "long message (chunking)",
			query: map[string]string{"splitLines": "false"},
			message: "E2E Test: Long message for chunking - " + strings.Repeat(
				"This is a very long message that should be split into multiple messages. ",
				50,
			),
		},
	}

	for _, tt := range sends {
		t.Run(tt.name, func(t *testing.T) {
			require.NoError(t, newService(t, tt.query).Send(tt.message, nil))
		})
	}
}
