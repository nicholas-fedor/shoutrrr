package pushover

import "errors"

// ErrSendFailed indicates a failure in sending the notification to a Pushover device.
var ErrSendFailed = errors.New("failed to send notification to pushover device")

// ErrUserMissing indicates the user key is missing from the Pushover config URL.
var ErrUserMissing = errors.New("user missing from config URL")

// ErrTokenMissing indicates the API token is missing from the Pushover config URL.
var ErrTokenMissing = errors.New("token missing from config URL")
