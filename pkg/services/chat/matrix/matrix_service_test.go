package matrix

import (
	"bytes"
	"context"
	"io"
	"log"
	"maps"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"testing/synctest"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	"github.com/nicholas-fedor/shoutrrr/pkg/format"
	"github.com/nicholas-fedor/shoutrrr/pkg/services/chat/matrix/mocks"
	"github.com/nicholas-fedor/shoutrrr/pkg/types"
)

// fakeHomeserver answers Matrix API requests and counts password logins.
type fakeHomeserver struct {
	// logins counts POST requests to the login endpoint.
	logins atomic.Int32
	// failFlows makes the next login flows request fail with a server error.
	failFlows atomic.Bool
	// flowsStarted, when set, receives a value as each login flows request arrives.
	flowsStarted chan struct{}
	// releaseFlows, when set, holds each login flows request until it is closed.
	releaseFlows chan struct{}
	// mu guards tokens, schemes, and deadlines.
	mu sync.Mutex
	// tokens records the Authorization header of each message sent.
	tokens []string
	// schemes records the URL scheme of each request.
	schemes []string
	// deadlines records the deadline of each request's context.
	deadlines []time.Time
}

// Service test URLs.
const (
	// passwordURL is a Matrix URL that authenticates with a user and password.
	passwordURL = "matrix://user:SECRETpassword@matrix.example.com/?rooms=!room:example.com"
	// tokenURL is a Matrix URL that authenticates with an access token.
	tokenURL = "matrix://:token@matrix.example.com/?rooms=!room:example.com"
)

// Do answers a Matrix API request: login flows, password login, the joined
// rooms lookup, and message sends.
//
// Parameters:
//   - req: the request.
//
// Returns:
//   - *http.Response: the homeserver's response.
//   - error: the request context's error when it ends while a login flows
//     request is held, and nil otherwise.
func (f *fakeHomeserver) Do(req *http.Request) (*http.Response, error) {
	if req.Body != nil {
		_, _ = io.Copy(io.Discard, req.Body)
		_ = req.Body.Close()
	}

	deadline, _ := req.Context().Deadline()

	f.mu.Lock()
	f.schemes = append(f.schemes, req.URL.Scheme)
	f.deadlines = append(f.deadlines, deadline)
	f.mu.Unlock()

	if req.Method == http.MethodGet && req.URL.Path == apiLogin && f.releaseFlows != nil {
		// The notification is best effort, so a repeated request cannot block on a
		// full channel, and the hold ends early if the request is canceled.
		select {
		case f.flowsStarted <- struct{}{}:
		default:
		}

		select {
		case <-f.releaseFlows:
		case <-req.Context().Done():
			return nil, req.Context().Err()
		}
	}

	switch {
	case req.Method == http.MethodGet && req.URL.Path == apiLogin && f.failFlows.CompareAndSwap(true, false):
		return homeserverResponse(http.StatusInternalServerError, `{"errcode":"M_UNKNOWN","error":"unavailable"}`), nil
	case req.Method == http.MethodGet && req.URL.Path == apiLogin:
		return homeserverResponse(http.StatusOK, `{"flows":[{"type":"m.login.password"}]}`), nil
	case req.Method == http.MethodPost && req.URL.Path == apiLogin:
		f.logins.Add(1)

		return homeserverResponse(http.StatusOK, `{"access_token":"access"}`), nil
	case req.Method == http.MethodGet && req.URL.Path == apiJoinedRooms:
		return homeserverResponse(http.StatusOK,
			`{"joined_rooms":["!a:example.com","!b:example.com"]}`), nil
	case req.Method == http.MethodPut && strings.Contains(req.URL.Path, "/send/"):
		f.mu.Lock()
		f.tokens = append(f.tokens, req.Header.Get("Authorization"))
		f.mu.Unlock()

		return homeserverResponse(http.StatusOK, `{"event_id":"$event"}`), nil
	default:
		return homeserverResponse(http.StatusNotFound, `{"errcode":"M_NOT_FOUND","error":"not found"}`), nil
	}
}

// requestDeadlines returns the deadline of each request's context so far.
//
// Returns:
//   - []time.Time: the deadlines, in order.
func (f *fakeHomeserver) requestDeadlines() []time.Time {
	f.mu.Lock()
	defer f.mu.Unlock()

	return append([]time.Time(nil), f.deadlines...)
}

// requestSchemes returns the URL scheme of each request so far and clears
// them.
//
// Returns:
//   - []string: the schemes, in order.
func (f *fakeHomeserver) requestSchemes() []string {
	f.mu.Lock()
	defer f.mu.Unlock()

	schemes := f.schemes
	f.schemes = nil

	return schemes
}

// sentTokens returns the Authorization header of each message sent so far.
//
// Returns:
//   - []string: the headers, in order.
func (f *fakeHomeserver) sentTokens() []string {
	f.mu.Lock()
	defer f.mu.Unlock()

	return append([]string(nil), f.tokens...)
}

// homeserverResponse builds a JSON response with the given status and body.
//
// Parameters:
//   - status: the HTTP status code.
//   - body: the response body.
//
// Returns:
//   - *http.Response: the response.
func homeserverResponse(status int, body string) *http.Response {
	return &http.Response{
		StatusCode: status,
		Status:     http.StatusText(status),
		Header:     http.Header{"Content-Type": []string{"application/json"}},
		Body:       io.NopCloser(strings.NewReader(body)),
	}
}

// newTLSHomeserverService returns a service initialized for a password
// login to a TLS server that answers through homeserver. The server's
// certificate is self-signed, so only a client that skips verification or
// trusts it can reach the server.
//
// Parameters:
//   - t: the test.
//   - homeserver: the fake homeserver that answers the requests.
//   - query: query params appended to the service URL, each starting with &.
//   - logger: the service's logger.
//
// Returns:
//   - *Service: the service.
//   - *httptest.Server: the server, closed when the test ends.
func newTLSHomeserverService(
	t *testing.T,
	homeserver *fakeHomeserver,
	query string,
	logger types.StdLogger,
) (*Service, *httptest.Server) {
	t.Helper()

	server := httptest.NewUnstartedServer(http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		res, err := homeserver.Do(req)
		if err != nil {
			http.Error(w, err.Error(), http.StatusBadGateway)

			return
		}

		defer func() { _ = res.Body.Close() }()

		maps.Copy(w.Header(), res.Header)
		w.WriteHeader(res.StatusCode)
		_, _ = io.Copy(w, res.Body)
	}))
	// The rejected TLS handshakes of the verifying sends log through the test.
	server.Config.ErrorLog = log.New(t.Output(), "", 0)
	server.StartTLS()
	t.Cleanup(server.Close)

	service := &Service{}
	rawURL := "matrix://user:SECRETpassword@" + server.Listener.Addr().String() + "/?rooms=!room:example.com" + query
	require.NoError(t, service.Initialize(parseTestURL(t, rawURL), logger))

	return service, server
}

// newHomeserverService returns a service, not yet initialized, whose requests
// go to server through a mock HTTP client.
//
// Parameters:
//   - t: the test.
//   - server: the fake homeserver.
//
// Returns:
//   - *Service: the service.
//   - *mocks.MockHTTPClient: the HTTP client, for checking calls.
func newHomeserverService(t *testing.T, server *fakeHomeserver) (*Service, *mocks.MockHTTPClient) {
	t.Helper()

	httpClient := mocks.NewMockHTTPClient(t)
	httpClient.EXPECT().Do(mock.Anything).RunAndReturn(server.Do).Maybe()

	service := &Service{}
	service.SetHTTPClient(httpClient)

	return service, httpClient
}

// TestGetID verifies the service identifier, with and without a config.
func TestGetID(t *testing.T) {
	t.Parallel()

	assert.Equal(t, Scheme, (&Service{}).GetID())
	assert.Equal(t, "matrix", (&Service{Config: &Config{Host: "matrix.example.com"}}).GetID())
}

// TestInitializeRejectsIncompleteURLs verifies that Initialize requires a host
// and credentials.
func TestInitializeRejectsIncompleteURLs(t *testing.T) {
	t.Parallel()

	// tests lists incomplete URLs with the error each should produce.
	tests := []struct {
		name    string
		rawURL  string
		wantErr error
	}{
		{name: "no host", rawURL: "matrix://user:password@", wantErr: ErrMissingHost},
		{name: "no password", rawURL: "matrix://user@matrix.example.com", wantErr: ErrMissingCredentials},
		{name: "no user info", rawURL: "matrix://matrix.example.com", wantErr: ErrMissingCredentials},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			require.ErrorIs(t, (&Service{}).Initialize(parseTestURL(t, tt.rawURL), nil), tt.wantErr)
		})
	}
}

// TestInitializeDoesNotLogIn verifies that Initialize makes no requests.
func TestInitializeDoesNotLogIn(t *testing.T) {
	t.Parallel()

	service, httpClient := newHomeserverService(t, &fakeHomeserver{})

	require.NoError(t, service.Initialize(parseTestURL(t, passwordURL), nil))
	httpClient.AssertNotCalled(t, "Do", mock.Anything)
}

// TestFirstSendLogsIn verifies that the first send logs in through the injected
// client and later sends reuse the access token.
func TestFirstSendLogsIn(t *testing.T) {
	t.Parallel()

	server := &fakeHomeserver{}
	service, _ := newHomeserverService(t, server)
	require.NoError(t, service.Initialize(parseTestURL(t, passwordURL), nil))

	require.NoError(t, service.Send("first", nil))
	require.NoError(t, service.Send("second", nil))

	assert.Equal(t, int32(1), server.logins.Load())
	assert.Equal(t, []string{"Bearer access", "Bearer access"}, server.sentTokens())
}

// TestConcurrentSendsLogInOnce verifies that concurrent first sends share a
// single login.
func TestConcurrentSendsLogInOnce(t *testing.T) {
	t.Parallel()

	server := &fakeHomeserver{}
	service, _ := newHomeserverService(t, server)
	require.NoError(t, service.Initialize(parseTestURL(t, passwordURL), nil))

	var wg sync.WaitGroup
	for range 8 {
		wg.Go(func() {
			assert.NoError(t, service.Send("concurrent", nil))
		})
	}

	wg.Wait()
	assert.Equal(t, int32(1), server.logins.Load())
}

// TestFailedLoginIsRetried verifies that a failed login is reported without the
// password and retried on the next send.
func TestFailedLoginIsRetried(t *testing.T) {
	t.Parallel()

	server := &fakeHomeserver{}
	service, _ := newHomeserverService(t, server)
	require.NoError(t, service.Initialize(parseTestURL(t, passwordURL), nil))

	server.failFlows.Store(true)

	err := service.Send("first", nil)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "logging in to Matrix")
	assert.NotContains(t, err.Error(), "SECRETpassword")

	require.NoError(t, service.Send("second", nil))
	assert.Equal(t, int32(1), server.logins.Load())
}

// TestLoginWaitEndsWithContext verifies that a send waiting for another send's
// login stops when its own context ends, while the login continues.
func TestLoginWaitEndsWithContext(t *testing.T) {
	t.Parallel()

	server := &fakeHomeserver{flowsStarted: make(chan struct{}, 1), releaseFlows: make(chan struct{})}
	service, _ := newHomeserverService(t, server)
	require.NoError(t, service.Initialize(parseTestURL(t, passwordURL), nil))

	firstDone := make(chan error, 1)

	go func() { firstDone <- service.Send("first", nil) }()

	<-server.flowsStarted

	ctx, cancel := context.WithCancel(t.Context())
	cancel()

	require.ErrorIs(t, service.SendContext(ctx, "second", nil), context.Canceled)

	close(server.releaseFlows)
	require.NoError(t, <-firstDone)
	assert.Equal(t, int32(1), server.logins.Load())
}

// TestFailedInitializeDropsSession verifies that reinitializing with an
// invalid URL leaves the service unable to send, instead of sending with the
// earlier session's access token and a partly read config.
func TestFailedInitializeDropsSession(t *testing.T) {
	t.Parallel()

	service, httpClient := newHomeserverService(t, &fakeHomeserver{})
	require.NoError(t, service.Initialize(parseTestURL(t, tokenURL), nil))

	require.ErrorIs(t, service.Initialize(parseTestURL(t, "matrix://user@matrix.example.com"), nil), ErrMissingCredentials)

	require.ErrorIs(t, service.Send("message", nil), ErrClientNotInitialized)
	httpClient.AssertNotCalled(t, "Do", mock.Anything)
}

// TestAccessTokenSkipsLogin verifies that an access token is used without a
// login.
func TestAccessTokenSkipsLogin(t *testing.T) {
	t.Parallel()

	server := &fakeHomeserver{}
	service, _ := newHomeserverService(t, server)
	require.NoError(t, service.Initialize(parseTestURL(t, tokenURL), nil))

	require.NoError(t, service.Send("message", nil))
	assert.Zero(t, server.logins.Load())
	assert.Equal(t, []string{"Bearer token"}, server.sentTokens())
}

// TestSendWithoutClient verifies that a service without a homeserver to send
// to reports ErrClientNotInitialized.
func TestSendWithoutClient(t *testing.T) {
	t.Parallel()

	for _, config := range []*Config{{}, {Host: "matrix.example.com"}} {
		require.ErrorIs(t, (&Service{Config: config}).Send("test message", nil), ErrClientNotInitialized)
	}
}

// TestServiceTimeout verifies that the budget covers the login and a join and
// message per room, uses rooms from valid params, and has a two-request floor.
func TestServiceTimeout(t *testing.T) {
	t.Parallel()

	// tests lists service URLs and params with the budget to expect. An empty
	// URL leaves the service uninitialized.
	tests := []struct {
		name   string
		rawURL string
		params *types.Params
		want   int
	}{
		{name: "configured room", rawURL: passwordURL, want: 4},
		{name: "rooms from params", rawURL: passwordURL, params: &types.Params{"rooms": "!a:example.com,!b:example.com,!c:example.com"}, want: 8},
		{name: "configured room when a param is invalid", rawURL: passwordURL, params: &types.Params{"rooms": "!a:example.com,!b:example.com", "bogus": "value"}, want: 4},
		{name: "joined rooms without configured rooms", rawURL: "matrix://:token@matrix.example.com/", want: 2},
		{name: "before initialization", want: 2},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			service := &Service{}
			if tt.rawURL != "" {
				require.NoError(t, service.Initialize(parseTestURL(t, tt.rawURL), nil))
			}

			assert.Equal(t, defaultHTTPTimeout*time.Duration(tt.want), service.ServiceTimeout(tt.params))
		})
	}
}

// TestSendShares verifies that each room's requests may use an equal share of
// the time left, so a room that does not answer cannot use up the other's
// time. The first of two rooms gets half of the time, and the last gets the
// rest. For joined rooms, the time is what is left after the lookup.
func TestSendShares(t *testing.T) {
	t.Parallel()

	// tests lists URLs with two rooms, with the number of requests before the
	// room sends.
	tests := []struct {
		name   string
		rawURL string
		before int
	}{
		{
			name:   "configured rooms",
			rawURL: "matrix://:token@matrix.example.com/?rooms=!a:example.com,!b:example.com",
		},
		{name: "joined rooms", rawURL: "matrix://:token@matrix.example.com/", before: 1},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			synctest.Test(t, func(t *testing.T) {
				server := &fakeHomeserver{}
				service, _ := newHomeserverService(t, server)
				require.NoError(t, service.Initialize(parseTestURL(t, tt.rawURL), nil))

				start := time.Now()
				budget := service.ServiceTimeout(nil)

				require.NoError(t, service.SendContext(t.Context(), "message", nil))

				deadlines := server.requestDeadlines()
				require.Len(t, deadlines, tt.before+2)
				assert.Equal(t, []time.Time{start.Add(budget / 2), start.Add(budget)},
					deadlines[tt.before:])
			})
		})
	}
}

// TestSendBudget verifies that every request of a send, the login flows
// lookup, the login, and the message, carries the budget that ServiceTimeout
// reports when the caller sets no deadline, and the caller's deadline
// otherwise.
func TestSendBudget(t *testing.T) {
	t.Parallel()

	// tests lists the caller deadlines, or zero for none, with the request
	// deadline to expect, measured from the start of the send.
	tests := []struct {
		name           string
		callerDeadline time.Duration
	}{
		{name: "no caller deadline"},
		{name: "caller deadline", callerDeadline: time.Hour},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			synctest.Test(t, func(t *testing.T) {
				server := &fakeHomeserver{}
				service, _ := newHomeserverService(t, server)
				require.NoError(t, service.Initialize(parseTestURL(t, passwordURL), nil))

				ctx := t.Context()
				want := time.Now().Add(service.ServiceTimeout(nil))

				if tt.callerDeadline > 0 {
					var cancel context.CancelFunc

					ctx, cancel = context.WithTimeout(ctx, tt.callerDeadline)
					defer cancel()

					want = time.Now().Add(tt.callerDeadline)
				}

				require.NoError(t, service.SendContext(ctx, "message", nil))
				assert.Equal(t, []time.Time{want, want, want}, server.requestDeadlines())
			})
		})
	}
}

// TestSendWarnsPlaintext verifies that sends without TLS log the plaintext
// warning once, and again after the service is initialized again.
func TestSendWarnsPlaintext(t *testing.T) {
	t.Parallel()

	var logs bytes.Buffer

	server := &fakeHomeserver{}
	service, _ := newHomeserverService(t, server)
	require.NoError(t, service.Initialize(parseTestURL(t, tokenURL+"&disabletls=yes"), log.New(&logs, "", 0)))

	require.NoError(t, service.Send("first", nil))
	require.NoError(t, service.Send("second", nil))
	assert.Equal(t, 1, strings.Count(logs.String(), "WARN sending without TLS"))

	require.NoError(t, service.Initialize(parseTestURL(t, tokenURL+"&disabletls=yes"), log.New(&logs, "", 0)))
	require.NoError(t, service.Send("third", nil))
	assert.Equal(t, 2, strings.Count(logs.String(), "WARN sending without TLS"))
}

// TestSendAppliesDisableTLSParam verifies that a disabletls send param decides
// the protocol of every request in that send, including the first send's
// login, and that later sends keep the URL's setting and the access token.
func TestSendAppliesDisableTLSParam(t *testing.T) {
	t.Parallel()

	// tests lists URLs and send params with the scheme each request of the
	// first send uses and the scheme of a later send without params.
	tests := []struct {
		params      types.Params
		name        string
		rawURL      string
		wantScheme  string
		laterScheme string
		wantWarning bool
	}{
		{
			name:        "param turns TLS off",
			rawURL:      passwordURL,
			params:      types.Params{"disabletls": "yes"},
			wantScheme:  "http",
			laterScheme: "https",
			wantWarning: true,
		},
		{
			name:        "param turns TLS back on",
			rawURL:      passwordURL + "&disabletls=yes",
			params:      types.Params{"disabletls": "no"},
			wantScheme:  "https",
			laterScheme: "http",
			wantWarning: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			var logs bytes.Buffer

			server := &fakeHomeserver{}
			service, _ := newHomeserverService(t, server)
			require.NoError(t, service.Initialize(parseTestURL(t, tt.rawURL), log.New(&logs, "", 0)))

			require.NoError(t, service.Send("first", &tt.params))

			// The login flows lookup, the login, and the message.
			assert.Equal(t, []string{tt.wantScheme, tt.wantScheme, tt.wantScheme}, server.requestSchemes())
			assert.Equal(t, tt.wantWarning, strings.Contains(logs.String(), "WARN sending without TLS"))

			require.NoError(t, service.Send("second", nil))

			assert.Equal(t, []string{tt.laterScheme}, server.requestSchemes())
			assert.Equal(t, int32(1), server.logins.Load())
			assert.Equal(t, []string{"Bearer access", "Bearer access"}, server.sentTokens())
		})
	}
}

// TestSendSkipTLSVerify verifies sends to a homeserver whose certificate
// cannot be verified. Without skiptlsverify the send fails. With it in the
// URL or the params, the login and the message go through the insecure
// default client, and the warning is logged once over two sends. An injected
// client is used unchanged and never warned about.
func TestSendSkipTLSVerify(t *testing.T) {
	t.Parallel()

	const insecureWarning = "WARN skipping TLS certificate verification"

	// tests lists the URL query, the send params, and whether a client is
	// injected, with the expected outcome of both sends.
	tests := []struct {
		params       *types.Params
		name         string
		query        string
		injected     bool
		wantErr      bool
		wantWarnings int
	}{
		{name: "verified by default", wantErr: true},
		{name: "URL", query: "&skiptlsverify=yes", wantWarnings: 1},
		{name: "params", params: &types.Params{"skiptlsverify": "yes"}, wantWarnings: 1},
		{name: "injected client", query: "&skiptlsverify=yes", injected: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			var logs strings.Builder

			homeserver := &fakeHomeserver{}
			service, server := newTLSHomeserverService(t, homeserver, tt.query, log.New(&logs, "", 0))

			if tt.injected {
				service.SetHTTPClient(server.Client())
			}

			for range 2 {
				err := service.Send("message", tt.params)
				require.Equal(t, tt.wantErr, err != nil, "send error: %v", err)
			}

			wantLogins := int32(1)
			if tt.wantErr {
				wantLogins = 0
			}

			assert.Equal(t, wantLogins, homeserver.logins.Load())
			assert.Equal(t, tt.wantWarnings, strings.Count(logs.String(), insecureWarning))
		})
	}

	t.Run("params apply to one send", func(t *testing.T) {
		t.Parallel()

		homeserver := &fakeHomeserver{}
		service, _ := newTLSHomeserverService(t, homeserver, "", log.New(t.Output(), "", 0))

		require.NoError(t, service.Send("first", &types.Params{"skiptlsverify": "yes"}))
		require.Error(t, service.Send("second", nil))
		assert.Equal(t, []string{"Bearer access"}, homeserver.sentTokens())
	})
}

// TestSendRejectsInvalidParamBeforeLogin verifies that an invalid send param
// fails the send before the login makes any request.
func TestSendRejectsInvalidParamBeforeLogin(t *testing.T) {
	t.Parallel()

	service, httpClient := newHomeserverService(t, &fakeHomeserver{})
	require.NoError(t, service.Initialize(parseTestURL(t, passwordURL), nil))

	err := service.Send("message", &types.Params{"disabletls": "maybe"})

	require.ErrorIs(t, err, format.ErrInvalidBoolValue)
	httpClient.AssertNotCalled(t, "Do", mock.Anything)
}

// TestDefaultHTTPClient verifies that the default client has no request
// timeout, since the send budget bounds each send, and sends through
// http.DefaultTransport, so it honors the proxy environment variables.
func TestDefaultHTTPClient(t *testing.T) {
	t.Parallel()

	service := &Service{}
	require.NoError(t, service.Initialize(parseTestURL(t, "matrix://user:pass@matrix.example.com"), log.New(t.Output(), "", 0)))

	httpClient, ok := service.httpClient.Client().(*http.Client)
	require.True(t, ok)
	assert.Zero(t, httpClient.Timeout)
	assert.Nil(t, httpClient.Transport)
}
