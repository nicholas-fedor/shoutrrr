package pushover

import "errors"

// TODO: Cleanup

// ErrorMessage for error events within the pushover service.
type ErrorMessage string

const (
	// UserMissing should be used when a config URL is missing a user.
	UserMissing ErrorMessage = "user missing from config URL"
	// TokenMissing should be used when a config URL is missing a token.
	TokenMissing ErrorMessage = "token missing from config URL"
)

// ErrSendFailed indicates a failure in sending the notification to a Pushover device.
var ErrSendFailed = errors.New("failed to send notification to pushover device")
