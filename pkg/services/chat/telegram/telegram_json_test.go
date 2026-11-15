package telegram

import (
	"encoding/json/v2"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/nicholas-fedor/shoutrrr/internal/jsonopts"
)

// htmlMessage contains characters that HTML parse mode must escape.
const htmlMessage = `Oh wow! <3 Cool & stuff ->`

// payloadFromURL builds the sendMessage payload a send of message gives every
// chat of the service configured by rawURL.
//
// Parameters:
//   - t: the test.
//   - rawURL: the service URL.
//   - message: the message to send.
//
// Returns:
//   - SendMessagePayload: the payload without a chat.
func payloadFromURL(t *testing.T, rawURL, message string) SendMessagePayload {
	t.Helper()

	return newMessagePayload(message, newTestService(t, rawURL).GetConfig())
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

// TestPayloadTitle verifies the text a send carries for each parse mode. A
// title goes on a bold line before the message, with the characters the mode
// reads as markup escaped. With no parse mode, a title makes the message HTML
// with its text escaped. Without a title, the message goes as written.
func TestPayloadTitle(t *testing.T) {
	t.Parallel()

	// tests lists URL query strings and messages with the text the payload
	// should carry.
	tests := []struct {
		name    string
		query   string
		message string
		want    string
	}{
		{name: "no title", query: "&parsemode=HTML", message: htmlMessage, want: htmlMessage},
		{
			name:    "no parse mode",
			query:   "&title=Disk%20%3C90%25%20%26%20rising",
			message: htmlMessage,
			want:    "<b>Disk &lt;90% &amp; rising</b>\nOh wow! &lt;3 Cool &amp; stuff -&gt;",
		},
		{name: "HTML", query: "&parsemode=HTML&title=Disk%20%3C90%25", message: "<i>Message</i>", want: "<b>Disk &lt;90%</b>\n<i>Message</i>"},
		{name: "MarkdownV2", query: "&parsemode=MarkdownV2&title=v1.2_(beta)%21", message: "Message", want: "*v1\\.2\\_\\(beta\\)\\!*\nMessage"},
		{name: "MarkdownV2 backslash", query: "&parsemode=MarkdownV2&title=a%5Cb", message: "Message", want: "*a\\\\b*\nMessage"},
		{name: "Markdown", query: "&parsemode=Markdown&title=Alert", message: "Message", want: "*Alert*\nMessage"},
		{
			name:    "Markdown asterisks",
			query:   "&parsemode=Markdown&title=2*2=4",
			message: "Message",
			want:    "*2*\\**2=4*\nMessage",
		},
		{name: "Markdown other markup characters", query: "&parsemode=Markdown&title=disk_usage%20%60df%60%20%5Bprod%5D", message: "Message", want: "*disk_usage `df` [prod]*\nMessage"},
		{name: "Markdown title of asterisks", query: "&parsemode=Markdown&title=**_", message: "Message", want: "\\*\\**_*\nMessage"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			payload := payloadFromURL(t, "telegram://12345:mock-token@telegram/?channels=channel-1"+tt.query, tt.message)

			assert.Equal(t, tt.want, payload.Text)
		})
	}
}

// TestUpdateUnmarshal verifies that an update decodes from the member names
// the Bot API documents, which are lowercase and match the field tags exactly.
func TestUpdateUnmarshal(t *testing.T) {
	t.Parallel()

	input := `{"update_id":7,` +
		`"message":{"message_id":2,"text":"hi","chat":{"id":3,"type":"private"}},` +
		`"callback_query":{"id":"q","data":"d","message":{"message_id":4,"text":"pick"}}}`

	var update Update
	require.NoError(t, json.Unmarshal([]byte(input), &update, jsonopts.Default))

	assert.Equal(t, 7, update.UpdateID)
	require.NotNil(t, update.Message)
	assert.Equal(t, "hi", update.Message.Text)
	require.NotNil(t, update.Message.Chat)
	assert.Equal(t, int64(3), update.Message.Chat.ID)
	require.NotNil(t, update.CallbackQuery)
	require.NotNil(t, update.CallbackQuery.Message)
	assert.Equal(t, "pick", update.CallbackQuery.Message.Text)
}
