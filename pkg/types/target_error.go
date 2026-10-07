package types

import "fmt"

// TargetError wraps a failure from one configured notification target.
//
// It is returned by ServiceRouter.Send, SendAsync, and SendItems. Callers can use
// errors.As to recover the *TargetError, read the identifier and position of the
// failed target, and use errors.Unwrap to reach the underlying error.
type TargetError struct {
	// URL identifies the failed target by its service ID, such as "telegram".
	// It never contains the service URL, so the error is safe to log.
	URL string
	// Index is the position of the failed target among the router's configured
	// URLs. It identifies the target when several share a service ID, and when
	// results arrive in completion order from SendAsync.
	Index int
	// Err is the underlying error from the service.
	Err error
}

// Error returns the formatted error message including the target URL.
//
// Returns:
//   - string: the formatted error message.
func (e *TargetError) Error() string {
	return fmt.Sprintf("%s: %v", e.URL, e.Err)
}

// Unwrap returns the underlying error wrapped by TargetError.
//
// Returns:
//   - error: the wrapped error.
func (e *TargetError) Unwrap() error {
	return e.Err
}
