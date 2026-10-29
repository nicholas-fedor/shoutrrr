package sequential

import (
	"context"
	"errors"
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestSend verifies that every item is sent in order after a failure, and
// that every failure is returned.
func TestSend(t *testing.T) {
	t.Parallel()

	errB := errors.New("b failed")
	errD := errors.New("d failed")

	var sent []string

	err := Send(t.Context(), []string{"a", "b", "c", "d"}, func(_ context.Context, item string) error {
		sent = append(sent, item)

		switch item {
		case "b":
			return errB
		case "d":
			return errD
		default:
			return nil
		}
	})

	assert.Equal(t, []string{"a", "b", "c", "d"}, sent)
	require.ErrorIs(t, err, errB)
	require.ErrorIs(t, err, errD)
	assert.Len(t, unwrapAll(t, err), 2)
}

// TestSendSuccess verifies that sending every item, or no items, returns
// nil.
func TestSendSuccess(t *testing.T) {
	t.Parallel()

	succeed := func(context.Context, int) error { return nil }

	require.NoError(t, Send(t.Context(), []int{1, 2}, succeed))
	require.NoError(t, Send(t.Context(), nil, succeed))
}

// TestSendContextEnds verifies that a context that ends stops the remaining
// items, and that its error is reported once.
func TestSendContextEnds(t *testing.T) {
	t.Parallel()

	// tests lists how the item that sees the context end reports it.
	tests := []struct {
		send func(ctx context.Context, cancel context.CancelFunc) error
		name string
		want int
	}{
		{
			name: "item returns the context's error",
			send: func(ctx context.Context, cancel context.CancelFunc) error {
				cancel()

				return fmt.Errorf("sending: %w", ctx.Err())
			},
			want: 1,
		},
		{
			name: "item succeeds",
			send: func(_ context.Context, cancel context.CancelFunc) error {
				cancel()

				return nil
			},
			want: 1,
		},
		{
			name: "item fails with another error",
			send: func(_ context.Context, cancel context.CancelFunc) error {
				cancel()

				return errors.New("rejected")
			},
			want: 2,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			ctx, cancel := context.WithCancel(t.Context())
			t.Cleanup(cancel)

			var sent int

			err := Send(ctx, []string{"a", "b", "c"}, func(ctx context.Context, _ string) error {
				sent++

				return tt.send(ctx, cancel)
			})

			assert.Equal(t, 1, sent)
			require.ErrorIs(t, err, context.Canceled)
			assert.Len(t, unwrapAll(t, err), tt.want)
		})
	}
}

// TestSendCanceledBeforeStart verifies that a context that ended before the
// first item sends none and reports its error.
func TestSendCanceledBeforeStart(t *testing.T) {
	t.Parallel()

	ctx, cancel := context.WithCancel(t.Context())
	cancel()

	err := Send(ctx, []string{"a"}, func(context.Context, string) error {
		t.Error("send called after the context ended")

		return nil
	})

	require.ErrorIs(t, err, context.Canceled)
}

// unwrapAll returns the errors that err joins.
//
// Parameters:
//   - t: the test.
//   - err: an error made by errors.Join.
//
// Returns:
//   - []error: the joined errors.
func unwrapAll(t *testing.T, err error) []error {
	t.Helper()

	var joined interface{ Unwrap() []error }

	require.ErrorAs(t, err, &joined)

	return joined.Unwrap()
}
