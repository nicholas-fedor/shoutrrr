package mattermost

import "errors"

// ErrorMessage represents error events within the Mattermost service.
type ErrorMessage string

// Errors returned by the Mattermost service.
var (
	// ErrSendFailed indicates that the notification failed due to an unexpected response status code.
	ErrSendFailed = errors.New(
		"failed to send notification to service, response status code unexpected",
	)
	// ErrInvalidURL indicates that the provided URL is invalid.
	ErrInvalidURL = errors.New("invalid URL: scheme must be http or https and host must be present")
	// ErrHostMissing indicates that the config URL has no Mattermost server host.
	ErrHostMissing = errors.New("host missing from config URL")
	// ErrNotEnoughArguments indicates that the API URL does not include enough arguments.
	ErrNotEnoughArguments = errors.New(
		"the apiURL does not include enough arguments, either provide 1 or 3 arguments (they may be empty)",
	)
)
