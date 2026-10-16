package signal

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

// TestCreatePayload verifies that createPayload maps the config and message
// to the /v2/send fields, sending notify_self only to turn it off.
func TestCreatePayload(t *testing.T) {
	t.Parallel()

	// tests lists configs with the payload each should produce for the
	// message "Hello".
	tests := []struct {
		name   string
		config Config
		want   sendMessagePayload
	}{
		{
			name:   "defaults",
			config: Config{Source: testSource, Recipients: []string{testRecipient}, NotifySelf: true},
			want:   sendMessagePayload{Message: "Hello", Number: testSource, Recipients: []string{testRecipient}},
		},
		{
			name: "every setting",
			config: Config{
				Source:      testSource,
				Recipients:  []string{"u:alice.01", "u:bob.02"},
				Title:       "Alert",
				Attachments: "aGVsbG8=,d29ybGQ=",
				TextMode:    TextModeStyled,
			},
			want: sendMessagePayload{
				Message:           "**Alert**\nHello",
				Number:            testSource,
				Recipients:        []string{"alice.01", "bob.02"},
				Base64Attachments: []string{"aGVsbG8=", "d29ybGQ="},
				TextMode:          "styled",
				NotifySelf:        new(false),
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			assert.Equal(t, tt.want, createPayload("Hello", &tt.config))
		})
	}
}

// TestComposeMessage verifies that a title goes on its own line before the
// message, in bold when styled, and alone when the message is empty.
func TestComposeMessage(t *testing.T) {
	t.Parallel()

	// tests lists titles and messages with the text each should produce.
	tests := []struct {
		name    string
		title   string
		message string
		want    string
		styled  bool
	}{
		{name: "no title", message: "body", want: "body"},
		{name: "no title styled", message: "body", styled: true, want: "body"},
		{name: "plain title", title: "Alert", message: "body", want: "Alert\nbody"},
		{name: "styled title", title: "Alert", message: "body", styled: true, want: "**Alert**\nbody"},
		{name: "title only", title: "Alert", want: "Alert"},
		{name: "styled title only", title: "Alert", styled: true, want: "**Alert**"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			assert.Equal(t, tt.want, composeMessage(tt.title, tt.message, tt.styled))
		})
	}
}

// TestAPIRecipients verifies that usernames lose the u: prefix and that other
// recipients pass through unchanged.
func TestAPIRecipients(t *testing.T) {
	t.Parallel()

	assert.Equal(t,
		[]string{"alice.01", testRecipient, "group.ABCD/EFGH=", "u:"},
		apiRecipients([]string{"u:alice.01", testRecipient, "group.ABCD/EFGH=", "u:"}))
	assert.Empty(t, apiRecipients(nil))
}

// TestParseAttachments verifies that raw base64 values are split on commas
// and trimmed, that a value with a data URI stays whole, and that an empty
// value yields no attachments.
func TestParseAttachments(t *testing.T) {
	t.Parallel()

	// tests lists attachment values with the attachments each should produce.
	tests := []struct {
		name string
		raw  string
		want []string
	}{
		{name: "empty", raw: ""},
		{name: "whitespace", raw: "   "},
		{name: "only commas", raw: " , ,"},
		{name: "one value", raw: "aGVsbG8=", want: []string{"aGVsbG8="}},
		{name: "comma separated", raw: " aGVsbG8=, ,d29ybGQ= ", want: []string{"aGVsbG8=", "d29ybGQ="}},
		{
			name: "data URI",
			raw:  " data:image/png;filename=dot.png;base64,iVBORw0KGgo= ",
			want: []string{"data:image/png;filename=dot.png;base64,iVBORw0KGgo="},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			assert.Equal(t, tt.want, parseAttachments(tt.raw))
		})
	}
}
