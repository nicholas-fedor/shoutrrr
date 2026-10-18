package slack

import (
	"encoding/json"
	"fmt"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestCreateJSONPayload verifies that each message line becomes an attachment
// with the configured color, that the title, bot name, thread, and icon are
// set on the payload, and that the channel is set except for a webhook.
func TestCreateJSONPayload(t *testing.T) {
	t.Parallel()

	config := &Config{
		Title: "Alert", BotName: "testbot", Color: "good", ThreadTS: "1503435956.000247",
		Icon: "ghost", Channel: "C0123456789",
	}

	payload, ok := CreateJSONPayload(config, "One\nTwo").(MessagePayload)
	require.True(t, ok)

	assert.Equal(t, MessagePayload{
		Text: "Alert", BotName: "testbot", ThreadTS: "1503435956.000247", Channel: "C0123456789", IconEmoji: "ghost",
		Attachments: []attachment{{Text: "One", Color: "good"}, {Text: "Two", Color: "good"}},
	}, payload)

	config.Channel = "webhook"

	payload, ok = CreateJSONPayload(config, "One").(MessagePayload)
	require.True(t, ok)
	assert.Empty(t, payload.Channel)
}

// TestCreateJSONPayloadLines verifies that lines past the attachment limit
// join the last attachment, that a trailing newline adds no empty attachment,
// and that an empty message has no attachments.
func TestCreateJSONPayloadLines(t *testing.T) {
	t.Parallel()

	lines := make([]string, 0, 110)
	for i := 1; i <= 110; i++ {
		lines = append(lines, fmt.Sprintf("Line %d", i))
	}

	payload, ok := CreateJSONPayload(&Config{}, strings.Join(lines, "\n")).(MessagePayload)
	require.True(t, ok)

	require.Len(t, payload.Attachments, MaxAttachments)
	assert.Equal(t, "Line 99", payload.Attachments[98].Text)
	assert.Equal(t, strings.Join(lines[99:], "\n"),
		payload.Attachments[MaxAttachments-1].Text)

	payload, ok = CreateJSONPayload(&Config{}, "One\nTwo\n").(MessagePayload)
	require.True(t, ok)
	assert.Equal(t, []attachment{{Text: "One"}, {Text: "Two"}}, payload.Attachments)

	payload, ok = CreateJSONPayload(&Config{}, "").(MessagePayload)
	require.True(t, ok)
	assert.Empty(t, payload.Attachments)
}

// TestSetIcon verifies that an icon with an http or https URL sets the icon
// URL, any other icon sets the emoji, and an empty icon clears both.
func TestSetIcon(t *testing.T) {
	t.Parallel()

	// tests lists icons with the URL and emoji each should set.
	tests := []struct {
		icon      string
		wantURL   string
		wantEmoji string
	}{
		{icon: "https://example.com/logo.png", wantURL: "https://example.com/logo.png"},
		{icon: "http://example.com/logo.png", wantURL: "http://example.com/logo.png"},
		{icon: ":tanabata_tree:", wantEmoji: ":tanabata_tree:"},
		{icon: ""},
	}

	for _, tt := range tests {
		t.Run(tt.icon, func(t *testing.T) {
			t.Parallel()

			payload := MessagePayload{IconURL: "stale", IconEmoji: "stale"}
			payload.SetIcon(tt.icon)

			assert.Equal(t, tt.wantURL, payload.IconURL)
			assert.Equal(t, tt.wantEmoji, payload.IconEmoji)
		})
	}
}

// TestAPIResponse verifies that an API response decodes its status, error,
// warning, and metadata warnings.
func TestAPIResponse(t *testing.T) {
	t.Parallel()

	var response APIResponse

	body := `{"ok":false,"error":"invalid_auth","warning":"superfluous_charset","response_metadata":{"warnings":["superfluous_charset"]}}`
	require.NoError(t, json.Unmarshal([]byte(body), &response))

	assert.False(t, response.Ok)
	assert.Equal(t, "invalid_auth", response.Error)
	assert.Equal(t, "superfluous_charset", response.Warning)
	assert.Equal(t, []string{"superfluous_charset"}, response.MetaData.Warnings)
}

// TestMinInt verifies that minInt returns the smaller value.
func TestMinInt(t *testing.T) {
	t.Parallel()

	assert.Equal(t, 1, minInt(1, 2))
	assert.Equal(t, 1, minInt(2, 1))
	assert.Equal(t, 3, minInt(3, 3))
}
