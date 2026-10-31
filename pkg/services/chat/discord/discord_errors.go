package discord

import "errors"

// Error definitions for the Discord service.
var (
	// ErrEmptyMessage is returned when attempting to send an empty message.
	ErrEmptyMessage = errors.New("message is empty")

	// ErrUnknownAPIError indicates an unknown error from the Discord API.
	ErrUnknownAPIError = errors.New("unknown error from Discord API")

	// ErrUnexpectedStatus indicates an unexpected HTTP response status code.
	ErrUnexpectedStatus = errors.New("unexpected response status code")

	// ErrInvalidWebhookID indicates a webhook ID that is not a Discord
	// snowflake, a number of up to 64 bits.
	ErrInvalidWebhookID = errors.New("invalid webhook ID")

	// ErrInvalidThreadID indicates a thread ID that is not a Discord
	// snowflake, a number of up to 64 bits.
	ErrInvalidThreadID = errors.New("invalid thread ID")

	// ErrRateLimited indicates rate limiting by Discord.
	ErrRateLimited = errors.New("rate limited by Discord")

	// ErrMaxRetries indicates max retries exceeded.
	ErrMaxRetries = errors.New("max retries exceeded")

	// ErrIllegalURLArgument indicates an illegal argument in config URL.
	ErrIllegalURLArgument = errors.New("illegal argument in config URL")

	// ErrMissingWebhookID indicates webhook ID missing from config URL.
	ErrMissingWebhookID = errors.New("webhook ID missing from config URL")

	// ErrMissingToken indicates token missing from config URL.
	ErrMissingToken = errors.New("token missing from config URL")
)
