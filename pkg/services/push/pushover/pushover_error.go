package pushover

import "errors"

// Errors returned by the Pushover service.
var (
	// ErrSendFailed indicates a failure in sending the notification to a Pushover device.
	ErrSendFailed = errors.New("failed to send notification to pushover device")

	// ErrUserMissing indicates a config without a user key.
	ErrUserMissing = errors.New("user missing from config")

	// ErrTokenMissing indicates a config without an API token.
	ErrTokenMissing = errors.New("token missing from config")

	// ErrInvalidEncryptionKey indicates the E2EE key is not a 64-character hex string.
	ErrInvalidEncryptionKey = errors.New("invalid encryption key")

	// ErrEncryptionFailed indicates a failure while encrypting a Pushover field.
	ErrEncryptionFailed = errors.New("encryption failed")

	// ErrInvalidTimestamp indicates a timestamp that is not a positive Unix time.
	ErrInvalidTimestamp = errors.New("timestamp must be a positive Unix time in seconds")

	// ErrInvalidPriority indicates a priority outside -2 to 2, which the
	// Message API rejects.
	ErrInvalidPriority = errors.New("priority must be from -2 to 2")

	// ErrInvalidRetry indicates an emergency retry interval shorter than 30
	// seconds or not in whole seconds.
	ErrInvalidRetry = errors.New("retry must be whole seconds and at least 30s")

	// ErrInvalidExpire indicates an emergency expiry longer than 3 hours, not
	// positive, or not in whole seconds.
	ErrInvalidExpire = errors.New("expire must be whole seconds from 1s to 3h")

	// ErrMessageMissing indicates an empty message, which the Message API
	// rejects.
	ErrMessageMissing = errors.New("message missing")
)
