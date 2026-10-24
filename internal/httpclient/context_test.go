package httpclient

import (
	"context"
	"testing"
	"testing/synctest"
	"time"

	"github.com/stretchr/testify/assert"

	"github.com/nicholas-fedor/shoutrrr/pkg/types"
)

// TestBoundContext verifies that a context with a deadline is returned as it
// is. A context without one gets the budget, or the default send timeout when
// the budget is not positive.
func TestBoundContext(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name   string
		budget time.Duration
		want   time.Duration
	}{
		{name: "budget", budget: 30 * time.Second, want: 30 * time.Second},
		{name: "zero budget", budget: 0, want: types.DefaultSendTimeout},
		{name: "negative budget", budget: -time.Second, want: types.DefaultSendTimeout},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			synctest.Test(t, func(t *testing.T) {
				ctx, cancel := BoundContext(t.Context(), tt.budget)
				defer cancel()

				deadline, ok := ctx.Deadline()
				assert.True(t, ok, "the context should have a deadline")
				assert.Equal(t, tt.want, time.Until(deadline))
			})
		})
	}

	t.Run("caller deadline kept", func(t *testing.T) {
		t.Parallel()

		parent, cancelParent := context.WithTimeout(t.Context(), time.Hour)
		defer cancelParent()

		ctx, cancel := BoundContext(parent, time.Second)
		defer cancel()

		assert.Equal(t, parent, ctx)

		cancel()
		assert.NoError(t, ctx.Err(), "cancel should not end the caller's context")
	})
}
