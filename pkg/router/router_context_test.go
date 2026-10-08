package router

import (
	"context"
	"errors"
	"testing"
	"testing/synctest"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	"github.com/nicholas-fedor/shoutrrr/pkg/types"
	"github.com/nicholas-fedor/shoutrrr/pkg/types/mocks"
)

// closerService is a service that holds resources between sends and records Close calls.
type closerService struct {
	*mocks.MockService

	// closeErr is returned by Close.
	closeErr error
	// closed counts Close calls.
	closed int
}

// errCloseFailed is returned by the closer service that fails to close.
var errCloseFailed = errors.New("close failed")

// TestSendContextReturnsWhenCallerCancels verifies that SendContext returns as soon
// as the caller's context is canceled, without waiting for a service that ignores
// contexts, and that each unfinished service reports the cancellation at its URL's
// index while finished services keep their results.
//
//nolint:paralleltest // Modifies shared serviceMap and cannot run in parallel.
func TestSendContextReturnsWhenCallerCancels(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		release := make(chan struct{})

		contextService := newBudgetMock(t, "ctxsvc", func(ctx context.Context) error {
			<-ctx.Done()

			return ctx.Err()
		})
		contextService.expectBudget(time.Minute, false)
		registerService(t, "ctxsvc", contextService)

		newSendMock(t, "plainsvc", func(string, *types.Params) error {
			<-release

			return nil
		})
		newSendMock(t, "fastsvc", func(string, *types.Params) error { return nil })

		r, err := NewWithOptions(nil, types.SenderOptions{}, "ctxsvc://", "plainsvc://", "fastsvc://")
		require.NoError(t, err)

		ctx, cancel := context.WithCancel(context.Background())
		time.AfterFunc(time.Second, cancel)

		start := time.Now()
		errs := r.SendContext(ctx, "message", nil)

		assert.Equal(t, time.Second, time.Since(start), "SendContext must return when ctx is canceled")
		require.Len(t, errs, 3)

		for i, sendErr := range errs[:2] {
			require.ErrorIs(t, sendErr, context.Canceled)
			require.NotErrorIs(t, sendErr, ErrServiceTimeout)

			targetErr, ok := errors.AsType[*types.TargetError](sendErr)
			require.True(t, ok)
			assert.Equal(t, i, targetErr.Index)
		}

		require.NoError(t, errs[2])

		close(release)
		synctest.Wait()
	})
}

// TestSendContextReportsCallerDeadline verifies that a caller's deadline that ends
// a send before the service's budget is reported as the caller's deadline, not as
// the service timing out.
//
//nolint:paralleltest // Modifies shared serviceMap and cannot run in parallel.
func TestSendContextReportsCallerDeadline(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		contextService := newBudgetMock(t, "ctxsvc", func(ctx context.Context) error {
			<-ctx.Done()

			return ctx.Err()
		})
		contextService.expectBudget(time.Minute, false)
		registerService(t, "ctxsvc", contextService)

		r, err := NewWithOptions(nil, types.SenderOptions{}, "ctxsvc://")
		require.NoError(t, err)

		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		defer cancel()

		errs := r.SendContext(ctx, "message", nil)

		require.Len(t, errs, 1)
		require.ErrorIs(t, errs[0], context.DeadlineExceeded)
		require.NotErrorIs(t, errs[0], ErrServiceTimeout)
	})
}

// TestSendItemsContextReturnsWhenCallerCancels verifies that SendItemsContext also
// returns when the caller's context is canceled, for both context-aware services
// and services that ignore contexts.
//
//nolint:paralleltest // Modifies shared serviceMap and cannot run in parallel.
func TestSendItemsContextReturnsWhenCallerCancels(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		release := make(chan struct{})

		contextService := newBudgetMock(t, "ctxsvc", func(ctx context.Context) error {
			<-ctx.Done()

			return ctx.Err()
		})
		contextService.expectBudget(time.Minute, false)
		registerService(t, "ctxsvc", contextService)

		newSendMock(t, "plainsvc", func(string, *types.Params) error {
			<-release

			return nil
		})

		r, err := NewWithOptions(nil, types.SenderOptions{}, "ctxsvc://", "plainsvc://")
		require.NoError(t, err)

		ctx, cancel := context.WithCancel(context.Background())
		time.AfterFunc(time.Second, cancel)

		errs := r.SendItemsContext(ctx, []types.MessageItem{{Text: "message"}}, types.Params{})

		require.Len(t, errs, 2)
		require.ErrorIs(t, errs[0], context.Canceled)
		require.ErrorIs(t, errs[1], context.Canceled)

		close(release)
		synctest.Wait()
	})
}

// TestCloseClosesServicesThatHoldResources verifies that Close closes every service
// that implements io.Closer, skips the others, and reports each close failure as a
// TargetError at the service's index.
//
//nolint:paralleltest // Modifies shared serviceMap and cannot run in parallel.
func TestCloseClosesServicesThatHoldResources(t *testing.T) {
	healthy := newCloserService(t, "okcloser", nil)
	failing := newCloserService(t, "badcloser", errCloseFailed)
	newSendMock(t, "plainsvc", func(string, *types.Params) error { return nil })

	r, err := NewWithOptions(nil, types.SenderOptions{}, "okcloser://", "badcloser://", "plainsvc://")
	require.NoError(t, err)

	err = r.Close()

	require.ErrorIs(t, err, errCloseFailed)

	targetErr, ok := errors.AsType[*types.TargetError](err)
	require.True(t, ok)
	assert.Equal(t, 1, targetErr.Index)
	assert.Equal(t, "badcloser", targetErr.URL)

	assert.Equal(t, 1, healthy.closed)
	assert.Equal(t, 1, failing.closed)

	var nilRouter *ServiceRouter
	assert.NoError(t, nilRouter.Close())
}

// newCloserService registers a closer service under scheme.
//
// Parameters:
//   - t: the test that owns the mock and the registration.
//   - scheme: the scheme to register the service under.
//   - closeErr: the error Close returns.
//
// Returns:
//   - *closerService: the registered service.
func newCloserService(t *testing.T, scheme string, closeErr error) *closerService {
	t.Helper()

	service := mocks.NewMockService(t)
	service.EXPECT().Initialize(mock.Anything, mock.Anything).Return(nil)
	service.EXPECT().GetID().Return(scheme).Maybe()

	closer := &closerService{MockService: service, closeErr: closeErr}
	registerService(t, scheme, closer)

	return closer
}

// Close records the call and returns the configured error.
//
// Returns:
//   - error: the configured close error.
func (c *closerService) Close() error {
	c.closed++

	return c.closeErr
}
