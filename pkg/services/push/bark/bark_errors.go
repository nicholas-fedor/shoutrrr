package bark

import "errors"

// Error definitions for the Bark service.
var (
	// ErrFailedAPIRequest indicates the HTTP request to the Bark API failed.
	ErrFailedAPIRequest = errors.New("failed to make API request")

	// ErrUnexpectedStatus indicates an unexpected HTTP response status code.
	ErrUnexpectedStatus = errors.New("unexpected status code")

	// ErrUpdateParamsFailed indicates failure to update configuration from parameters.
	ErrUpdateParamsFailed = errors.New("failed to update config from params")

	// ErrMissingDeviceKey indicates the DeviceKey is required but was not provided.
	ErrMissingDeviceKey = errors.New("device key is required but not specified in the configuration")

	// ErrMissingHost indicates the Host is required but was not provided.
	ErrMissingHost = errors.New("host is required but not specified in the configuration")
)
