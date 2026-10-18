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
// appends a formatted detail.
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
}

// TestIsTestSetupFailure verifies that only a [FailTestSetup] failure is
// reported as a test setup failure, with a message that carries its cause.
func TestIsTestSetupFailure(t *testing.T) {
	t.Parallel()

	msg, isSetupFailure := IsTestSetupFailure(Failure(FailTestSetup, errors.New("setup issue")))
	assert.True(t, isSetupFailure)
	assert.Contains(t, msg, "setup issue")

	msg, isSetupFailure = IsTestSetupFailure(Failure(FailParseURL, errCause))
	assert.False(t, isSetupFailure)
	assert.Empty(t, msg)

	msg, isSetupFailure = IsTestSetupFailure(nil)
	assert.False(t, isSetupFailure)
	assert.Empty(t, msg)
}
