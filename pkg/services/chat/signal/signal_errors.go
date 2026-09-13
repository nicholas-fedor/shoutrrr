package signal

import "errors"

// Error definitions for the Signal service.
var (
	// ErrInvalidPhoneNumber indicates that a phone number does not match the required format.
	ErrInvalidPhoneNumber = errors.New("invalid phone number format")
	// ErrNoRecipients indicates that no recipients were specified.
	ErrNoRecipients = errors.New("no recipients specified")
	// ErrInvalidRecipient indicates that a recipient is neither a valid phone number, group ID, nor username.
	ErrInvalidRecipient = errors.New("invalid recipient: must be phone number, group ID, or username")
	// ErrSendFailed indicates a failure to send a Signal message.
	ErrSendFailed = errors.New("failed to send Signal message")
)
