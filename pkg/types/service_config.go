package types

import "net/url"

// Enummer contains fields that have associated EnumFormatter instances.
type Enummer interface {
	// Enums returns the formatters of the config's enum fields.
	//
	// Returns:
	//   - map[string]EnumFormatter: the formatters, keyed by Go struct field
	//     name.
	Enums() map[string]EnumFormatter
}

// ServiceConfig is the common interface for all types of service configurations.
type ServiceConfig interface {
	// Enummer reports the formatters of the config's enum fields.
	Enummer
	// GetURL returns the service URL that represents the config.
	//
	// Returns:
	//   - *url.URL: the service URL.
	GetURL() *url.URL
	// SetURL updates the config from a service URL.
	//
	// Parameters:
	//   - serviceURL: the service URL to read.
	//
	// Returns:
	//   - error: an error when serviceURL is not a valid URL for the service.
	SetURL(serviceURL *url.URL) error
}

// ConfigQueryResolver is the interface used to get/set and list service config query fields.
type ConfigQueryResolver interface {
	// Get returns the formatted value of the config prop for a query key.
	//
	// Parameters:
	//   - key: the query key.
	//
	// Returns:
	//   - value: the formatted value.
	//   - err: the failure, such as an unknown key, or nil on success.
	Get(key string) (value string, err error)
	// Set parses a value and sets the config prop for a query key.
	//
	// Parameters:
	//   - key: the query key.
	//   - value: the formatted value.
	//
	// Returns:
	//   - error: the failure, such as an unknown key or an invalid value.
	Set(key, value string) error
	// QueryFields returns the query keys of the config props.
	//
	// Returns:
	//   - []string: the keys that Get and Set accept.
	QueryFields() []string
}
