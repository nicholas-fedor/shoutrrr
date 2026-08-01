package types

import "fmt"

// TargetError wraps a failure from one configured notification target.
//
// It is returned by ServiceRouter.Send, SendAsync, and SendItems in the
// per-service error positions. Callers can use errors.As to recover the
// *TargetError and read the URL/ID of the failed target, and errors.Unwrap
// to reach the underlying error.
type TargetError struct {
	// URL is the service URL or identifier that failed.
	URL string
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
