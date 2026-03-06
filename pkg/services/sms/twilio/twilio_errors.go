package twilio

import "errors"

// Error definitions for the Twilio service.
var (
	// ErrSendFailed indicates a failure in sending the SMS via Twilio.
	ErrSendFailed = errors.New("failed to send SMS via Twilio")
)

// Static errors for configuration validation.
var (
	ErrAccountSIDMissing = errors.New("account SID missing from config URL")
	ErrAuthTokenMissing  = errors.New("auth token missing from config URL")
	ErrFromNumberMissing = errors.New(
		"from number or messaging service SID missing from config URL",
	)
	ErrToFromNumberSame = errors.New("to and from phone numbers must not be the same")
	ErrToNumbersMissing = errors.New("recipient phone number(s) missing from config URL")
)
