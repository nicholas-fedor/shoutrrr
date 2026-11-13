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

	// ErrMissingWebhookID indicates a config without a webhook ID.
	ErrMissingWebhookID = errors.New("webhook ID missing from config")

	// ErrMissingToken indicates a config without a token.
	ErrMissingToken = errors.New("token missing from config")

	// ErrInvalidThreadName indicates a thread name longer than Discord's 100
	// characters.
	ErrInvalidThreadName = errors.New("invalid thread name")

	// ErrThreadIDAndName indicates a config that sets both a thread ID and a
	// thread name, which Discord takes one at a time.
	ErrThreadIDAndName = errors.New("thread ID and thread name both set")

	// ErrThreadUnknown indicates a request of a send that names a thread, made
	// after the request that was to create the thread failed in a way that
	// leaves the thread unknown. The request is not sent, since it has no
	// thread to go into.
	ErrThreadUnknown = errors.New("thread for the rest of the message is unknown")

	// ErrInvalidMention indicates a mention that is not everyone, here,
	// user:ID, or role:ID with a Discord snowflake as the ID, or more than 100
	// user or role mentions.
	ErrInvalidMention = errors.New("invalid mention")
)
