package matrix_test

import (
	"context"
	"encoding/json/v2"
	"io"
	"log"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"sync"
	"sync/atomic"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	"github.com/nicholas-fedor/shoutrrr/pkg/services/chat/matrix"
	"github.com/nicholas-fedor/shoutrrr/pkg/services/chat/matrix/mocks"
	"github.com/nicholas-fedor/shoutrrr/pkg/types"
)

// apiRequest records one request the fake homeserver received.
type apiRequest struct {
	// method is the request method.
	method string
	// path is the decoded request path. For a message send it ends at the event
	// type, without the transaction ID.
	path string
	// auth is the Authorization header.
	auth string
	// body is the JSON request body without the device ID, or empty for a
	// request without a body.
	body string
}

// apiReply is how the fake homeserver answers one request.
type apiReply struct {
	// body is the response body.
	body string
	// statusCode is the response status code.
	statusCode int
	// hold keeps the request open until the client gives up on it.
	hold bool
}

// homeserver is a fake Matrix homeserver on a loopback [httptest.Server]. It
// offers the password login flow, accepts only testUser with testPassword,
// requires a known access token for every other endpoint, resolves a room alias
// #name:server to the room ID !name:server, and records every request.
type homeserver struct {
	// overrides maps a route, the method and recorded path joined by a space, to
	// the reply that replaces the default one.
	overrides map[string]apiReply
	// held receives a value when a held request arrives.
	held chan struct{}
	// server is the loopback HTTP server.
	server *httptest.Server
	// mu guards requests.
	mu sync.Mutex
	// requests holds the received requests in order.
	requests []apiRequest
}

// roundTripFunc adapts a function to an [http.RoundTripper].
type roundTripFunc func(req *http.Request) (*http.Response, error)

// Credentials, rooms, and API paths of the fake homeserver.
const (
	// testUser is the user the fake homeserver accepts.
	testUser = "user"
	// testPassword is the password the fake homeserver accepts for testUser.
	// It contains characters that need escaping in a URL.
	testPassword = "p@ss w0rd!"
	// escapedPassword is testPassword escaped for the user info of a URL.
	escapedPassword = "p%40ss%20w0rd%21"
	// loginToken is the access token a password login returns.
	loginToken = "syt_login_token"
	// accessToken is an access token the fake homeserver accepts without a login.
	accessToken = "syt_access_token"
	// joinedRoomA is the first room the access token has joined.
	joinedRoomA = "!a:example.com"
	// joinedRoomB is the second room the access token has joined.
	joinedRoomB = "!b:example.com"
	// loginPath is the login endpoint.
	loginPath = "/_matrix/client/v3/login"
	// joinedRoomsPath is the joined rooms endpoint.
	joinedRoomsPath = "/_matrix/client/v3/joined_rooms"
	// joinPathPrefix is the join endpoint without the room.
	joinPathPrefix = "/_matrix/client/v3/join/"
	// roomsPathPrefix is the rooms endpoint without the room.
	roomsPathPrefix = "/_matrix/client/v3/rooms/"
	// sendPathSuffix follows the room in a message send path, before the
	// transaction ID.
	sendPathSuffix = "/send/m.room.message"
)

// RoundTrip calls f with the request.
//
// Parameters:
//   - req: the request to send.
//
// Returns:
//   - *http.Response: the response from f.
//   - error: the error from f.
func (f roundTripFunc) RoundTrip(req *http.Request) (*http.Response, error) {
	return f(req)
}

// ServeHTTP records the request and answers it with its override or the
// default reply for its route.
//
// Parameters:
//   - w: the response writer.
//   - r: the request.
func (h *homeserver) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	recorded := apiRequest{method: r.Method, path: r.URL.Path, auth: r.Header.Get("Authorization")}

	if prefix, _, ok := strings.Cut(r.URL.Path, sendPathSuffix+"/"); ok {
		recorded.path = prefix + sendPathSuffix
	}

	fields := map[string]any{}

	raw, err := io.ReadAll(r.Body)
	if err == nil && len(raw) > 0 {
		err = json.Unmarshal(raw, &fields)
	}

	if err != nil {
		writeReply(w, apiReply{statusCode: http.StatusBadRequest, body: `{"errcode":"M_NOT_JSON","error":"Content not JSON."}`})

		return
	}

	delete(fields, "device_id")

	if len(raw) > 0 {
		body, err := json.Marshal(fields)
		if err != nil {
			writeReply(w, apiReply{statusCode: http.StatusInternalServerError, body: `{"errcode":"M_UNKNOWN","error":"Internal error"}`})

			return
		}

		recorded.body = string(body)
	}

	h.mu.Lock()
	h.requests = append(h.requests, recorded)
	h.mu.Unlock()

	reply, ok := h.overrides[recorded.method+" "+recorded.path]
	if !ok {
		reply = h.defaultReply(&recorded, fields)
	}

	if reply.hold {
		h.held <- struct{}{}

		<-r.Context().Done()

		return
	}

	writeReply(w, reply)
}

// defaultReply returns the reply a homeserver gives to a request without an
// override.
//
// Parameters:
//   - req: the recorded request.
//   - fields: the decoded JSON body.
//
// Returns:
//   - apiReply: the reply.
func (*homeserver) defaultReply(req *apiRequest, fields map[string]any) apiReply {
	if req.path == loginPath {
		if req.method == http.MethodGet {
			return apiReply{statusCode: http.StatusOK, body: `{"flows":[{"type":"m.login.password"}]}`}
		}

		identifier, _ := fields["identifier"].(map[string]any)
		if fields["type"] != "m.login.password" || identifier["user"] != testUser || fields["password"] != testPassword {
			return apiReply{statusCode: http.StatusForbidden, body: `{"errcode":"M_FORBIDDEN","error":"Invalid username or password"}`}
		}

		return apiReply{
			statusCode: http.StatusOK,
			body:       `{"access_token":"` + loginToken + `","user_id":"@user:example.com","home_server":"example.com","device_id":"SHDEVICEID"}`,
		}
	}

	if req.auth != "Bearer "+loginToken && req.auth != "Bearer "+accessToken {
		return apiReply{statusCode: http.StatusUnauthorized, body: `{"errcode":"M_UNKNOWN_TOKEN","error":"Invalid access token passed."}`}
	}

	switch {
	case req.method == http.MethodGet && req.path == joinedRoomsPath:
		return apiReply{statusCode: http.StatusOK, body: `{"joined_rooms":["` + joinedRoomA + `","` + joinedRoomB + `"]}`}
	case req.method == http.MethodPost && strings.HasPrefix(req.path, joinPathPrefix+"#"):
		roomID := "!" + strings.TrimPrefix(req.path, joinPathPrefix+"#")

		return apiReply{statusCode: http.StatusOK, body: `{"room_id":"` + roomID + `"}`}
	case req.method == http.MethodPut && strings.HasPrefix(req.path, roomsPathPrefix) && strings.HasSuffix(req.path, sendPathSuffix):
		return apiReply{statusCode: http.StatusOK, body: `{"event_id":"$event"}`}
	default:
		return apiReply{statusCode: http.StatusNotFound, body: `{"errcode":"M_UNRECOGNIZED","error":"Unrecognized request"}`}
	}
}

// received returns the requests received so far.
//
// Returns:
//   - []apiRequest: the requests, in order.
func (h *homeserver) received() []apiRequest {
	h.mu.Lock()
	defer h.mu.Unlock()

	return append([]apiRequest(nil), h.requests...)
}

// newHomeserver starts a fake homeserver that the test stops when it ends.
//
// Parameters:
//   - t: the test that owns the server.
//   - overrides: replies that replace the default reply for their routes.
//
// Returns:
//   - *homeserver: the server.
func newHomeserver(t *testing.T, overrides map[string]apiReply) *homeserver {
	t.Helper()

	h := &homeserver{overrides: overrides, held: make(chan struct{}, 1)}
	h.server = httptest.NewServer(h)
	t.Cleanup(h.server.Close)

	return h
}

// writeReply writes reply as a JSON response.
//
// Parameters:
//   - w: the response writer.
//   - reply: the reply to write.
func writeReply(w http.ResponseWriter, reply apiReply) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(reply.statusCode)
	_, _ = io.WriteString(w, reply.body)
}

// joinPath returns the path that joins room.
//
// Parameters:
//   - room: the room alias.
//
// Returns:
//   - string: the path.
func joinPath(room string) string {
	return joinPathPrefix + room
}

// sendPath returns the recorded path of a message send to room.
//
// Parameters:
//   - room: the room ID.
//
// Returns:
//   - string: the path, without the transaction ID.
func sendPath(room string) string {
	return roomsPathPrefix + room + sendPathSuffix
}

// messageBody returns the JSON body of a text message send.
//
// Parameters:
//   - t: the test.
//   - body: the message text.
//
// Returns:
//   - string: the JSON body.
func messageBody(t *testing.T, body string) string {
	t.Helper()

	encoded, err := json.Marshal(map[string]string{"msgtype": "m.text", "body": body})
	require.NoError(t, err)

	return string(encoded)
}

// serviceURL returns a service URL for the fake homeserver over plain HTTP.
//
// Parameters:
//   - t: the test.
//   - server: the fake homeserver.
//   - userInfo: the escaped user info, user:password or :token.
//   - query: further query parameters, each starting with &.
//
// Returns:
//   - *url.URL: the service URL.
func serviceURL(t *testing.T, server *homeserver, userInfo, query string) *url.URL {
	t.Helper()

	host := strings.TrimPrefix(server.server.URL, "http://")

	parsed, err := url.Parse("matrix://" + userInfo + "@" + host + "/?disableTLS=true" + query)
	require.NoError(t, err)

	return parsed
}

// newTestService returns a service initialized from a URL for the fake
// homeserver that logs to the test output.
//
// Parameters:
//   - t: the test.
//   - server: the fake homeserver.
//   - userInfo: the escaped user info, user:password or :token.
//   - query: further query parameters, each starting with &.
//
// Returns:
//   - *matrix.Service: the service.
func newTestService(t *testing.T, server *homeserver, userInfo, query string) *matrix.Service {
	t.Helper()

	service := &matrix.Service{}
	require.NoError(t, service.Initialize(serviceURL(t, server, userInfo, query), log.New(t.Output(), "", 0)))

	return service
}

// assertRequests asserts that the fake homeserver received the wanted
// requests, in order.
//
// Parameters:
//   - t: the test.
//   - want: the wanted requests.
//   - got: the received requests.
func assertRequests(t *testing.T, want, got []apiRequest) {
	t.Helper()

	require.Len(t, got, len(want))

	for i := range want {
		assert.Equal(t, want[i].method, got[i].method, "request %d", i)
		assert.Equal(t, want[i].path, got[i].path, "request %d", i)
		assert.Equal(t, want[i].auth, got[i].auth, "request %d", i)

		if want[i].body == "" {
			assert.Empty(t, got[i].body, "request %d", i)

			continue
		}

		assert.JSONEq(t, want[i].body, got[i].body, "request %d", i)
	}
}

// TestIntegrationSend verifies the requests a service initialized from a URL
// makes for a send: a password login on the first send for a user, the access
// token otherwise, a join for each room alias before its message, a lookup of
// the joined rooms when no room is configured, and the title from the URL or
// params on its own paragraph.
func TestIntegrationSend(t *testing.T) {
	t.Parallel()

	// tests lists service URLs, params, and messages with the requests the
	// homeserver should receive.
	tests := []struct {
		params   *types.Params
		name     string
		userInfo string
		query    string
		message  string
		want     func(t *testing.T) []apiRequest
	}{
		{
			name:     "password login and room alias without prefix",
			userInfo: testUser + ":" + escapedPassword,
			query:    "&rooms=general:example.com&title=Alert",
			message:  "Hello",
			want: func(t *testing.T) []apiRequest {
				t.Helper()

				return []apiRequest{
					{method: http.MethodGet, path: loginPath},
					{
						method: http.MethodPost,
						path:   loginPath,
						body:   `{"type":"m.login.password","identifier":{"type":"m.id.user","user":"user"},"password":"p@ss w0rd!"}`,
					},
					{method: http.MethodPost, path: joinPath("#general:example.com"), auth: "Bearer " + loginToken, body: `{}`},
					{method: http.MethodPut, path: sendPath("!general:example.com"), auth: "Bearer " + loginToken, body: messageBody(t, "Alert\n\nHello")},
				}
			},
		},
		{
			name:     "access token and room ID",
			userInfo: ":" + accessToken,
			query:    "&rooms=!ops:example.com",
			message:  "Hello",
			want: func(t *testing.T) []apiRequest {
				t.Helper()

				return []apiRequest{
					{method: http.MethodPut, path: sendPath("!ops:example.com"), auth: "Bearer " + accessToken, body: messageBody(t, "Hello")},
				}
			},
		},
		{
			name:     "joined rooms without configured rooms",
			userInfo: ":" + accessToken,
			message:  "Hello",
			want: func(t *testing.T) []apiRequest {
				t.Helper()

				return []apiRequest{
					{method: http.MethodGet, path: joinedRoomsPath, auth: "Bearer " + accessToken},
					{method: http.MethodPut, path: sendPath(joinedRoomA), auth: "Bearer " + accessToken, body: messageBody(t, "Hello")},
					{method: http.MethodPut, path: sendPath(joinedRoomB), auth: "Bearer " + accessToken, body: messageBody(t, "Hello")},
				}
			},
		},
		{
			name:     "rooms and title from params",
			userInfo: ":" + accessToken,
			query:    "&rooms=!ops:example.com&title=Configured",
			params:   &types.Params{"rooms": "#deploys:example.com,!alerts:example.com", "title": "Deploy"},
			message:  "Finished",
			want: func(t *testing.T) []apiRequest {
				t.Helper()

				return []apiRequest{
					{method: http.MethodPost, path: joinPath("#deploys:example.com"), auth: "Bearer " + accessToken, body: `{}`},
					{method: http.MethodPut, path: sendPath("!deploys:example.com"), auth: "Bearer " + accessToken, body: messageBody(t, "Deploy\n\nFinished")},
					{method: http.MethodPut, path: sendPath("!alerts:example.com"), auth: "Bearer " + accessToken, body: messageBody(t, "Deploy\n\nFinished")},
				}
			},
		},
		{
			name:     "special characters",
			userInfo: ":" + accessToken,
			query:    "&rooms=!ops:example.com",
			message:  "<p>**bold**</p> & \"quotes\" `code`\nПривет 🌍\t{\"key\": 1}",
			want: func(t *testing.T) []apiRequest {
				t.Helper()

				return []apiRequest{{
					method: http.MethodPut,
					path:   sendPath("!ops:example.com"),
					auth:   "Bearer " + accessToken,
					body:   messageBody(t, "<p>**bold**</p> & \"quotes\" `code`\nПривет 🌍\t{\"key\": 1}"),
				}}
			},
		},
		{
			name:     "empty message",
			userInfo: ":" + accessToken,
			query:    "&rooms=!ops:example.com",
			want: func(t *testing.T) []apiRequest {
				t.Helper()

				return []apiRequest{
					{method: http.MethodPut, path: sendPath("!ops:example.com"), auth: "Bearer " + accessToken, body: messageBody(t, "")},
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			server := newHomeserver(t, nil)
			service := newTestService(t, server, tt.userInfo, tt.query)

			require.NoError(t, service.SendContext(t.Context(), tt.message, tt.params))

			assertRequests(t, tt.want(t), server.received())
		})
	}
}

// TestIntegrationSendErrors verifies the errors a caller sees when the
// homeserver rejects a login, a join, a lookup, or a message, and when a param
// or room is invalid. A failed room does not stop the send to the others.
func TestIntegrationSendErrors(t *testing.T) {
	t.Parallel()

	// tests lists service URLs, params, and homeserver overrides with the error
	// the send should return, the text it should and should not contain, and
	// the number of requests the homeserver should receive.
	tests := []struct {
		wantErr      error
		params       *types.Params
		overrides    map[string]apiReply
		name         string
		userInfo     string
		query        string
		wantText     []string
		notWantText  []string
		wantRequests int
	}{
		{
			name:         "wrong password",
			userInfo:     testUser + ":SECRETwrong",
			query:        "&rooms=!ops:example.com",
			wantText:     []string{"logging in to Matrix", "Invalid username or password"},
			notWantText:  []string{"SECRETwrong"},
			wantRequests: 2,
		},
		{
			name:         "unsupported login flows",
			userInfo:     testUser + ":" + escapedPassword,
			query:        "&rooms=!ops:example.com",
			overrides:    map[string]apiReply{http.MethodGet + " " + loginPath: {statusCode: http.StatusOK, body: `{"flows":[{"type":"m.login.sso"}]}`}},
			wantErr:      matrix.ErrUnsupportedLoginFlows,
			wantText:     []string{"m.login.sso"},
			wantRequests: 1,
		},
		{
			name:         "unknown access token",
			userInfo:     ":syt_revoked",
			wantText:     []string{"sending message: ", "failed to get joined rooms", "Invalid access token passed."},
			wantRequests: 1,
		},
		{
			name:     "unknown room alias",
			userInfo: ":" + accessToken,
			query:    "&rooms=%23missing:example.com",
			overrides: map[string]apiReply{
				http.MethodPost + " " + joinPath("#missing:example.com"): {
					statusCode: http.StatusNotFound,
					body:       `{"errcode":"M_NOT_FOUND","error":"Room alias not found"}`,
				},
			},
			wantText:     []string{"error joining room #missing:example.com", "Room alias not found"},
			wantRequests: 1,
		},
		{
			name:     "message forbidden",
			userInfo: ":" + accessToken,
			query:    "&rooms=!ops:example.com",
			overrides: map[string]apiReply{
				http.MethodPut + " " + sendPath("!ops:example.com"): {
					statusCode: http.StatusForbidden,
					body:       `{"errcode":"M_FORBIDDEN","error":"You are not in this room"}`,
				},
			},
			wantText:     []string{"failed to send message to room '!ops:example.com'", "You are not in this room"},
			wantRequests: 1,
		},
		{
			// The homeserver asks for a minute, which would end after the
			// room's share of the send's deadline, so the send fails without
			// waiting.
			name:     "rate limited past the deadline",
			userInfo: ":" + accessToken,
			query:    "&rooms=!ops:example.com",
			overrides: map[string]apiReply{
				http.MethodPut + " " + sendPath("!ops:example.com"): {
					statusCode: http.StatusTooManyRequests,
					body:       `{"errcode":"M_LIMIT_EXCEEDED","error":"Too many requests","retry_after_ms":60000}`,
				},
			},
			wantText:     []string{"Too many requests"},
			wantRequests: 1,
		},
		{
			name:         "first of two rooms fails",
			userInfo:     ":" + accessToken,
			query:        "&rooms=!a:example.com,!b:example.com",
			overrides:    map[string]apiReply{http.MethodPut + " " + sendPath("!a:example.com"): {statusCode: http.StatusBadGateway, body: "upstream down"}},
			wantErr:      matrix.ErrUnexpectedStatus,
			wantText:     []string{"sending message: ", "failed to send message to room '!a:example.com'"},
			notWantText:  []string{"'!b:example.com'"},
			wantRequests: 2,
		},
		{
			name:     "both of two rooms fail",
			userInfo: ":" + accessToken,
			query:    "&rooms=!a:example.com,!b:example.com",
			overrides: map[string]apiReply{
				http.MethodPut + " " + sendPath("!a:example.com"): {statusCode: http.StatusBadGateway, body: "upstream down"},
				http.MethodPut + " " + sendPath("!b:example.com"): {statusCode: http.StatusBadGateway, body: "upstream down"},
			},
			wantErr: matrix.ErrUnexpectedStatus,
			wantText: []string{
				"sending message: ",
				"failed to send message to room '!a:example.com'",
				"failed to send message to room '!b:example.com'",
			},
			wantRequests: 2,
		},
		{
			name:     "blank room param",
			userInfo: ":" + accessToken,
			params:   &types.Params{"rooms": " "},
			wantErr:  matrix.ErrEmptyRoom,
		},
		{
			name:     "unknown param",
			userInfo: ":" + accessToken,
			query:    "&rooms=!ops:example.com",
			params:   &types.Params{"bogus": "value"},
			wantText: []string{"updating config from params"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			server := newHomeserver(t, tt.overrides)
			service := newTestService(t, server, tt.userInfo, tt.query)

			err := service.SendContext(t.Context(), "Hello", tt.params)

			require.Error(t, err)

			if tt.wantErr != nil {
				require.ErrorIs(t, err, tt.wantErr)
			}

			for _, want := range tt.wantText {
				assert.Contains(t, err.Error(), want)
			}

			for _, notWant := range tt.notWantText {
				assert.NotContains(t, err.Error(), notWant)
			}

			assert.Len(t, server.received(), tt.wantRequests)
		})
	}
}

// TestIntegrationSendContext verifies that a send stops with the caller's
// context error when the context is canceled before the send or while a
// request is in flight.
func TestIntegrationSendContext(t *testing.T) {
	t.Parallel()

	t.Run("canceled before the login", func(t *testing.T) {
		t.Parallel()

		server := newHomeserver(t, nil)
		service := newTestService(t, server, testUser+":"+escapedPassword, "&rooms=!ops:example.com")

		ctx, cancel := context.WithCancel(t.Context())
		cancel()

		err := service.SendContext(ctx, "Hello", nil)

		require.ErrorIs(t, err, context.Canceled)
		assert.Empty(t, server.received())
	})

	t.Run("canceled during a message send", func(t *testing.T) {
		t.Parallel()

		server := newHomeserver(t, map[string]apiReply{http.MethodPut + " " + sendPath("!ops:example.com"): {hold: true}})
		service := newTestService(t, server, ":"+accessToken, "&rooms=!ops:example.com")

		ctx, cancel := context.WithCancel(t.Context())
		defer cancel()

		go func() {
			<-server.held
			cancel()
		}()

		err := service.SendContext(ctx, "Hello", nil)

		require.ErrorIs(t, err, context.Canceled)
		assert.Contains(t, err.Error(), "failed to send message to room '!ops:example.com'")
	})
}

// TestIntegrationSetHTTPClient verifies that sends use the HTTP client set
// before or after Initialize, and the default client when none is set or a nil
// *http.Client is set.
func TestIntegrationSetHTTPClient(t *testing.T) {
	t.Parallel()

	// tests lists ways to set up the service's HTTP client with whether the
	// counting client should carry the send. Each setup receives the counting
	// client, which forwards to the fake homeserver.
	tests := []struct {
		beforeInit func(t *testing.T, service *matrix.Service, counting types.HTTPClient)
		afterInit  func(t *testing.T, service *matrix.Service, counting types.HTTPClient)
		name       string
		wantUsed   bool
	}{
		{name: "default client"},
		{
			name: "client set before Initialize",
			beforeInit: func(_ *testing.T, service *matrix.Service, counting types.HTTPClient) {
				service.SetHTTPClient(counting)
			},
			wantUsed: true,
		},
		{
			name: "client set after Initialize",
			afterInit: func(_ *testing.T, service *matrix.Service, counting types.HTTPClient) {
				service.SetHTTPClient(counting)
			},
			wantUsed: true,
		},
		{
			name: "custom client implementation",
			afterInit: func(t *testing.T, service *matrix.Service, counting types.HTTPClient) {
				t.Helper()

				client := mocks.NewMockHTTPClient(t)
				client.EXPECT().Do(mock.Anything).RunAndReturn(counting.Do).Once()
				service.SetHTTPClient(client)
			},
			wantUsed: true,
		},
		{
			name: "nil *http.Client restores the default client",
			afterInit: func(_ *testing.T, service *matrix.Service, counting types.HTTPClient) {
				service.SetHTTPClient(counting)
				service.SetHTTPClient((*http.Client)(nil))
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			var used atomic.Int32

			counting := &http.Client{Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
				used.Add(1)

				return http.DefaultTransport.RoundTrip(req)
			})}

			server := newHomeserver(t, nil)
			service := &matrix.Service{}

			if tt.beforeInit != nil {
				tt.beforeInit(t, service, counting)
			}

			rawURL := serviceURL(t, server, ":"+accessToken, "&rooms=!ops:example.com")
			require.NoError(t, service.Initialize(rawURL, log.New(t.Output(), "", 0)))

			if tt.afterInit != nil {
				tt.afterInit(t, service, counting)
			}

			require.NoError(t, service.SendContext(t.Context(), "Hello", nil))

			assert.Len(t, server.received(), 1)

			if tt.wantUsed {
				assert.Equal(t, int32(1), used.Load())
			} else {
				assert.Zero(t, used.Load())
			}
		})
	}
}

// TestIntegrationSetLogger verifies that a logger set after Initialize, as the
// router sets one, receives the records of the Matrix client that Initialize
// built.
func TestIntegrationSetLogger(t *testing.T) {
	t.Parallel()

	server := newHomeserver(t, nil)
	service := newTestService(t, server, ":"+accessToken, "&rooms=!a:example.com")

	var logs strings.Builder

	service.SetLogger(log.New(&logs, "", 0))

	require.NoError(t, service.Send("Hello", nil))
	assert.Contains(t, logs.String(), "DEBUG sending message\n")
}
