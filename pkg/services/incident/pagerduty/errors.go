package pagerduty

import "errors"

var (
	// errInvalidIntegrationKey is returned when the integration key format is invalid.
	errInvalidIntegrationKey = errors.New(
		"invalid integration key format: must be a 32-character hexadecimal string",
	)
	// errMissingIntegrationKey is returned when the integration key is missing from the URL path.
	errMissingIntegrationKey = errors.New(
		"integration key is missing from URL path",
	)

	// errInvalidContextFormat is returned when a context string does not match the expected 'type:value' format.
	errInvalidContextFormat = errors.New(
		"invalid context format, expected 'type:value'",
	)

	// errEmptyContextTypeOrValue is returned when a context type or value is empty.
	errEmptyContextTypeOrValue = errors.New(
		"invalid context format, type and value cannot be empty",
	)

	// errInvalidSeverity is returned when the severity value is not one of the allowed values.
	errInvalidSeverity = errors.New(
		"invalid severity: must be one of 'critical', 'error', 'warning', or 'info'",
	)

	// errInvalidEventAction is returned when the event action is not one of the allowed values.
	errInvalidEventAction = errors.New(
		"invalid event action: must be one of 'trigger', 'acknowledge', or 'resolve'",
	)

	// errInvalidContextType is returned when a context type is not one of the allowed values for JSON format.
	errInvalidContextType = errors.New(
		"invalid context type: must be one of 'link' or 'image'",
	)
)
