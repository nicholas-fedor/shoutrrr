package teams

import "errors"

// Error variables for the Teams package.
var (
	// ErrInvalidWebhookFormat indicates the webhook URL doesn't contain the organization domain.
	ErrInvalidWebhookFormat = errors.New(
		"invalid webhook URL format - must contain organization domain",
	)

	// ErrMissingHostParameter indicates the required host parameter is missing.
	ErrMissingHostParameter = errors.New(
		"missing required host parameter (organization.webhook.office.com)",
	)

	// ErrMissingExtraIDComponent indicates the URL is missing the extraId component.
	ErrMissingExtraIDComponent = errors.New("invalid URL format: missing extraId component")

	// ErrMissingHost indicates the host is not specified in the configuration.
	ErrMissingHost = errors.New("host is required but not specified in the configuration")

	// ErrSetParameterFailed indicates failure to set a configuration parameter.
	ErrSetParameterFailed = errors.New("failed to set configuration parameter")

	// ErrSendFailedStatus indicates an unexpected status code in the response.
	ErrSendFailedStatus = errors.New(
		"failed to send notification to teams, response status code unexpected",
	)

	// ErrSendFailed indicates a general failure in sending the notification.
	ErrSendFailed = errors.New("an error occurred while sending notification to teams")

	// ErrInvalidWebhookURL indicates the webhook URL format is invalid.
	ErrInvalidWebhookURL = errors.New("invalid webhook URL format")

	// ErrInvalidHostFormat indicates the host format is invalid.
	ErrInvalidHostFormat = errors.New("invalid host format")

	// ErrInvalidWebhookComponents indicates a mismatch in expected webhook URL components.
	ErrInvalidWebhookComponents = errors.New(
		"invalid webhook URL format: expected component count mismatch",
	)

	// ErrInvalidPartLength indicates a webhook component has an incorrect length.
	ErrInvalidPartLength = errors.New("invalid webhook part length")

	// ErrMissingExtraID indicates the extraID is missing.
	ErrMissingExtraID = errors.New("extraID is required")
)
