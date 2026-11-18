package pushbullet

import "strings"

// PushRequest is the body of a request to create a push.
type PushRequest struct {
	// Type is the push type, "note" for a title and a body.
	Type string `json:"type"`
	// Title is the push title.
	Title string `json:"title"`
	// Body is the push body, the message.
	Body string `json:"body"`

	// Email sends the push to the Pushbullet user with this e-mail address.
	// When the address has no Pushbullet user, Pushbullet sends an e-mail to
	// it instead.
	Email string `json:"email"`
	// ChannelTag sends the push to the subscribers of the channel with this
	// tag.
	ChannelTag string `json:"channel_tag"`
	// DeviceIden sends the push to the device with this identifier.
	DeviceIden string `json:"device_iden"`
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
	if isEmailTarget(target) {
		p.Email = target

		return
	}

	if target != "" && string(target[0]) == "#" {
		p.ChannelTag = target[1:]

		return
	}

	p.DeviceIden = target
}

// isEmailTarget reports whether a target names an e-mail address. A target
// does when one of its lines holds a "." after an "@".
//
// Parameters:
//   - target: a URL path target.
//
// Returns:
//   - bool: true when the target names an e-mail address.
func isEmailTarget(target string) bool {
	for {
		at := strings.IndexByte(target, '@')
		if at < 0 {
			return false
		}

		// Only the rest of the line after the "@" can hold its ".". Each line is
		// scanned once, so a long target with many lines stays linear.
		line, rest, more := strings.Cut(target[at+1:], "\n")
		if strings.IndexByte(line, '.') >= 0 {
			return true
		}

		if !more {
			return false
		}

		target = rest
	}
}
