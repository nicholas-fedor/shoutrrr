package failures

import (
	"errors"
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// testID is the failure ID shared by the tests.
const testID FailureID = 42

// errUnderlying is the error the tests wrap in a failure.
var errUnderlying = errors.New("underlying error")

// TestWrap verifies that Wrap builds a failure with the given ID, formats the
// message when arguments are given, and joins the message with the wrapped
// error's message.
func TestWrap(t *testing.T) {
	t.Parallel()

	// tests lists Wrap inputs with the error text and wrapped error the failure
	// should report.
	tests := []struct {
		name        string
		message     string
		wrapped     error
		args        []any
		wantMessage string
	}{
		{name: "message only", message: "test failure occurred", wantMessage: "test failure occurred"},
		{name: "wrapped error", message: "test failure occurred", wrapped: errUnderlying, wantMessage: "test failure occurred: underlying error"},
		{name: "format arguments", message: "test failure %d", args: []any{123}, wantMessage: "test failure 123"},
		{name: "empty message", wrapped: errUnderlying, wantMessage: ": underlying error"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			failure := Wrap(tt.message, testID, tt.wrapped, tt.args...)

			require.EqualError(t, failure, tt.wantMessage)
			assert.Equal(t, testID, failure.ID())
			assert.Equal(t, tt.wrapped, failure.Unwrap())
		})
	}
}

// TestFailureIs verifies that a failure matches another failure only when their
// IDs are equal, and never matches an error that is not a failure.
func TestFailureIs(t *testing.T) {
	t.Parallel()

	// tests lists targets compared against a failure with testID.
	tests := []struct {
		name   string
		target error
		want   bool
	}{
		{name: "same ID", target: Wrap("second", testID, nil), want: true},
		{name: "different ID", target: Wrap("second", testID+1, nil), want: false},
		{name: "not a failure", target: errUnderlying, want: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			assert.Equal(t, tt.want, Wrap("first", testID, nil).Is(tt.target))
		})
	}
}

// TestFailureChain verifies that nested failures join their messages and that
// the standard errors functions walk the chain by wrapped error and by ID.
func TestFailureChain(t *testing.T) {
	t.Parallel()

	middle := Wrap("middle", testID+1, errUnderlying)
	outer := Wrap("outer", testID, middle)

	require.EqualError(t, outer, "outer: middle: underlying error")
	assert.Equal(t, middle, outer.Unwrap())
	require.ErrorIs(t, outer, errUnderlying)
	require.ErrorIs(t, outer, Wrap("any message", testID+1, nil), "a failure in the chain matches by ID")
	assert.NotErrorIs(t, outer, Wrap("any message", testID+2, nil))
}

// TestFailureInStandardWrapping verifies that a failure wrapped with fmt.Errorf
// keeps its message, can be unwrapped, and still matches by ID and as a Failure.
func TestFailureInStandardWrapping(t *testing.T) {
	t.Parallel()

	failure := Wrap("failure", testID, nil)
	wrapped := fmt.Errorf("additional context: %w", failure)

	require.EqualError(t, wrapped, "additional context: failure")
	assert.Equal(t, failure, errors.Unwrap(wrapped))
	require.ErrorIs(t, wrapped, Wrap("other message", testID, nil))

	found, ok := errors.AsType[Failure](wrapped)
	require.True(t, ok)
	assert.Equal(t, testID, found.ID())
}
