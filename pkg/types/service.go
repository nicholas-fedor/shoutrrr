package types

import (
	"net/url"
)

// Service is the public common interface for all notification services.
type Service interface {
	// Sender sends notifications.
	Sender
	// Templater manages the service's message templates.
	Templater
	// Initialize configures the service from a service URL and sets its
	// logger. It must run before the service sends.
	//
	// Parameters:
	//   - serviceURL: the service URL to read the config from.
	//   - logger: the logger for service output.
	//
	// Returns:
	//   - error: an error when serviceURL is not a valid URL for the service.
	Initialize(serviceURL *url.URL, logger StdLogger) error
	// SetLogger sets the logger for service output.
	//
	// Parameters:
	//   - logger: the logger to write to.
	SetLogger(logger StdLogger)
	// GetID returns the identifier of the service, such as "telegram".
	//
	// Returns:
	//   - string: the service identifier.
	GetID() string
}
