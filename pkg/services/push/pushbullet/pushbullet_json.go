package pushbullet

import (
	"regexp"
)

// PushRequest is the body of a request to create a push.
type PushRequest struct {
	// Type is the push type, "note" for a title and a body.
	Type string `json:"type"`
	// Title is the push title.
	Title string `json:"title"`
	// Body is the push body, the message.
	Body string `json:"body"`

	// Email sends the push to the user with this e-mail address.
	Email string `json:"email"`
	// ChannelTag sends the push to the subscribers of the channel with this
	// tag.
	ChannelTag string `json:"channel_tag"`
	// DeviceIden sends the push to the device with this identifier.
	DeviceIden string `json:"device_iden"`
}

// PushResponse is the push the API returns after creating one.
type PushResponse struct {
	// Body is the push body.
	Body string `json:"body"`
	// Direction is self, outgoing, or incoming, relative to the token's user.
	Direction string `json:"direction"`
	// Iden is the push's identifier.
	Iden string `json:"iden"`
	// ReceiverEmail is the receiving user's e-mail address.
	ReceiverEmail string `json:"receiver_email"`
	// ReceiverEmailNormalized is ReceiverEmail in canonical form.
	ReceiverEmailNormalized string `json:"receiver_email_normalized"`
	// ReceiverIden is the receiving user's identifier.
	ReceiverIden string `json:"receiver_iden"`
	// SenderEmail is the sending user's e-mail address.
	SenderEmail string `json:"sender_email"`
	// SenderEmailNormalized is SenderEmail in canonical form.
	SenderEmailNormalized string `json:"sender_email_normalized"`
	// SenderIden is the sending user's identifier.
	SenderIden string `json:"sender_iden"`
	// SenderName is the sending user's name.
	SenderName string `json:"sender_name"`
	// Title is the push title.
	Title string `json:"title"`
	// Type is the push type.
	Type string `json:"type"`
	// Created is the creation time, in Unix seconds.
	Created float64 `json:"created"`
	// Modified is the time of the last change, in Unix seconds.
	Modified float64 `json:"modified"`
	// Active is false for a deleted push.
	Active bool `json:"active"`
	// Dismissed reports whether the push was dismissed.
	Dismissed bool `json:"dismissed"`
}

// ResponseError is the error body the API returns for a failed request.
type ResponseError struct {
	// ErrorData describes the error.
	ErrorData struct {
		// Cat is an ASCII-art cat the API includes in every error.
		Cat string `json:"cat"`
		// Message describes the error.
		Message string `json:"message"`
		// Type is the error type, such as invalid_request.
		Type string `json:"type"`
	} `json:"error"`
}

// emailPattern matches a target that is an e-mail address.
var emailPattern = regexp.MustCompile(`.*@.*\..*`)

// Error returns the error message from the API.
//
// Returns:
//   - string: the message.
func (err *ResponseError) Error() string {
	return err.ErrorData.Message
}

// NewNotePush creates a new push request.
//
// Parameters:
//   - message: the push body.
//   - title: the push title.
//
// Returns:
//   - *PushRequest: a note push without a target.
//
//nolint:exhaustruct_v5 // PushRequest targeting fields (Email, ChannelTag, DeviceIden) are set by SetTarget method
func NewNotePush(message, title string) *PushRequest {
	return &PushRequest{
		Type:  "note",
		Title: title,
		Body:  message,
	}
}

// SetTarget sets the push's target from a URL path target.
//
// Parameters:
//   - target: an e-mail address for a user, a #tag for a channel, or a device
//     identifier. Empty sends the push to all of the token user's devices.
func (p *PushRequest) SetTarget(target string) {
	if emailPattern.MatchString(target) {
		p.Email = target

		return
	}

	if target != "" && string(target[0]) == "#" {
		p.ChannelTag = target[1:]

		return
	}

	p.DeviceIden = target
}
