package zulip

import "errors"

// Error variables for the Zulip package.
var (
	// ErrMissingHost indicates the host is not specified in the configuration.
	ErrMissingHost = errors.New("host missing from config URL")

	// ErrMissingAPIKey indicates the API key is missing from the configuration.
	ErrMissingAPIKey = errors.New("API key missing from config URL")

	// ErrMissingBotMail indicates the bot mail address is missing from the configuration.
	ErrMissingBotMail = errors.New("bot mail missing from config URL")

	// ErrTopicTooLong indicates the topic exceeds the maximum allowed length.
	ErrTopicTooLong = errors.New("topic exceeds max length")

	// ErrMessageTooLong indicates the message exceeds the maximum allowed size.
	ErrMessageTooLong = errors.New("message exceeds max size")

	// ErrResponseStatusFailure indicates an unexpected HTTP response status code.
	ErrResponseStatusFailure = errors.New("response status code unexpected")

	// ErrInvalidHost indicates the host format is invalid.
	ErrInvalidHost = errors.New("invalid host format")
)
