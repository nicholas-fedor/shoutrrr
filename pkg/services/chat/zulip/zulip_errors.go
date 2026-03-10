package zulip

import "errors"

// ErrMissingHost indicates the host is not specified in the configuration.
var ErrMissingHost = errors.New("host missing from config URL")

// ErrMissingAPIKey indicates the API key is missing from the configuration.
var ErrMissingAPIKey = errors.New("API key missing from config URL")

// ErrMissingBotMail indicates the bot mail address is missing from the configuration.
var ErrMissingBotMail = errors.New("bot mail missing from config URL")

// ErrTopicTooLong indicates the topic exceeds the maximum allowed length.
var ErrTopicTooLong = errors.New("topic exceeds max length")

// ErrMessageTooLong indicates the message exceeds the maximum allowed size.
var ErrMessageTooLong = errors.New("message exceeds max size")

// ErrResponseStatusFailure indicates an unexpected HTTP response status code.
var ErrResponseStatusFailure = errors.New("response status code unexpected")

// ErrInvalidHost indicates the host format is invalid.
var ErrInvalidHost = errors.New("invalid host format")
