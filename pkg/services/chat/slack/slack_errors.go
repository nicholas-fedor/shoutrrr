package slack

import "errors"

// ErrInvalidToken is returned when the specified token does not match any known formats.
var ErrInvalidToken = errors.New("invalid slack token format")

// ErrMismatchedTokenSeparators is returned if the token uses different separators between parts (of the recognized `/-,`).
var ErrMismatchedTokenSeparators = errors.New("invalid webhook token format")

// ErrAPIResponseFailure indicates a failure in the Slack API response.
var ErrAPIResponseFailure = errors.New("api response failure")

// ErrUnknownAPIError indicates an unknown error from the Slack API.
var ErrUnknownAPIError = errors.New("unknown error from Slack API")

// ErrWebhookStatusFailure indicates a failure due to an unexpected webhook status code.
var ErrWebhookStatusFailure = errors.New("webhook status failure")

// ErrWebhookResponseFailure indicates a failure in the webhook response content.
var ErrWebhookResponseFailure = errors.New("webhook response failure")
