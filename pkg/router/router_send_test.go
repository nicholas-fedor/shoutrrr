package router

import (
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

// contextSenderMock is a context-aware service assembled from generated mocks.
type contextSenderMock struct {
	*mocks.MockService
	*mocks.MockContextSender
}

// Errors returned by the fake services in these tests.
var (
	// errSlowService is returned by the service that finishes last.
	errSlowService = errors.New("slow service failed")
	// errFastService is returned by the service that finishes first.
	errFastService = errors.New("fast service failed")
)

// newSendMock registers a service whose Send runs send.
//
// Parameters:
//   - t: the test that owns the mock and the registration.
//   - scheme: the scheme to register the service under.
//   - send: the behavior of Send.
func newSendMock(t *testing.T, scheme string, send func(string, *types.Params) error) {
	t.Helper()

	service := mocks.NewMockService(t)
	service.EXPECT().Initialize(mock.Anything, mock.Anything).Return(nil)
	service.EXPECT().GetID().Return(scheme).Maybe()
	service.EXPECT().Send(mock.Anything, mock.Anything).RunAndReturn(send).Maybe()

	registerService(t, scheme, service)
}

// TestSendErrorsFollowURLOrder verifies that Send and SendItems return each error at
// the index of its configured URL, even when services finish out of order, and that
// every TargetError carries that index.
//
//nolint:paralleltest // Modifies shared serviceMap and cannot run in parallel.
func TestSendErrorsFollowURLOrder(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		newSendMock(t, "slowsvc", func(string, *types.Params) error {
			time.Sleep(2 * time.Second)

			return errSlowService
		})
		newSendMock(t, "fastsvc", func(string, *types.Params) error {
			return errFastService
		})

		r, err := NewWithOptions(nil, types.SenderOptions{}, "slowsvc://", "fastsvc://")
		require.NoError(t, err)

		for name, errs := range map[string][]error{
			"Send":      r.Send("message", nil),
			"SendItems": r.SendItems([]types.MessageItem{{Text: "message"}}, types.Params{}),
		} {
			require.Len(t, errs, 2, name)
			require.ErrorIs(t, errs[0], errSlowService, name)
			require.ErrorIs(t, errs[1], errFastService, name)

			for i, sendErr := range errs {
				targetErr, ok := errors.AsType[*types.TargetError](sendErr)
				require.True(t, ok, "%s: error %d is not a TargetError", name, i)
				assert.Equal(t, i, targetErr.Index, name)
			}
		}
	})
}

// TestSendAsyncErrorsCarryIndex verifies that SendAsync, which reports results in
// completion order, still identifies each failed URL through TargetError.Index.
//
//nolint:paralleltest // Modifies shared serviceMap and cannot run in parallel.
func TestSendAsyncErrorsCarryIndex(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		newSendMock(t, "slowsvc", func(string, *types.Params) error {
			time.Sleep(2 * time.Second)

			return errSlowService
		})
		newSendMock(t, "fastsvc", func(string, *types.Params) error {
			return errFastService
		})

		r, err := NewWithOptions(nil, types.SenderOptions{}, "slowsvc://", "fastsvc://")
		require.NoError(t, err)

		indexes := map[error]int{}

		for sendErr := range r.SendAsync("message", nil) {
			targetErr, ok := errors.AsType[*types.TargetError](sendErr)
			require.True(t, ok, "error is not a TargetError: %v", sendErr)

			indexes[targetErr.Err] = targetErr.Index
		}

		assert.Equal(t, map[error]int{errSlowService: 0, errFastService: 1}, indexes)
	})
}

// TestSendAsyncReportsEverySendInCompletionOrder verifies that SendAsync delivers
// one result per service in the order the sends finish, including nil for a
// successful send.
//
//nolint:paralleltest // Modifies shared serviceMap and cannot run in parallel.
func TestSendAsyncReportsEverySendInCompletionOrder(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		newSendMock(t, "slowsvc", func(string, *types.Params) error {
			time.Sleep(2 * time.Second)

			return nil
		})
		newSendMock(t, "fastsvc", func(string, *types.Params) error {
			return errFastService
		})

		r, err := NewWithOptions(nil, types.SenderOptions{}, "slowsvc://", "fastsvc://")
		require.NoError(t, err)

		var results []error
		for sendErr := range r.SendAsync("message", nil) {
			results = append(results, sendErr)
		}

		require.Len(t, results, 2)
		require.ErrorIs(t, results[0], errFastService)
		assert.NoError(t, results[1])
	})
}

// TestSendIsolatesParams verifies that each service receives its own copy of the
// params, so one service changing them cannot affect another.
//
//nolint:paralleltest // Modifies shared serviceMap and cannot run in parallel.
func TestSendIsolatesParams(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		var seenTitle string

		newSendMock(t, "mutatesvc", func(_ string, params *types.Params) error {
			(*params)["title"] = "changed"

			return nil
		})
		newSendMock(t, "readsvc", func(_ string, params *types.Params) error {
			time.Sleep(time.Second)

			seenTitle = (*params)["title"]

			return nil
		})

		r, err := NewWithOptions(nil, types.SenderOptions{}, "mutatesvc://", "readsvc://")
		require.NoError(t, err)

		params := types.Params{"title": "original"}

		for _, sendErr := range r.Send("message", &params) {
			require.NoError(t, sendErr)
		}

		assert.Equal(t, "original", seenTitle)
		assert.Equal(t, "original", params["title"], "the caller's params must not change")
	})
}

// TestZeroValueRouterSends verifies that a zero-value ServiceRouter can send to
// context-aware services and send items without a constructor-provided context.
//
//nolint:paralleltest // Modifies shared serviceMap and cannot run in parallel.
func TestZeroValueRouterSends(t *testing.T) {
	service := mocks.NewMockService(t)
	sender := mocks.NewMockContextSender(t)

	service.EXPECT().Initialize(mock.Anything, mock.Anything).Return(nil)
	service.EXPECT().GetID().Return("ctxsvc").Maybe()
	sender.EXPECT().SendContext(mock.Anything, mock.Anything, mock.Anything).Return(nil).Times(2)

	registerService(t, "ctxsvc", &contextSenderMock{MockService: service, MockContextSender: sender})

	r := &ServiceRouter{}
	require.NoError(t, r.AddService("ctxsvc://"))

	assert.NotPanics(t, func() {
		for _, sendErr := range r.Send("message", nil) {
			assert.NoError(t, sendErr)
		}
	})

	assert.NotPanics(t, func() {
		for _, sendErr := range r.SendItems([]types.MessageItem{{Text: "message"}}, types.Params{}) {
			assert.NoError(t, sendErr)
		}
	})
}

// TestFlushEmptyQueue verifies that Flush sends nothing when no messages are queued.
//
//nolint:paralleltest // Modifies shared serviceMap and cannot run in parallel.
func TestFlushEmptyQueue(t *testing.T) {
	service := mocks.NewMockService(t)
	service.EXPECT().Initialize(mock.Anything, mock.Anything).Return(nil)

	registerService(t, "flushsvc", service)

	r, err := NewWithOptions(nil, types.SenderOptions{}, "flushsvc://")
	require.NoError(t, err)

	r.Flush(nil)
}
