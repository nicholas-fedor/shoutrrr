package matrix

import "errors"

// Error definitions for the Matrix service.
//
// These errors are returned by various methods in the Matrix service package
// to indicate specific failure conditions during configuration, authentication,
// and message sending operations.
var (
	// ErrMissingHost is returned when the Matrix server host is not provided
	// in the service configuration URL.
	ErrMissingHost = errors.New("host is required")

	// ErrMissingCredentials is returned when neither a password nor an access
	// token is provided in the service configuration URL.
	ErrMissingCredentials = errors.New("password or access token is required")

	// ErrClientNotInitialized is returned when attempting to send a message
	// but the Matrix client has not been properly initialized.
	ErrClientNotInitialized = errors.New("client not initialized; cannot send message")

	// ErrUnsupportedLoginFlows is returned when none of the login flows
	// supported by the server are compatible with this client.
	ErrUnsupportedLoginFlows = errors.New("none of the server login flows are supported")

	// ErrUnexpectedStatus is returned when the Matrix API returns an HTTP
	// status code that was not expected by the client.
	ErrUnexpectedStatus = errors.New("unexpected HTTP status")
)
