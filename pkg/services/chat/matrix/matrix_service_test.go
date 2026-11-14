package matrix

import (
	"bytes"
	"context"
	"encoding/json/v2"
	"fmt"
	"io"
	"log"
	"maps"
	"math"
	"net/http"
	"net/http/httptest"
	"slices"
	"strconv"
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
	"github.com/nicholas-fedor/shoutrrr/pkg/util"
)

// fakeHomeserver answers Matrix API requests and counts password logins.
type fakeHomeserver struct {
	// logins counts POST requests to the login endpoint.
	logins atomic.Int32
	// failFlows makes the next login flows request fail with a server error.
	failFlows atomic.Bool
	// omitToken makes the next login reply leave out the access token.
	omitToken atomic.Bool
	// authorizedLogins counts password logins that carried an Authorization
	// header, which the login endpoint does not take.
	authorizedLogins atomic.Int32
	// rejectBelow, when not zero, makes the homeserver reject as unknown each
	// message sent with a token from a login numbered below it, or with a
	// token no login issued. It stands for a token that another login to the
	// same device replaced, or that was revoked.
	rejectBelow atomic.Int32
	// releaseRejections, when set, holds each rejection of a message until it
	// is closed.
	releaseRejections chan struct{}
	// flowsStarted, when set, receives a value as each login flows request arrives.
	flowsStarted chan struct{}
	// releaseFlows, when set, holds each login flows request until it is closed.
	releaseFlows chan struct{}
	// mu guards tokens, hosts, loginAccounts, schemes, and deadlines.
	mu sync.Mutex
	// tokens records the Authorization header of each message sent.
	tokens []string
	// hosts records the URL host of each message sent.
	hosts []string
	// loginAccounts records the host, user, and password of each password
	// login, separated by spaces.
	loginAccounts []string
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
//     request or a rejection is held, and nil otherwise.
func (f *fakeHomeserver) Do(req *http.Request) (*http.Response, error) {
	var body []byte

	if req.Body != nil {
		body, _ = io.ReadAll(req.Body)
		_ = req.Body.Close()
	}

	deadline, _ := req.Context().Deadline()

	f.mu.Lock()
	f.schemes = append(f.schemes, req.URL.Scheme)
	f.deadlines = append(f.deadlines, deadline)
	f.mu.Unlock()

	if req.Method == http.MethodGet && req.URL.Path == apiLogin && f.releaseFlows != nil {
		if err := hold(req.Context(), f.flowsStarted, f.releaseFlows); err != nil {
			return nil, err
		}
	}

	switch {
	case req.Method == http.MethodGet && req.URL.Path == apiLogin && f.failFlows.CompareAndSwap(true, false):
		return homeserverResponse(http.StatusInternalServerError, `{"errcode":"M_UNKNOWN","error":"unavailable"}`), nil
	case req.Method == http.MethodGet && req.URL.Path == apiLogin:
		return homeserverResponse(http.StatusOK, `{"flows":[{"type":"m.login.password"}]}`), nil
	case req.Method == http.MethodPost && req.URL.Path == apiLogin:
		if req.Header.Get("Authorization") != "" {
			f.authorizedLogins.Add(1)
		}

		var login apiReqLogin
		if err := json.Unmarshal(body, &login); err == nil && login.Identifier.User != "" {
			f.mu.Lock()
			f.loginAccounts = append(f.loginAccounts, req.URL.Host+" "+login.Identifier.User+" "+login.Password)
			f.mu.Unlock()
		}

		// Each login issues a numbered token, so a test can tell which login a
		// message used.
		token := fmt.Sprintf("access-%d", f.logins.Add(1))
		if f.omitToken.CompareAndSwap(true, false) {
			return homeserverResponse(http.StatusOK, `{"home_server":"matrix.example.com"}`), nil
		}

		return homeserverResponse(http.StatusOK, `{"access_token":"`+token+`"}`), nil
	case req.Method == http.MethodGet && req.URL.Path == apiJoinedRooms:
		return homeserverResponse(http.StatusOK,
			`{"joined_rooms":["!a:example.com","!b:example.com"]}`), nil
	case req.Method == http.MethodPut && strings.Contains(req.URL.Path, "/send/"):
		f.mu.Lock()
		f.tokens = append(f.tokens, req.Header.Get("Authorization"))
		f.hosts = append(f.hosts, req.URL.Host)
		f.mu.Unlock()

		if f.rejects(req.Header.Get("Authorization")) {
			if f.releaseRejections != nil {
				if err := hold(req.Context(), nil, f.releaseRejections); err != nil {
					return nil, err
				}
			}

			return homeserverResponse(unknownToken.status, unknownToken.body), nil
		}

		return homeserverResponse(http.StatusOK, `{"event_id":"$event"}`), nil
	default:
		return homeserverResponse(http.StatusNotFound, `{"errcode":"M_NOT_FOUND","error":"not found"}`), nil
	}
}

// loggedInAccounts returns the host, user, and password of each password login
// so far, separated by spaces.
//
// Returns:
//   - []string: the accounts, in order.
func (f *fakeHomeserver) loggedInAccounts() []string {
	f.mu.Lock()
	defer f.mu.Unlock()

	return append([]string(nil), f.loginAccounts...)
}

// rejects reports whether the homeserver rejects a message sent with the
// Authorization header authorization. Once a test names the first login
// whose tokens it accepts, it rejects the tokens of earlier logins and tokens
// no login issued.
//
// Parameters:
//   - authorization: the Authorization header of the message.
//
// Returns:
//   - bool: whether the token is unknown to the homeserver.
func (f *fakeHomeserver) rejects(authorization string) bool {
	below := f.rejectBelow.Load()
	if below == 0 {
		return false
	}

	login, err := strconv.Atoi(strings.TrimPrefix(authorization, "Bearer access-"))

	return err != nil || int32(login) < below
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

// sentHosts returns the URL host of each message sent so far.
//
// Returns:
//   - []string: the hosts, in order.
func (f *fakeHomeserver) sentHosts() []string {
	f.mu.Lock()
	defer f.mu.Unlock()

	return append([]string(nil), f.hosts...)
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

// hold blocks a request of the fake homeserver until release is closed.
//
// The notification on started is best effort, so a repeated request never
// blocks on a full channel. The hold ends early when ctx ends.
//
// Parameters:
//   - ctx: the request's context.
//   - started: receives a value as the hold starts, while it has room, or nil.
//   - release: holds the request until it is closed.
//
// Returns:
//   - error: ctx's error when it ends during the hold, or nil.
func hold(ctx context.Context, started, release chan struct{}) error {
	select {
	case started <- struct{}{}:
	default:
	}

	select {
	case <-release:
		return nil
	case <-ctx.Done():
		return ctx.Err()
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
	assert.Equal(t, []string{"Bearer access-1", "Bearer access-1"}, server.sentTokens())
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

// TestLoginWithoutTokenIsRetried verifies that a login reply without an access
// token fails the send and stores no session. The next send logs in again and
// sends with the new token, instead of sending without authentication.
func TestLoginWithoutTokenIsRetried(t *testing.T) {
	t.Parallel()

	server := &fakeHomeserver{}
	service, _ := newHomeserverService(t, server)
	require.NoError(t, service.Initialize(parseTestURL(t, passwordURL), nil))

	server.omitToken.Store(true)

	require.ErrorIs(t, service.Send("first", nil), ErrLoginReplyWithoutToken)
	require.NoError(t, service.Send("second", nil))
	assert.Equal(t, int32(2), server.logins.Load())
	assert.Equal(t, []string{"Bearer access-2"}, server.sentTokens())
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

// TestRejectedTokenLogsInAgain verifies that a send whose stored token the
// homeserver rejects as unknown logs in again and sends the message with the
// new token, as after another process logs in to the same device. The login
// carries no token, and later sends reuse the new one.
func TestRejectedTokenLogsInAgain(t *testing.T) {
	t.Parallel()

	server := &fakeHomeserver{}
	service, _ := newHomeserverService(t, server)
	require.NoError(t, service.Initialize(parseTestURL(t, passwordURL), nil))

	require.NoError(t, service.Send("first", nil))

	server.rejectBelow.Store(2)

	require.NoError(t, service.Send("second", nil))
	require.NoError(t, service.Send("third", nil))

	assert.Equal(t, int32(2), server.logins.Load())
	assert.Equal(t,
		[]string{"Bearer access-1", "Bearer access-1", "Bearer access-2", "Bearer access-2"},
		server.sentTokens())
	assert.Zero(t, server.authorizedLogins.Load(), "logins carry no access token")
}

// TestConcurrentRejectedSendsLogInOnce verifies that concurrent sends whose
// stored token is rejected share one new login. The rejections are held
// until every send has sent the rejected token, so every send logs in again
// at once. A send that waited for the gate takes the token the first one
// stored.
func TestConcurrentRejectedSendsLogInOnce(t *testing.T) {
	t.Parallel()

	synctest.Test(t, func(t *testing.T) {
		const sends = 8

		server := &fakeHomeserver{}
		service, _ := newHomeserverService(t, server)
		require.NoError(t, service.Initialize(parseTestURL(t, passwordURL), nil))
		require.NoError(t, service.Send("first", nil))

		server.rejectBelow.Store(2)
		server.releaseRejections = make(chan struct{})

		var wg sync.WaitGroup
		for range sends {
			wg.Go(func() {
				assert.NoError(t, service.Send("concurrent", nil))
			})
		}

		// Every send now waits on its rejection.
		synctest.Wait()
		close(server.releaseRejections)
		wg.Wait()

		assert.Equal(t, int32(2), server.logins.Load())
		assert.Equal(t,
			append(slices.Repeat([]string{"Bearer access-1"}, sends+1), slices.Repeat([]string{"Bearer access-2"}, sends)...),
			server.sentTokens())
	})
}

// TestRejectedTokenLogsInOncePerSend verifies that a send logs in again at
// most once, even when the token its own first login stored is rejected.
// When the homeserver also rejects the new token, the send fails with its
// error.
func TestRejectedTokenLogsInOncePerSend(t *testing.T) {
	t.Parallel()

	server := &fakeHomeserver{}
	server.rejectBelow.Store(math.MaxInt32)

	service, _ := newHomeserverService(t, server)
	require.NoError(t, service.Initialize(parseTestURL(t, passwordURL), nil))

	err := service.Send("message", nil)

	assert.Equal(t, errCodeUnknownToken, requireAPIError(t, err).Code)
	assert.Equal(t, int32(2), server.logins.Load())
}

// TestRejectedAccessTokenFails verifies that a configured access token the
// homeserver rejects fails the send without a login, since the service has no
// password to log in with.
func TestRejectedAccessTokenFails(t *testing.T) {
	t.Parallel()

	server := &fakeHomeserver{}
	server.rejectBelow.Store(1)

	service, _ := newHomeserverService(t, server)
	require.NoError(t, service.Initialize(parseTestURL(t, tokenURL), nil))

	err := service.Send("message", nil)

	assert.Equal(t, errCodeUnknownToken, requireAPIError(t, err).Code)
	assert.Zero(t, server.logins.Load())
}

// TestFailedReloginDropsSession verifies that a failed login after a rejected
// token fails the send without the password. The send's later rooms fail
// with the login's error without being sent. The next send logs in at once
// instead of trying the rejected token again.
func TestFailedReloginDropsSession(t *testing.T) {
	t.Parallel()

	server := &fakeHomeserver{}
	service, _ := newHomeserverService(t, server)
	require.NoError(t, service.Initialize(parseTestURL(t, passwordURL), nil))
	require.NoError(t, service.Send("first", nil))

	server.rejectBelow.Store(2)
	server.failFlows.Store(true)

	err := service.Send("second", &types.Params{"rooms": "!a:example.com,!b:example.com"})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "access token rejected")
	assert.NotContains(t, err.Error(), "SECRETpassword")

	require.NoError(t, service.Send("third", nil))
	assert.Equal(t,
		[]string{"Bearer access-1", "Bearer access-1", "Bearer access-2"},
		server.sentTokens())
}

// TestRenewalOutOfRoomTimeIsRetried verifies that a login after a rejected
// token that runs out of its room's share of the send fails only that room.
// The next room, with its own share, logs in again before it sends, so it
// never sends the rejected token, and sends with the new token.
func TestRenewalOutOfRoomTimeIsRetried(t *testing.T) {
	t.Parallel()

	synctest.Test(t, func(t *testing.T) {
		server := &fakeHomeserver{}
		service, _ := newHomeserverService(t, server)
		require.NoError(t, service.Initialize(parseTestURL(t, passwordURL), nil))
		require.NoError(t, service.Send("first", nil))

		server.rejectBelow.Store(2)
		server.flowsStarted = make(chan struct{}, 1)
		server.releaseFlows = make(chan struct{})

		done := make(chan error, 1)

		go func() {
			done <- service.Send("second", &types.Params{"rooms": "!a:example.com,!b:example.com"})
		}()

		// The first room's login waits until its share runs out. The second
		// room's login goes through.
		<-server.flowsStarted
		<-server.flowsStarted
		close(server.releaseFlows)

		err := <-done
		require.ErrorIs(t, err, context.DeadlineExceeded)
		assert.Contains(t, err.Error(), "!a:example.com")
		assert.NotContains(t, err.Error(), "!b:example.com")
		assert.Equal(t, int32(2), server.logins.Load())
		assert.Equal(t,
			[]string{"Bearer access-1", "Bearer access-1", "Bearer access-2"},
			server.sentTokens())
	})
}

// TestFailedInitializeDropsSession verifies that reinitializing with an
// invalid URL leaves the service unable to send, instead of sending with the
// earlier session's access token and a partly read config.
func TestFailedInitializeDropsSession(t *testing.T) {
	t.Parallel()

	service, httpClient := newHomeserverService(t, &fakeHomeserver{})
	require.NoError(t, service.Initialize(parseTestURL(t, tokenURL), nil))

	require.ErrorIs(t, service.Initialize(parseTestURL(t, "matrix://user@matrix.example.com"), nil), ErrMissingCredentials)

	err := service.Send("message", nil)
	require.ErrorIs(t, err, ErrClientNotInitialized)
	require.ErrorIs(t, err, types.ErrServiceNotInitialized)
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

// TestSessionFollowsConfig verifies that a send authenticates for the account
// its config names. A config replaced in code after Initialize sends its own
// access token. With a password, it logs in again for another homeserver,
// user, or password. The token of the earlier account never reaches the new
// one.
func TestSessionFollowsConfig(t *testing.T) {
	t.Parallel()

	t.Run("access token for another homeserver", func(t *testing.T) {
		t.Parallel()

		server := &fakeHomeserver{}
		service, _ := newHomeserverService(t, server)
		require.NoError(t, service.Initialize(parseTestURL(t, tokenURL), nil))
		require.NoError(t, service.Send("first", nil))

		config := *service.Config
		config.Host, config.Password = "other.example.com", "other-token"
		service.Config = &config

		require.NoError(t, service.Send("second", nil))
		assert.Equal(t, []string{"matrix.example.com", "other.example.com"}, server.sentHosts())
		assert.Equal(t, []string{"Bearer token", "Bearer other-token"}, server.sentTokens())
	})

	// changes lists config changes that name another account, each of which
	// needs a new login, with the account and host of the second send.
	changes := map[string]struct {
		change      func(config *Config)
		wantAccount string
		wantHost    string
	}{
		"another homeserver": {
			change:      func(config *Config) { config.Host = "other.example.com" },
			wantAccount: "other.example.com user SECRETpassword",
			wantHost:    "other.example.com",
		},
		"another user": {
			change:      func(config *Config) { config.User = "other" },
			wantAccount: "matrix.example.com other SECRETpassword",
			wantHost:    "matrix.example.com",
		},
		"another password": {
			change:      func(config *Config) { config.Password = "other-password" },
			wantAccount: "matrix.example.com user other-password",
			wantHost:    "matrix.example.com",
		},
	}

	for name, tt := range changes {
		t.Run("password login for "+name, func(t *testing.T) {
			t.Parallel()

			server := &fakeHomeserver{}
			service, _ := newHomeserverService(t, server)
			require.NoError(t, service.Initialize(parseTestURL(t, passwordURL), nil))
			require.NoError(t, service.Send("first", nil))

			config := *service.Config
			tt.change(&config)
			service.Config = &config

			require.NoError(t, service.Send("second", nil))
			assert.Equal(t, int32(2), server.logins.Load())
			assert.Equal(t, []string{"matrix.example.com user SECRETpassword", tt.wantAccount}, server.loggedInAccounts())
			assert.Equal(t, []string{"matrix.example.com", tt.wantHost}, server.sentHosts())
			assert.Equal(t, []string{"Bearer access-1", "Bearer access-2"}, server.sentTokens())
		})
	}
}

// TestInitializeEndsSession verifies that Initialize drops the token of the
// earlier login, so the next send logs in again even for the same account.
func TestInitializeEndsSession(t *testing.T) {
	t.Parallel()

	server := &fakeHomeserver{}
	service, _ := newHomeserverService(t, server)

	for range 2 {
		require.NoError(t, service.Initialize(parseTestURL(t, passwordURL), nil))
		require.NoError(t, service.Send("message", nil))
	}

	assert.Equal(t, int32(2), server.logins.Load())
}

// TestSendWithoutClient verifies that a service without a homeserver to send
// to reports ErrClientNotInitialized, wrapped with the shared
// types.ErrServiceNotInitialized that every service returns when it cannot
// send.
func TestSendWithoutClient(t *testing.T) {
	t.Parallel()

	for _, config := range []*Config{{}, {Host: "matrix.example.com"}} {
		err := (&Service{Config: config}).Send("test message", nil)
		require.ErrorIs(t, err, ErrClientNotInitialized)
		require.ErrorIs(t, err, types.ErrServiceNotInitialized)
	}
}

// TestServiceTimeout verifies that the budget covers the login and a join and
// message per room, uses rooms from valid params, and has a two-request floor.
//
// Empty rooms in a config built in code do not count, since the send drops
// them.
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
		{name: "configured room", rawURL: passwordURL, want: 5},
		{name: "rooms from params", rawURL: passwordURL, params: &types.Params{"rooms": "!a:example.com,!b:example.com,!c:example.com"}, want: 9},
		{name: "configured room when a param is invalid", rawURL: passwordURL, params: &types.Params{"rooms": "!a:example.com,!b:example.com", "bogus": "value"}, want: 5},
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

	t.Run("empty rooms", func(t *testing.T) {
		t.Parallel()

		service := &Service{}
		require.NoError(t, service.Initialize(parseTestURL(t, passwordURL), nil))
		service.Config.Rooms = []string{"", "!ops:example.com", ""}

		assert.Equal(t, 5*defaultHTTPTimeout, service.ServiceTimeout(nil))
	})
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

// TestSendBudgetFromParams verifies that rooms set by params set the send's
// budget, though the service URL names one room. The login requests carry the
// budget for two rooms. The first room's message gets half of it, and the
// last room's message keeps the rest.
func TestSendBudgetFromParams(t *testing.T) {
	t.Parallel()

	synctest.Test(t, func(t *testing.T) {
		server := &fakeHomeserver{}
		service, _ := newHomeserverService(t, server)
		require.NoError(t, service.Initialize(parseTestURL(t, passwordURL), nil))

		params := types.Params{"rooms": "!first:example.com,!second:example.com"}
		start := time.Now()
		budget := service.ServiceTimeout(&params)

		require.NoError(t, service.Send("message", &params))
		assert.Equal(t,
			[]time.Time{start.Add(budget), start.Add(budget), start.Add(budget / 2), start.Add(budget)},
			server.requestDeadlines())
	})
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
			assert.Equal(t, []string{"Bearer access-1", "Bearer access-1"}, server.sentTokens())
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
		assert.Equal(t, []string{"Bearer access-1"}, homeserver.sentTokens())
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

// TestSendChecksConfig verifies that a send checks a config built in code as
// reading a URL does.
//
// A config without credentials or a homeserver host fails before the login
// makes any request. So does a homeserver port outside 1 to 65535.
func TestSendChecksConfig(t *testing.T) {
	t.Parallel()

	// tests lists config changes with the error the send must return.
	tests := []struct {
		change  func(config *Config)
		wantErr error
		name    string
	}{
		{name: "zero config", change: func(config *Config) { *config = Config{} }, wantErr: ErrMissingCredentials},
		{name: "no password", change: func(config *Config) { config.Password = "" }, wantErr: ErrMissingCredentials},
		{name: "port out of range", change: func(config *Config) { config.Host = "matrix.example.com:99999" }, wantErr: util.ErrInvalidPort},
		{name: "no host", change: func(config *Config) { config.Host = "" }, wantErr: ErrMissingHost},
		{name: "port without a host", change: func(config *Config) { config.Host = ":8448" }, wantErr: ErrMissingHost},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			service, httpClient := newHomeserverService(t, &fakeHomeserver{})
			require.NoError(t, service.Initialize(parseTestURL(t, passwordURL), nil))

			tt.change(service.Config)

			require.ErrorIs(t, service.Send("message", nil), tt.wantErr)
			httpClient.AssertNotCalled(t, "Do", mock.Anything)
		})
	}
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
