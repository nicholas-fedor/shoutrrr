package pushbullet

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

// TestNewNotePush verifies that a new push is a note with the message as its
// body, the given title, and no target.
func TestNewNotePush(t *testing.T) {
	t.Parallel()

	assert.Equal(t, &PushRequest{Type: "note", Title: "Title", Body: "Message"}, NewNotePush("Message", "Title"))
}

// TestPushRequestSetTarget verifies that SetTarget fills exactly one target
// field: the email for an address, the channel tag without its # for a
// channel, and the device iden otherwise.
func TestPushRequestSetTarget(t *testing.T) {
	t.Parallel()

	// tests lists targets with the push fields each should set.
	tests := []struct {
		name   string
		target string
		want   PushRequest
	}{
		{name: "email", target: "iam@email.com", want: PushRequest{Email: "iam@email.com"}},
		{name: "channel", target: "#channel", want: PushRequest{ChannelTag: "channel"}},
		{name: "device", target: "device", want: PushRequest{DeviceIden: "device"}},
		{name: "empty", target: "", want: PushRequest{}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			push := PushRequest{}
			push.SetTarget(tt.target)

			assert.Equal(t, tt.want, push)
		})
	}
}

// TestResponseErrorError verifies that an API error reports the server's
// message as its error text.
func TestResponseErrorError(t *testing.T) {
	t.Parallel()

	responseErr := &ResponseError{}
	responseErr.ErrorData.Message = "Access token is missing or invalid."

	assert.EqualError(t, responseErr, "Access token is missing or invalid.")
}
