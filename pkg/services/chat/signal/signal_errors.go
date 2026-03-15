package signal

import "errors"

// Error definitions for the Signal service.
var (
	// ErrInvalidPhoneNumber indicates that a phone number does not match the required format.
	ErrInvalidPhoneNumber = errors.New("invalid phone number format")
	// ErrInvalidGroupID indicates that a group ID does not match the required format.
	ErrInvalidGroupID = errors.New("invalid group ID format")
	// ErrNoRecipients indicates that no recipients were specified.
	ErrNoRecipients = errors.New("no recipients specified")
	// ErrInvalidRecipient indicates that a recipient is neither a valid phone number nor a valid group ID.
	ErrInvalidRecipient = errors.New("invalid recipient: must be phone number or group ID")
	// ErrSendFailed indicates a failure to send a Signal message.
	ErrSendFailed = errors.New("failed to send Signal message")
	// ErrInvalidHost indicates that the configured host is not allowed for security reasons (SSRF protection).
	ErrInvalidHost = errors.New("invalid host: private or loopback addresses are not permitted")
)
