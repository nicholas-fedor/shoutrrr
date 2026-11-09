package twilio

import "errors"

// Error definitions for the Twilio service.
var (
	// ErrSendFailed indicates a failure in sending the SMS via Twilio.
	ErrSendFailed = errors.New("failed to send SMS via Twilio")
)

// Static errors for configuration validation.
var (
	// ErrAccountSIDMissing indicates a config without an account SID.
	ErrAccountSIDMissing = errors.New("account SID missing from config")
	// ErrInvalidAccountSID indicates an account SID other than AC followed by
	// 32 hexadecimal characters, the format of every Twilio account SID.
	ErrInvalidAccountSID = errors.New("account SID must be AC followed by 32 hexadecimal characters")
	// ErrAuthTokenMissing indicates a config without an auth token.
	ErrAuthTokenMissing = errors.New("auth token missing from config")
	// ErrFromNumberMissing indicates a config without a sender phone number or
	// Messaging Service SID.
	ErrFromNumberMissing = errors.New(
		"from number or messaging service SID missing from config",
	)
	// ErrToFromNumberSame indicates that a recipient is the sender's phone
	// number. It does not apply when the sender is a Messaging Service SID.
	ErrToFromNumberSame = errors.New("to and from phone numbers must not be the same")
	// ErrToNumbersMissing indicates a config without recipient phone numbers.
	ErrToNumbersMissing = errors.New("recipient phone number(s) missing from config")
)
