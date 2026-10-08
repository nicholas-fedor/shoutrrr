package ntfy_test

import (
	"fmt"
	"net/http"
	"sync"
	"testing"
	"testing/synctest"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

// concurrentSends is the number of simultaneous sends in the concurrency test.
const concurrentSends = 8

// TestSendParamsApplyToOneSend verifies that params and the headers they produce
// affect only the send they are passed to: a later send without params carries
// none of them.
func TestSendParamsApplyToOneSend(t *testing.T) {
	t.Parallel()
	synctest.Test(t, func(t *testing.T) {
		mockClient := expectSuccessfulSends(2)

		service := createTestService(t, validNtfyURL, mockClient)

		params := createTestParams("title", "First", "tags", "warning", "token", "tk_first")
		require.NoError(t, service.Send("first", params))
		require.NoError(t, service.Send("second", nil))

		requests := sentRequests(mockClient)
		require.Len(t, requests, 2)

		assert.Equal(t, []string{"First"}, requests[0].Header.Values("Title"))
		assert.Equal(t, "Bearer tk_first", requests[0].Header.Get("Authorization"))

		assert.Empty(t, requests[1].Header.Values("Title"))
		assert.Empty(t, requests[1].Header.Values("Tags"))
		assert.Empty(t, requests[1].Header.Get("Authorization"))
		assert.Empty(t, service.Config.Title)
	})
}

// TestConcurrentSendsUseSeparateHeaders verifies that concurrent sends on one
// service each send their own headers, so no request carries another send's title.
// Run with -race, this also guards against concurrent writes to a shared header map.
func TestConcurrentSendsUseSeparateHeaders(t *testing.T) {
	t.Parallel()
	synctest.Test(t, func(t *testing.T) {
		mockClient := expectSuccessfulSends(concurrentSends)

		service := createTestService(t, validNtfyURL, mockClient)

		var wg sync.WaitGroup
		for i := range concurrentSends {
			wg.Go(func() {
				assert.NoError(t, service.Send("concurrent", createTestParams("title", fmt.Sprintf("Title %d", i))))
			})
		}

		wg.Wait()

		requests := sentRequests(mockClient)
		require.Len(t, requests, concurrentSends)

		titles := make([]string, 0, concurrentSends)

		for _, req := range requests {
			require.Len(t, req.Header.Values("Title"), 1)
			titles = append(titles, req.Header.Get("Title"))
		}

		for i := range concurrentSends {
			assert.Contains(t, titles, fmt.Sprintf("Title %d", i))
		}
	})
}

// expectSuccessfulSends returns a mock client that answers count requests with a
// successful response each.
//
// Parameters:
//   - count: the number of requests to answer.
//
// Returns:
//   - *MockHTTPClient: the mock client.
func expectSuccessfulSends(count int) *MockHTTPClient {
	mockClient := &MockHTTPClient{}

	for range count {
		mockClient.On("Do", mock.Anything).
			Return(createMockResponse(http.StatusOK, `{}`), nil).
			Once()
	}

	return mockClient
}

// sentRequests returns the requests passed to the mock client, in call order.
//
// Parameters:
//   - mockClient: the mock HTTP client.
//
// Returns:
//   - []*http.Request: the requests sent through mockClient.
func sentRequests(mockClient *MockHTTPClient) []*http.Request {
	var requests []*http.Request

	for i := range mockClient.Calls {
		call := &mockClient.Calls[i]
		if req, ok := call.Arguments[0].(*http.Request); ok && call.Method == "Do" {
			requests = append(requests, req)
		}
	}

	return requests
}
