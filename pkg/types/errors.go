package types

import "errors"

// Errors shared by every service.
var (
	// ErrServiceNotInitialized indicates a send on a service whose Initialize has
	// not run, so the service has no configuration to send with.
	ErrServiceNotInitialized = errors.New("service not initialized")
	// ErrNilHTTPClient indicates a request through an injected HTTP client that
	// is a nil pointer or another nil value of a type other than *http.Client,
	// and that panicked with a runtime error, such as a nil pointer
	// dereference. A nil *http.Client selects the service's default client
	// instead.
	ErrNilHTTPClient = errors.New("injected HTTP client is nil")
)
