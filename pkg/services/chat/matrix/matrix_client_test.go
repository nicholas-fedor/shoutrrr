package matrix

import (
	"bytes"
	"context"
	"encoding/json/jsontext"
	"encoding/json/v2"
	"errors"
	"io"
	"log"
	"maps"
	"math"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"testing"
	"testing/synctest"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/nicholas-fedor/shoutrrr/internal/httpbody"
	"github.com/nicholas-fedor/shoutrrr/pkg/services/standard"
)

// failingTransport is a transport whose requests fail with err.
type failingTransport struct {
	// err is the error every request fails with.
	err error
}

// reply is one scripted answer of a scriptedHTTPClient.
type reply struct {
	// status is the HTTP status code of the response.
	status int
	// body is the response body.
	body string
	// rawBody, when set, is used as the response body instead of body.
	rawBody io.ReadCloser
	// err, when set, is returned as the transport error instead of a response.
	err error
	// header, when set, holds response headers besides the JSON Content-Type.
	header http.Header
}

// scriptedHTTPClient answers requests with scripted replies, in order, and
// records each request with its body.
type scriptedHTTPClient struct {
	// t is the test that owns the client.
	t *testing.T
	// mu guards the fields below.
	mu sync.Mutex
	// replies holds the replies not yet used.
	replies []reply
	// requests records each request received.
	requests []*http.Request
	// bodies records the body of each request received.
	bodies []string
}

// errorReadCloser is a response body whose reads fail.
type errorReadCloser struct{}

// errUnexpectedRequest answers a request after the scripted replies run out.
var errUnexpectedRequest = errors.New("unexpected request")

// Do answers req with the next scripted reply. A request whose context has
// ended uses up its reply and returns the context's error.
//
// Parameters:
//   - req: the request.
//
// Returns:
//   - *http.Response: the scripted response.
//   - error: the scripted transport error or the context's error.
func (s *scriptedHTTPClient) Do(req *http.Request) (*http.Response, error) {
	var body []byte
	if req.Body != nil {
		body, _ = io.ReadAll(req.Body)
		_ = req.Body.Close()
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	s.requests = append(s.requests, req)
	s.bodies = append(s.bodies, string(body))

	if len(s.replies) == 0 {
		s.t.Errorf("unexpected request: %s %s", req.Method, req.URL.Path)

		return nil, errUnexpectedRequest
	}

	next := s.replies[0]
	s.replies = s.replies[1:]

	if err := req.Context().Err(); err != nil {
		return nil, err
	}

	if next.err != nil {
		return nil, next.err
	}

	responseBody := next.rawBody
	if responseBody == nil {
		responseBody = io.NopCloser(strings.NewReader(next.body))
	}

	header := http.Header{"Content-Type": []string{"application/json"}}
	maps.Copy(header, next.header)

	return &http.Response{
		StatusCode: next.status,
		Status:     http.StatusText(next.status),
		Header:     header,
		Body:       responseBody,
	}, nil
}

// requestCount returns the number of requests received.
//
// Returns:
//   - int: the number of requests.
func (s *scriptedHTTPClient) requestCount() int {
	s.mu.Lock()
	defer s.mu.Unlock()

	return len(s.requests)
}

// Close does nothing.
//
// Returns:
//   - error: nil.
func (errorReadCloser) Close() error {
	return nil
}

// Read always fails.
//
// Parameters:
//   - []byte: unused.
//
// Returns:
//   - int: zero.
//   - error: a read error.
func (errorReadCloser) Read([]byte) (int, error) {
	return 0, errors.New("read error")
}

// ok returns a successful reply with body.
//
// Parameters:
//   - body: the response body.
//
// Returns:
//   - reply: the reply.
func ok(body string) reply {
	return reply{status: http.StatusOK, body: body}
}

// failed returns a reply whose request fails with a transport error.
//
// Parameters:
//   - message: the transport error message.
//
// Returns:
//   - reply: the reply.
func failed(message string) reply {
	return reply{err: errors.New(message)}
}

// newScriptedClient returns a client for matrix.example.com whose requests get
// replies in order. The test fails if a request arrives after the replies run
// out or if any reply is left unused.
//
// Parameters:
//   - t: the test.
//   - replies: the replies, in the order requests should receive them.
//
// Returns:
//   - *client: the client.
//   - *scriptedHTTPClient: the HTTP client, for inspecting requests.
func newScriptedClient(t *testing.T, replies ...reply) (*client, *scriptedHTTPClient) {
	t.Helper()

	httpClient := &scriptedHTTPClient{t: t, replies: replies}

	t.Cleanup(func() {
		httpClient.mu.Lock()
		defer httpClient.mu.Unlock()

		if len(httpClient.replies) > 0 {
			t.Errorf("%d scripted replies were not used", len(httpClient.replies))
		}
	})

	logger := &standard.Logger{}
	logger.SetLogger(log.New(t.Output(), "", 0))

	return &client{
		apiURL:     url.URL{Scheme: "https", Host: "matrix.example.com"},
		httpClient: httpClient,
		logger:     logger,
	}, httpClient
}

// loginRequest returns the login request the client sent, after checking that
// it was the only request and went to the login endpoint.
//
// Parameters:
//   - t: the test.
//   - httpClient: the HTTP client that recorded the request.
//
// Returns:
//   - apiReqLogin: the decoded login request.
func loginRequest(t *testing.T, httpClient *scriptedHTTPClient) apiReqLogin {
	t.Helper()

	require.Len(t, httpClient.requests, 1)
	assert.Equal(t, http.MethodPost, httpClient.requests[0].Method)
	assert.Equal(t, apiLogin, httpClient.requests[0].URL.Path)

	var request apiReqLogin
	require.NoError(t, json.Unmarshal([]byte(httpClient.bodies[0]), &request))

	return request
}

// TestNewClient verifies that the client sends to the host with the given
// scheme through the given HTTP client, and that a nil logger is replaced.
func TestNewClient(t *testing.T) {
	t.Parallel()

	httpClient := &scriptedHTTPClient{t: t}
	c := newClient("matrix.example.com", "http", httpClient, nil)

	assert.Equal(t, url.URL{Scheme: "http", Host: "matrix.example.com"}, c.apiURL)
	assert.Same(t, httpClient, c.httpClient)
	assert.NotNil(t, c.logger)
	assert.Empty(t, c.accessToken)
}

// TestAPIRequestFailures verifies that requests with and without a body report
// transport errors, Matrix API errors, error statuses without a Matrix error,
// invalid JSON, and unreadable bodies. A body over [httpbody.MaxSize] still yields the
// Matrix API error of an error status, and fails a success status as too large.
func TestAPIRequestFailures(t *testing.T) {
	t.Parallel()

	// tests lists replies with the error each should produce. wantMessage is
	// the Matrix API error message when the reply carries one.
	tests := []struct {
		name        string
		reply       reply
		wantErr     error
		wantText    string
		wantMessage string
	}{
		{name: "transport error", reply: failed("mock error"), wantText: "executing"},
		{
			name:        "Matrix API error",
			reply:       reply{status: http.StatusBadRequest, body: `{"error":"invalid request","errcode":"M_INVALID_PARAM"}`},
			wantMessage: "invalid request",
		},
		{name: "error status without a Matrix error", reply: reply{status: http.StatusInternalServerError, body: "internal server error"}, wantErr: ErrUnexpectedStatus},
		{name: "invalid JSON", reply: ok("not valid json"), wantText: "unmarshaling"},
		{name: "unreadable body", reply: reply{status: http.StatusOK, rawBody: errorReadCloser{}}, wantText: "reading"},
		{name: "unreadable error body", reply: reply{status: http.StatusInternalServerError, rawBody: errorReadCloser{}}, wantText: "reading"},
		{
			name: "oversized Matrix API error",
			reply: reply{
				status: http.StatusBadRequest,
				body:   `{"error":"invalid request","errcode":"M_INVALID_PARAM"}` + strings.Repeat(" ", httpbody.MaxSize),
			},
			wantMessage: "invalid request",
		},
		{name: "oversized success body", reply: ok(`{}` + strings.Repeat(" ", httpbody.MaxSize)), wantErr: httpbody.ErrTooLarge, wantText: "reading"},
	}

	// methods lists a GET request without a body and a POST request with one.
	methods := map[string]func(*client, context.Context) error{
		"GET": func(c *client, ctx context.Context) error {
			return c.apiDo(ctx, http.MethodGet, apiLogin, nil, &apiResLogin{})
		},
		"POST": func(c *client, ctx context.Context) error {
			return c.apiDo(ctx, http.MethodPost, apiLogin, &struct{}{}, &apiResLogin{})
		},
	}

	for method, call := range methods {
		for _, tt := range tests {
			t.Run(method+" "+tt.name, func(t *testing.T) {
				t.Parallel()

				c, _ := newScriptedClient(t, tt.reply)
				err := call(c, t.Context())

				require.Error(t, err)

				if tt.wantErr != nil {
					require.ErrorIs(t, err, tt.wantErr)
				}

				if tt.wantText != "" {
					assert.Contains(t, err.Error(), tt.wantText)
				}

				if tt.wantMessage != "" {
					apiErr, isAPIErr := errors.AsType[*apiResError](err)
					require.True(t, isAPIErr, "the error should be the Matrix API error")
					assert.Equal(t, tt.wantMessage, apiErr.Message)
				}
			})
		}
	}
}

// TestAPIDo verifies that apiDo reports marshaling and request creation errors
// without sending, sends a request value as JSON with the given method, sends
// raw JSON byte for byte, and sends no body for a nil request.
func TestAPIDo(t *testing.T) {
	t.Parallel()

	t.Run("unmarshalable request", func(t *testing.T) {
		t.Parallel()

		c, _ := newScriptedClient(t)
		err := c.apiDo(t.Context(), http.MethodPost, apiLogin, make(chan string), &apiResLogin{})

		require.Error(t, err)
		assert.Contains(t, err.Error(), "marshaling")
	})

	t.Run("invalid method", func(t *testing.T) {
		t.Parallel()

		c, _ := newScriptedClient(t)
		err := c.apiDo(t.Context(), "BAD METHOD", apiLogin, nil, &apiResLogin{})

		require.Error(t, err)
		assert.Contains(t, err.Error(), "creating")
	})

	t.Run("JSON request", func(t *testing.T) {
		t.Parallel()

		c, httpClient := newScriptedClient(t, ok(`{}`))
		request := apiReqSend{MsgType: msgTypeText, Body: "test message"}
		require.NoError(t, c.apiDo(t.Context(), http.MethodPut, "/_matrix/client/v3/rooms/!room/send/m.room.message/txn", request, &apiResEvent{}))

		require.Len(t, httpClient.requests, 1)
		assert.Equal(t, http.MethodPut, httpClient.requests[0].Method)
		assert.Equal(t, "application/json", httpClient.requests[0].Header.Get("Content-Type"))
		assert.JSONEq(t, `{"msgtype":"m.text","body":"test message"}`, httpClient.bodies[0])
	})

	t.Run("raw JSON request", func(t *testing.T) {
		t.Parallel()

		c, httpClient := newScriptedClient(t, ok(`{}`))
		require.NoError(t, c.apiDo(t.Context(), http.MethodPost, apiLogin, jsontext.Value(`{ "kept": "as is" }`), &apiResLogin{}))

		require.Len(t, httpClient.requests, 1)
		assert.Equal(t, "application/json", httpClient.requests[0].Header.Get("Content-Type"))
		//nolint:testifylint // The bytes must match exactly, which JSONEq would not check.
		assert.Equal(t, `{ "kept": "as is" }`, httpClient.bodies[0], "the request is sent byte for byte")
	})

	t.Run("no request body", func(t *testing.T) {
		t.Parallel()

		c, httpClient := newScriptedClient(t, ok(`{}`))
		require.NoError(t, c.apiDo(t.Context(), http.MethodGet, apiJoinedRooms, nil, &apiResRoom{}))

		require.Len(t, httpClient.requests, 1)
		assert.Equal(t, http.MethodGet, httpClient.requests[0].Method)
		assert.Empty(t, httpClient.requests[0].Header.Get("Content-Type"))
		assert.Empty(t, httpClient.bodies[0])
	})
}

// rateLimited returns a 429 reply in the Matrix error format. A non-empty
// retryAfter sets the Retry-After header.
//
// Parameters:
//   - retryAfter: the Retry-After header value, or empty for none.
//
// Returns:
//   - reply: the reply.
func rateLimited(retryAfter string) reply {
	var header http.Header
	if retryAfter != "" {
		header = http.Header{"Retry-After": []string{retryAfter}}
	}

	return reply{
		status: http.StatusTooManyRequests,
		body:   `{"errcode":"M_LIMIT_EXCEEDED","error":"Too many requests"}`,
		header: header,
	}
}

// TestAPIDoRetriesRateLimits verifies that a rate-limited request is sent again
// with the same body after the wait the homeserver asks for. [TestRetryWait]
// covers how the wait is read.
func TestAPIDoRetriesRateLimits(t *testing.T) {
	t.Parallel()

	synctest.Test(t, func(t *testing.T) {
		start := time.Now()
		request := apiReqSend{MsgType: msgTypeText, Body: "test message"}

		c, httpClient := newScriptedClient(t, rateLimited("2"), ok(`{"event_id":"$event"}`))
		require.NoError(t, c.apiDo(t.Context(), http.MethodPut, apiSendMessage, request, &apiResEvent{}))

		assert.Equal(t, 2*time.Second, time.Since(start))
		require.Equal(t, 2, httpClient.requestCount())
		assert.Equal(t, httpClient.bodies[0], httpClient.bodies[1], "the retry carries the same body")
	})
}

// TestAPIDoRetriesProxyRateLimit verifies that a 429 whose body is not a
// Matrix error, as a proxy in front of the homeserver can send, is retried
// after the wait of its Retry-After header.
func TestAPIDoRetriesProxyRateLimit(t *testing.T) {
	t.Parallel()

	synctest.Test(t, func(t *testing.T) {
		start := time.Now()
		proxyLimit := reply{
			status: http.StatusTooManyRequests,
			body:   "<html>Too Many Requests</html>",
			header: http.Header{"Retry-After": []string{"3"}},
		}

		c, httpClient := newScriptedClient(t, proxyLimit, ok(`{"joined_rooms":[]}`))
		require.NoError(t, c.apiDo(t.Context(), http.MethodGet, apiJoinedRooms, nil, &apiResJoinedRooms{}))

		assert.Equal(t, 3*time.Second, time.Since(start))
		assert.Equal(t, 2, httpClient.requestCount())
	})
}

// TestAPIDoFailsRateLimitAtOnce verifies that a request fails at once with the
// Matrix error when the homeserver gives no usable wait, when the wait would
// end after the context's deadline, and for an error status other than 429
// that carries a wait.
func TestAPIDoFailsRateLimitAtOnce(t *testing.T) {
	t.Parallel()

	// tests lists the reply and the context's deadline, or zero for none.
	tests := []struct {
		name     string
		reply    reply
		deadline time.Duration
	}{
		{name: "no hint", reply: rateLimited("")},
		{name: "past the deadline", reply: rateLimited("2"), deadline: time.Second},
		{
			name: "other error status",
			reply: reply{
				status: http.StatusServiceUnavailable,
				body:   `{"errcode":"M_UNKNOWN","error":"Unavailable"}`,
				header: http.Header{"Retry-After": []string{"1"}},
			},
		},
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

				start := time.Now()
				c, httpClient := newScriptedClient(t, tt.reply)

				err := c.apiDo(ctx, http.MethodGet, apiJoinedRooms, nil, &apiResRoom{})

				_, isAPIErr := errors.AsType[*apiResError](err)
				assert.True(t, isAPIErr, "error %v is not the Matrix error", err)
				assert.Equal(t, 1, httpClient.requestCount())
				assert.Zero(t, time.Since(start))
			})
		})
	}
}

// TestAPIDoRateLimitBounds verifies that a wait for a rate limit stops when the
// context ends with an error that keeps the Matrix error, and that a request rate-limited on every attempt fails after
// three retries.
func TestAPIDoRateLimitBounds(t *testing.T) {
	t.Parallel()

	t.Run("context ends during the wait", func(t *testing.T) {
		t.Parallel()

		synctest.Test(t, func(t *testing.T) {
			ctx, cancel := context.WithCancel(t.Context())
			time.AfterFunc(time.Second, cancel)

			start := time.Now()
			c, httpClient := newScriptedClient(t, rateLimited("5"))

			err := c.apiDo(ctx, http.MethodGet, apiJoinedRooms, nil, &apiResRoom{})
			require.ErrorIs(t, err, context.Canceled)

			_, isAPIErr := errors.AsType[*apiResError](err)
			assert.True(t, isAPIErr, "error %v does not carry the Matrix error", err)
			assert.Equal(t, 1, httpClient.requestCount())
			assert.Equal(t, time.Second, time.Since(start))
		})
	})

	t.Run("gives up after three retries", func(t *testing.T) {
		t.Parallel()

		synctest.Test(t, func(t *testing.T) {
			start := time.Now()
			c, httpClient := newScriptedClient(t,
				rateLimited("1"), rateLimited("1"), rateLimited("1"), rateLimited("1"))

			_, isAPIErr := errors.AsType[*apiResError](c.apiDo(t.Context(), http.MethodGet, apiJoinedRooms, nil, &apiResRoom{}))
			assert.True(t, isAPIErr)
			assert.Equal(t, 4, httpClient.requestCount())
			assert.Equal(t, 3*time.Second, time.Since(start))
		})
	})
}

// TestAPIDoLogsRateLimitWait verifies that a wait for a rate limit logs a
// Debug record with the wait.
func TestAPIDoLogsRateLimitWait(t *testing.T) {
	t.Parallel()

	synctest.Test(t, func(t *testing.T) {
		var logs strings.Builder

		c, _ := newScriptedClient(t, rateLimited("2"), ok(`{"joined_rooms":[]}`))
		c.logger = &standard.Logger{}
		c.logger.SetLogger(log.New(&logs, "", 0))

		require.NoError(t, c.apiDo(t.Context(), http.MethodGet, apiJoinedRooms, nil, &apiResRoom{}))
		assert.Equal(t, "DEBUG rate limited, waiting to retry wait=2s\n", logs.String())
	})
}

// TestRetryWait verifies that a usable Retry-After header sets the wait, even a
// zero wait from 0 seconds or a past date, and that the deprecated
// retry_after_ms is the fallback when the header is missing or unusable. A retry_after_ms too long for a duration becomes the
// longest wait. [httpclient.RetryAfter] covers how the header is read.
func TestRetryWait(t *testing.T) {
	t.Parallel()

	// tests lists a Retry-After value and a retry_after_ms value with the wait
	// each should give.
	tests := []struct {
		name         string
		header       string
		retryAfterMS int64
		want         time.Duration
	}{
		{name: "header", header: "7", want: 7 * time.Second},
		{name: "header wins over body", header: "1", retryAfterMS: 5000, want: time.Second},
		{name: "zero header wins over body", header: "0", retryAfterMS: 5000, want: 0},
		{name: "past date wins over body", header: time.Now().Add(-time.Hour).UTC().Format(http.TimeFormat), retryAfterMS: 5000, want: 0},
		{name: "unusable header falls back to body", header: "-0", retryAfterMS: 300, want: 300 * time.Millisecond},
		{name: "unusable header without body", header: "later", want: noRetry},
		{name: "body only", retryAfterMS: 2500, want: 2500 * time.Millisecond},
		{name: "huge body", retryAfterMS: math.MaxInt64, want: math.MaxInt64},
		{name: "nothing", want: noRetry},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			header := http.Header{}
			if tt.header != "" {
				header.Set("Retry-After", tt.header)
			}

			assert.Equal(t, tt.want, retryWait(header, &apiResError{RetryAfterMS: tt.retryAfterMS}))
		})
	}
}

// TestAPIDoContextErrors verifies that apiDo returns the context's error when
// the context ends before or during the request.
func TestAPIDoContextErrors(t *testing.T) {
	t.Parallel()

	t.Run("canceled before the request", func(t *testing.T) {
		t.Parallel()

		ctx, cancel := context.WithCancel(t.Context())
		cancel()

		c, _ := newScriptedClient(t, ok(`{}`))
		require.ErrorIs(t, c.apiDo(ctx, http.MethodPost, apiLogin, nil, &apiResLogin{}), context.Canceled)
	})

	t.Run("canceled during the request", func(t *testing.T) {
		t.Parallel()

		c, _ := newScriptedClient(t, reply{err: context.Canceled})
		require.ErrorIs(t, c.apiDo(t.Context(), http.MethodPost, apiLogin, nil, &apiResLogin{}), context.Canceled)
	})

	t.Run("deadline exceeded during the request", func(t *testing.T) {
		t.Parallel()

		c, _ := newScriptedClient(t, reply{err: context.DeadlineExceeded})
		require.ErrorIs(t, c.apiDo(t.Context(), http.MethodPost, apiLogin, nil, &apiResLogin{}), context.DeadlineExceeded)
	})
}

// TestGetJoinedRooms verifies that the joined rooms lookup returns the rooms,
// or an error and no rooms when the request fails.
func TestGetJoinedRooms(t *testing.T) {
	t.Parallel()

	// tests lists replies with the rooms they should produce.
	tests := []struct {
		name    string
		reply   reply
		want    []string
		wantErr bool
	}{
		{name: "request fails", reply: failed("mock error"), wantErr: true},
		{name: "two rooms", reply: ok(`{"joined_rooms":["!room1:matrix.example.com","!room2:matrix.example.com"]}`), want: []string{"!room1:matrix.example.com", "!room2:matrix.example.com"}},
		{name: "no rooms", reply: ok(`{"joined_rooms":[]}`), want: []string{}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			c, _ := newScriptedClient(t, tt.reply)
			rooms, err := c.getJoinedRooms(t.Context())

			if tt.wantErr {
				require.Error(t, err)
				assert.Empty(t, rooms)

				return
			}

			require.NoError(t, err)
			assert.Equal(t, tt.want, rooms)
		})
	}
}

// TestJoinRoom verifies that joining a room returns its ID, sends an empty JSON
// object, and reports a failed request.
func TestJoinRoom(t *testing.T) {
	t.Parallel()

	t.Run("success", func(t *testing.T) {
		t.Parallel()

		c, httpClient := newScriptedClient(t, ok(`{"room_id":"!joinedroom:matrix.example.com"}`))
		roomID, err := c.joinRoom(t.Context(), "#test:matrix.example.com")

		require.NoError(t, err)
		assert.Equal(t, "!joinedroom:matrix.example.com", roomID)
		assert.Equal(t, []string{"{}"}, httpClient.bodies, "the join request body must be an object, not null")
	})

	t.Run("failure", func(t *testing.T) {
		t.Parallel()

		c, _ := newScriptedClient(t, failed("mock error"))
		roomID, err := c.joinRoom(t.Context(), "#test:matrix.example.com")

		require.Error(t, err)
		assert.Empty(t, roomID)
	})
}

// TestLoginLog verifies that a login logs the homeserver and device it
// reports at Debug, and leaves the access token out of the record.
func TestLoginLog(t *testing.T) {
	t.Parallel()

	var output bytes.Buffer

	c, _ := newScriptedClient(t, ok(`{"access_token":"mytoken123","home_server":"matrix.example.com","device_id":"TESTDEVICE"}`))
	c.logger.SetLogger(log.New(&output, "", 0))

	require.NoError(t, c.loginPassword(t.Context(), "user", "password"))
	assert.Equal(t, "DEBUG logged in homeserver=matrix.example.com device=TESTDEVICE\n", output.String())
	assert.NotContains(t, output.String(), "myt")
}

// TestLogin verifies that login reports a failed flows request and unsupported
// flows, and logs in with a token when no user is given.
func TestLogin(t *testing.T) {
	t.Parallel()

	t.Run("flows request fails", func(t *testing.T) {
		t.Parallel()

		c, _ := newScriptedClient(t, failed("mock error"))
		err := c.login(t.Context(), "user", "password")

		require.Error(t, err)
		assert.Contains(t, err.Error(), "login flows")
	})

	t.Run("unsupported flows", func(t *testing.T) {
		t.Parallel()

		c, _ := newScriptedClient(t, ok(`{"flows":[{"type":"m.login.unknown"}]}`))
		require.ErrorIs(t, c.login(t.Context(), "user", "password"), ErrUnsupportedLoginFlows)
	})

	t.Run("token login without a user", func(t *testing.T) {
		t.Parallel()

		c, _ := newScriptedClient(t,
			ok(`{"flows":[{"type":"m.login.token"}]}`),
			ok(`{"access_token":"testtoken123","home_server":"matrix.example.com","user_id":"@user:matrix.example.com"}`),
		)

		require.NoError(t, c.login(t.Context(), "", "testtoken123"))
		assert.Equal(t, "testtoken123", c.accessToken)
	})
}

// TestLoginPassword verifies that a password login sends the user, password,
// and device ID, stores the access token, for plain and fully qualified user
// names, and reports a failed request.
func TestLoginPassword(t *testing.T) {
	t.Parallel()

	// tests lists users that log in successfully.
	tests := []struct {
		name string
		user string
	}{
		{name: "plain user", user: "user"},
		{name: "fully qualified user", user: "@user:matrix.example.com"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			c, httpClient := newScriptedClient(t, ok(`{"access_token":"mytoken123","home_server":"matrix.example.com","user_id":"@user:matrix.example.com","device_id":"TESTDEVICE"}`))

			require.NoError(t, c.loginPassword(t.Context(), tt.user, "password"))
			assert.Equal(t, "mytoken123", c.accessToken)
			assert.Equal(t, apiReqLogin{
				Type:       flowLoginPassword,
				Identifier: newUserIdentifier(tt.user),
				Password:   "password",
				DeviceID:   generateDeviceID(tt.user, "matrix.example.com"),
			}, loginRequest(t, httpClient))
		})
	}

	t.Run("request fails", func(t *testing.T) {
		t.Parallel()

		c, _ := newScriptedClient(t, failed("mock error"))
		require.Error(t, c.loginPassword(t.Context(), "user", "password"))
		assert.Empty(t, c.accessToken)
	})
}

// TestLoginToken verifies that a token login sends the token and device ID,
// stores the access token, and reports a failed request.
func TestLoginToken(t *testing.T) {
	t.Parallel()

	t.Run("success", func(t *testing.T) {
		t.Parallel()

		c, httpClient := newScriptedClient(t, ok(`{"access_token":"token123","home_server":"matrix.example.com","user_id":"@user:matrix.example.com"}`))
		require.NoError(t, c.loginToken(t.Context(), "token123"))
		assert.Equal(t, "token123", c.accessToken)
		assert.Equal(t, apiReqLogin{
			Type:     flowLoginToken,
			Token:    "token123",
			DeviceID: generateDeviceID("", "matrix.example.com"),
		}, loginRequest(t, httpClient))
	})

	t.Run("request fails", func(t *testing.T) {
		t.Parallel()

		c, _ := newScriptedClient(t, failed("mock error"))
		require.Error(t, c.loginToken(t.Context(), "token"))
	})
}

// TestSendMessage verifies that a send without rooms looks up the joined rooms,
// and a send with rooms joins them.
func TestSendMessage(t *testing.T) {
	t.Parallel()

	// tests lists rooms with the API path the first request should use.
	tests := []struct {
		name     string
		rooms    []string
		wantPath string
	}{
		{name: "no rooms", rooms: nil, wantPath: apiJoinedRooms},
		{name: "explicit rooms", rooms: []string{"#room:matrix.example.com"}, wantPath: "/_matrix/client/v3/join/#room:matrix.example.com"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			c, httpClient := newScriptedClient(t, failed("mock error"))

			assert.Equal(t, 1, errorCount(c.sendMessage(t.Context(), "test message", tt.rooms)))
			require.Len(t, httpClient.requests, 1)
			assert.Equal(t, tt.wantPath, httpClient.requests[0].URL.Path)
		})
	}
}

// TestSendMessageToRoom verifies that a failed message request is reported.
func TestSendMessageToRoom(t *testing.T) {
	t.Parallel()

	c, _ := newScriptedClient(t, failed("mock error"))
	require.Error(t, c.sendMessageToRoom(t.Context(), "test message", "!room:matrix.example.com"))
}

// TestSendToExplicitRooms verifies that aliases are joined before sending, room
// IDs are sent to directly, blank rooms are rejected without requests, and one
// error is reported for each failed room.
func TestSendToExplicitRooms(t *testing.T) {
	t.Parallel()

	joined := ok(`{"room_id":"!roomid:matrix.example.com"}`)
	sent := ok(`{"event_id":"$eventid"}`)

	// tests lists rooms and replies with the number of errors and requests the
	// send should produce.
	tests := []struct {
		name         string
		rooms        []string
		replies      []reply
		wantErrs     int
		wantRequests int
		wantErr      error
	}{
		{name: "join fails", rooms: []string{"#room:matrix.example.com"}, replies: []reply{failed("mock error")}, wantErrs: 1, wantRequests: 1},
		{name: "join and send", rooms: []string{"#room:matrix.example.com"}, replies: []reply{joined, sent}, wantRequests: 2},
		{name: "send fails after join", rooms: []string{"#room:matrix.example.com"}, replies: []reply{joined, failed("send error")}, wantErrs: 1, wantRequests: 2},
		{name: "room ID skips join", rooms: []string{"!roomid:matrix.example.com"}, replies: []reply{sent}, wantRequests: 1},
		{name: "alias and room ID", rooms: []string{"#room:matrix.example.com", "!otherroom:matrix.example.com"}, replies: []reply{joined, sent, sent}, wantRequests: 3},
		{name: "empty room", rooms: []string{""}, wantErrs: 1, wantErr: ErrEmptyRoom},
		{name: "whitespace room", rooms: []string{"   "}, wantErrs: 1, wantErr: ErrEmptyRoom},
		{name: "room with surrounding spaces", rooms: []string{"  !roomid:matrix.example.com  "}, replies: []reply{sent}, wantRequests: 1},
		{
			name:         "one of two rooms fails",
			rooms:        []string{"#room1:matrix.example.com", "#room2:matrix.example.com"},
			replies:      []reply{joined, sent, joined, failed("send error")},
			wantErrs:     1,
			wantRequests: 4,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			c, httpClient := newScriptedClient(t, tt.replies...)
			err := c.sendToExplicitRooms(t.Context(), tt.rooms, "test message")

			assert.Equal(t, tt.wantErrs, errorCount(err))
			assert.Equal(t, tt.wantRequests, httpClient.requestCount())

			if tt.wantErr != nil {
				require.ErrorIs(t, err, tt.wantErr)
			}
		})
	}
}

// TestSendToExplicitRoomsContextEnded verifies that a send whose context has
// ended sends to no room and reports the context's error once. The scripted
// client fails the test on any request.
func TestSendToExplicitRoomsContextEnded(t *testing.T) {
	t.Parallel()

	ctx, cancel := context.WithCancel(t.Context())
	cancel()

	c, httpClient := newScriptedClient(t)
	err := c.sendToExplicitRooms(ctx, []string{"!a:matrix.example.com", "!b:matrix.example.com"}, "test message")

	require.ErrorIs(t, err, context.Canceled)
	assert.Equal(t, 1, errorCount(err))
	assert.Zero(t, httpClient.requestCount())
}

// TestSendToJoinedRooms verifies that the message goes to every joined room and
// that one error is reported for a failed lookup or each failed room.
func TestSendToJoinedRooms(t *testing.T) {
	t.Parallel()

	sent := ok(`{"event_id":"$eventid"}`)

	// tests lists replies with the number of errors the send should produce.
	tests := []struct {
		name     string
		replies  []reply
		wantErrs int
	}{
		{name: "lookup fails", replies: []reply{failed("mock error")}, wantErrs: 1},
		{name: "one room", replies: []reply{ok(`{"joined_rooms":["!room1:matrix.example.com"]}`), sent}},
		{name: "send fails", replies: []reply{ok(`{"joined_rooms":["!room1:matrix.example.com"]}`), failed("send error")}, wantErrs: 1},
		{
			name:     "one of two rooms fails",
			replies:  []reply{ok(`{"joined_rooms":["!room1:matrix.example.com","!room2:matrix.example.com"]}`), sent, failed("send error")},
			wantErrs: 1,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			c, _ := newScriptedClient(t, tt.replies...)
			assert.Equal(t, tt.wantErrs, errorCount(c.sendToJoinedRooms(t.Context(), "test message")))
		})
	}
}

// TestSetAuthorizationHeader verifies that the access token is sent as a bearer
// token, replacing any existing header, and that no header is set without one.
func TestSetAuthorizationHeader(t *testing.T) {
	t.Parallel()

	// tests lists access tokens and existing headers with the header to expect.
	tests := []struct {
		name     string
		token    string
		existing string
		want     string
	}{
		{name: "token", token: "test-token-123", want: "Bearer test-token-123"},
		{name: "no token", token: "", want: ""},
		{name: "replaces existing header", token: "new-token", existing: "Existing-Token", want: "Bearer new-token"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			req, err := http.NewRequestWithContext(t.Context(), http.MethodGet, "https://matrix.example.com/_matrix", http.NoBody)
			require.NoError(t, err)

			if tt.existing != "" {
				req.Header.Set("Authorization", tt.existing)
			}

			(&client{accessToken: tt.token}).setAuthorizationHeader(req)

			assert.Equal(t, tt.want, req.Header.Get("Authorization"))
		})
	}
}

// TestGenerateTransactionID verifies that transaction IDs are unique and use
// the timestamp-hex format.
func TestGenerateTransactionID(t *testing.T) {
	t.Parallel()

	first, second := generateTransactionID(), generateTransactionID()

	assert.NotEqual(t, first, second)
	assert.Regexp(t, `^[0-9]+-[a-f0-9]+$`, first)
}

// TestGenerateDeviceID verifies that device IDs are deterministic per user and
// host, use the SH-prefixed alphanumeric format, and ignore default ports.
func TestGenerateDeviceID(t *testing.T) {
	t.Parallel()

	base := generateDeviceID("user", "matrix.example.com")

	assert.Equal(t, base, generateDeviceID("user", "matrix.example.com"), "the same user and host give the same ID")
	assert.NotEqual(t, base, generateDeviceID("user1", "matrix.example.com"), "users get different IDs")
	assert.NotEqual(t, base, generateDeviceID("user", "matrix2.example.com"), "hosts get different IDs")

	for _, user := range []string{"user", "", "@user:matrix.example.com"} {
		id := generateDeviceID(user, "matrix.example.com")
		assert.Regexp(t, `^SH[A-Za-z0-9]{8}$`, id, "user %q", user)
		assert.Equal(t, id, generateDeviceID(user, "matrix.example.com"), "user %q", user)
	}

	assert.Equal(t, base, generateDeviceID("user", "matrix.example.com:443"), "the default HTTPS port is ignored")
	assert.Equal(t, base, generateDeviceID("user", "matrix.example.com:80"), "the default HTTP port is ignored")
	assert.NotEqual(t, base, generateDeviceID("user", "matrix.example.com:8008"), "other ports change the ID")
}

// TestCreateMessage verifies that a non-blank title is trimmed and prepended to
// the message.
func TestCreateMessage(t *testing.T) {
	t.Parallel()

	// tests lists messages and titles with the text to expect.
	tests := []struct {
		name    string
		message string
		title   string
		want    string
	}{
		{name: "no title", message: "Hello world", want: "Hello world"},
		{name: "title", message: "Hello world", title: "Alert", want: "Alert\n\nHello world"},
		{name: "title with spaces", message: "Hello world", title: "  Alert  ", want: "Alert\n\nHello world"},
		{name: "empty message", title: "Alert", want: "Alert\n\n"},
		{name: "nothing", want: ""},
		{name: "blank title", message: "Hello world", title: "   ", want: "Hello world"},
		{name: "multiline message", message: "Line1\nLine2\nLine3", title: "Alert", want: "Alert\n\nLine1\nLine2\nLine3"},
		{name: "special characters", message: `Message with <html> & "quotes"`, title: "Alert", want: "Alert\n\nMessage with <html> & \"quotes\""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			assert.Equal(t, tt.want, createMessage(tt.message, tt.title))
		})
	}
}

// RoundTrip fails the request with the transport's error.
//
// Returns:
//   - *http.Response: always nil.
//   - error: the transport's error.
func (f failingTransport) RoundTrip(*http.Request) (*http.Response, error) {
	return nil, f.err
}

// TestDefaultHTTPClientDo verifies that the deprecated DefaultHTTPClient
// still sends through its client and redacts the URL in a transport error.
func TestDefaultHTTPClientDo(t *testing.T) {
	t.Parallel()

	errTransport := errors.New("transport failure")
	client := &DefaultHTTPClient{client: &http.Client{Transport: failingTransport{err: errTransport}}}

	req, err := http.NewRequestWithContext(t.Context(), http.MethodGet, "https://matrix.example.com/SECRETpath", http.NoBody)
	require.NoError(t, err)

	res, err := client.Do(req)

	require.ErrorIs(t, err, errTransport)
	assert.Nil(t, res)
	assert.NotContains(t, err.Error(), "SECRETpath")
	assert.Contains(t, err.Error(), "matrix.example.com")

	t.Run("zero value", func(t *testing.T) {
		t.Parallel()

		ctx, cancel := context.WithCancel(t.Context())
		cancel()

		req, err := http.NewRequestWithContext(ctx, http.MethodGet, "https://matrix.example.com/", http.NoBody)
		require.NoError(t, err)

		_, err = (&DefaultHTTPClient{}).Do(req)
		require.ErrorIs(t, err, context.Canceled)
	})
}

// errorCount returns how many errors err holds.
//
// Parameters:
//   - err: the error, which may join several.
//
// Returns:
//   - int: 0 for nil, the number of joined errors, or 1 otherwise.
func errorCount(err error) int {
	if err == nil {
		return 0
	}

	if joined, ok := err.(interface{ Unwrap() []error }); ok {
		return len(joined.Unwrap())
	}

	return 1
}
