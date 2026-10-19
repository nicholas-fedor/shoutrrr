package homeassistant_test

import (
	"context"
	"errors"
	"io"
	"log"
	"net"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/nicholas-fedor/shoutrrr/internal/meta"
	"github.com/nicholas-fedor/shoutrrr/pkg/services/push/homeassistant"
	"github.com/nicholas-fedor/shoutrrr/pkg/types"
)

// receivedRequest records the parts of a request that the fake Home Assistant
// API received.
type receivedRequest struct {
	// method is the request method.
	method string
	// path is the request path.
	path string
	// contentType is the request's Content-Type header.
	contentType string
	// authorization is the request's Authorization header.
	authorization string
	// userAgent is the request's User-Agent header.
	userAgent string
	// body is the request body.
	body string
}

// fakeHomeAssistant is a loopback fake of the Home Assistant REST API. It
// answers 401 for any token other than [testToken], like Home Assistant, and
// otherwise answers with its configured status.
type fakeHomeAssistant struct {
	// server is the loopback HTTP server.
	server *httptest.Server
	// requests receives each authenticated request the server handles.
	requests chan receivedRequest
	// status is the status code for an authenticated request.
	status int
	// body is the response body for an authenticated request.
	body string
}

// testToken is the long-lived access token the fake API accepts.
const testToken = "s3cret"

// errConnectionRefused is the error the refused dials report.
var errConnectionRefused = errors.New("connection refused")

// newFakeHomeAssistant starts a fake Home Assistant API on a loopback port
// and stops it when the test ends.
//
// Parameters:
//   - t: the test.
//   - status: the status code for an authenticated request.
//   - body: the response body for an authenticated request.
//
// Returns:
//   - *fakeHomeAssistant: the running fake.
func newFakeHomeAssistant(t *testing.T, status int, body string) *fakeHomeAssistant {
	t.Helper()

	fake := &fakeHomeAssistant{requests: make(chan receivedRequest, 1), status: status, body: body}
	fake.server = httptest.NewServer(http.HandlerFunc(fake.serveHTTP))
	t.Cleanup(fake.server.Close)

	return fake
}

// serveHTTP rejects a request without the expected bearer token and records
// any other request before it answers.
//
// Parameters:
//   - w: the response writer.
//   - r: the request.
func (f *fakeHomeAssistant) serveHTTP(w http.ResponseWriter, r *http.Request) {
	if r.Header.Get("Authorization") != "Bearer "+testToken {
		http.Error(w, "401: Unauthorized", http.StatusUnauthorized)

		return
	}

	body, err := io.ReadAll(r.Body)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)

		return
	}

	f.requests <- receivedRequest{
		method:        r.Method,
		path:          r.URL.Path,
		contentType:   r.Header.Get("Content-Type"),
		authorization: r.Header.Get("Authorization"),
		userAgent:     r.Header.Get("User-Agent"),
		body:          string(body),
	}

	w.WriteHeader(f.status)
	_, _ = io.WriteString(w, f.body)
}

// serviceURL returns a service URL that reaches the fake over plain HTTP.
//
// Parameters:
//   - token: the access token.
//   - path: the reverse-proxy path prefix, or "" for none.
//   - query: further URL query parameters, or "" for none.
//
// Returns:
//   - string: the service URL.
func (f *fakeHomeAssistant) serviceURL(token, path, query string) string {
	rawURL := homeassistant.Scheme + "://" + token + "@" + f.server.Listener.Addr().String() + path + "?disabletls=yes"
	if query != "" {
		rawURL += "&" + query
	}

	return rawURL
}

// newIntegrationService returns a service initialized from rawURL that logs to
// the test output and sends through its default HTTP client.
//
// Parameters:
//   - t: the test.
//   - rawURL: the service URL.
//
// Returns:
//   - *homeassistant.Service: the service.
func newIntegrationService(t *testing.T, rawURL string) *homeassistant.Service {
	t.Helper()

	serviceURL, err := url.Parse(rawURL)
	require.NoError(t, err)

	service := &homeassistant.Service{}
	require.NoError(t, service.Initialize(serviceURL, log.New(t.Output(), "", 0)))

	return service
}

// TestIntegrationSend verifies that a service initialized from a URL sends an
// authenticated JSON POST over real HTTP, through its default client, to the
// service call endpoint the URL and params select.
func TestIntegrationSend(t *testing.T) {
	t.Parallel()

	// tests lists URL parts and params with the request path and JSON body
	// the fake API should receive.
	tests := []struct {
		params   *types.Params
		name     string
		path     string
		query    string
		wantPath string
		wantBody string
	}{
		{
			name:     "persistent notification with title and nid",
			query:    "title=Alert&nid=watchtower",
			wantPath: "/api/services/persistent_notification/create",
			wantBody: `{"message":"Something happened","title":"Alert","notification_id":"watchtower"}`,
		},
		{
			name:     "notify action with params",
			query:    "service=notify.mobile_app_phone&nid=ignored",
			params:   &types.Params{"title": "Override", "targets": "device1,device2"},
			wantPath: "/api/services/notify/mobile_app_phone",
			wantBody: `{"message":"Something happened","title":"Override","target":["device1","device2"]}`,
		},
		{
			name:     "reverse-proxy prefix with a trailing slash",
			path:     "/hass/",
			wantPath: "/hass/api/services/persistent_notification/create",
			wantBody: `{"message":"Something happened"}`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			fake := newFakeHomeAssistant(t, http.StatusOK, "[]")
			service := newIntegrationService(t, fake.serviceURL(testToken, tt.path, tt.query))

			require.NoError(t, service.SendContext(t.Context(), "Something happened", tt.params))

			got := <-fake.requests
			assert.Equal(t, http.MethodPost, got.method)
			assert.Equal(t, tt.wantPath, got.path)
			assert.Equal(t, "application/json", got.contentType)
			assert.Equal(t, "Bearer "+testToken, got.authorization)
			assert.Equal(t, meta.UserAgent(), got.userAgent)
			assert.JSONEq(t, tt.wantBody, got.body)
		})
	}
}

// TestIntegrationSendErrors verifies that a send rejected by the API fails with
// ErrSendFailed and ErrUnexpectedStatus and reports the status and body, as a
// caller sees it over real HTTP.
func TestIntegrationSendErrors(t *testing.T) {
	t.Parallel()

	// tests lists tokens and fake API responses with the status line and body
	// the error should report.
	tests := []struct {
		name       string
		token      string
		body       string
		wantStatus string
		wantBody   string
		status     int
	}{
		{
			name:       "wrong token",
			token:      "wrong",
			status:     http.StatusOK,
			body:       "[]",
			wantStatus: "401 Unauthorized",
			wantBody:   "401: Unauthorized",
		},
		{
			name:       "internal server error",
			token:      testToken,
			status:     http.StatusInternalServerError,
			body:       "boom",
			wantStatus: "500 Internal Server Error",
			wantBody:   "boom",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			fake := newFakeHomeAssistant(t, tt.status, tt.body)
			service := newIntegrationService(t, fake.serviceURL(tt.token, "", ""))

			err := service.SendContext(t.Context(), "Test message", nil)

			require.ErrorIs(t, err, homeassistant.ErrSendFailed)
			require.ErrorIs(t, err, homeassistant.ErrUnexpectedStatus)
			assert.Contains(t, err.Error(), tt.wantStatus)
			assert.Contains(t, err.Error(), tt.wantBody)
		})
	}
}

// TestIntegrationSendConnectionRefused verifies that a send whose connection
// is refused fails with ErrSendFailed and keeps the dial error.
func TestIntegrationSendConnectionRefused(t *testing.T) {
	t.Parallel()

	fake := newFakeHomeAssistant(t, http.StatusOK, "[]")
	service := newIntegrationService(t, fake.serviceURL(testToken, "", ""))
	service.SetHTTPClient(&http.Client{Transport: &http.Transport{
		DialContext: func(context.Context, string, string) (net.Conn, error) {
			return nil, &net.OpError{Op: "dial", Net: "tcp", Err: errConnectionRefused}
		},
	}})

	err := service.SendContext(t.Context(), "Test message", nil)

	require.ErrorIs(t, err, homeassistant.ErrSendFailed)

	_, isOpErr := errors.AsType[*net.OpError](err)
	assert.True(t, isOpErr, "error %v does not wrap a *net.OpError", err)
}
