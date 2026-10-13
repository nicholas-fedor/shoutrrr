package telegram

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// htmlMessage contains characters that HTML parse mode must escape.
const htmlMessage = `Oh wow! <3 Cool & stuff ->`

// payloadFromURL builds the sendMessage payload for message and the first chat
// of the service configured by rawURL.
//
// Parameters:
//   - t: the test.
//   - rawURL: the service URL.
//   - message: the message to send.
//
// Returns:
//   - SendMessagePayload: the payload for the first chat.
func payloadFromURL(t *testing.T, rawURL, message string) SendMessagePayload {
	t.Helper()

	config := newTestService(t, rawURL).GetConfig()
	require.NotEmpty(t, config.Chats)

	return createSendMessagePayload(message, config.Chats[0], config)
}

// TestPayloadParseMode verifies that parse_mode is sent only for a parse mode
// other than None, and that a title switches the payload to HTML.
func TestPayloadParseMode(t *testing.T) {
	t.Parallel()

	// tests lists service URLs with the parse mode the payload should carry, or
	// an empty string when the payload should omit parse_mode.
	tests := []struct {
		name   string
		rawURL string
		want   string
	}{
		{name: "not set", rawURL: "telegram://12345:mock-token@telegram/?channels=channel-1", want: ""},
		{name: "Markdown", rawURL: "telegram://12345:mock-token@telegram/?channels=channel-1&parsemode=Markdown", want: "Markdown"},
		{name: "None", rawURL: "telegram://12345:mock-token@telegram/?channels=channel-1&parsemode=None", want: ""},
		{name: "title with no parse mode", rawURL: "telegram://12345:mock-token@telegram/?channels=channel-1&title=MessageTitle", want: "HTML"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			payload := payloadFromURL(t, tt.rawURL, "Message")
			assert.Equal(t, tt.want, payload.ParseMode)

			raw, err := json.Marshal(payload)
			require.NoError(t, err)

			var fields map[string]any
			require.NoError(t, json.Unmarshal(raw, &fields))

			if tt.want == "" {
				assert.NotContains(t, fields, "parse_mode")
			} else {
				assert.Equal(t, tt.want, fields["parse_mode"])
			}
		})
	}
}

// TestPayloadTitleEscapesHTML verifies that a title is prepended to the message
// and that the message's HTML characters are escaped.
func TestPayloadTitleEscapesHTML(t *testing.T) {
	t.Parallel()

	payload := payloadFromURL(t, "telegram://12345:mock-token@telegram/?channels=channel-1&title=MessageTitle", htmlMessage)

	assert.Contains(t, payload.Text, "MessageTitle")
	assert.Contains(t, payload.Text, "&lt;3")
	assert.Contains(t, payload.Text, "Cool &amp; stuff")
	assert.Contains(t, payload.Text, "-&gt;")
}

// TestPayloadMessageThreadID verifies that a numeric thread ID after the chat
// sets message_thread_id, and that a missing or non-numeric one leaves it unset.
func TestPayloadMessageThreadID(t *testing.T) {
	t.Parallel()

	// tests lists chats with the thread ID the payload should carry, or nil
	// when it should carry none.
	tests := []struct {
		name string
		chat string
		want *int
	}{
		{name: "no thread ID", chat: "channel-1", want: nil},
		{name: "numeric thread ID", chat: "channel-1:10", want: new(10)},
		{name: "non-numeric thread ID", chat: "channel-1:invalid", want: nil},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			payload := payloadFromURL(t, "telegram://12345:mock-token@telegram/?channels="+tt.chat+"&title=MessageTitle", htmlMessage)

			assert.Equal(t, tt.want, payload.MessageThreadID)
		})
	}
}
