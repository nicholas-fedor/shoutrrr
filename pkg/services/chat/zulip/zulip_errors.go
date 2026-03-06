package zulip

import "errors"

// TODO: Cleanup

// ErrorMessage for error events within the zulip service.
type ErrorMessage string

const (
	// MissingAPIKey from the service URL.
	MissingAPIKey ErrorMessage = "missing API key"
	// MissingHost from the service URL.
	MissingHost ErrorMessage = "missing Zulip host"
	// MissingBotMail from the service URL.
	MissingBotMail ErrorMessage = "missing Bot mail address"
	// TopicTooLong if topic is more than 60 characters.
	TopicTooLong ErrorMessage = "topic exceeds max length (%d characters): was %d characters"
)

var (
	// ErrTopicTooLong indicates the topic exceeds the maximum allowed length.
	ErrTopicTooLong          = errors.New("topic exceeds max length")
	ErrMessageTooLong        = errors.New("message exceeds max size")
	ErrResponseStatusFailure = errors.New("response status code unexpected")
	ErrInvalidHost           = errors.New("invalid host format")
)

// Static errors for configuration validation.
var (
	ErrMissingBotMail = errors.New("bot mail missing from config URL")
	ErrMissingAPIKey  = errors.New("API key missing from config URL")
	ErrMissingHost    = errors.New("host missing from config URL")
)
