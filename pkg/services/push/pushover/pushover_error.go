package pushover

import "errors"

var (
	// ErrSendFailed indicates a failure in sending the notification to a Pushover device.
	ErrSendFailed = errors.New("failed to send notification to pushover device")

	// ErrUserMissing indicates the user key is missing from the Pushover config URL.
	ErrUserMissing = errors.New("user missing from config URL")

	// ErrTokenMissing indicates the API token is missing from the Pushover config URL.
	ErrTokenMissing = errors.New("token missing from config URL")

	// ErrInvalidEncryptionKey indicates the E2EE key is not a 64-character hex string.
	ErrInvalidEncryptionKey = errors.New("invalid encryption key")

	// ErrEncryptionFailed indicates a failure while encrypting a Pushover field.
	ErrEncryptionFailed = errors.New("encryption failed")
)
