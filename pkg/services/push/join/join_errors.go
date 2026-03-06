package join

// ErrorMessage for error events within the join service.
type ErrorMessage string

const (
	// APIKeyMissing should be used when a config URL is missing a token.
	//nolint:gosec // Error message constant, not a hardcoded credential.
	APIKeyMissing ErrorMessage = "API key missing from config URL"

	// DevicesMissing should be used when a config URL is missing devices.
	DevicesMissing ErrorMessage = "devices missing from config URL"
)
