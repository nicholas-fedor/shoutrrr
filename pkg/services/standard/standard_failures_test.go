package standard

import (
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/nicholas-fedor/shoutrrr/internal/failures"
)

// errCause is the underlying error the failure tests wrap.
var errCause = errors.New("invalid URL")

// TestFailure verifies that Failure wraps the error with the message for its
// ID, falls back to the unknown error message for an ID without one, and
// appends a detail: a format string with its arguments, a single string as
// is, or other values separated by spaces.
func TestFailure(t *testing.T) {
	t.Parallel()

	// tests lists failure IDs and args with the message each failure should
	// carry.
	tests := []struct {
		name string
		args []any
		id   failures.FailureID
		want string
	}{
		{name: "parse URL", id: FailParseURL, want: "error parsing Service URL: invalid URL"},
		{name: "test setup", id: FailTestSetup, want: "test setup failed: invalid URL"},
		{name: "unknown", id: FailUnknown, want: "an unknown error occurred: invalid URL"},
		{name: "unrecognized ID", id: failures.FailureID(999), want: "an unknown error occurred: invalid URL"},
		{name: "service init", id: FailServiceInit, want: "error initializing service: invalid URL"},
		{
			name: "string detail",
			id:   FailParseURL,
			args: []any{"invalid URL format"},
			want: "error parsing Service URL invalid URL format: invalid URL",
		},
		{
			name: "string detail with a percent sign",
			id:   FailParseURL,
			args: []any{"90% done"},
			want: "error parsing Service URL 90% done: invalid URL",
		},
		{
			name: "value details",
			id:   FailParseURL,
			args: []any{42, true},
			want: "error parsing Service URL 42 true: invalid URL",
		},
		{
			name: "value and string details",
			id:   FailParseURL,
			args: []any{404, "not found"},
			want: "error parsing Service URL 404 not found: invalid URL",
		},
		{
			name: "formatted detail",
			id:   FailParseURL,
			args: []any{"extra info: %s", "details"},
			want: "error parsing Service URL extra info: details: invalid URL",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			failure := Failure(tt.id, errCause, tt.args...)

			assert.Equal(t, tt.id, failure.ID())
			require.EqualError(t, failure, tt.want)
			require.ErrorIs(t, failure, errCause)
		})
	}

	require.EqualError(t, Failure(FailParseURL, nil, "invalid URL format"), "error parsing Service URL invalid URL format")
}

// TestIsTestSetupFailure verifies that only a [FailTestSetup] failure is
// reported as a test setup failure, with a message that starts with the test
// setup prefix once and carries its cause.
func TestIsTestSetupFailure(t *testing.T) {
	t.Parallel()

	msg, isSetupFailure := IsTestSetupFailure(Failure(FailTestSetup, errors.New("setup issue")))
	assert.True(t, isSetupFailure)
	assert.Equal(t, "test setup failed: setup issue", msg)

	msg, isSetupFailure = IsTestSetupFailure(failures.Wrap("fixture missing", FailTestSetup, errors.New("setup issue")))
	assert.True(t, isSetupFailure)
	assert.Equal(t, "test setup failed: fixture missing: setup issue", msg)

	msg, isSetupFailure = IsTestSetupFailure(Failure(FailParseURL, errCause))
	assert.False(t, isSetupFailure)
	assert.Empty(t, msg)

	msg, isSetupFailure = IsTestSetupFailure(nil)
	assert.False(t, isSetupFailure)
	assert.Empty(t, msg)
}
