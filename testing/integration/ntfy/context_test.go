package ntfy_test

import (
	"context"
	"net/http"
	"testing"
	"testing/synctest"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// hangingClient is an HTTP client whose requests never complete. Each request
// waits until its context ends and then returns the context's error.
type hangingClient struct{}

// cancelAfter is when the caller cancels the send in the cancellation test.
const cancelAfter = time.Second

// TestSendContextReturnsWhenCallerCancels verifies that SendContext returns as
// soon as the caller cancels, with an error that matches [context.Canceled],
// while the server has not answered.
func TestSendContextReturnsWhenCallerCancels(t *testing.T) {
	t.Parallel()
	synctest.Test(t, func(t *testing.T) {
		service := createTestService(t, validNtfyURL, hangingClient{})

		ctx, cancel := context.WithCancel(t.Context())
		time.AfterFunc(cancelAfter, cancel)

		start := time.Now()
		err := service.SendContext(ctx, "message", nil)

		require.ErrorIs(t, err, context.Canceled)
		assert.Equal(t, cancelAfter, time.Since(start))
	})
}

// TestSendContextReturnsAtCallerDeadline verifies that a caller's deadline that
// is shorter than the client timeout ends the send, with an error that matches
// [context.DeadlineExceeded].
func TestSendContextReturnsAtCallerDeadline(t *testing.T) {
	t.Parallel()
	synctest.Test(t, func(t *testing.T) {
		service := createTestService(t, validNtfyURL, hangingClient{})

		ctx, cancel := context.WithTimeout(t.Context(), 2*time.Second)
		defer cancel()

		start := time.Now()
		err := service.SendContext(ctx, "message", nil)

		require.ErrorIs(t, err, context.DeadlineExceeded)
		assert.Equal(t, 2*time.Second, time.Since(start))
	})
}

// Do waits until the request's context ends and returns its error.
//
// Parameters:
//   - req: the request, whose context ends the wait.
//
// Returns:
//   - *http.Response: always nil.
//   - error: the request context's error.
func (hangingClient) Do(req *http.Request) (*http.Response, error) {
	<-req.Context().Done()

	return nil, req.Context().Err()
}
