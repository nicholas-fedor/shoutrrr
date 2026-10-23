package smtp

import (
	"context"
	"encoding/base64"
	"io"
	"log"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"testing/synctest"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"golang.org/x/oauth2"

	"github.com/nicholas-fedor/shoutrrr/pkg/types"
)

// tokenResponse is one scripted answer of a fakeTokenEndpoint.
type tokenResponse struct {
	// body is the JSON response body.
	body string
	// status is the HTTP status code.
	status int
}

// fakeTokenEndpoint is an OAuth2 token endpoint, used as the injected HTTP
// client, that records each token request and answers from a script.
type fakeTokenEndpoint struct {
	// responses are the answers in order. The last one repeats.
	responses []tokenResponse
	// forms holds the form of each token request received.
	forms []url.Values
	// mu guards responses and forms.
	mu sync.Mutex
}

// blockingTokenEndpoint is an OAuth2 token endpoint, used as the injected HTTP
// client, that holds requests for the refresh tokens "slow" and "slow-revoked"
// until release is closed or the request's context ends. It rejects
// "slow-revoked" with invalid_grant and answers every other grant with the
// access token "<refresh token>-token".
type blockingTokenEndpoint struct {
	// release lets held requests answer when closed.
	release chan struct{}
	// mu guards requests.
	mu sync.Mutex
	// requests counts the token requests received.
	requests int
}

// failingTokenClient is an injected HTTP client whose token requests fail with
// a transport error that names the full token URL, as net/http's do.
type failingTokenClient struct{}

// testRefreshURL is a service URL that refreshes its access token from the
// fake token endpoint and authenticates with XOAUTH2.
const testRefreshURL = "smtp://user@localhost:2525/?fromaddress=sender@example.com&toaddresses=rec@example.com&usestarttls=no" +
	"&clientid=client&clientsecret=secret&refreshtoken=refresh&tokenurl=https%3A%2F%2Foauth.example.com%2Ftoken"

// newFakeTokenEndpoint returns an endpoint that answers with the given
// responses in order.
//
// Parameters:
//   - responses: the answers, the last of which repeats.
//
// Returns:
//   - *fakeTokenEndpoint: the endpoint.
func newFakeTokenEndpoint(responses ...tokenResponse) *fakeTokenEndpoint {
	return &fakeTokenEndpoint{responses: responses, forms: nil, mu: sync.Mutex{}}
}

// grant returns a successful token response with an access token, its
// lifetime in seconds, and an optional new refresh token.
//
// Parameters:
//   - accessToken: the access token.
//   - expiresIn: the lifetime in seconds, as the expires_in field.
//   - refreshToken: a rotated refresh token, or empty for none.
//
// Returns:
//   - tokenResponse: the response.
func grant(accessToken, expiresIn, refreshToken string) tokenResponse {
	body := `{"access_token":"` + accessToken + `","token_type":"Bearer","expires_in":` + expiresIn
	if refreshToken != "" {
		body += `,"refresh_token":"` + refreshToken + `"`
	}

	return tokenResponse{body: body + "}", status: http.StatusOK}
}

// Do records the token request and answers it.
//
// Parameters:
//   - req: the token request.
//
// Returns:
//   - *http.Response: the scripted response.
//   - error: the error reading the request body.
func (e *fakeTokenEndpoint) Do(req *http.Request) (*http.Response, error) {
	body, err := io.ReadAll(req.Body)
	if err != nil {
		return nil, err
	}

	form, err := url.ParseQuery(string(body))
	if err != nil {
		return nil, err
	}

	e.mu.Lock()
	defer e.mu.Unlock()

	e.forms = append(e.forms, form)

	response := e.responses[min(len(e.forms), len(e.responses))-1]

	return &http.Response{
		StatusCode: response.status,
		Status:     http.StatusText(response.status),
		Header:     http.Header{"Content-Type": []string{"application/json"}},
		Body:       io.NopCloser(strings.NewReader(response.body)),
		Request:    req,
	}, nil
}

// requests returns the forms of the token requests received so far.
//
// Returns:
//   - []url.Values: the forms, in order.
func (e *fakeTokenEndpoint) requests() []url.Values {
	e.mu.Lock()
	defer e.mu.Unlock()

	return append([]url.Values(nil), e.forms...)
}

// newRefreshingService returns a service from rawURL that gets its tokens from
// endpoint and connects to server.
//
// Parameters:
//   - t: the test.
//   - rawURL: the service URL.
//   - server: the SMTP server.
//   - endpoint: the token endpoint.
//
// Returns:
//   - *Service: the service.
func newRefreshingService(t *testing.T, rawURL string, server *fakeSMTP, endpoint types.HTTPClient) *Service {
	t.Helper()

	service := newTestService(t, rawURL, server)
	service.SetHTTPClient(endpoint)

	return service
}

// xoauth2Line returns the AUTH line a client sends for the test URLs' user,
// "user", with accessToken.
//
// Parameters:
//   - accessToken: the access token.
//
// Returns:
//   - string: the AUTH XOAUTH2 command line.
func xoauth2Line(accessToken string) string {
	return "AUTH XOAUTH2 " + base64.StdEncoding.EncodeToString([]byte("user=user\x01auth=Bearer "+accessToken+"\x01\x01"))
}

// TestValidateOAuth2Refresh verifies that refresh settings are optional, but
// when set need OAuth2, a refresh token, a client ID, and an HTTPS token URL
// or an HTTP one to a loopback host.
func TestValidateOAuth2Refresh(t *testing.T) {
	t.Parallel()

	valid := Config{
		Auth: AuthTypes.OAuth2, ClientID: "client", ClientSecret: "secret",
		RefreshToken: "refresh", TokenURL: "https://oauth.example.com/token",
	}

	tests := map[string]struct {
		change  func(*Config)
		wantErr bool
	}{
		"valid":                  {change: func(*Config) {}},
		"no client secret":       {change: func(c *Config) { c.ClientSecret = "" }},
		"HTTP to localhost":      {change: func(c *Config) { c.TokenURL = "http://localhost:8080/token" }},
		"HTTP to IPv6 loopback":  {change: func(c *Config) { c.TokenURL = "http://[::1]:8080/token" }},
		"no refresh settings":    {change: func(c *Config) { *c = Config{Auth: AuthTypes.Plain} }},
		"other auth":             {change: func(c *Config) { c.Auth = AuthTypes.Plain }, wantErr: true},
		"no refresh token":       {change: func(c *Config) { c.RefreshToken = "" }, wantErr: true},
		"no client ID":           {change: func(c *Config) { c.ClientID = "" }, wantErr: true},
		"no token URL":           {change: func(c *Config) { c.TokenURL = "" }, wantErr: true},
		"HTTP to another host":   {change: func(c *Config) { c.TokenURL = "http://oauth.example.com/token" }, wantErr: true},
		"other scheme":           {change: func(c *Config) { c.TokenURL = "ftp://oauth.example.com/token" }, wantErr: true},
		"token URL without host": {change: func(c *Config) { c.TokenURL = "https://:443/token" }, wantErr: true},
		"token URL not parsing":  {change: func(c *Config) { c.TokenURL = "https://oauth.example.com:port" }, wantErr: true},
	}

	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			config := valid
			tc.change(&config)

			err := validateOAuth2Refresh(&config)
			if tc.wantErr {
				require.ErrorIs(t, err, ErrInvalidOAuth2Refresh)
			} else {
				require.NoError(t, err)
			}
		})
	}
}

// TestInitializeOAuth2Refresh verifies that a refresh token without an auth
// setting selects OAuth2, and that refresh settings with another explicit
// auth setting are rejected.
func TestInitializeOAuth2Refresh(t *testing.T) {
	t.Parallel()

	service := newTestService(t, testRefreshURL, newFakeSMTP())
	assert.Equal(t, AuthTypes.OAuth2, service.Config.Auth)

	err := (&Service{}).Initialize(parseConfigURL(t, testRefreshURL+"&auth=Plain"), log.New(t.Output(), "", 0))
	require.ErrorIs(t, err, ErrInvalidOAuth2Refresh)
}

// TestConfigURLRefreshSettings verifies that the config URL holds the refresh
// settings, which read back to the same config.
func TestConfigURLRefreshSettings(t *testing.T) {
	t.Parallel()

	config := newTestService(t, testRefreshURL, newFakeSMTP()).Config

	reread := newTestService(t, config.GetURL().String(), newFakeSMTP()).Config
	assert.Equal(t, config, reread)
	assert.Equal(t, "secret", reread.ClientSecret)
	assert.Equal(t, "refresh", reread.RefreshToken)
}

// TestSendRefreshesAccessToken verifies that a send gets an access token from
// the token endpoint with the refresh token and authenticates with it instead
// of the password.
func TestSendRefreshesAccessToken(t *testing.T) {
	t.Parallel()

	server := newFakeSMTP("AUTH XOAUTH2")
	endpoint := newFakeTokenEndpoint(grant("fresh-token", "3600", ""))
	service := newRefreshingService(t, strings.Replace(testRefreshURL, "user@", "user:stale@", 1), server, endpoint)

	require.NoError(t, service.Send("Hello", nil))

	requests := endpoint.requests()
	require.Len(t, requests, 1)
	assert.Equal(t, "refresh_token", requests[0].Get("grant_type"))
	assert.Equal(t, "refresh", requests[0].Get("refresh_token"))
	assert.Contains(t, server.received(), xoauth2Line("fresh-token"))
}

// TestSendCachesAccessToken verifies that a token is reused until it expires,
// that an expired one is refreshed with the latest refresh token the provider
// returned, and that a send with other refresh settings, including only another
// client secret, gets its own token.
func TestSendCachesAccessToken(t *testing.T) {
	t.Parallel()

	t.Run("valid token reused", func(t *testing.T) {
		t.Parallel()

		endpoint := newFakeTokenEndpoint(grant("token-1", "3600", ""))
		service := newRefreshingService(t, testRefreshURL, newFakeSMTP("AUTH XOAUTH2"), endpoint)

		require.NoError(t, service.Send("one", nil))
		require.NoError(t, service.Send("two", nil))
		assert.Len(t, endpoint.requests(), 1)
	})

	// oauth2 treats a token as expired 10 seconds before its expiry, so a
	// 1 second lifetime is expired by the next send.
	t.Run("expired token refreshed with the rotated refresh token", func(t *testing.T) {
		t.Parallel()

		server := newFakeSMTP("AUTH XOAUTH2")
		endpoint := newFakeTokenEndpoint(grant("token-1", "1", "rotated"), grant("token-2", "3600", ""))
		service := newRefreshingService(t, testRefreshURL, server, endpoint)

		require.NoError(t, service.Send("one", nil))
		require.NoError(t, service.Send("two", nil))

		requests := endpoint.requests()
		require.Len(t, requests, 2)
		assert.Equal(t, "refresh", requests[0].Get("refresh_token"))
		assert.Equal(t, "rotated", requests[1].Get("refresh_token"))
		assert.Contains(t, server.received(), xoauth2Line("token-2"))
	})

	t.Run("other refresh settings", func(t *testing.T) {
		t.Parallel()

		endpoint := newFakeTokenEndpoint(grant("token-1", "3600", ""), grant("token-2", "3600", ""))
		service := newRefreshingService(t, testRefreshURL, newFakeSMTP("AUTH XOAUTH2"), endpoint)

		require.NoError(t, service.Send("one", nil))
		require.NoError(t, service.Send("two", &types.Params{"refreshtoken": "other"}))

		requests := endpoint.requests()
		require.Len(t, requests, 2)
		assert.Equal(t, "other", requests[1].Get("refresh_token"))
	})

	t.Run("other client secret", func(t *testing.T) {
		t.Parallel()

		endpoint := newFakeTokenEndpoint(grant("token-1", "3600", ""), grant("token-2", "3600", ""))
		service := newRefreshingService(t, testRefreshURL, newFakeSMTP("AUTH XOAUTH2"), endpoint)

		require.NoError(t, service.Send("one", nil))
		require.NoError(t, service.Send("two", &types.Params{"clientsecret": "other"}))

		assert.Len(t, endpoint.requests(), 2)
	})
}

// TestSendForgetsRejectedToken verifies that an access token the server
// rejects is dropped, so the next send refreshes it, and that the refresh uses
// the rotated refresh token the provider returned, since a provider that
// detects reuse may revoke the original.
func TestSendForgetsRejectedToken(t *testing.T) {
	t.Parallel()

	server := newFakeSMTP("AUTH XOAUTH2")
	server.replies["AUTH"] = []string{"535 5.7.8 Username and Password not accepted"}

	endpoint := newFakeTokenEndpoint(grant("revoked", "3600", "rotated"), grant("fresh", "3600", ""))
	service := newRefreshingService(t, testRefreshURL, server, endpoint)

	require.Error(t, service.Send("one", nil))
	require.NoError(t, service.Send("two", nil))

	requests := endpoint.requests()
	require.Len(t, requests, 2)
	assert.Equal(t, "refresh", requests[0].Get("refresh_token"))
	assert.Equal(t, "rotated", requests[1].Get("refresh_token"))
	assert.Contains(t, server.received(), xoauth2Line("fresh"))
}

// TestSendFailedRefresh verifies that a failed refresh fails the send before
// any SMTP connection.
func TestSendFailedRefresh(t *testing.T) {
	t.Parallel()

	server := newFakeSMTP("AUTH XOAUTH2")
	endpoint := newFakeTokenEndpoint(tokenResponse{
		body:   `{"error":"invalid_grant","error_description":"Token has been expired or revoked."}`,
		status: http.StatusBadRequest,
	})
	service := newRefreshingService(t, testRefreshURL, server, endpoint)

	err := service.Send("Hello", nil)

	require.Error(t, err)
	assert.Contains(t, err.Error(), "error refreshing the OAuth2 access token")
	assert.NotEmpty(t, endpoint.requests())
	assert.Empty(t, server.received(), "a failed refresh must not connect to the SMTP server")
}

// TestSendRejectsInvalidRefreshParams verifies that refresh settings from send
// params are checked before any request.
func TestSendRejectsInvalidRefreshParams(t *testing.T) {
	t.Parallel()

	server := newFakeSMTP("AUTH XOAUTH2")
	endpoint := newFakeTokenEndpoint(grant("token", "3600", ""))
	service := newRefreshingService(t, testRefreshURL, server, endpoint)

	err := service.Send("Hello", &types.Params{"tokenurl": "http://oauth.example.com/token"})

	require.ErrorIs(t, err, ErrInvalidOAuth2Refresh)
	assert.Empty(t, endpoint.requests())
	assert.Empty(t, server.received())
}

// TestSendConcurrentRefresh verifies that concurrent sends share one refresh.
func TestSendConcurrentRefresh(t *testing.T) {
	t.Parallel()

	endpoint := newFakeTokenEndpoint(grant("token", "3600", ""))
	service := newRefreshingService(t, testRefreshURL, newFakeSMTP("AUTH XOAUTH2"), endpoint)

	var wg sync.WaitGroup

	for range 5 {
		wg.Go(func() {
			assert.NoError(t, service.SendContext(context.Background(), "Hello", nil))
		})
	}

	wg.Wait()

	assert.Len(t, endpoint.requests(), 1)
}

// TestSetHTTPClient verifies that SetHTTPClient makes token requests use a
// custom client, and that nil and a nil *http.Client restore the default.
func TestSetHTTPClient(t *testing.T) {
	t.Parallel()

	service := &Service{}
	defaultClient := service.client.Client()
	endpoint := newFakeTokenEndpoint(grant("token", "3600", ""))

	service.SetHTTPClient(endpoint)
	assert.Same(t, endpoint, service.client.Client())

	service.SetHTTPClient((*http.Client)(nil))
	assert.Same(t, defaultClient, service.client.Client())

	service.SetHTTPClient(endpoint)
	service.SetHTTPClient(nil)
	assert.Same(t, defaultClient, service.client.Client())
}

// TestSendDefaultTokenClient verifies that without an injected client the
// token request goes through the service's own client, built once and reused,
// to a loopback HTTP token endpoint.
func TestSendDefaultTokenClient(t *testing.T) {
	t.Parallel()

	var requests atomic.Int32

	tokenServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		requests.Add(1)
		w.Header().Set("Content-Type", "application/json")
		_, _ = io.WriteString(w, `{"access_token":"default-token","token_type":"Bearer","expires_in":1}`)
	}))
	t.Cleanup(tokenServer.Close)

	server := newFakeSMTP("AUTH XOAUTH2")
	rawURL := strings.Replace(testRefreshURL, "https%3A%2F%2Foauth.example.com", url.QueryEscape(tokenServer.URL), 1)
	service := newTestService(t, rawURL, server)

	require.NoError(t, service.Send("Hello", nil))

	client := service.client.Client()
	assert.IsType(t, &http.Client{}, client)

	// The token expires within the oauth2 package's early-expiry window, so
	// the second send refreshes it again through the same client.
	require.NoError(t, service.Send("Hello", nil))
	assert.Same(t, client, service.client.Client())
	assert.Equal(t, int32(2), requests.Load())
	assert.Contains(t, server.received(), xoauth2Line("default-token"))
}

// TestForgetTokenKeepsNewerToken verifies that a send rejected for an older
// access token does not drop the newer one another send cached since.
func TestForgetTokenKeepsNewerToken(t *testing.T) {
	t.Parallel()

	config := refreshConfig("refresh")
	service := &Service{tokens: map[tokenKey]*oauth2.Token{
		config.tokenKey(): {AccessToken: "newer", RefreshToken: "refresh", Expiry: time.Now().Add(time.Hour)},
	}}

	config.Password = "older"
	service.forgetToken(config)

	assert.Equal(t, "newer", service.tokens[config.tokenKey()].AccessToken)

	config.Password = "newer"
	service.forgetToken(config)

	assert.Empty(t, service.tokens[config.tokenKey()].AccessToken)
	assert.Equal(t, "refresh", service.tokens[config.tokenKey()].RefreshToken)
}

// TestForgetTokenWithoutCache verifies that forgetting a token that was never
// cached leaves the cache empty.
func TestForgetTokenWithoutCache(t *testing.T) {
	t.Parallel()

	service := &Service{}
	service.forgetToken(&Config{TokenURL: "https://oauth.example.com/token", ClientID: "client", RefreshToken: "refresh"})

	assert.Empty(t, service.tokens)
}

// Do answers a token request, holding it first when it redeems a slow token.
//
// Parameters:
//   - req: the token request.
//
// Returns:
//   - *http.Response: the grant.
//   - error: the request context's error when it ends while held.
func (e *blockingTokenEndpoint) Do(req *http.Request) (*http.Response, error) {
	if err := req.ParseForm(); err != nil {
		return nil, err
	}

	e.mu.Lock()
	e.requests++
	e.mu.Unlock()

	refreshToken := req.PostForm.Get("refresh_token")
	if strings.HasPrefix(refreshToken, "slow") {
		select {
		case <-e.release:
		case <-req.Context().Done():
			return nil, req.Context().Err()
		}
	}

	status := http.StatusOK
	body := `{"access_token":"` + refreshToken + `-token","token_type":"Bearer","expires_in":3600}`

	if refreshToken == "slow-revoked" {
		status = http.StatusBadRequest
		body = `{"error":"invalid_grant"}`
	}

	return &http.Response{
		StatusCode: status,
		Header:     http.Header{"Content-Type": []string{"application/json"}},
		Body:       io.NopCloser(strings.NewReader(body)),
		Request:    req,
	}, nil
}

// count returns the number of token requests received.
//
// Returns:
//   - int: the request count.
func (e *blockingTokenEndpoint) count() int {
	e.mu.Lock()
	defer e.mu.Unlock()

	return e.requests
}

// refreshConfig returns valid refresh settings that redeem refreshToken.
//
// Parameters:
//   - refreshToken: the refresh token.
//
// Returns:
//   - *Config: the configuration.
func refreshConfig(refreshToken string) *Config {
	return &Config{
		Auth:         AuthTypes.OAuth2,
		ClientID:     "client",
		RefreshToken: refreshToken,
		TokenURL:     "https://oauth.example.com/token",
	}
}

// TestAccessTokenInFlight verifies how sends coordinate while a token request
// is in flight. Waiters give up when their own context ends. They retry when
// the sender that made the request was canceled, and otherwise share its
// result. Sends with other refresh settings do not wait at all.
func TestAccessTokenInFlight(t *testing.T) {
	t.Parallel()

	t.Run("waiters share the token or return when their context ends", func(t *testing.T) {
		t.Parallel()

		synctest.Test(t, func(t *testing.T) {
			endpoint := &blockingTokenEndpoint{release: make(chan struct{})}
			service := &Service{}
			service.SetHTTPClient(endpoint)

			var leaderToken string

			var leaderErr error

			go func() { leaderToken, leaderErr = service.accessToken(t.Context(), refreshConfig("slow")) }()

			synctest.Wait()

			waiterCtx, cancel := context.WithCancel(t.Context())

			var waiterErr error

			go func() { _, waiterErr = service.accessToken(waiterCtx, refreshConfig("slow")) }()

			synctest.Wait()
			cancel()
			synctest.Wait()

			require.ErrorIs(t, waiterErr, context.Canceled)

			var sharedToken string

			var sharedErr error

			go func() { sharedToken, sharedErr = service.accessToken(t.Context(), refreshConfig("slow")) }()

			synctest.Wait()
			close(endpoint.release)
			synctest.Wait()

			require.NoError(t, leaderErr)
			require.NoError(t, sharedErr)
			assert.Equal(t, "slow-token", leaderToken)
			assert.Equal(t, "slow-token", sharedToken)
			assert.Equal(t, 1, endpoint.count())
		})
	})

	t.Run("waiter retries when the leader is canceled", func(t *testing.T) {
		t.Parallel()

		synctest.Test(t, func(t *testing.T) {
			endpoint := &blockingTokenEndpoint{release: make(chan struct{})}
			service := &Service{}
			service.SetHTTPClient(endpoint)

			leaderCtx, cancel := context.WithCancel(t.Context())

			var leaderErr error

			go func() { _, leaderErr = service.accessToken(leaderCtx, refreshConfig("slow")) }()

			synctest.Wait()

			var waiterToken string

			var waiterErr error

			go func() { waiterToken, waiterErr = service.accessToken(t.Context(), refreshConfig("slow")) }()

			synctest.Wait()
			cancel()
			synctest.Wait()

			require.ErrorIs(t, leaderErr, context.Canceled)

			close(endpoint.release)
			synctest.Wait()

			require.NoError(t, waiterErr)
			assert.Equal(t, "slow-token", waiterToken)
		})
	})

	t.Run("waiter shares the provider's error", func(t *testing.T) {
		t.Parallel()

		synctest.Test(t, func(t *testing.T) {
			endpoint := &blockingTokenEndpoint{release: make(chan struct{})}
			service := &Service{}
			service.SetHTTPClient(endpoint)

			var leaderErr error

			go func() { _, leaderErr = service.accessToken(t.Context(), refreshConfig("slow-revoked")) }()

			synctest.Wait()

			var waiterErr error

			go func() { _, waiterErr = service.accessToken(t.Context(), refreshConfig("slow-revoked")) }()

			synctest.Wait()
			close(endpoint.release)
			synctest.Wait()

			require.Error(t, leaderErr)
			assert.Contains(t, leaderErr.Error(), "invalid_grant")
			assert.Equal(t, leaderErr, waiterErr)
		})
	})

	t.Run("other refresh settings do not wait", func(t *testing.T) {
		t.Parallel()

		synctest.Test(t, func(t *testing.T) {
			endpoint := &blockingTokenEndpoint{release: make(chan struct{})}
			service := &Service{}
			service.SetHTTPClient(endpoint)

			go func() { _, _ = service.accessToken(t.Context(), refreshConfig("slow")) }()

			synctest.Wait()

			token, err := service.accessToken(t.Context(), refreshConfig("fast"))
			require.NoError(t, err)
			assert.Equal(t, "fast-token", token)

			token, err = service.accessToken(t.Context(), refreshConfig("fast"))
			require.NoError(t, err)
			assert.Equal(t, "fast-token", token)
			assert.Equal(t, 2, endpoint.count())

			close(endpoint.release)
			synctest.Wait()
		})
	})
}

// Do fails the request with a *url.Error carrying its full URL.
//
// Parameters:
//   - req: the token request.
//
// Returns:
//   - *http.Response: always nil.
//   - error: a *url.Error with the request URL.
func (failingTokenClient) Do(req *http.Request) (*http.Response, error) {
	return nil, &url.Error{Op: "Post", URL: req.URL.String(), Err: io.ErrUnexpectedEOF}
}

// TestSendRedactsTokenURL verifies that a failed token request reports the
// token endpoint by its scheme and host alone, through the default client and
// through a client injected without the router, so a secret in the token
// URL's path stays out of the error.
func TestSendRedactsTokenURL(t *testing.T) {
	t.Parallel()

	t.Run("injected client", func(t *testing.T) {
		t.Parallel()

		rawURL := strings.Replace(testRefreshURL, "%2Ftoken", "%2FSECRETtenant%2Ftoken", 1)
		service := newRefreshingService(t, rawURL, newFakeSMTP("AUTH XOAUTH2"), failingTokenClient{})

		err := service.Send("Hello", nil)

		require.ErrorIs(t, err, io.ErrUnexpectedEOF)
		assert.NotContains(t, err.Error(), "SECRETtenant")
		assert.Contains(t, err.Error(), "oauth.example.com")
	})

	t.Run("default client", func(t *testing.T) {
		t.Parallel()

		// The token server drops every connection, so the request fails in
		// the transport.
		tokenServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			conn, _, err := http.NewResponseController(w).Hijack()
			if err == nil {
				_ = conn.Close()
			}
		}))
		t.Cleanup(tokenServer.Close)

		tokenURL := url.QueryEscape(tokenServer.URL + "/SECRETtenant/token")
		rawURL := strings.Replace(testRefreshURL, "https%3A%2F%2Foauth.example.com%2Ftoken", tokenURL, 1)
		service := newTestService(t, rawURL, newFakeSMTP("AUTH XOAUTH2"))

		err := service.Send("Hello", nil)

		require.Error(t, err)
		assert.NotContains(t, err.Error(), "SECRETtenant")
		assert.Contains(t, err.Error(), strings.TrimPrefix(tokenServer.URL, "http://"))
	})
}
