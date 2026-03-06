package join

import "errors"

// TODO: Cleanup

// ErrorMessage for error events within the join service.
type ErrorMessage string

const (
	// APIKeyMissing should be used when a config URL is missing a token.
	//nolint:gosec // Error message constant, not a hardcoded credential.
	APIKeyMissing ErrorMessage = "API key missing from config URL"

	// DevicesMissing should be used when a config URL is missing devices.
	DevicesMissing ErrorMessage = "devices missing from config URL"
)

// ErrSendFailed indicates a failure to send a notification to Join devices.
var ErrSendFailed = errors.New("failed to send notification to join devices")

// ErrDevicesMissing indicates that no devices are specified in the configuration.
var (
	ErrDevicesMissing = errors.New("devices missing from config URL")
	ErrAPIKeyMissing  = errors.New("API key missing from config URL")
)
