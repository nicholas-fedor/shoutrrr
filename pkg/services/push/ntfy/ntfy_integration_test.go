package ntfy_test

import (
	"context"
	"io"
	"log"
	"net/http"
	"net/http/httptest"
	"net/url"
	"slices"
	"strings"
	"sync"
	"testing"
	"testing/synctest"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	"github.com/nicholas-fedor/shoutrrr/internal/meta"
	"github.com/nicholas-fedor/shoutrrr/pkg/services/push/ntfy"
	"github.com/nicholas-fedor/shoutrrr/pkg/types"
	"github.com/nicholas-fedor/shoutrrr/pkg/types/mocks"
)

// receivedRequest records what the fake ntfy server received.
type receivedRequest struct {
	// header holds the request's ntfy headers, as listed in ntfyHeaders.
	header http.Header
	// method is the request method.
	method string
	// path is the request path.
	path string
	// body is the request body.
	body string
}

// Values shared by the integration tests.
const (
	// testTopic is the topic the tests publish to.
	testTopic = "alerts"
	// publishedBody is ntfy's response to a published message.
	publishedBody = `{"id":"sPs71M8A2T","time":1760500000,"expires":1760543200,"event":"message","topic":"alerts","message":"Hello"}`
	// cancelAfter is when the caller cancels a send in the context tests.
	cancelAfter = time.Second
	// callerDeadline is the caller's deadline for a send in the context tests.
	callerDeadline = 2 * time.Second
)

// ntfyHeaders lists the headers the service sets on a publish request.
var ntfyHeaders = []string{
	"Actions", "Attach", "Authorization", "Cache", "Click", "Content-Type", "Delay", "Email",
	"Filename", "Firebase", "Priority", "Tags", "Title", "User-Agent", "X-Icon",
}

// newFakeNtfy starts a loopback ntfy server that records each request and
// answers it with the given status and body.
//
// Parameters:
//   - t: the test.
//   - statusCode: the response status code.
//   - body: the response body.
//
// Returns:
//   - string: the service URL for testTopic on the server, over plain HTTP.
//   - func() []receivedRequest: returns the requests received so far.
func newFakeNtfy(t *testing.T, statusCode int, body string) (string, func() []receivedRequest) {
	t.Helper()

	var (
		mu       sync.Mutex
		received []receivedRequest
	)

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requestBody, err := io.ReadAll(r.Body)
		assert.NoError(t, err)

		header := http.Header{}

		for _, name := range ntfyHeaders {
			if values := r.Header.Values(name); len(values) > 0 {
				header[name] = values
			}
		}

		mu.Lock()

		received = append(received, receivedRequest{header: header, method: r.Method, path: r.URL.Path, body: string(requestBody)})
		mu.Unlock()

		w.WriteHeader(statusCode)
		_, _ = io.WriteString(w, body)
	}))
	t.Cleanup(server.Close)

	serviceURL := "ntfy://" + server.Listener.Addr().String() + "/" + testTopic + "?disabletls=yes"

	return serviceURL, func() []receivedRequest {
		mu.Lock()
		defer mu.Unlock()

		return slices.Clone(received)
	}
}

// newTestService returns a service initialized from rawURL that logs to the
// test output.
//
// Parameters:
//   - t: the test.
//   - rawURL: the service URL.
//
// Returns:
//   - *ntfy.Service: the service.
func newTestService(t *testing.T, rawURL string) *ntfy.Service {
	t.Helper()

	serviceURL, err := url.Parse(rawURL)
	require.NoError(t, err)

	service := &ntfy.Service{}
	require.NoError(t, service.Initialize(serviceURL, log.New(t.Output(), "", 0)))

	return service
}

// wantHeader returns the ntfy headers every publish carries, with the given
// headers added.
//
// Parameters:
//   - extra: header names and values to add, in pairs.
//
// Returns:
//   - http.Header: the headers.
func wantHeader(extra ...string) http.Header {
	header := http.Header{
		"Content-Type": {"text/plain; charset=utf-8"},
		"User-Agent":   {meta.UserAgent()},
		"Priority":     {"Default"},
	}

	for i := 0; i+1 < len(extra); i += 2 {
		header.Set(extra[i], extra[i+1])
	}

	return header
}

// TestIntegrationSend verifies what a loopback ntfy server receives from a
// send through the default client: the message as the exact body, whatever
// characters and length it has, and each send param as its ntfy header.
func TestIntegrationSend(t *testing.T) {
	t.Parallel()

	// tests lists sends with the headers the server should receive.
	tests := []struct {
		params     *types.Params
		wantHeader http.Header
		name       string
		message    string
	}{
		{name: "unicode message", message: "Hello 世界 🌍", wantHeader: wantHeader()},
		{name: "special characters", message: "Special: <>@#$%^&*()", wantHeader: wantHeader()},
		{name: "newlines", message: "Line 1\nLine 2\nLine 3", wantHeader: wantHeader()},
		{name: "long message", message: strings.Repeat("a", 10000), wantHeader: wantHeader()},
		{name: "empty params", message: "Hello", params: &types.Params{}, wantHeader: wantHeader()},
		{name: "title", message: "Hello", params: &types.Params{"title": "Alert"}, wantHeader: wantHeader("Title", "Alert")},
		{name: "priority", message: "Hello", params: &types.Params{"priority": "High"}, wantHeader: wantHeader("Priority", "High")},
		{
			name:       "tags",
			message:    "Hello",
			params:     &types.Params{"tags": "warning,skull,fire"},
			wantHeader: wantHeader("Tags", "warning,skull,fire"),
		},
		{
			name:       "click",
			message:    "Hello",
			params:     &types.Params{"click": "https://example.com"},
			wantHeader: wantHeader("Click", "https://example.com"),
		},
		{
			name:       "attach",
			message:    "Hello",
			params:     &types.Params{"attach": "https://example.com/image.png"},
			wantHeader: wantHeader("Attach", "https://example.com/image.png"),
		},
		{
			name:       "filename",
			message:    "Hello",
			params:     &types.Params{"filename": "report.pdf"},
			wantHeader: wantHeader("Filename", "report.pdf"),
		},
		{
			name:       "actions",
			message:    "Hello",
			params:     &types.Params{"actions": "view;open;reply"},
			wantHeader: wantHeader("Actions", "view;open;reply"),
		},
		{name: "delay", message: "Hello", params: &types.Params{"delay": "2h"}, wantHeader: wantHeader("Delay", "2h")},
		{
			name:       "email",
			message:    "Hello",
			params:     &types.Params{"email": "user@example.com"},
			wantHeader: wantHeader("Email", "user@example.com"),
		},
		{
			name:       "icon",
			message:    "Hello",
			params:     &types.Params{"icon": "https://example.com/icon.png"},
			wantHeader: wantHeader("X-Icon", "https://example.com/icon.png"),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			serviceURL, received := newFakeNtfy(t, http.StatusOK, publishedBody)

			require.NoError(t, newTestService(t, serviceURL).Send(tt.message, tt.params))

			requests := received()
			require.Len(t, requests, 1)
			assert.Equal(t, http.MethodPost, requests[0].method)
			assert.Equal(t, "/"+testTopic, requests[0].path)
			assert.Equal(t, tt.wantHeader, requests[0].header)
			assert.Equal(t, tt.message, requests[0].body)
		})
	}
}

// TestIntegrationSendParamsApplyToOneSend verifies that params, including an
// access token, apply only to the send they are passed to: a later send
// without params carries none of their headers, and the stored config keeps
// its values.
func TestIntegrationSendParamsApplyToOneSend(t *testing.T) {
	t.Parallel()

	serviceURL, received := newFakeNtfy(t, http.StatusOK, publishedBody)
	service := newTestService(t, serviceURL)

	require.NoError(t, service.Send("first", &types.Params{"title": "First", "tags": "warning", "token": "tk_first"}))
	require.NoError(t, service.Send("second", nil))

	requests := received()
	require.Len(t, requests, 2)
	assert.Equal(t, wantHeader("Title", "First", "Tags", "warning", "Authorization", "Bearer tk_first"), requests[0].header)
	assert.Equal(t, wantHeader(), requests[1].header)
	assert.Empty(t, service.Config.Title)
	assert.Empty(t, service.Config.Token)
}

// TestIntegrationSendErrors verifies the error a caller sees when the ntfy
// server rejects a message: ntfy's error response as its message, code, and
// documentation link, or the status alone when the body is not an ntfy error.
func TestIntegrationSendErrors(t *testing.T) {
	t.Parallel()

	// tests lists error responses with the error each should produce.
	tests := []struct {
		name       string
		body       string
		wantError  string
		statusCode int
	}{
		{
			name:       "bad request with a link",
			statusCode: http.StatusBadRequest,
			body:       `{"code":400,"error":"invalid request","link":"https://docs.ntfy.sh"}`,
			wantError:  "failed to send ntfy notification: server response: invalid request (400), see: https://docs.ntfy.sh",
		},
		{
			name:       "unauthorized",
			statusCode: http.StatusUnauthorized,
			body:       `{"code":401,"error":"unauthorized"}`,
			wantError:  "failed to send ntfy notification: server response: unauthorized (401)",
		},
		{
			name:       "forbidden",
			statusCode: http.StatusForbidden,
			body:       `{"code":403,"error":"forbidden"}`,
			wantError:  "failed to send ntfy notification: server response: forbidden (403)",
		},
		{
			name:       "not found",
			statusCode: http.StatusNotFound,
			body:       `{"code":404,"error":"not found"}`,
			wantError:  "failed to send ntfy notification: server response: not found (404)",
		},
		{
			name:       "internal server error",
			statusCode: http.StatusInternalServerError,
			body:       `{"code":500,"error":"internal server error"}`,
			wantError:  "failed to send ntfy notification: server response: internal server error (500)",
		},
		{
			name:       "malformed error body",
			statusCode: http.StatusBadRequest,
			body:       "not json",
			wantError:  "failed to send ntfy notification: posting to ntfy API: got unexpected HTTP status: 400 Bad Request",
		},
		{
			name:       "empty error body",
			statusCode: http.StatusInternalServerError,
			wantError:  "failed to send ntfy notification: posting to ntfy API: got unexpected HTTP status: 500 Internal Server Error",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			serviceURL, received := newFakeNtfy(t, tt.statusCode, tt.body)

			err := newTestService(t, serviceURL).Send("Test message", nil)

			require.EqualError(t, err, tt.wantError)
			assert.Len(t, received(), 1)
		})
	}
}

// TestIntegrationSendContextEnds verifies that SendContext returns as soon as
// the caller cancels or the caller's deadline passes while the server has not
// answered, with an error that matches the context's error.
func TestIntegrationSendContextEnds(t *testing.T) {
	t.Parallel()

	// tests lists caller contexts with the error and the elapsed time each
	// should produce.
	tests := []struct {
		wantErr     error
		newContext  func(parent context.Context) (context.Context, context.CancelFunc)
		name        string
		wantElapsed time.Duration
	}{
		{
			name: "caller cancels",
			newContext: func(parent context.Context) (context.Context, context.CancelFunc) {
				ctx, cancel := context.WithCancel(parent)
				time.AfterFunc(cancelAfter, cancel)

				return ctx, cancel
			},
			wantErr:     context.Canceled,
			wantElapsed: cancelAfter,
		},
		{
			name: "caller deadline",
			newContext: func(parent context.Context) (context.Context, context.CancelFunc) {
				return context.WithTimeout(parent, callerDeadline)
			},
			wantErr:     context.DeadlineExceeded,
			wantElapsed: callerDeadline,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			synctest.Test(t, func(t *testing.T) {
				// The client never answers. It waits until the request's
				// context ends and returns the context's error.
				client := mocks.NewMockHTTPClient(t)
				client.EXPECT().Do(mock.Anything).RunAndReturn(func(req *http.Request) (*http.Response, error) {
					<-req.Context().Done()

					return nil, req.Context().Err()
				}).Once()

				service := newTestService(t, "ntfy://ntfy.example.com/"+testTopic)
				service.SetHTTPClient(client)

				ctx, cancel := tt.newContext(t.Context())
				defer cancel()

				start := time.Now()
				err := service.SendContext(ctx, "message", nil)

				require.ErrorIs(t, err, tt.wantErr)
				assert.Equal(t, tt.wantElapsed, time.Since(start))
			})
		})
	}
}
