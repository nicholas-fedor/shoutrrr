package router

import (
	"context"
	"errors"
	"fmt"
	"testing"
	"testing/synctest"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	"github.com/nicholas-fedor/shoutrrr/pkg/types"
	"github.com/nicholas-fedor/shoutrrr/pkg/types/mocks"
)

// budgetMock is a service assembled from generated mocks.
type budgetMock struct {
	*mocks.MockService
	*mocks.MockContextSender
	*mocks.MockServiceTimeout
}

// pastDeadlineContext is a context whose deadline has passed but whose Done
// channel never closes, so a result arriving with an expired budget is handled
// by the result branch of awaitResult rather than racing the Done branch.
type pastDeadlineContext struct {
	context.Context //nolint:containedctx // The test context wraps one to change its deadline.

	// deadline is the deadline the context reports.
	deadline time.Time
}

func newBudgetMock(t *testing.T, id string, block func(context.Context) error) *budgetMock {
	t.Helper()

	service := mocks.NewMockService(t)
	sender := mocks.NewMockContextSender(t)
	timeout := mocks.NewMockServiceTimeout(t)

	service.EXPECT().Initialize(mock.Anything, mock.Anything).Return(nil)
	service.EXPECT().GetID().Return(id)
	sender.EXPECT().SendContext(mock.Anything, mock.Anything, mock.Anything).RunAndReturn(
		func(ctx context.Context, _ string, _ *types.Params) error {
			return block(ctx)
		},
	)

	return &budgetMock{
		MockService:        service,
		MockContextSender:  sender,
		MockServiceTimeout: timeout,
	}
}

func (m *budgetMock) expectBudget(budget time.Duration, optional bool) {
	call := m.MockServiceTimeout.EXPECT().ServiceTimeout(mock.Anything).Return(budget)
	if optional {
		call.Maybe()
	}
}

func registerService(t *testing.T, scheme string, service types.Service) {
	t.Helper()

	serviceMap[scheme] = func() types.Service { return service }

	t.Cleanup(func() { delete(serviceMap, scheme) })
}

func requireTimeout(t *testing.T, err error) {
	t.Helper()

	if !errors.Is(err, ErrServiceTimeout) {
		t.Fatalf("error = %v, want ErrServiceTimeout", err)
	}
}

//nolint:paralleltest // Modifies shared serviceMap and cannot run in parallel.
func TestSendBudget(t *testing.T) {
	tests := []struct {
		name     string
		scheme   string
		timeout  time.Duration
		assign   time.Duration
		reported time.Duration
		want     time.Duration
	}{
		{
			name:     "service budget extends the default",
			scheme:   "mock-budget",
			reported: 30 * time.Second,
			want:     30 * time.Second,
		},
		{
			name:     "sender timeout caps the service",
			scheme:   "mock-cap",
			timeout:  2 * time.Second,
			reported: 30 * time.Second,
			want:     2 * time.Second,
		},
		{
			name:     "assigned timeout caps the service",
			scheme:   "mock-assign",
			assign:   2 * time.Second,
			reported: 30 * time.Second,
			want:     2 * time.Second,
		},
		{
			name:     "shorter service budget keeps the default",
			scheme:   "mock-short",
			reported: time.Second,
			want:     DefaultTimeout,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			service := newBudgetMock(t, tt.scheme, func(ctx context.Context) error {
				<-ctx.Done()

				return ctx.Err()
			})
			service.expectBudget(tt.reported, tt.timeout > 0 || tt.assign > 0)
			registerService(t, tt.scheme, service)

			synctest.Test(t, func(t *testing.T) {
				serviceRouter, err := NewWithOptions(nil, types.SenderOptions{Timeout: tt.timeout}, tt.scheme+"://")
				if err != nil {
					t.Fatalf("NewWithOptions: %v", err)
				}

				if tt.assign > 0 {
					serviceRouter.Timeout = tt.assign
				}

				start := time.Now()
				errs := serviceRouter.Send("hello", nil)

				if time.Since(start) != tt.want {
					t.Fatalf("elapsed = %s, want %s", time.Since(start), tt.want)
				}

				if len(errs) != 1 {
					t.Fatalf("Send returned %d errors, want 1", len(errs))
				}

				requireTimeout(t, errs[0])
			})
		})
	}
}

//nolint:paralleltest // Modifies shared serviceMap and cannot run in parallel.
func TestSendWithoutServiceTimeoutUsesDefault(t *testing.T) {
	const scheme = "mock-hang"

	service := mocks.NewMockService(t)
	service.EXPECT().Initialize(mock.Anything, mock.Anything).Return(nil)
	service.EXPECT().GetID().Return(scheme)
	registerService(t, scheme, service)

	synctest.Test(t, func(t *testing.T) {
		// The release channel has to be created in the bubble. A channel from
		// outside is not a durable block, so the fake clock never advances.
		release := make(chan struct{})

		service.EXPECT().Send(mock.Anything, mock.Anything).RunAndReturn(func(string, *types.Params) error {
			<-release

			return nil
		})

		serviceRouter, err := NewWithOptions(nil, types.SenderOptions{}, scheme+"://")
		if err != nil {
			t.Fatalf("NewWithOptions: %v", err)
		}

		start := time.Now()
		errs := serviceRouter.Send("hello", nil)

		close(release)
		synctest.Wait()

		if time.Since(start) != DefaultTimeout {
			t.Fatalf("elapsed = %s, want %s", time.Since(start), DefaultTimeout)
		}

		if len(errs) != 1 {
			t.Fatalf("Send returned %d errors, want 1", len(errs))
		}

		requireTimeout(t, errs[0])
	})
}

// Deadline reports the deadline that has passed.
//
// Returns:
//   - time.Time: the deadline.
//   - bool: always true.
func (c pastDeadlineContext) Deadline() (time.Time, bool) {
	return c.deadline, true
}

// TestAwaitResultTimeoutLabel verifies that a service error that matches
// context.DeadlineExceeded is labeled ErrServiceTimeout only when the service's
// budget ran out, and keeps the service's error. An error from a part of the
// send whose own deadline came earlier is returned as the service reported it.
func TestAwaitResultTimeoutLabel(t *testing.T) {
	t.Parallel()

	serviceErr := fmt.Errorf("sending to chat %q: %w", "hung", context.DeadlineExceeded)

	// tests lists send contexts with whether the result should be labeled.
	tests := []struct {
		sendCtx context.Context //nolint:containedctx // Each case needs its own send context.
		name    string
		labeled bool
	}{
		{
			name:    "budget ran out",
			sendCtx: pastDeadlineContext{Context: t.Context(), deadline: time.Now().Add(-time.Second)},
			labeled: true,
		},
		{
			name:    "budget still live",
			sendCtx: pastDeadlineContext{Context: t.Context(), deadline: time.Now().Add(time.Hour)},
			labeled: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			result := make(chan error, 1)
			result <- serviceErr

			err := awaitResult(t.Context(), tt.sendCtx, result, "telegram", 0)

			var targetErr *types.TargetError

			require.ErrorAs(t, err, &targetErr)
			require.ErrorIs(t, err, serviceErr)
			require.ErrorIs(t, err, context.DeadlineExceeded)

			if tt.labeled {
				require.ErrorIs(t, err, ErrServiceTimeout)
			} else {
				require.NotErrorIs(t, err, ErrServiceTimeout)
				assert.Equal(t, serviceErr, targetErr.Err)
			}
		})
	}
}
