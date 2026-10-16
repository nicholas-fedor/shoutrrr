package zulip

import (
	"net/url"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestCreatePayload verifies the form fields for a channel message, with the
// channel as the recipient and an optional topic, and for a direct message,
// with a JSON list of recipient e-mail addresses and no topic.
func TestCreatePayload(t *testing.T) {
	t.Parallel()

	// tests lists configs with the form each should produce for the message.
	tests := []struct {
		name    string
		message string
		config  Config
		want    url.Values
	}{
		{
			name:    "channel by default",
			config:  Config{Stream: "general"},
			message: "Hello",
			want:    url.Values{"type": {"channel"}, "to": {"general"}, "content": {"Hello"}},
		},
		{
			name:    "channel with a topic",
			config:  Config{Type: MessageTypeChannel, Stream: "general", Topic: "alerts"},
			message: "Hello",
			want:    url.Values{"type": {"channel"}, "to": {"general"}, "topic": {"alerts"}, "content": {"Hello"}},
		},
		{
			name:    "empty channel and message",
			config:  Config{},
			message: "",
			want:    url.Values{"type": {"channel"}, "to": {""}, "content": {""}},
		},
		{
			name:    "read by sender",
			config:  Config{Stream: "general", ReadBySender: true},
			message: "Hello",
			want:    url.Values{"type": {"channel"}, "to": {"general"}, "content": {"Hello"}, "read_by_sender": {"true"}},
		},
		{
			name:    "direct message",
			config:  Config{Type: MessageTypeDirect, To: " alice@example.com, ,bob@example.com ", Topic: "alerts", Stream: "general"},
			message: "Hello",
			want:    url.Values{"type": {"direct"}, "to": {`["alice@example.com","bob@example.com"]`}, "content": {"Hello"}},
		},
		{
			name:    "direct message to the stream recipients",
			config:  Config{Type: MessageTypeDirect, Stream: "alice@example.com"},
			message: "Hello",
			want:    url.Values{"type": {"direct"}, "to": {`["alice@example.com"]`}, "content": {"Hello"}},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			payload := CreatePayload(&tt.config, tt.message)

			assert.Equal(t, tt.want, payload)

			decoded, err := url.ParseQuery(payload.Encode())
			require.NoError(t, err)
			assert.Equal(t, tt.want, decoded)
		})
	}
}

// TestParseRecipients verifies that a comma-separated list is split and
// trimmed, with empty entries dropped.
func TestParseRecipients(t *testing.T) {
	t.Parallel()

	// tests lists recipient lists with the recipients each should produce.
	tests := []struct {
		name string
		raw  string
		want []string
	}{
		{name: "empty", raw: "", want: []string{}},
		{name: "only separators", raw: " , ,", want: []string{}},
		{name: "one", raw: "alice@example.com", want: []string{"alice@example.com"}},
		{name: "several", raw: " alice@example.com ,bob@example.com,, ", want: []string{"alice@example.com", "bob@example.com"}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			assert.Equal(t, tt.want, parseRecipients(tt.raw))
		})
	}
}

// TestValidateMessageType verifies that channel, direct, and an empty type
// are accepted and that any other type is rejected.
func TestValidateMessageType(t *testing.T) {
	t.Parallel()

	for _, valid := range []MessageType{MessageTypeChannel, MessageTypeDirect, ""} {
		require.NoError(t, ValidateMessageType(valid), valid)
	}

	for _, invalid := range []MessageType{"group", "topic"} {
		require.ErrorIs(t, ValidateMessageType(invalid), ErrInvalidMessageType, invalid)
	}
}
