package dingding

import "errors"

// Error variables for the Dingding package.
var (
	// ErrInvalidKind indicates an invalid service kind was provided in the configuration.
	ErrInvalidKind = errors.New("invalid service kind, must be either 'custombot' or 'worknotice'")

	// ErrMissingCred indicates the credentials are missing from the configuration.
	ErrMissingCred = errors.New("credentials missing from config URL")

	// ErrMissingUserIDs indicates that user IDs are required but missing from the configuration.
	ErrMissingUserIDs = errors.New("userids are required but missing from config URL")

	// ErrTemplate indicates an error occurred while creating the message payload from the template.
	ErrTemplate = errors.New("error creating message payload from template")

	// ErrResponseStatusFailure indicates a non-successful HTTP response status code was received from the Dingding API.
	ErrResponseStatusFailure = errors.New("received non-successful response status code from Dingding API")
)
