// Package cmd provides exit codes and result types for the Shoutrrr CLI.
package cmd

// ExitError contains the final exit message and code for a CLI session.
type ExitError struct {
	// ExitCode is the numeric exit code returned to the operating system.
	ExitCode int
	// Message is the human-readable error message.
	Message string
}

const (
	// ExSuccess is the exit code that signals that everything went as expected.
	ExSuccess = 0
	// ExUsage is the exit code that signals that the application was not started with the correct arguments.
	ExUsage = 64
	// ExUnavailable is the exit code that signals that the application failed to perform the intended task.
	ExUnavailable = 69
	// ExConfig is the exit code that signals that the task failed due to a configuration error.
	ExConfig = 78
)

// ErrNil is the empty ExitError that is used whenever the command ran successfully.
var ErrNil = ExitError{
	ExitCode: 0,
	Message:  "",
}

// Error returns the error message for the ExitError.
// This implements the error interface.
func (e ExitError) Error() string {
	return e.Message
}

// InvalidUsage returns an ExitError with the exit code ExUsage.
//
// Parameters:
//   - message: The error message describing the invalid usage.
//
// Returns:
//   - ExitError: An ExitError with ExitCode set to ExUsage.
func InvalidUsage(message string) ExitError {
	return ExitError{
		ExUsage,
		message,
	}
}

// TaskUnavailable returns an ExitError with the exit code ExUnavailable.
//
// Parameters:
//   - message: The error message describing why the task was unavailable.
//
// Returns:
//   - ExitError: An ExitError with ExitCode set to ExUnavailable.
func TaskUnavailable(message string) ExitError {
	return ExitError{
		ExUnavailable,
		message,
	}
}

// ConfigurationError returns an ExitError with the exit code ExConfig.
//
// Parameters:
//   - message: The error message describing the configuration error.
//
// Returns:
//   - ExitError: An ExitError with ExitCode set to ExConfig.
func ConfigurationError(message string) ExitError {
	return ExitError{
		ExConfig,
		message,
	}
}
