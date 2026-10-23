package homeassistant

import (
	"bytes"
	"context"
	"crypto/tls"
	"errors"
	"io"
	"log"
	"net/http"
	"net/url"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	"github.com/nicholas-fedor/shoutrrr/internal/httpbody"
	"github.com/nicholas-fedor/shoutrrr/internal/meta"
	"github.com/nicholas-fedor/shoutrrr/pkg/format"
	"github.com/nicholas-fedor/shoutrrr/pkg/types"
	"github.com/nicholas-fedor/shoutrrr/pkg/types/mocks"
)

// capturedRequest records the parts of a request that an HTTP client received.
type capturedRequest struct {
	// method is the request method.
	method string
	// url is the full request URL.
	url string
	// contentType is the request's Content-Type header.
	contentType string
	// authorization is the request's Authorization header.
	authorization string
	// userAgent is the request's User-Agent header.
	userAgent string
	// body is the request body.
	body string
}

// errTransport is the transport failure returned by the failing HTTP client.
var errTransport = errors.New("transport failure")

// newRespondingClient returns a mock HTTP client that expects one request,
// records it, and answers with the given status and body.
//
// Parameters:
//   - t: the test that owns the mock.
//   - status: the response status line, such as "200 OK".
//   - statusCode: the response status code.
//   - body: the response body.
//
// Returns:
//   - *mocks.MockHTTPClient: the client.
//   - *capturedRequest: the request, filled in once the client receives it.
func newRespondingClient(t *testing.T, status string, statusCode int, body string) (*mocks.MockHTTPClient, *capturedRequest) {
	t.Helper()

	captured := &capturedRequest{}
	client := mocks.NewMockHTTPClient(t)
	client.EXPECT().Do(mock.Anything).RunAndReturn(func(req *http.Request) (*http.Response, error) {
		requestBody, err := io.ReadAll(req.Body)
		require.NoError(t, err)

		*captured = capturedRequest{
			method:        req.Method,
			url:           req.URL.String(),
			contentType:   req.Header.Get("Content-Type"),
			authorization: req.Header.Get("Authorization"),
			userAgent:     req.Header.Get("User-Agent"),
			body:          string(requestBody),
		}

		return &http.Response{
			Status:     status,
			StatusCode: statusCode,
			Body:       io.NopCloser(strings.NewReader(body)),
		}, nil
	}).Once()

	return client, captured
}

// newTestService returns a service initialized from rawURL that logs to the
// test output.
//
// Parameters:
//   - t: the test.
//   - rawURL: the service URL.
//
// Returns:
//   - *Service: the service.
func newTestService(t *testing.T, rawURL string) *Service {
	t.Helper()

	service := &Service{}
	require.NoError(t, service.Initialize(parseTestURL(t, rawURL), log.New(t.Output(), "", 0)))

	return service
}

// TestGetID verifies that the service identifies itself by its scheme.
func TestGetID(t *testing.T) {
	t.Parallel()

	assert.Equal(t, "homeassistant", (&Service{}).GetID())
}

// TestInitialize verifies that Initialize parses the service URL, keeps an
// injected client, builds a TLS 1.2 client otherwise, skips verification with
// a warning when asked, and rejects a URL without a token.
func TestInitialize(t *testing.T) {
	t.Parallel()

	t.Run("keeps an injected client", func(t *testing.T) {
		t.Parallel()

		client := mocks.NewMockHTTPClient(t)
		service := &Service{}
		service.SetHTTPClient(client)
		require.NoError(t, service.Initialize(parseTestURL(t, "homeassistant://s3cret@ha.example.com"), log.New(t.Output(), "", 0)))

		assert.Equal(t, "s3cret", service.Config.Token)
		assert.Equal(t, "ha.example.com", service.Config.Host)
		assert.Same(t, client, service.httpClient)
	})

	// clientTests lists service URLs with the TLS settings of the client
	// Initialize should build.
	clientTests := []struct {
		name           string
		rawURL         string
		wantSkipVerify bool
	}{
		{name: "verified TLS", rawURL: "homeassistant://s3cret@ha.example.com"},
		{name: "skiptlsverify", rawURL: "homeassistant://s3cret@ha.example.com?skiptlsverify=yes", wantSkipVerify: true},
	}

	for _, tt := range clientTests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			var logs bytes.Buffer

			service := &Service{}
			require.NoError(t, service.Initialize(parseTestURL(t, tt.rawURL), log.New(&logs, "", 0)))

			httpClient, ok := service.httpClient.(*http.Client)
			require.True(t, ok, "client is a %T", service.httpClient)
			assert.Equal(t, defaultHTTPTimeout, httpClient.Timeout)

			httpTransport, ok := httpClient.Transport.(*http.Transport)
			require.True(t, ok, "transport is a %T", httpClient.Transport)
			require.NotNil(t, httpTransport.TLSClientConfig)
			assert.Equal(t, uint16(tls.VersionTLS12), httpTransport.TLSClientConfig.MinVersion)
			assert.Equal(t, tt.wantSkipVerify, httpTransport.TLSClientConfig.InsecureSkipVerify)
			assert.Equal(t, tt.wantSkipVerify, strings.Contains(logs.String(), "TLS verification is disabled"))
		})
	}

	t.Run("docs dummy URL", func(t *testing.T) {
		t.Parallel()

		require.NoError(t, (&Service{}).Initialize(parseTestURL(t, dummyServiceURL), log.New(t.Output(), "", 0)))
	})

	t.Run("missing token", func(t *testing.T) {
		t.Parallel()

		err := (&Service{}).Initialize(parseTestURL(t, "homeassistant://ha.example.com"), log.New(t.Output(), "", 0))

		require.ErrorIs(t, err, ErrTokenMissing)
	})
}

// TestSend verifies the request a send makes: an authenticated JSON POST to
// the service call endpoint, with notification_id only for persistent
// notifications, target only for notify actions, and title only when set.
func TestSend(t *testing.T) {
	t.Parallel()

	// tests lists service URLs and params with the request URL and JSON body
	// the send should produce.
	tests := []struct {
		params   *types.Params
		name     string
		rawURL   string
		wantURL  string
		wantBody string
	}{
		{
			name:     "persistent notification",
			rawURL:   "homeassistant://s3cret@ha.example.com",
			wantURL:  "https://ha.example.com:443/api/services/persistent_notification/create",
			wantBody: `{"message":"Host unreachable"}`,
		},
		{
			name:     "persistent notification with title, nid, and ignored targets",
			rawURL:   "homeassistant://s3cret@ha.example.com?title=Server%20Down&nid=watchtower&targets=device1",
			wantURL:  "https://ha.example.com:443/api/services/persistent_notification/create",
			wantBody: `{"message":"Host unreachable","title":"Server Down","notification_id":"watchtower"}`,
		},
		{
			name:     "notify action with targets and ignored nid",
			rawURL:   "homeassistant://s3cret@ha.example.com?service=notify.mobile_app_phone&targets=device1,device2&nid=ignored",
			wantURL:  "https://ha.example.com:443/api/services/notify/mobile_app_phone",
			wantBody: `{"message":"Host unreachable","target":["device1","device2"]}`,
		},
		{
			name:     "bare notify target over HTTP with a prefix",
			rawURL:   "homeassistant://s3cret@homeassistant.local/hass?service=mobile_app_phone&disabletls=yes&targets=device1",
			wantURL:  "http://homeassistant.local:8123/hass/api/services/notify/mobile_app_phone",
			wantBody: `{"message":"Host unreachable","target":["device1"]}`,
		},
		{
			name:     "notify action without targets",
			rawURL:   "homeassistant://s3cret@ha.example.com?service=notify.notify",
			wantURL:  "https://ha.example.com:443/api/services/notify/notify",
			wantBody: `{"message":"Host unreachable"}`,
		},
		{
			name:     "bare notify target without targets",
			rawURL:   "homeassistant://s3cret@ha.example.com?service=mobile_app",
			wantURL:  "https://ha.example.com:443/api/services/notify/mobile_app",
			wantBody: `{"message":"Host unreachable"}`,
		},
		{
			name:     "empty targets prop",
			rawURL:   "homeassistant://s3cret@ha.example.com?service=notify.notify&targets=",
			wantURL:  "https://ha.example.com:443/api/services/notify/notify",
			wantBody: `{"message":"Host unreachable"}`,
		},
		{
			name:     "empty entries in a targets param",
			rawURL:   "homeassistant://s3cret@ha.example.com?service=notify.notify",
			params:   &types.Params{"targets": "device1,,device2,"},
			wantURL:  "https://ha.example.com:443/api/services/notify/notify",
			wantBody: `{"message":"Host unreachable","target":["device1","device2"]}`,
		},
		{
			name:     "params",
			rawURL:   "homeassistant://s3cret@ha.example.com?title=Configured",
			params:   &types.Params{"title": "Override", "service": "notify.phone", "targets": "a"},
			wantURL:  "https://ha.example.com:443/api/services/notify/phone",
			wantBody: `{"message":"Host unreachable","title":"Override","target":["a"]}`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			service := newTestService(t, tt.rawURL)
			client, captured := newRespondingClient(t, "200 OK", http.StatusOK, "[]")
			service.SetHTTPClient(client)

			require.NoError(t, service.Send("Host unreachable", tt.params))

			assert.Equal(t, http.MethodPost, captured.method)
			assert.Equal(t, tt.wantURL, captured.url)
			assert.Equal(t, contentType, captured.contentType)
			assert.Equal(t, "Bearer s3cret", captured.authorization)
			assert.Equal(t, meta.UserAgent(), captured.userAgent)
			assert.JSONEq(t, tt.wantBody, captured.body)
		})
	}
}

// TestSendAcceptsCreated verifies that a 201 status counts as success, as the
// REST API documents.
func TestSendAcceptsCreated(t *testing.T) {
	t.Parallel()

	service := newTestService(t, "homeassistant://s3cret@ha.example.com")
	client, _ := newRespondingClient(t, "201 Created", http.StatusCreated, "[]")
	service.SetHTTPClient(client)

	require.NoError(t, service.Send("created", nil))
}

// TestSendOversizedBody verifies that a response body over [httpbody.MaxSize]
// does not change the outcome of a send. A success status still succeeds. An
// error status still fails, and the error carries only the first
// [httpbody.MaxSize] bytes of the body.
func TestSendOversizedBody(t *testing.T) {
	t.Parallel()

	t.Run("success status", func(t *testing.T) {
		t.Parallel()

		service := newTestService(t, "homeassistant://s3cret@ha.example.com")
		client, _ := newRespondingClient(t, "200 OK", http.StatusOK, "[]"+strings.Repeat(" ", httpbody.MaxSize))
		service.SetHTTPClient(client)

		require.NoError(t, service.Send("oversized", nil))
	})

	t.Run("error status", func(t *testing.T) {
		t.Parallel()

		service := newTestService(t, "homeassistant://s3cret@ha.example.com")
		body := strings.Repeat("#", httpbody.MaxSize+10)
		client, _ := newRespondingClient(t, "500 Internal Server Error", http.StatusInternalServerError, body)
		service.SetHTTPClient(client)

		err := service.Send("oversized", nil)

		require.ErrorIs(t, err, ErrUnexpectedStatus)
		assert.Equal(t, httpbody.MaxSize, strings.Count(err.Error(), "#"))
	})
}

// TestSendParamsLeaveConfigUnchanged verifies that params apply to a copy of
// the config for one send, so the stored config and the next send keep the
// configured values.
func TestSendParamsLeaveConfigUnchanged(t *testing.T) {
	t.Parallel()

	service := newTestService(t, "homeassistant://s3cret@ha.example.com")
	want := *service.Config

	client, _ := newRespondingClient(t, "200 OK", http.StatusOK, "[]")
	service.SetHTTPClient(client)

	require.NoError(t, service.Send("first", &types.Params{"title": "Override", "nid": "one"}))
	assert.Equal(t, want, *service.Config)

	client, captured := newRespondingClient(t, "200 OK", http.StatusOK, "[]")
	service.SetHTTPClient(client)

	require.NoError(t, service.Send("second", nil))
	assert.JSONEq(t, `{"message":"second"}`, captured.body)
}

// TestSendLeavesConfiguredTargetsUnchanged verifies that dropping empty
// targets from a payload does not modify the stored targets.
func TestSendLeavesConfiguredTargetsUnchanged(t *testing.T) {
	t.Parallel()

	service := newTestService(t, "homeassistant://s3cret@ha.example.com?service=notify.notify&targets=,device1,,device2")
	want := []string{"", "device1", "", "device2"}
	require.Equal(t, want, service.Config.Targets)

	client, captured := newRespondingClient(t, "200 OK", http.StatusOK, "[]")
	service.SetHTTPClient(client)

	require.NoError(t, service.Send("Message", nil))
	assert.JSONEq(t, `{"message":"Message","target":["device1","device2"]}`, captured.body)
	assert.Equal(t, want, service.Config.Targets)
}

// TestSendContextErrors verifies that a send rejects an empty message or an
// invalid param before any request, fails with ErrSendFailed and
// ErrUnexpectedStatus for an error status, wraps a transport error, and stops
// when the caller's context is canceled.
func TestSendContextErrors(t *testing.T) {
	t.Parallel()

	// rejectedSends lists messages and params that fail before any request.
	rejectedSends := []struct {
		params  *types.Params
		wantErr error
		name    string
		message string
	}{
		{name: "empty message", message: "", wantErr: ErrMessageEmpty},
		{name: "unknown param", message: "Message", params: &types.Params{"foo": "bar"}, wantErr: format.ErrInvalidConfigKey},
		{name: "invalid service param", message: "Message", params: &types.Params{"service": "../config"}, wantErr: ErrInvalidService},
	}

	for _, tt := range rejectedSends {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			service := newTestService(t, "homeassistant://s3cret@ha.example.com")
			// The mock has no expectations, so a request fails the test.
			service.SetHTTPClient(mocks.NewMockHTTPClient(t))

			require.ErrorIs(t, service.SendContext(t.Context(), tt.message, tt.params), tt.wantErr)
		})
	}

	// statusTests lists error responses the REST API documents.
	statusTests := []struct {
		name       string
		status     string
		body       string
		statusCode int
	}{
		{name: "unauthorized", status: "401 Unauthorized", statusCode: http.StatusUnauthorized, body: "401: Unauthorized"},
		{name: "bad request", status: "400 Bad Request", statusCode: http.StatusBadRequest, body: `{"message":"Invalid JSON specified."}`},
		{name: "not found", status: "404 Not Found", statusCode: http.StatusNotFound, body: "404: Not Found"},
	}

	for _, tt := range statusTests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			service := newTestService(t, "homeassistant://s3cret@ha.example.com")
			client, _ := newRespondingClient(t, tt.status, tt.statusCode, tt.body)
			service.SetHTTPClient(client)

			err := service.SendContext(t.Context(), "fail", nil)

			require.ErrorIs(t, err, ErrSendFailed)
			require.ErrorIs(t, err, ErrUnexpectedStatus)
			assert.Contains(t, err.Error(), tt.status)
			assert.Contains(t, err.Error(), tt.body)
		})
	}

	t.Run("transport error", func(t *testing.T) {
		t.Parallel()

		service := newTestService(t, "homeassistant://s3cret@ha.example.com/SECRETprefix")
		client := mocks.NewMockHTTPClient(t)
		client.EXPECT().Do(mock.Anything).RunAndReturn(func(req *http.Request) (*http.Response, error) {
			return nil, &url.Error{Op: "Post", URL: req.URL.String(), Err: errTransport}
		}).Once()
		service.SetHTTPClient(client)

		err := service.SendContext(t.Context(), "fail", nil)

		require.ErrorIs(t, err, ErrSendFailed)
		require.ErrorIs(t, err, errTransport)
		assert.NotContains(t, err.Error(), "SECRETprefix")
	})

	t.Run("canceled context", func(t *testing.T) {
		t.Parallel()

		ctx, cancel := context.WithCancel(t.Context())
		cancel()

		client := mocks.NewMockHTTPClient(t)
		client.EXPECT().Do(mock.Anything).RunAndReturn(func(req *http.Request) (*http.Response, error) {
			return nil, req.Context().Err()
		}).Once()

		service := newTestService(t, "homeassistant://token@ha.example.invalid")
		service.SetHTTPClient(client)

		require.ErrorIs(t, service.SendContext(ctx, "message", nil), context.Canceled)
	})
}

// TestSendRedactsUnparsableURL verifies that a host that makes the API URL
// unparsable fails the send before any request, without the URL in the error.
func TestSendRedactsUnparsableURL(t *testing.T) {
	t.Parallel()

	service := newTestService(t, "homeassistant://s3cret@ha.example.com/SECRETprefix")
	service.Config.Host = "bad host"
	// The mock has no expectations, so a request fails the test.
	service.SetHTTPClient(mocks.NewMockHTTPClient(t))

	err := service.Send("message", nil)

	_, isURLErr := errors.AsType[*url.Error](err)
	require.True(t, isURLErr, "error %v is not a *url.Error", err)
	assert.NotContains(t, err.Error(), "SECRETprefix")
}

// TestSendDefaultClient verifies that a send with a nil *http.Client uses a
// default client that honors the caller's context. The context is canceled
// before the send, so the transport returns before it dials Home Assistant.
func TestSendDefaultClient(t *testing.T) {
	t.Parallel()

	service := newTestService(t, "homeassistant://s3cret@ha.example.com/SECRETprefix")
	service.SetHTTPClient((*http.Client)(nil))

	ctx, cancel := context.WithCancel(t.Context())
	cancel()

	err := service.SendContext(ctx, "Message", nil)

	require.ErrorIs(t, err, context.Canceled)
	assert.NotContains(t, err.Error(), "SECRETprefix")
}

// TestSetHTTPClient verifies that SetHTTPClient stores a custom client and
// clears it for a nil *http.Client.
func TestSetHTTPClient(t *testing.T) {
	t.Parallel()

	t.Run("custom client", func(t *testing.T) {
		t.Parallel()

		client := mocks.NewMockHTTPClient(t)
		service := &Service{}
		service.SetHTTPClient(client)

		assert.Same(t, client, service.httpClient)
	})

	t.Run("nil *http.Client", func(t *testing.T) {
		t.Parallel()

		service := &Service{httpClient: mocks.NewMockHTTPClient(t)}
		service.SetHTTPClient((*http.Client)(nil))

		assert.Nil(t, service.httpClient)
	})
}
