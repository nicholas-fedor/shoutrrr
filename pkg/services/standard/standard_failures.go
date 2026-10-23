package standard

import (
	"fmt"
	"strings"

	"github.com/nicholas-fedor/shoutrrr/internal/failures"
)

// failureLike is the failure that [IsTestSetupFailure] inspects.
//
// It is unexported, so a generated mock of it could not be used outside this
// package, and the directive below stops mockery from generating one.
//
//mockery:generate: false
type failureLike interface {
	failures.Failure
}

const (
	// FailTestSetup is the FailureID used to represent an error that is part of the setup for tests.
	FailTestSetup failures.FailureID = -1
	// FailParseURL is the FailureID used to represent failing to parse the service URL.
	FailParseURL failures.FailureID = -2
	// FailServiceInit is the FailureID used to represent failure of a service.Initialize method.
	FailServiceInit failures.FailureID = -3
)

// FailUnknown is the default FailureID.
const FailUnknown failures.FailureID = iota

// testSetupFailedMessage is the message of a [FailTestSetup] failure.
const testSetupFailedMessage = "test setup failed"

// Failure creates a Failure instance corresponding to the provided failureID, wrapping the provided error.
//
// The message is the one for failureID, or the unknown error message for an
// ID without one, followed by a detail from args when any are given. A string
// first argument followed by others is a format string for them, a single
// string is appended as is, and other values are appended separated by spaces.
//
// Parameters:
//   - failureID: the ID that selects the message.
//   - err: the error to wrap, which may be nil.
//   - args: an optional detail for the message.
//
// Returns:
//   - failures.Failure: the failure with failureID, wrapping err.
func Failure(failureID failures.FailureID, err error, args ...any) failures.Failure {
	messages := map[int]string{
		int(FailTestSetup):   testSetupFailedMessage,
		int(FailParseURL):    "error parsing Service URL",
		int(FailServiceInit): "error initializing service",
		int(FailUnknown):     "an unknown error occurred",
	}

	msg := messages[int(failureID)]
	if msg == "" {
		msg = messages[int(FailUnknown)]
	}

	if len(args) > 0 {
		msg += " " + failureDetail(args)
	}

	return failures.Wrap(msg, failureID, err)
}

// IsTestSetupFailure checks whether the given failure is due to the test setup being broken.
//
// Parameters:
//   - failure: the failure to check, which may be nil.
//
// Returns:
//   - string: the failure message, starting with the test setup prefix once, or
//     "" when failure is not a test setup failure.
//   - bool: true for a [FailTestSetup] failure.
func IsTestSetupFailure(failure failureLike) (string, bool) {
	if failure == nil || failure.ID() != FailTestSetup {
		return "", false
	}

	msg := failure.Error()
	if !strings.HasPrefix(msg, testSetupFailedMessage) {
		msg = testSetupFailedMessage + ": " + msg
	}

	return msg, true
}

// failureDetail formats the detail [Failure] appends to its message.
//
// Parameters:
//   - args: a format string and its arguments, a single string, or other values.
//
// Returns:
//   - string: the formatted detail.
func failureDetail(args []any) string {
	format, isString := args[0].(string)

	switch {
	case isString && len(args) > 1:
		return fmt.Sprintf(format, args[1:]...)
	case isString:
		return format
	default:
		return strings.TrimSuffix(fmt.Sprintln(args...), "\n")
	}
}
