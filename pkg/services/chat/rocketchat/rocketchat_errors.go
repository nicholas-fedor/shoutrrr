package rocketchat

import "errors"

// ErrInvalidURLScheme indicates the URL scheme is not valid for Rocket.Chat webhooks.
var ErrInvalidURLScheme = errors.New("rocketchat: invalid URL scheme, only https is allowed")

// ErrNotificationFailed indicates a failure in sending the notification.
var ErrNotificationFailed = errors.New("notification failed")

// ErrNotEnoughArguments indicates the API URL does not include enough arguments.
var ErrNotEnoughArguments = errors.New("the apiURL does not include enough arguments")
