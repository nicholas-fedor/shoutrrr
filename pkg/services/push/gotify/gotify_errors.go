package gotify

import (
	"errors"
)

// Service/Initialization errors.
var (
	// ErrServiceNotInitialized indicates that the service has not been initialized.
	ErrServiceNotInitialized = errors.New("service not initialized")
)

// Input validation errors.
var (
	// ErrInvalidToken indicates an invalid Gotify token format or content.
	ErrInvalidToken = errors.New("invalid gotify token")

	// ErrEmptyMessage indicates that the message to send is empty.
	ErrEmptyMessage = errors.New("message cannot be empty")

	// ErrInvalidPriority indicates that the priority value is outside the valid range.
	ErrInvalidPriority = errors.New("priority must be between -2 and 10")

	// ErrInvalidDate indicates that the date format is invalid.
	ErrInvalidDate = errors.New("invalid date format")

	// ErrExtrasUnmarshalFailed indicates failure to unmarshal extras JSON.
	ErrExtrasUnmarshalFailed = errors.New("failed to unmarshal extras JSON")

	// ErrExtrasParseFailed indicates failure to parse extras JSON from URL query.
	ErrExtrasParseFailed = errors.New("failed to parse extras JSON from URL query")
)

// HTTP/Communication errors.
var (
	// ErrUnexpectedStatus indicates an unexpected HTTP response status.
	ErrUnexpectedStatus = errors.New("got unexpected HTTP status")

	// ErrSendFailed indicates failure to send notification to Gotify.
	ErrSendFailed = errors.New("failed to send notification to Gotify")

	// ErrMarshalRequest indicates failure to marshal request.
	ErrMarshalRequest = errors.New("failed to marshal request")

	// ErrCreateRequest indicates failure to create HTTP request.
	ErrCreateRequest = errors.New("failed to create HTTP request")

	// ErrSendRequest indicates failure to send HTTP request.
	ErrSendRequest = errors.New("failed to send HTTP request")

	// ErrReadResponse indicates failure to read HTTP response.
	ErrReadResponse = errors.New("failed to read HTTP response")

	// ErrParseResponse indicates failure to parse HTTP response.
	ErrParseResponse = errors.New("failed to parse HTTP response")
)

// Configuration errors.
var (
	// ErrConfigUpdateFailed indicates failure to update configuration.
	ErrConfigUpdateFailed = errors.New("failed to update configuration")

	// ErrConfigPropertyFailed indicates failure with a configuration property.
	ErrConfigPropertyFailed = errors.New("failed to set configuration property")
)
