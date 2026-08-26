package signalgrid

import "errors"

var (
	// ErrClientKeyMissing indicates that the Signalgrid client key is absent from the URL.
	ErrClientKeyMissing = errors.New("signalgrid client key is missing")

	// ErrChannelMissing indicates that the Signalgrid channel token is absent from the URL.
	ErrChannelMissing = errors.New("signalgrid channel token is missing")

	// ErrSendFailed indicates that sending a Signalgrid notification failed.
	ErrSendFailed = errors.New("failed to send signalgrid notification")

	// ErrUnexpectedStatus indicates that the Signalgrid API returned a non-success HTTP status.
	ErrUnexpectedStatus = errors.New("signalgrid API returned unexpected status")
)
