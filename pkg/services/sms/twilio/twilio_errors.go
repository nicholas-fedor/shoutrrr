package twilio

import "errors"

// Error definitions for the Twilio service.
var (
	// ErrSendFailed indicates a failure in sending the SMS via Twilio.
	ErrSendFailed = errors.New("failed to send SMS via Twilio")
)
