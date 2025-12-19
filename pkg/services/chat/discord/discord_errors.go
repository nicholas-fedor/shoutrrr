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

	// ErrInvalidURLPrefix indicates the URL must start with Discord webhook base URL.
	ErrInvalidURLPrefix = errors.New("URL must start with Discord webhook base URL")

	// ErrInvalidScheme indicates an invalid URL scheme; must be https.
	ErrInvalidScheme = errors.New("invalid URL scheme: must be https")

	// ErrInvalidHost indicates an invalid host; must be discord.com.
	ErrInvalidHost = errors.New("invalid host: must be discord.com")

	// ErrInvalidWebhookID indicates an invalid webhook ID.
	ErrInvalidWebhookID = errors.New("invalid webhook ID")

	// ErrInvalidToken indicates an invalid token.
	ErrInvalidToken = errors.New("invalid token")

	// ErrEmptyURL indicates an empty URL was provided.
	ErrEmptyURL = errors.New("empty URL provided")

	// ErrMalformedURL indicates a malformed URL missing webhook ID or token.
	ErrMalformedURL = errors.New("malformed URL: missing webhook ID or token")

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
