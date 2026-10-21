package rocketchat

import "errors"

// Errors returned by the Rocket.Chat service.
var (
	// ErrInvalidURLScheme indicates the URL scheme is not valid for Rocket.Chat webhooks.
	ErrInvalidURLScheme = errors.New("rocketchat: invalid URL scheme, only https is allowed")
	// ErrNotificationFailed indicates a failure in sending the notification.
	ErrNotificationFailed = errors.New("notification failed")
	// ErrHostMissing indicates that the config URL has no Rocket.Chat server host.
	ErrHostMissing = errors.New("host missing from config URL")
	// ErrNotEnoughArguments indicates the API URL does not include enough arguments.
	ErrNotEnoughArguments = errors.New("the apiURL does not include enough arguments")
)
