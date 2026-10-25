package types

import "log"

// ServiceOpts is the interface describing the service options.
type ServiceOpts interface {
	// Verbose reports whether verbose output is enabled.
	//
	// Returns:
	//   - bool: true when verbose output is enabled.
	Verbose() bool
	// Logger returns the logger for service output.
	//
	// Returns:
	//   - *log.Logger: the logger.
	Logger() *log.Logger
	// Props returns the service props, keyed by name.
	//
	// Returns:
	//   - map[string]string: the props.
	Props() map[string]string
}
