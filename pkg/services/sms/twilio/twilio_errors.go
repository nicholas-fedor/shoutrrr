package twilio

import "errors"

// Error definitions for the Twilio service.
var (
	// ErrSendFailed indicates a failure in sending the SMS via Twilio.
	ErrSendFailed = errors.New("failed to send SMS via Twilio")
)

// Static errors for configuration validation.
var (
	// ErrAccountSIDMissing indicates that the config URL has no account SID.
	ErrAccountSIDMissing = errors.New("account SID missing from config URL")
	// ErrAuthTokenMissing indicates that the config URL has no auth token.
	ErrAuthTokenMissing = errors.New("auth token missing from config URL")
	// ErrFromNumberMissing indicates that the config URL has no sender phone
	// number or Messaging Service SID.
	ErrFromNumberMissing = errors.New(
		"from number or messaging service SID missing from config URL",
	)
	// ErrToFromNumberSame indicates that a recipient is the sender's phone
	// number. It does not apply when the sender is a Messaging Service SID.
	ErrToFromNumberSame = errors.New("to and from phone numbers must not be the same")
	// ErrToNumbersMissing indicates that the config URL has no recipient
	// phone numbers.
	ErrToNumbersMissing = errors.New("recipient phone number(s) missing from config URL")
)
