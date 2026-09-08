package homeassistant

import "errors"

var (
	// ErrTokenMissing indicates that the long-lived access token is absent from the URL.
	ErrTokenMissing = errors.New("homeassistant access token is missing")

	// ErrHostMissing indicates that the Home Assistant hostname is absent from the URL.
	ErrHostMissing = errors.New("homeassistant host is missing")

	// ErrMessageEmpty indicates that the notification message is empty.
	ErrMessageEmpty = errors.New("homeassistant message is empty")

	// ErrInvalidService indicates that the service query value is not a valid domain.action pair.
	ErrInvalidService = errors.New("homeassistant service is invalid")

	// ErrInvalidPort indicates that the URL port is not a valid TCP port.
	ErrInvalidPort = errors.New("homeassistant port is invalid")

	// ErrSendFailed indicates that sending a Home Assistant notification failed.
	ErrSendFailed = errors.New("failed to send homeassistant notification")

	// ErrUnexpectedStatus indicates that the Home Assistant API returned a non-success HTTP status.
	ErrUnexpectedStatus = errors.New("homeassistant API returned unexpected status")
)
