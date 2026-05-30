package teams

import "errors"

// Error variables for the Teams package.
var (
	// ErrMissingHost indicates the host webhook URL is not specified in the configuration.
	ErrMissingHost = errors.New("host is required but not specified in the configuration")

	// ErrSendFailed indicates a general failure in sending the notification.
	ErrSendFailed = errors.New("an error occurred while sending notification to teams")

	// ErrSendFailedStatus indicates an unexpected status code in the response.
	ErrSendFailedStatus = errors.New(
		"failed to send notification to teams, response status code unexpected",
	)

	// ErrInvalidWebhookURL indicates the webhook URL format is invalid.
	ErrInvalidWebhookURL = errors.New("invalid webhook URL format")
)
