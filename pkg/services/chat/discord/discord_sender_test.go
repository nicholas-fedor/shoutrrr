package discord

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"sync/atomic"
	"testing"
	"testing/synctest"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	"github.com/nicholas-fedor/shoutrrr/pkg/services/chat/discord/mocks"
	"github.com/nicholas-fedor/shoutrrr/pkg/types"
)

// roundTripFunc adapts a function to [http.RoundTripper].
type roundTripFunc func(*http.Request) (*http.Response, error)

// mockTemporaryError is a [net.Error] that reports itself as temporary.
type mockTemporaryError struct{}

// mockTimeoutError is a [net.Error] that reports itself as a timeout.
type mockTimeoutError struct{}

// trackedBody is a response body that records whether it was read to its end
// before it was closed.
type trackedBody struct {
	// Reader supplies the body.
	*strings.Reader

	// drainedAtClose is true when Close found nothing left to read.
	drainedAtClose bool
}

// RoundTrip calls f with the request.
//
// Parameters:
//   - req: the outgoing request.
//
// Returns:
//   - *http.Response: the response f returns.
//   - error: the transport error f returns.
func (f roundTripFunc) RoundTrip(req *http.Request) (*http.Response, error) {
	return f(req)
}

// Error returns the error message.
//
// Returns:
//   - string: the message.
func (m *mockTemporaryError) Error() string { return "temporary error" }

// Temporary reports that the error is temporary.
//
// Returns:
//   - bool: true.
func (m *mockTemporaryError) Temporary() bool { return true }

// Timeout reports that the error is not a timeout.
//
// Returns:
//   - bool: false.
func (m *mockTemporaryError) Timeout() bool { return false }

// Error returns the error message.
//
// Returns:
//   - string: the message.
func (m *mockTimeoutError) Error() string { return "timeout error" }

// Temporary reports that the error is not temporary.
//
// Returns:
//   - bool: false.
func (m *mockTimeoutError) Temporary() bool { return false }

// Timeout reports that the error is a timeout.
//
// Returns:
//   - bool: true.
func (m *mockTimeoutError) Timeout() bool { return true }

// Close records whether the body was read to its end first.
//
// Returns:
//   - error: always nil.
func (b *trackedBody) Close() error {
	b.drainedAtClose = b.Len() == 0

	return nil
}

// statusResponse returns a response with the given status, headers, and an
// empty body.
//
// Parameters:
//   - status: the HTTP status code.
//   - header: the response headers, or nil for none.
//
// Returns:
//   - *http.Response: the response.
func statusResponse(status int, header http.Header) *http.Response {
	if header == nil {
		header = http.Header{}
	}

	return &http.Response{
		StatusCode: status,
		Status:     http.StatusText(status),
		Header:     header,
		Body:       io.NopCloser(strings.NewReader("")),
	}
}

// newJSONPreparer returns a JSON request preparer with a small payload.
//
// Returns:
//   - *JSONRequestPreparer: the preparer.
func newJSONPreparer() *JSONRequestPreparer {
	return &JSONRequestPreparer{payload: []byte(`{"content":"test"}`)}
}

// newRecordingSleeper returns a mock sleeper that expects exactly calls waits
// and records their durations in order.
//
// Parameters:
//   - t: the test that owns the mock.
//   - calls: the number of waits to expect.
//
// Returns:
//   - *mocks.MockSleeper: the sleeper.
//   - *[]time.Duration: the recorded waits, in order.
func newRecordingSleeper(t *testing.T, calls int) (*mocks.MockSleeper, *[]time.Duration) {
	t.Helper()

	var waits []time.Duration

	sleeper := mocks.NewMockSleeper(t)
	sleeper.EXPECT().Sleep(mock.Anything).Run(func(d time.Duration) {
		waits = append(waits, d)
	}).Return().Times(calls)

	return sleeper, &waits
}

// TestNewDefaultHTTPClient verifies the default client's timeout.
func TestNewDefaultHTTPClient(t *testing.T) {
	t.Parallel()

	client := NewDefaultHTTPClient()

	require.NotNil(t, client.client)
	assert.Equal(t, 30*time.Second, client.client.Timeout)
}

// TestDefaultHTTPClientDo verifies that the default client returns the response
// and wraps a transport error.
func TestDefaultHTTPClientDo(t *testing.T) {
	t.Parallel()

	t.Run("success", func(t *testing.T) {
		t.Parallel()

		client := NewDefaultHTTPClient()
		client.client.Transport = roundTripFunc(func(*http.Request) (*http.Response, error) {
			return &http.Response{StatusCode: http.StatusOK, Body: io.NopCloser(strings.NewReader("success"))}, nil
		})

		req, err := http.NewRequestWithContext(t.Context(), http.MethodGet, "http://example.com", http.NoBody)
		require.NoError(t, err)

		resp, err := client.Do(req)
		require.NoError(t, err)

		defer func() { _ = resp.Body.Close() }()

		assert.Equal(t, http.StatusOK, resp.StatusCode)

		body, err := io.ReadAll(resp.Body)
		require.NoError(t, err)
		assert.Equal(t, "success", string(body))
	})

	t.Run("transport error", func(t *testing.T) {
		t.Parallel()

		errNetwork := errors.New("network error")

		client := NewDefaultHTTPClient()
		client.client.Transport = roundTripFunc(func(*http.Request) (*http.Response, error) {
			return nil, errNetwork
		})

		req, err := http.NewRequestWithContext(t.Context(), http.MethodGet, "http://example.com", http.NoBody)
		require.NoError(t, err)

		_, err = client.Do(req)
		require.ErrorIs(t, err, errNetwork)
		assert.Contains(t, err.Error(), "performing HTTP request")
	})

	t.Run("zero value", func(t *testing.T) {
		t.Parallel()

		ctx, cancel := context.WithCancel(t.Context())
		cancel()

		req, err := http.NewRequestWithContext(ctx, http.MethodGet, "http://example.com", http.NoBody)
		require.NoError(t, err)

		_, err = (&DefaultHTTPClient{}).Do(req)
		require.ErrorIs(t, err, context.Canceled)
	})
}

// TestJSONRequestPreparer verifies that the JSON preparer builds a POST with the
// payload, even with a canceled context.
func TestJSONRequestPreparer(t *testing.T) {
	t.Parallel()

	payload := []byte(`{"content":"test message"}`)
	preparer := &JSONRequestPreparer{payload: payload}

	req, err := preparer.PrepareRequest(t.Context(), testWebhookURL)
	require.NoError(t, err)

	assert.Equal(t, http.MethodPost, req.Method)
	assert.Equal(t, testWebhookURL, req.URL.String())
	assert.Equal(t, "application/json", req.Header.Get("Content-Type"))

	body, err := io.ReadAll(req.Body)
	require.NoError(t, err)
	assert.Equal(t, payload, body)

	ctx, cancel := context.WithCancel(t.Context())
	cancel()

	_, err = preparer.PrepareRequest(ctx, testWebhookURL)
	require.NoError(t, err, "building a request does not check the context")
}

// TestMultipartRequestPreparer verifies that the multipart preparer builds a
// POST with the payload and a part for each file.
func TestMultipartRequestPreparer(t *testing.T) {
	t.Parallel()

	// tests lists files with whether the body should contain a file part.
	tests := []struct {
		name      string
		files     []types.File
		wantFiles bool
	}{
		{name: "with a file", files: []types.File{{Name: "test.txt", Data: []byte("file content")}}, wantFiles: true},
		{name: "without files", files: []types.File{}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			preparer := &MultipartRequestPreparer{payload: &WebhookPayload{Content: "test message"}, files: tt.files}

			req, err := preparer.PrepareRequest(t.Context(), testWebhookURL)
			require.NoError(t, err)

			assert.Equal(t, http.MethodPost, req.Method)
			assert.Equal(t, testWebhookURL, req.URL.String())
			assert.True(t, strings.HasPrefix(req.Header.Get("Content-Type"), "multipart/form-data"))

			body, err := io.ReadAll(req.Body)
			require.NoError(t, err)

			assert.Contains(t, string(body), "payload_json")
			assert.Contains(t, string(body), "test message")
			assert.Equal(t, tt.wantFiles, strings.Contains(string(body), "files[0]"))
			assert.Equal(t, tt.wantFiles, strings.Contains(string(body), "test.txt"))
		})
	}
}

// TestSendWithRetryResults verifies the result of a single attempt for
// success, error status, nil response, and transport error.
func TestSendWithRetryResults(t *testing.T) {
	t.Parallel()

	// tests lists the client's single reply with the error the send should
	// produce. wantText is checked when set.
	tests := []struct {
		name     string
		resp     *http.Response
		err      error
		wantErr  error
		wantText string
	}{
		{name: "no content", resp: statusResponse(http.StatusNoContent, nil)},
		{name: "OK", resp: statusResponse(http.StatusOK, nil)},
		{name: "bad request", resp: statusResponse(http.StatusBadRequest, nil), wantText: "unexpected response status code"},
		{name: "nil response", wantErr: ErrUnknownAPIError},
		{name: "transport error", err: http.ErrHandlerTimeout, wantErr: http.ErrHandlerTimeout, wantText: "making HTTP POST request"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			client := mocks.NewMockHTTPClient(t)
			client.EXPECT().Do(mock.Anything).Return(tt.resp, tt.err).Once()

			_, err := sendWithRetry(t.Context(), newJSONPreparer(), "http://example.com", client, mocks.NewMockSleeper(t), nil)

			if tt.wantErr == nil && tt.wantText == "" {
				require.NoError(t, err)

				return
			}

			require.Error(t, err)

			if tt.wantErr != nil {
				require.ErrorIs(t, err, tt.wantErr)
			}

			if tt.wantText != "" {
				assert.Contains(t, err.Error(), tt.wantText)
			}
		})
	}
}

// TestSendWithRetryReadsReply verifies that a successful reply's body, the
// message Discord created, is read to its end before it is closed, so the
// connection can carry the next batch.
func TestSendWithRetryReadsReply(t *testing.T) {
	t.Parallel()

	body := &trackedBody{Reader: strings.NewReader(`{"id":"1","content":"Test message"}`), drainedAtClose: false}
	client := mocks.NewMockHTTPClient(t)
	client.EXPECT().Do(mock.Anything).Return(&http.Response{
		StatusCode: http.StatusOK,
		Status:     "200 OK",
		Body:       body,
	}, nil).Once()

	_, err := sendWithRetry(t.Context(), newJSONPreparer(), "http://example.com", client, mocks.NewMockSleeper(t), nil)
	require.NoError(t, err)

	assert.True(t, body.drainedAtClose, "the body is read to its end before it is closed")
}

// TestSendWithRetryReadsRetriedReply verifies that a rate-limited or failed
// reply's body is read to its end and closed before the wait, so the retry
// can reuse the connection instead of holding it through the wait.
func TestSendWithRetryReadsRetriedReply(t *testing.T) {
	t.Parallel()

	// tests lists the retried statuses with a body each.
	tests := []struct {
		name   string
		body   string
		status int
	}{
		{name: "rate limited", status: http.StatusTooManyRequests, body: `{"message":"You are being rate limited.","retry_after":0.5,"global":false}`},
		{name: "server error", status: http.StatusBadGateway, body: "<html>Bad Gateway</html>"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			body := &trackedBody{Reader: strings.NewReader(tt.body), drainedAtClose: false}
			client := mocks.NewMockHTTPClient(t)
			client.EXPECT().Do(mock.Anything).Return(&http.Response{
				StatusCode: tt.status,
				Status:     http.StatusText(tt.status),
				Header:     http.Header{},
				Body:       body,
			}, nil).Once()
			client.EXPECT().Do(mock.Anything).Return(statusResponse(http.StatusNoContent, nil), nil).Once()

			sleeper := mocks.NewMockSleeper(t)
			sleeper.EXPECT().Sleep(mock.Anything).Run(func(time.Duration) {
				assert.True(t, body.drainedAtClose, "the body is read to its end and closed before the wait")
			}).Return().Once()

			_, err := sendWithRetry(t.Context(), newJSONPreparer(), "http://example.com", client, sleeper, nil)
			require.NoError(t, err)
		})
	}
}

// TestSendWithRetryMayHaveCreated verifies when a send reports that its
// request may have created a message.
//
// A success, a transport failure after the dial, and a client that answers
// with neither a response nor an error may have. So may any attempt that got
// a server error other than 502, even when a later attempt fails otherwise.
// An error status, 502s until the retries run out, a rate limit that cannot
// be waited out, a failed dial, and a request that cannot be built created
// nothing.
func TestSendWithRetryMayHaveCreated(t *testing.T) {
	t.Parallel()

	refused := &net.OpError{Op: "dial", Net: "tcp", Err: errors.New("connect: connection refused")}

	// respond answers every request with a fresh response of status.
	respond := func(status int, header http.Header) func(*http.Request) (*http.Response, error) {
		return func(*http.Request) (*http.Response, error) {
			return statusResponse(status, header), nil
		}
	}

	// tests lists the client's answers with the sleeps they need and the
	// result to expect.
	tests := []struct {
		name   string
		url    string
		expect func(client *mocks.MockHTTPClient)
		sleeps int
		want   bool
	}{
		{
			name: "success",
			expect: func(c *mocks.MockHTTPClient) {
				c.EXPECT().Do(mock.Anything).RunAndReturn(respond(http.StatusNoContent, nil)).Once()
			},
			want: true,
		},
		{
			name: "transport failure after the dial",
			expect: func(c *mocks.MockHTTPClient) {
				c.EXPECT().Do(mock.Anything).Return(nil, errors.New("connection lost")).Once()
			},
			want: true,
		},
		{
			name: "client without a response or an error",
			expect: func(c *mocks.MockHTTPClient) {
				c.EXPECT().Do(mock.Anything).Return(nil, nil).Once()
			},
			want: true,
		},
		{
			name: "error status",
			expect: func(c *mocks.MockHTTPClient) {
				c.EXPECT().Do(mock.Anything).RunAndReturn(respond(http.StatusBadRequest, nil)).Once()
			},
		},
		{
			name: "rate limit that cannot be waited out",
			expect: func(c *mocks.MockHTTPClient) {
				c.EXPECT().Do(mock.Anything).RunAndReturn(respond(http.StatusTooManyRequests, http.Header{"Retry-After": {"600"}})).Once()
			},
		},
		{
			name: "server errors until the retries run out",
			expect: func(c *mocks.MockHTTPClient) {
				c.EXPECT().Do(mock.Anything).RunAndReturn(respond(http.StatusServiceUnavailable, nil)).Times(maxRetries + 1)
			},
			sleeps: maxRetries,
			want:   true,
		},
		{
			name: "no gateway until the retries run out",
			expect: func(c *mocks.MockHTTPClient) {
				c.EXPECT().Do(mock.Anything).RunAndReturn(respond(http.StatusBadGateway, nil)).Times(maxRetries + 1)
			},
			sleeps: maxRetries,
		},
		{
			name: "server error, then an error status",
			expect: func(c *mocks.MockHTTPClient) {
				c.EXPECT().Do(mock.Anything).RunAndReturn(respond(http.StatusInternalServerError, nil)).Once()
				c.EXPECT().Do(mock.Anything).RunAndReturn(respond(http.StatusBadRequest, nil)).Once()
			},
			sleeps: 1,
			want:   true,
		},
		{
			name: "server error, then a failed dial",
			expect: func(c *mocks.MockHTTPClient) {
				c.EXPECT().Do(mock.Anything).RunAndReturn(respond(http.StatusInternalServerError, nil)).Once()
				c.EXPECT().Do(mock.Anything).Return(nil, refused).Times(4)
			},
			sleeps: 4,
			want:   true,
		},
		{
			name: "server error, then a rate limit that cannot be waited out",
			expect: func(c *mocks.MockHTTPClient) {
				c.EXPECT().Do(mock.Anything).RunAndReturn(respond(http.StatusInternalServerError, nil)).Once()
				c.EXPECT().Do(mock.Anything).RunAndReturn(respond(http.StatusTooManyRequests, http.Header{"Retry-After": {"600"}})).Once()
			},
			sleeps: 1,
			want:   true,
		},
		{
			name: "no gateway, then an error status",
			expect: func(c *mocks.MockHTTPClient) {
				c.EXPECT().Do(mock.Anything).RunAndReturn(respond(http.StatusBadGateway, nil)).Once()
				c.EXPECT().Do(mock.Anything).RunAndReturn(respond(http.StatusBadRequest, nil)).Once()
			},
			sleeps: 1,
		},
		{
			name:   "failed dial",
			expect: func(c *mocks.MockHTTPClient) { c.EXPECT().Do(mock.Anything).Return(nil, refused).Times(4) },
			sleeps: 3,
		},
		{name: "request that cannot be built", url: "://bad", expect: func(*mocks.MockHTTPClient) {}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			client := mocks.NewMockHTTPClient(t)
			tt.expect(client)

			sleeper := mocks.NewMockSleeper(t)
			if tt.sleeps > 0 {
				sleeper.EXPECT().Sleep(mock.Anything).Return().Times(tt.sleeps)
			}

			requestURL := tt.url
			if requestURL == "" {
				requestURL = "http://example.com"
			}

			mayHaveCreated, _ := sendWithRetry(t.Context(), newJSONPreparer(), requestURL, client, sleeper, nil)
			assert.Equal(t, tt.want, mayHaveCreated)
		})
	}
}

// TestErrorBodyReason verifies that Discord's error message is quoted on one
// line, with characters that are not printable replaced, and cut to 200
// characters.
func TestErrorBodyReason(t *testing.T) {
	t.Parallel()

	// tests lists messages with the reason each gives.
	tests := []struct {
		name    string
		message string
		want    string
	}{
		{name: "plain", message: "Unknown Webhook", want: "Unknown Webhook"},
		{name: "empty", message: "", want: ""},
		{name: "only spaces and control characters", message: " \n\t\x07 ", want: ""},
		{name: "line breaks and tabs", message: "Invalid\r\nForm\tBody\n", want: "Invalid Form Body"},
		{name: "bidirectional override", message: "Unknown\u202eWebhook", want: "Unknown Webhook"},
		{name: "at the limit", message: strings.Repeat("a", 200), want: strings.Repeat("a", 200)},
		{name: "over the limit", message: strings.Repeat("é", 201), want: strings.Repeat("é", 194) + " [...]"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			assert.Equal(t, tt.want, errorBody{RetryAfter: nil, Errors: nil, Code: nil, Message: tt.message}.reason())
		})
	}
}

// TestErrorBodyReasonFields verifies that a reason names the invalid fields of
// a Discord errors object after the message and code.
//
// The first three cases are the array, object, and request errors from
// Discord's Error Messages reference. The others cover several errors, the
// limit of five, and parts of the object that do not have Discord's shape,
// which are skipped while the rest is kept. Invalid UTF-8, which the body
// allows, ends the list after the fields before it. Discord's per-field
// messages are never quoted.
func TestErrorBodyReasonFields(t *testing.T) {
	t.Parallel()

	const invalid = "Invalid Form Body (code 50035)"

	// tests lists error bodies with the reason each gives.
	tests := []struct {
		name string
		body string
		want string
	}{
		{
			name: "array error",
			body: `{"code": 50035, "errors": {"activities": {"0": {"platform": {"_errors": [{"code": "BASE_TYPE_CHOICES", "message": "Value must be one of ('desktop', 'android', 'ios')."}]}, "type": {"_errors": [{"code": "BASE_TYPE_CHOICES", "message": "Value must be one of (0, 1, 2, 3, 4, 5)."}]}}}}, "message": "Invalid Form Body"}`,
			want: invalid + ": activities.0.platform: BASE_TYPE_CHOICES, activities.0.type: BASE_TYPE_CHOICES",
		},
		{
			name: "object error",
			body: `{"code": 50035, "errors": {"access_token": {"_errors": [{"code": "BASE_TYPE_REQUIRED", "message": "This field is required"}]}}, "message": "Invalid Form Body"}`,
			want: invalid + ": access_token: BASE_TYPE_REQUIRED",
		},
		{
			name: "request error",
			body: `{"code": 50035, "message": "Invalid Form Body", "errors": {"_errors": [{"code": "APPLICATION_COMMAND_TOO_LARGE", "message": "Command exceeds maximum size (8000)"}]}}`,
			want: invalid + ": APPLICATION_COMMAND_TOO_LARGE",
		},
		{
			name: "several errors for one field",
			body: `{"code": 50035, "message": "Invalid Form Body", "errors": {"content": {"_errors": [{"code": "A"}, {"code": "B"}]}}}`,
			want: invalid + ": content: A, content: B",
		},
		{
			name: "more than five errors",
			body: `{"code": 50035, "message": "Invalid Form Body", "errors": {"a": {"_errors": [{"code": "A1"}, {"code": "A2"}, {"code": "A3"}]}, "b": {"_errors": [{"code": "B1"}, {"code": "B2"}, {"code": "B3"}]}}}`,
			want: invalid + ": a: A1, a: A2, a: A3, b: B1, b: B2",
		},
		{
			name: "errors that are null",
			body: `{"code": 50035, "message": "Invalid Form Body", "errors": null}`,
			want: invalid,
		},
		{
			name: "errors that are a list",
			body: `{"code": 50035, "message": "Invalid Form Body", "errors": [{"code": "A"}]}`,
			want: invalid,
		},
		{
			name: "member that is not an object",
			body: `{"code": 50035, "message": "Invalid Form Body", "errors": {"content": 5, "embeds": {"0": {"title": {"_errors": [{"code": "BASE_TYPE_REQUIRED"}]}}}}}`,
			want: invalid + ": embeds.0.title: BASE_TYPE_REQUIRED",
		},
		{
			name: "_errors that is not a list of objects",
			body: `{"code": 50035, "message": "Invalid Form Body", "errors": {"content": {"_errors": "x"}, "username": {"_errors": [{"code": 5}]}, "embeds": {"_errors": [{"code": "MAX_EMBEDS"}]}}}`,
			want: invalid + ": embeds: MAX_EMBEDS",
		},
		{
			name: "invalid UTF-8 ends the list",
			body: `{"code": 50035, "message": "Invalid Form Body", "errors": {"a": {"_errors": [{"code": "A"}]}, "b` + "\xff" + `": {"_errors": [{"code": "B"}]}}}`,
			want: invalid + ": a: A",
		},
		{
			name: "malformed entries among valid ones",
			body: `{"code": 50035, "message": "Invalid Form Body", "errors": {"content": {"_errors": [{"code": 5}, {"code": "BASE_TYPE_REQUIRED"}, "x", {"code": "BASE_TYPE_MAX_LENGTH"}]}}}`,
			want: invalid + ": content: BASE_TYPE_REQUIRED, content: BASE_TYPE_MAX_LENGTH",
		},
		{
			name: "entry without a code",
			body: `{"code": 50035, "message": "Invalid Form Body", "errors": {"content": {"_errors": [{"message": "Must be 2000 or fewer in length."}, {"code": "BASE_TYPE_MAX_LENGTH"}]}}}`,
			want: invalid + ": content: BASE_TYPE_MAX_LENGTH",
		},
		{
			name: "path and code with characters that are not printable",
			body: `{"code": 50035, "message": "Invalid Form Body", "errors": {"con\ntent": {"_errors": [{"code": "BASE\tTYPE"}]}}}`,
			want: invalid + ": con tent: BASE TYPE",
		},
		{
			name: "long list",
			body: `{"code": 50035, "message": "Invalid Form Body", "errors": {"` + strings.Repeat("p", 300) + `": {"_errors": [{"code": "X"}]}}}`,
			want: invalid + ": " + strings.Repeat("p", 194) + " [...]",
		},
		{
			name: "no code",
			body: `{"message": "Invalid Form Body", "errors": {"content": {"_errors": [{"code": "BASE_TYPE_REQUIRED"}]}}}`,
			want: "Invalid Form Body: content: BASE_TYPE_REQUIRED",
		},
		{
			name: "errors without a message",
			body: `{"code": 50035, "errors": {"content": {"_errors": [{"code": "BASE_TYPE_REQUIRED"}]}}}`,
			want: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			body, ok := readErrorBody(strings.NewReader(tt.body))
			require.True(t, ok, "the body decodes")

			assert.Equal(t, tt.want, body.reason())
		})
	}
}

// TestSendWithRetryRateLimitRetryAfter verifies that a 429 with Retry-After
// waits that long and then retries successfully.
func TestSendWithRetryRateLimitRetryAfter(t *testing.T) {
	t.Parallel()

	synctest.Test(t, func(t *testing.T) {
		client := mocks.NewMockHTTPClient(t)
		client.EXPECT().Do(mock.Anything).Return(statusResponse(http.StatusTooManyRequests, http.Header{"Retry-After": []string{"1"}}), nil).Once()
		client.EXPECT().Do(mock.Anything).Return(statusResponse(http.StatusNoContent, nil), nil).Once()

		sleeper := mocks.NewMockSleeper(t)
		sleeper.EXPECT().Sleep(time.Second).Return().Once()

		_, err := sendWithRetry(t.Context(), newJSONPreparer(), "http://example.com", client, sleeper, nil)
		require.NoError(t, err)
	})
}

// TestSendWithRetryMaxRetriesExceeded verifies that persistent server errors
// are retried with exponential backoff until ErrMaxRetries.
func TestSendWithRetryMaxRetriesExceeded(t *testing.T) {
	t.Parallel()

	synctest.Test(t, func(t *testing.T) {
		client := mocks.NewMockHTTPClient(t)
		client.EXPECT().Do(mock.Anything).RunAndReturn(func(*http.Request) (*http.Response, error) {
			return statusResponse(http.StatusInternalServerError, nil), nil
		}).Times(maxRetries + 1)

		sleeper, waits := newRecordingSleeper(t, maxRetries)

		_, err := sendWithRetry(t.Context(), newJSONPreparer(), "http://example.com", client, sleeper, nil)
		require.ErrorIs(t, err, ErrMaxRetries)
		assert.Equal(t, []time.Duration{time.Second, 2 * time.Second, 4 * time.Second, 8 * time.Second, 16 * time.Second}, *waits)
	})
}

// TestSendWithRetryRateLimitedUntilMaxRetries verifies that persistent rate
// limits are retried after each wait until ErrMaxRetries, with no wait after
// the last attempt.
func TestSendWithRetryRateLimitedUntilMaxRetries(t *testing.T) {
	t.Parallel()

	synctest.Test(t, func(t *testing.T) {
		client := mocks.NewMockHTTPClient(t)
		client.EXPECT().Do(mock.Anything).RunAndReturn(func(*http.Request) (*http.Response, error) {
			return statusResponse(http.StatusTooManyRequests, nil), nil
		}).Times(maxRetries + 1)

		sleeper, waits := newRecordingSleeper(t, maxRetries)

		_, err := sendWithRetry(t.Context(), newJSONPreparer(), "http://example.com", client, sleeper, nil)
		require.ErrorIs(t, err, ErrMaxRetries)
		assert.Equal(t, []time.Duration{time.Second, 2 * time.Second, 4 * time.Second, 8 * time.Second, 16 * time.Second}, *waits)
	})
}

// TestSendWithRetryRateLimitTooLong verifies that a Retry-After beyond the
// retry limit or the send deadline stops retrying with ErrRateLimited.
func TestSendWithRetryRateLimitTooLong(t *testing.T) {
	t.Parallel()

	// tests lists Retry-After values and send deadlines that the wait cannot
	// fit.
	tests := []struct {
		name       string
		retryAfter string
		deadline   time.Duration
	}{
		{name: "beyond the retry limit", retryAfter: "400"},
		{name: "beyond the send deadline", retryAfter: "65", deadline: 5 * time.Second},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			synctest.Test(t, func(t *testing.T) {
				ctx := t.Context()

				if tt.deadline > 0 {
					var cancel context.CancelFunc

					ctx, cancel = context.WithTimeout(ctx, tt.deadline)
					defer cancel()
				}

				client := mocks.NewMockHTTPClient(t)
				client.EXPECT().Do(mock.Anything).Return(statusResponse(http.StatusTooManyRequests, http.Header{"Retry-After": []string{tt.retryAfter}}), nil).Once()

				// The sleeper has no expectations, so a wait fails the test.
				_, err := sendWithRetry(ctx, newJSONPreparer(), "http://example.com", client, mocks.NewMockSleeper(t), nil)

				require.ErrorIs(t, err, ErrRateLimited)
				assert.Contains(t, err.Error(), "rate limited by Discord")
			})
		})
	}
}

// TestWaitWithTimeout verifies that a wait runs when the retry limit and the
// context deadline allow it, and otherwise fails without sleeping with
// ErrRateLimited or the context's own error.
func TestWaitWithTimeout(t *testing.T) {
	t.Parallel()

	// tests lists waits and contexts with the error to expect, or nil when the
	// sleeper should wait.
	tests := []struct {
		name     string
		wait     time.Duration
		context  func(context.Context) (context.Context, context.CancelFunc)
		wantErr  error
		notErr   error
		wantText string
	}{
		{name: "normal wait", wait: time.Second},
		{name: "zero wait", wait: 0},
		{
			name: "wait within the deadline",
			wait: time.Second,
			context: func(ctx context.Context) (context.Context, context.CancelFunc) {
				return context.WithTimeout(ctx, time.Minute)
			},
		},
		{name: "wait beyond the retry limit", wait: 6 * time.Minute, wantErr: ErrRateLimited},
		{
			name: "wait beyond the deadline",
			wait: 65 * time.Second,
			context: func(ctx context.Context) (context.Context, context.CancelFunc) {
				return context.WithTimeout(ctx, 5*time.Second)
			},
			wantErr:  ErrRateLimited,
			wantText: "send deadline",
		},
		{
			name: "expired context",
			wait: time.Second,
			context: func(ctx context.Context) (context.Context, context.CancelFunc) {
				return context.WithDeadline(ctx, time.Now().Add(-time.Second))
			},
			wantErr: context.DeadlineExceeded,
			notErr:  ErrRateLimited,
		},
		{
			name: "canceled context",
			wait: time.Second,
			context: func(ctx context.Context) (context.Context, context.CancelFunc) {
				ctx, cancel := context.WithCancel(ctx)
				cancel()

				return ctx, cancel
			},
			wantErr: context.Canceled,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			synctest.Test(t, func(t *testing.T) {
				ctx := t.Context()

				if tt.context != nil {
					var cancel context.CancelFunc

					ctx, cancel = tt.context(ctx)
					defer cancel()
				}

				sleeper := mocks.NewMockSleeper(t)
				if tt.wantErr == nil {
					sleeper.EXPECT().Sleep(tt.wait).Return().Once()
				}

				err := waitWithTimeout(ctx, tt.wait, time.Now(), sleeper)

				if tt.wantErr == nil {
					require.NoError(t, err)

					return
				}

				require.ErrorIs(t, err, tt.wantErr)

				if tt.notErr != nil {
					require.NotErrorIs(t, err, tt.notErr)
				}

				if tt.wantText != "" {
					assert.Contains(t, err.Error(), tt.wantText)
				}
			})
		})
	}
}

// TestSleepContext verifies that a context-aware sleeper stops when the context
// is canceled and that other sleepers are called with Sleep.
func TestSleepContext(t *testing.T) {
	t.Parallel()

	t.Run("real sleeper stops on cancel", func(t *testing.T) {
		t.Parallel()

		synctest.Test(t, func(t *testing.T) {
			ctx, cancel := context.WithCancel(t.Context())
			time.AfterFunc(10*time.Millisecond, cancel)

			start := time.Now()

			require.ErrorIs(t, sleepContext(ctx, RealSleeper{}, time.Minute), context.Canceled)
			assert.Equal(t, 10*time.Millisecond, time.Since(start))
		})
	})

	t.Run("sleeper without SleepContext", func(t *testing.T) {
		t.Parallel()

		sleeper := mocks.NewMockSleeper(t)
		sleeper.EXPECT().Sleep(time.Second).Return().Once()

		require.NoError(t, sleepContext(t.Context(), sleeper, time.Second))
	})
}

// TestHandleRateLimitResponse verifies the wait for a 429. Retry-After in whole
// seconds sets it. The body's retry_after in seconds sets it when the header
// is absent or invalid, such as a fraction. Exponential backoff applies when
// the body has no usable retry_after either. ErrRateLimited is returned
// when the wait exceeds the retry limit.
func TestHandleRateLimitResponse(t *testing.T) {
	t.Parallel()

	// tests lists Retry-After values, bodies, and attempts with the wait to
	// expect, or the error when no wait should happen.
	tests := []struct {
		wantErr    error
		name       string
		retryAfter string
		body       string
		attempt    int
		wantWait   time.Duration
	}{
		{name: "Retry-After", retryAfter: "2", wantWait: 2 * time.Second},
		{name: "invalid Retry-After", retryAfter: "invalid", wantWait: time.Second},
		{name: "fractional Retry-After", retryAfter: "1.5", wantWait: time.Second},
		{name: "body retry_after", body: `{"message":"You are being rate limited.","retry_after":2.5}`, wantWait: 2500 * time.Millisecond},
		{name: "header wins over body", retryAfter: "1", body: `{"retry_after":2.5}`, wantWait: time.Second},
		{name: "fractional header falls back to body", retryAfter: "1.5", body: `{"retry_after":3}`, wantWait: 3 * time.Second},
		{name: "zero body retry_after", body: `{"retry_after":0}`},
		{name: "body without retry_after", body: `{"message":"You are being rate limited."}`, wantWait: time.Second},
		{name: "null body retry_after", body: `{"retry_after":null}`, wantWait: time.Second},
		{name: "negative body retry_after", body: `{"retry_after":-1}`, wantWait: time.Second},
		{name: "invalid body", body: "not json", wantWait: time.Second},
		{name: "body retry_after beyond the retry limit", body: `{"retry_after":1e30}`, wantErr: ErrRateLimited},
		{name: "backoff", attempt: 2, wantWait: 4 * time.Second},
		{name: "longer backoff", attempt: 4, wantWait: 16 * time.Second},
		{name: "last attempt", attempt: maxRetries, wantErr: ErrMaxRetries},
		{name: "last attempt with Retry-After", retryAfter: "1", attempt: maxRetries, wantErr: ErrMaxRetries},
		{name: "Retry-After beyond the retry limit", retryAfter: "400", wantErr: ErrRateLimited},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			header := http.Header{}
			if tt.retryAfter != "" {
				header.Set("Retry-After", tt.retryAfter)
			}

			sleeper := mocks.NewMockSleeper(t)
			if tt.wantErr == nil {
				sleeper.EXPECT().Sleep(tt.wantWait).Return().Once()
			}

			res := statusResponse(http.StatusTooManyRequests, header)
			res.Body = io.NopCloser(strings.NewReader(tt.body))

			err := handleRateLimitResponse(t.Context(), res, tt.attempt, time.Now(), sleeper)

			if tt.wantErr == nil {
				require.NoError(t, err)

				return
			}

			require.ErrorIs(t, err, tt.wantErr)
		})
	}
}

// TestHandleRateLimitResponseDate verifies that a Retry-After holding an HTTP
// date waits until that date.
func TestHandleRateLimitResponseDate(t *testing.T) {
	t.Parallel()

	synctest.Test(t, func(t *testing.T) {
		header := http.Header{"Retry-After": []string{time.Now().Add(3 * time.Second).UTC().Format(http.TimeFormat)}}

		sleeper := mocks.NewMockSleeper(t)
		sleeper.EXPECT().Sleep(3 * time.Second).Return().Once()

		require.NoError(t, handleRateLimitResponse(t.Context(), statusResponse(http.StatusTooManyRequests, header), 0, time.Now(), sleeper))
	})
}

// TestBackoff verifies that the backoff doubles from 1 second with each attempt
// and stops at 64 seconds.
func TestBackoff(t *testing.T) {
	t.Parallel()

	// tests lists attempts with the backoff each should give.
	tests := []struct {
		attempt int
		want    time.Duration
	}{
		{attempt: 0, want: time.Second},
		{attempt: 1, want: 2 * time.Second},
		{attempt: 4, want: 16 * time.Second},
		{attempt: 6, want: 64 * time.Second},
		{attempt: 10, want: 64 * time.Second},
	}

	for _, tt := range tests {
		assert.Equal(t, tt.want, backoff(tt.attempt), "attempt %d", tt.attempt)
	}
}

// TestIsTransientError verifies which errors count as transient.
func TestIsTransientError(t *testing.T) {
	t.Parallel()

	// tests lists errors with whether they are transient.
	tests := []struct {
		name     string
		err      error
		expected bool
	}{
		{name: "context canceled", err: context.Canceled, expected: false},
		{name: "context deadline exceeded", err: context.DeadlineExceeded, expected: false},
		{name: "timeout network error", err: &mockTimeoutError{}, expected: false},
		{name: "temporary network error that is not a timeout", err: &mockTemporaryError{}, expected: false},
		{name: "refused connection named only in text", err: errors.New("dial tcp 127.0.0.1:8080: connect: connection refused"), expected: false},
		{name: "reset connection named only in text", err: errors.New("read tcp 127.0.0.1:8080: connection reset by peer"), expected: false},
		{name: "deadline in a request error", err: &url.Error{Op: "Post", URL: "http://example.com", Err: context.DeadlineExceeded}, expected: false},
		{
			name: "dial timeout in a request error",
			err: &url.Error{Op: "Post", URL: "http://example.com", Err: &net.OpError{
				Op: "dial", Net: "tcp", Err: &mockTimeoutError{},
			}},
			expected: true,
		},
		{
			name: "read timeout in a request error",
			err: &url.Error{Op: "Post", URL: "http://example.com", Err: &net.OpError{
				Op: "read", Net: "tcp", Err: &mockTimeoutError{},
			}},
			expected: false,
		},
		{
			name: "refused dial in a request error",
			err: &url.Error{Op: "Post", URL: "http://example.com", Err: &net.OpError{
				Op: "dial", Net: "tcp", Err: errors.New("connect: connection refused"),
			}},
			expected: true,
		},
		{
			name: "reset read in a request error",
			err: &url.Error{Op: "Post", URL: "http://example.com", Err: &net.OpError{
				Op: "read", Net: "tcp", Err: errors.New("connection reset by peer"),
			}},
			expected: false,
		},
		{
			name: "broken write in a request error",
			err: &url.Error{Op: "Post", URL: "http://example.com", Err: &net.OpError{
				Op: "write", Net: "tcp", Err: errors.New("broken pipe"),
			}},
			expected: false,
		},
		{
			name: "canceled dial in a request error",
			err: &url.Error{Op: "Post", URL: "http://example.com", Err: &net.OpError{
				Op: "dial", Net: "tcp", Err: fmt.Errorf("dialing: %w", context.Canceled),
			}},
			expected: false,
		},
		{
			name: "TLS alert in a request error",
			err: &url.Error{Op: "Post", URL: "http://example.com", Err: &net.OpError{
				Op: "remote error", Err: errors.New("tls: handshake failure"),
			}},
			expected: false,
		},
		{name: "other request error", err: &url.Error{Op: "Post", URL: "http://example.com", Err: errors.New("bad gateway")}, expected: false},
		{name: "generic error", err: errors.New("some other error"), expected: false},
		{name: "nil error", err: nil, expected: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			assert.Equal(t, tt.expected, isTransientError(tt.err))
		})
	}
}

// TestIsTransientErrorConnections verifies on loopback, whatever errors the
// operating system running the test reports, that a refused dial is
// transient. A canceled dial and a reset read are not, since the webhook POST
// may have reached Discord before a read failed.
func TestIsTransientErrorConnections(t *testing.T) {
	t.Parallel()

	// The dial goes to a port whose listener was closed. Another test can take
	// the freed port before the dial and accept it, so the dial is tried on a
	// new port each time it connects, up to 10 times.
	t.Run("refused dial", func(t *testing.T) {
		t.Parallel()

		var err error

		for range 10 {
			listener, listenErr := (&net.ListenConfig{}).Listen(t.Context(), "tcp", "127.0.0.1:0")
			require.NoError(t, listenErr)

			addr := listener.Addr().String()
			require.NoError(t, listener.Close())

			var conn net.Conn

			conn, err = (&net.Dialer{}).DialContext(t.Context(), "tcp", addr)
			if err != nil {
				break
			}

			_ = conn.Close()
		}

		require.Error(t, err, "nothing listens on the port")
		assert.True(t, isTransientError(err), "%v is transient", err)
	})

	t.Run("canceled dial", func(t *testing.T) {
		t.Parallel()

		ctx, cancel := context.WithCancel(t.Context())
		cancel()

		conn, err := (&net.Dialer{}).DialContext(ctx, "tcp", "127.0.0.1:1")
		if conn != nil {
			_ = conn.Close()
		}

		require.Error(t, err, "the dial's context is canceled")
		assert.False(t, isTransientError(err), "%v is not transient", err)
	})

	t.Run("reset read", func(t *testing.T) {
		t.Parallel()

		listener, err := (&net.ListenConfig{}).Listen(t.Context(), "tcp", "127.0.0.1:0")
		require.NoError(t, err)

		defer func() { _ = listener.Close() }()

		accepted := make(chan net.Conn, 1)

		go func() {
			conn, err := listener.Accept()
			if err != nil {
				close(accepted)

				return
			}

			accepted <- conn
		}()

		client, err := (&net.Dialer{}).DialContext(t.Context(), "tcp", listener.Addr().String())
		require.NoError(t, err)

		defer func() { _ = client.Close() }()

		server, ok := (<-accepted).(*net.TCPConn)
		require.True(t, ok, "the listener accepts the connection")

		// Closing with a linger of zero sends a reset instead of a close.
		require.NoError(t, server.SetLinger(0))
		require.NoError(t, server.Close())

		_, err = client.Read(make([]byte, 1))

		require.Error(t, err, "the server reset the connection")
		assert.False(t, isTransientError(err), "%v is not transient", err)
	})
}

// TestExecuteWithTransportRetry verifies that transient transport errors are
// retried with exponential backoff, that the last error is returned once the
// retries run out, and that a context canceled before the first request stops
// the retry at the pre-wait check. TestExecuteWithTransportRetryStopsOnCancel
// covers a cancel during the backoff.
func TestExecuteWithTransportRetry(t *testing.T) {
	t.Parallel()

	refused := &net.OpError{Op: "dial", Net: "tcp", Err: errors.New("connect: connection refused")}

	// tests lists the client's replies with the backoff waits and error to
	// expect.
	tests := []struct {
		name          string
		replies       []error
		cancelContext bool
		wantSleeps    []time.Duration
		wantErr       error
	}{
		{name: "success on the first attempt", replies: []error{nil}},
		{name: "success after a refused dial", replies: []error{refused, nil}, wantSleeps: []time.Duration{time.Second}},
		{
			name:       "refused dials until the retries run out",
			replies:    []error{refused, refused, refused, refused},
			wantSleeps: []time.Duration{time.Second, 2 * time.Second, 4 * time.Second},
			wantErr:    refused,
		},
		{name: "deadline not retried", replies: []error{context.DeadlineExceeded}, wantErr: context.DeadlineExceeded},
		{name: "context canceled before the first request", replies: []error{refused}, cancelContext: true, wantErr: context.Canceled},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			synctest.Test(t, func(t *testing.T) {
				client := mocks.NewMockHTTPClient(t)

				for _, reply := range tt.replies {
					if reply == nil {
						client.EXPECT().Do(mock.Anything).Return(statusResponse(http.StatusOK, nil), nil).Once()
					} else {
						client.EXPECT().Do(mock.Anything).Return(nil, reply).Once()
					}
				}

				sleeper := mocks.NewMockSleeper(t)
				for _, wait := range tt.wantSleeps {
					sleeper.EXPECT().Sleep(wait).Return().Once()
				}

				ctx, cancel := context.WithCancel(t.Context())
				defer cancel()

				if tt.cancelContext {
					cancel()
				}

				req, err := http.NewRequestWithContext(ctx, http.MethodPost, "http://example.com", strings.NewReader("test"))
				require.NoError(t, err)

				resp, err := executeWithTransportRetry(ctx, client, req, sleeper)
				if resp != nil {
					_ = resp.Body.Close()
				}

				if tt.wantErr == nil {
					require.NoError(t, err)

					return
				}

				require.ErrorIs(t, err, tt.wantErr)
			})
		})
	}
}

// TestExecuteWithTransportRetryStopsOnCancel verifies that a real backoff wait
// ends as soon as the context is canceled.
func TestExecuteWithTransportRetryStopsOnCancel(t *testing.T) {
	t.Parallel()

	synctest.Test(t, func(t *testing.T) {
		client := mocks.NewMockHTTPClient(t)
		client.EXPECT().Do(mock.Anything).Return(nil, &net.OpError{Op: "dial", Net: "tcp", Err: errors.New("connect: connection refused")}).Once()

		ctx, cancel := context.WithCancel(t.Context())
		time.AfterFunc(10*time.Millisecond, cancel)

		req, err := http.NewRequestWithContext(ctx, http.MethodPost, "http://example.com", http.NoBody)
		require.NoError(t, err)

		start := time.Now()

		_, err = executeWithTransportRetry(ctx, client, req, RealSleeper{})

		require.ErrorIs(t, err, context.Canceled)
		assert.Equal(t, 10*time.Millisecond, time.Since(start), "the one second backoff ends at the cancel")
	})
}

// TestExecuteWithTransportRetryCanceledDial verifies that a dial its own
// context canceled is not retried while the send's context is still active.
func TestExecuteWithTransportRetryCanceledDial(t *testing.T) {
	t.Parallel()

	canceled := &url.Error{Op: "Post", URL: "http://example.com", Err: &net.OpError{
		Op: "dial", Net: "tcp", Err: fmt.Errorf("dialing: %w", context.Canceled),
	}}

	client := mocks.NewMockHTTPClient(t)
	client.EXPECT().Do(mock.Anything).Return(nil, canceled).Once()

	req, err := http.NewRequestWithContext(t.Context(), http.MethodPost, "http://example.com", http.NoBody)
	require.NoError(t, err)

	// The sleeper has no expectations, so a retry's wait fails the test.
	_, err = executeWithTransportRetry(t.Context(), client, req, mocks.NewMockSleeper(t))

	require.ErrorIs(t, err, context.Canceled)
	require.NoError(t, t.Context().Err(), "the send's context is still active")
}

// TestExecuteWithTransportRetryClientTimeout verifies that a client's own
// timeout while it awaits Discord's answer is not retried, since the webhook
// POST already reached the server.
func TestExecuteWithTransportRetryClientTimeout(t *testing.T) {
	t.Parallel()

	var requests atomic.Int32

	release := make(chan struct{})
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		requests.Add(1)
		<-release
		w.WriteHeader(http.StatusNoContent)
	}))

	defer server.Close()
	defer close(release)

	// The timeout leaves the request ample time to reach the server, even
	// under the load of the whole suite, before the client gives up.
	client := &http.Client{Timeout: time.Second}

	req, err := http.NewRequestWithContext(t.Context(), http.MethodPost, server.URL, strings.NewReader("test"))
	require.NoError(t, err)

	// The sleeper has no expectations, so a retry's wait fails the test.
	resp, err := executeWithTransportRetry(t.Context(), client, req, mocks.NewMockSleeper(t))
	if resp != nil {
		_ = resp.Body.Close()
	}

	require.ErrorIs(t, err, context.DeadlineExceeded, "the client's timeout ends the request")
	assert.Equal(t, int32(1), requests.Load(), "the POST reached the server once")
}

// TestRetryRequest verifies that a retry gets a copy of the request with the
// same URL and headers and a fresh body. A request without a body to replay
// keeps the body it has, and a failed replay ends the retry with its error.
func TestRetryRequest(t *testing.T) {
	t.Parallel()

	t.Run("fresh body", func(t *testing.T) {
		t.Parallel()

		req, err := http.NewRequestWithContext(t.Context(), http.MethodPost, testWebhookURL, strings.NewReader("payload"))
		require.NoError(t, err)
		req.Header.Set("Content-Type", "application/json")

		_, err = io.ReadAll(req.Body)
		require.NoError(t, err)

		next, err := retryRequest(t.Context(), req)
		require.NoError(t, err)
		assert.NotSame(t, req, next)
		assert.Equal(t, testWebhookURL, next.URL.String())
		assert.Equal(t, "application/json", next.Header.Get("Content-Type"))

		body, err := io.ReadAll(next.Body)
		require.NoError(t, err)
		assert.Equal(t, "payload", string(body))
	})

	t.Run("no body to replay", func(t *testing.T) {
		t.Parallel()

		req, err := http.NewRequestWithContext(t.Context(), http.MethodPost, testWebhookURL, http.NoBody)
		require.NoError(t, err)

		req.GetBody = nil

		next, err := retryRequest(t.Context(), req)
		require.NoError(t, err)
		assert.Equal(t, http.NoBody, next.Body)
	})

	t.Run("body that cannot be replayed", func(t *testing.T) {
		t.Parallel()

		req, err := http.NewRequestWithContext(t.Context(), http.MethodPost, testWebhookURL, strings.NewReader("payload"))
		require.NoError(t, err)

		errReplay := errors.New("replay failed")
		req.GetBody = func() (io.ReadCloser, error) { return nil, errReplay }

		_, err = retryRequest(t.Context(), req)
		require.ErrorIs(t, err, errReplay)
	})
}

// TestHandleServerError verifies that server errors wait with exponential
// backoff, other statuses do not wait, and the last attempt returns
// ErrMaxRetries.
func TestHandleServerError(t *testing.T) {
	t.Parallel()

	// tests lists statuses and attempts with the wait to expect, or zero for
	// none.
	tests := []struct {
		name       string
		statusCode int
		attempt    int
		wantWait   time.Duration
		wantErr    error
	}{
		{name: "status below 500", statusCode: http.StatusNotFound},
		{name: "500", statusCode: http.StatusInternalServerError, wantWait: time.Second},
		{name: "last attempt", statusCode: http.StatusInternalServerError, attempt: maxRetries, wantErr: ErrMaxRetries},
		{name: "backoff", statusCode: http.StatusBadGateway, attempt: 2, wantWait: 4 * time.Second},
		{name: "longer backoff", statusCode: http.StatusServiceUnavailable, attempt: 4, wantWait: 16 * time.Second},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			sleeper := mocks.NewMockSleeper(t)
			if tt.wantWait > 0 {
				sleeper.EXPECT().Sleep(tt.wantWait).Return().Once()
			}

			err := handleServerError(t.Context(), statusResponse(tt.statusCode, nil), tt.attempt, time.Now(), sleeper)

			if tt.wantErr == nil {
				require.NoError(t, err)

				return
			}

			require.ErrorIs(t, err, tt.wantErr)
		})
	}
}

// TestPrepareRequestRedactsUnparsableURL verifies that both request preparers
// report an unparsable webhook URL without the token in the error.
func TestPrepareRequestRedactsUnparsableURL(t *testing.T) {
	t.Parallel()

	const postURL = testHooksBaseURL + "/123/SECRETtoken%zz"

	// preparers lists the request preparers under test.
	preparers := map[string]RequestPreparer{
		"JSON":      newJSONPreparer(),
		"multipart": &MultipartRequestPreparer{payload: &WebhookPayload{Content: "message"}, files: nil},
	}

	for name, preparer := range preparers {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			_, err := preparer.PrepareRequest(t.Context(), postURL)

			_, isURLErr := errors.AsType[*url.Error](err)
			require.True(t, isURLErr, "error %v is not a *url.Error", err)
			assert.NotContains(t, err.Error(), "SECRETtoken")
		})
	}
}
