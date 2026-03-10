package join

import "errors"

// ErrSendFailed indicates a failure to send a notification to Join devices.
var ErrSendFailed = errors.New("failed to send notification to join devices")

// ErrDevicesMissing indicates that no devices are specified in the configuration.
var ErrDevicesMissing = errors.New("devices missing from config URL")

// ErrAPIKeyMissing indicates that the API key is missing from the configuration.
var ErrAPIKeyMissing = errors.New("API key missing from config URL")
