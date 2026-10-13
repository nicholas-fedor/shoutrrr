package matrix

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"log"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

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

	return &http.Response{
		StatusCode: next.status,
		Status:     http.StatusText(next.status),
		Header:     http.Header{"Content-Type": []string{"application/json"}},
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

	return &client{
		apiURL:     url.URL{Scheme: "https", Host: "matrix.example.com"},
		httpClient: httpClient,
		logger:     log.New(t.Output(), "", 0),
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

// TestNewClient verifies the API scheme for each TLS setting and that a nil
// logger is replaced.
func TestNewClient(t *testing.T) {
	t.Parallel()

	secure := newClient("matrix.example.com", false, nil)
	require.NotNil(t, secure)
	assert.Equal(t, "https", secure.apiURL.Scheme)
	assert.Equal(t, "matrix.example.com", secure.apiURL.Host)
	assert.NotNil(t, secure.logger)

	assert.Equal(t, "http", newClient("matrix.example.com", true, nil).apiURL.Scheme)
}

// TestAPIRequestFailures verifies that GET and POST requests report transport
// errors, Matrix API errors, error statuses without a Matrix error, invalid
// JSON, and unreadable bodies.
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
	}

	// methods lists the request functions under test.
	methods := map[string]func(*client, context.Context) error{
		"GET": func(c *client, ctx context.Context) error {
			return c.apiGet(ctx, apiLogin, &apiResLogin{})
		},
		"POST": func(c *client, ctx context.Context) error {
			return c.apiPost(ctx, apiLogin, nil, &apiResLogin{})
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
// without sending, sends JSON with the given method, and reports an empty body
// as invalid JSON.
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

	t.Run("method and content type", func(t *testing.T) {
		t.Parallel()

		c, httpClient := newScriptedClient(t, ok(`{}`))
		require.NoError(t, c.apiDo(t.Context(), http.MethodPut, "/_matrix/client/v3/rooms/!room/send/m.room.message/txn", nil, &apiResLogin{}))

		require.Len(t, httpClient.requests, 1)
		assert.Equal(t, http.MethodPut, httpClient.requests[0].Method)
		assert.Equal(t, "application/json", httpClient.requests[0].Header.Get("Content-Type"))
	})

	t.Run("empty body", func(t *testing.T) {
		t.Parallel()

		c, _ := newScriptedClient(t, ok(""))
		err := c.apiDo(t.Context(), http.MethodPost, apiLogin, nil, &apiResLogin{})

		require.Error(t, err)
		assert.Contains(t, err.Error(), "unmarshaling")
	})
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

// TestAPIPut verifies that apiPut sends a PUT request and reports its failure.
func TestAPIPut(t *testing.T) {
	t.Parallel()

	request := apiReqSend{MsgType: msgTypeText, Body: "test message"}
	path := "/_matrix/client/v3/rooms/!room/send/m.room.message/txn1"

	t.Run("success", func(t *testing.T) {
		t.Parallel()

		c, httpClient := newScriptedClient(t, ok(`{"event_id":"$eventid"}`))
		require.NoError(t, c.apiPut(t.Context(), path, request, &apiResEvent{}))
		require.Len(t, httpClient.requests, 1)
		assert.Equal(t, http.MethodPut, httpClient.requests[0].Method)
	})

	t.Run("failure", func(t *testing.T) {
		t.Parallel()

		c, _ := newScriptedClient(t, failed("PUT request failed"))
		err := c.apiPut(t.Context(), path, request, &apiResEvent{})

		require.Error(t, err)
		assert.Contains(t, err.Error(), "executing PUT")
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

// TestLogf verifies that logf writes through the client's logger.
func TestLogf(t *testing.T) {
	t.Parallel()

	var output bytes.Buffer

	(&client{logger: log.New(&output, "", 0)}).logf("test %s", "message")

	assert.Contains(t, output.String(), "test message")
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

			assert.Len(t, c.sendMessage(t.Context(), "test message", tt.rooms), 1)
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
			errs := c.sendToExplicitRooms(t.Context(), tt.rooms, "test message")

			assert.Len(t, errs, tt.wantErrs)
			assert.Equal(t, tt.wantRequests, httpClient.requestCount())

			if tt.wantErr != nil {
				require.NotEmpty(t, errs)
				require.ErrorIs(t, errs[0], tt.wantErr)
			}
		})
	}
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
			assert.Len(t, c.sendToJoinedRooms(t.Context(), "test message"), tt.wantErrs)
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

// TestUseToken verifies that useToken stores the access token.
func TestUseToken(t *testing.T) {
	t.Parallel()

	c := &client{}
	c.useToken("my-token")

	assert.Equal(t, "my-token", c.accessToken)
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
