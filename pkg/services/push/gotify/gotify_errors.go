package gotify

import (
	"errors"
)

// Input validation errors.
var (
	// ErrInvalidToken indicates an invalid Gotify token format or content.
	ErrInvalidToken = errors.New("invalid gotify token")

	// ErrHostMissing indicates a config without a Gotify server host.
	ErrHostMissing = errors.New("host missing from config")

	// ErrEmptyMessage indicates that the message to send is empty.
	ErrEmptyMessage = errors.New("message cannot be empty")

	// ErrInvalidPriority indicates that the priority value is outside the valid range.
	ErrInvalidPriority = errors.New("priority must be between -2 and 10")

	// ErrExtrasUnmarshalFailed indicates failure to unmarshal extras JSON.
	ErrExtrasUnmarshalFailed = errors.New("failed to unmarshal extras JSON")

	// ErrExtrasParseFailed indicates failure to parse extras JSON from URL query.
	ErrExtrasParseFailed = errors.New("failed to parse extras JSON from URL query")
)

// Send errors.
var (
	// ErrSendFailed indicates failure to send notification to Gotify.
	ErrSendFailed = errors.New("failed to send notification to Gotify")
)

// Configuration errors.
var (
	// ErrConfigPropertyFailed indicates failure with a configuration property.
	ErrConfigPropertyFailed = errors.New("failed to set configuration property")
)
