package sequential

import (
	"context"
	"errors"
	"fmt"
	"testing"
	"testing/synctest"
	"time"

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

	err := Send(t.Context(), []string{"a", "b", "c", "d"}, SharedDeadline,
		func(_ context.Context, item string) error {
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

	require.NoError(t, Send(t.Context(), []int{1, 2}, SharedDeadline, succeed))
	require.NoError(t, Send(t.Context(), nil, SharedDeadline, succeed))
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

			err := Send(ctx, []string{"a", "b", "c"}, SharedDeadline,
				func(ctx context.Context, _ string) error {
					sent++

					return tt.send(ctx, cancel)
				})

			assert.Equal(t, 1, sent)
			require.ErrorIs(t, err, context.Canceled)
			assert.Len(t, unwrapAll(t, err), tt.want)
		})
	}
}

// TestSendContextEndsAfterLastItem verifies that a context that ends during
// the last item's request does not fail a send whose every item was sent.
func TestSendContextEndsAfterLastItem(t *testing.T) {
	t.Parallel()

	ctx, cancel := context.WithCancel(t.Context())
	t.Cleanup(cancel)

	err := Send(ctx, []string{"a", "b"}, SharedDeadline,
		func(_ context.Context, item string) error {
			if item == "b" {
				cancel()
			}

			return nil
		})

	require.NoError(t, err)
}

// TestSendCanceledBeforeStart verifies that a context that ended before the
// first item sends none and reports its error.
func TestSendCanceledBeforeStart(t *testing.T) {
	t.Parallel()

	ctx, cancel := context.WithCancel(t.Context())
	cancel()

	err := Send(ctx, []string{"a"}, SharedDeadline, func(context.Context, string) error {
		t.Error("send called after the context ended")

		return nil
	})

	require.ErrorIs(t, err, context.Canceled)
}

// TestSendSharedDeadline verifies that SharedDeadline, and the zero Policy,
// give every item the send's context as it is, so the items share one
// deadline.
func TestSendSharedDeadline(t *testing.T) {
	t.Parallel()

	ctx, cancel := context.WithTimeout(t.Context(), time.Hour)
	defer cancel()

	for _, policy := range []Policy{SharedDeadline, {}} {
		var contexts []context.Context

		err := Send(ctx, []string{"a", "b"}, policy, func(itemCtx context.Context, _ string) error {
			contexts = append(contexts, itemCtx)

			return nil
		})

		require.NoError(t, err)
		assert.Equal(t, []context.Context{ctx, ctx}, contexts)
	}
}

// TestPerTargetHungTarget verifies that a target that does not answer fails
// when its share of the time runs out, with its own error, and that the next
// target is still sent.
func TestPerTargetHungTarget(t *testing.T) {
	t.Parallel()

	synctest.Test(t, func(t *testing.T) {
		ctx, cancel := context.WithTimeout(t.Context(), 20*time.Second)
		defer cancel()

		var sent []string

		start := time.Now()

		err := Send(ctx, []string{"hung", "next"}, PerTarget(10*time.Second),
			func(ctx context.Context, target string) error {
				sent = append(sent, target)

				if target == "hung" {
					<-ctx.Done()

					return fmt.Errorf("sending to %s: %w", target, ctx.Err())
				}

				return nil
			})

		require.ErrorIs(t, err, context.DeadlineExceeded)
		assert.Equal(t, "sending to hung: context deadline exceeded", err.Error())
		assert.Equal(t, []string{"hung", "next"}, sent)
		assert.Equal(t, 10*time.Second, time.Since(start))
	})
}

// TestPerTargetShares verifies that each target may use an equal share of
// the time left when that is longer than its budget, that time a target does
// not use carries over, and that the last target has all the time left.
func TestPerTargetShares(t *testing.T) {
	t.Parallel()

	synctest.Test(t, func(t *testing.T) {
		ctx, cancel := context.WithTimeout(t.Context(), 30*time.Second)
		defer cancel()

		var shares []time.Duration

		err := Send(ctx, []string{"a", "b", "c"}, PerTarget(time.Second),
			func(ctx context.Context, _ string) error {
				deadline, ok := ctx.Deadline()
				require.True(t, ok)

				shares = append(shares, time.Until(deadline))

				time.Sleep(3 * time.Second)

				return nil
			})

		require.NoError(t, err)
		assert.Equal(t,
			[]time.Duration{10 * time.Second, 13500 * time.Millisecond, 24 * time.Second},
			shares)
	})
}

// TestPerTargetShortDeadline verifies that a deadline too short for every
// target still gives each target at least its budget, so targets are sent one
// after another until the time runs out. Equal shares of 200ms would fail
// every 240ms request.
func TestPerTargetShortDeadline(t *testing.T) {
	t.Parallel()

	synctest.Test(t, func(t *testing.T) {
		ctx, cancel := context.WithTimeout(t.Context(), time.Second)
		defer cancel()

		var delivered int

		err := Send(ctx, []int{1, 2, 3, 4, 5}, PerTarget(300*time.Millisecond),
			func(ctx context.Context, _ int) error {
				select {
				case <-time.After(240 * time.Millisecond):
					delivered++

					return nil
				case <-ctx.Done():
					return ctx.Err()
				}
			})

		require.ErrorIs(t, err, context.DeadlineExceeded)
		assert.Equal(t, 4, delivered)
	})
}

// TestPerTargetLastTarget verifies that the last target, which has all the
// time left, keeps the send's deadline and ends with its error, reported once.
func TestPerTargetLastTarget(t *testing.T) {
	t.Parallel()

	synctest.Test(t, func(t *testing.T) {
		ctx, cancel := context.WithTimeout(t.Context(), 20*time.Second)
		defer cancel()

		start := time.Now()

		err := Send(ctx, []string{"quick", "hung"}, PerTarget(time.Second),
			func(ctx context.Context, target string) error {
				if target == "hung" {
					<-ctx.Done()

					return ctx.Err()
				}

				return nil
			})

		require.ErrorIs(t, err, context.DeadlineExceeded)
		assert.Len(t, unwrapAll(t, err), 1, "the context's error is reported once")
		assert.Equal(t, 20*time.Second, time.Since(start))
	})
}

// TestPerTargetEndsAfterTargetTimeout verifies that the send's context ending
// is reported even when an earlier target failed with the same sentinel when
// its own share ran out. The second target overruns its share, so the third is
// skipped, and the send's deadline error is the only sign of that.
func TestPerTargetEndsAfterTargetTimeout(t *testing.T) {
	t.Parallel()

	synctest.Test(t, func(t *testing.T) {
		ctx, cancel := context.WithTimeout(t.Context(), 30*time.Second)
		defer cancel()

		var sent []string

		err := Send(ctx, []string{"hung", "slow", "skipped"}, PerTarget(time.Second),
			func(ctx context.Context, target string) error {
				sent = append(sent, target)

				switch target {
				case "hung":
					<-ctx.Done()

					return fmt.Errorf("sending to %s: %w", target, ctx.Err())
				case "slow":
					time.Sleep(25 * time.Second)

					return nil
				default:
					return nil
				}
			})

		require.ErrorIs(t, err, context.DeadlineExceeded)
		assert.Equal(t, []string{"hung", "slow"}, sent)
		assert.Len(t, unwrapAll(t, err), 2,
			"the target's error and the send's error are both reported")
	})
}

// TestPerTargetUnshared verifies that a single target keeps the send's
// context, and that targets get no deadline when the send has none.
func TestPerTargetUnshared(t *testing.T) {
	t.Parallel()

	ctx, cancel := context.WithTimeout(t.Context(), time.Hour)
	defer cancel()

	err := Send(ctx, []string{"only"}, PerTarget(time.Second),
		func(itemCtx context.Context, _ string) error {
			assert.Equal(t, ctx, itemCtx)

			return nil
		})
	require.NoError(t, err)

	err = Send(t.Context(), []string{"a", "b"}, PerTarget(time.Second),
		func(itemCtx context.Context, _ string) error {
			_, ok := itemCtx.Deadline()
			assert.False(t, ok)

			return nil
		})
	require.NoError(t, err)
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
