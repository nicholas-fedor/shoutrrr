package bark

import (
	"context"
	"errors"
	"io"
	"log"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	"github.com/nicholas-fedor/shoutrrr/pkg/format"
	"github.com/nicholas-fedor/shoutrrr/pkg/services/push/bark/mocks"
	"github.com/nicholas-fedor/shoutrrr/pkg/types"
	"github.com/nicholas-fedor/shoutrrr/pkg/util/jsonclient"
)

// capturedRequest records the parts of a request that an HTTP client received.
type capturedRequest struct {
	// method is the request method.
	method string
	// url is the full request URL.
	url string
	// contentType is the request's Content-Type header.
	contentType string
	// body is the request body.
	body string
}

// Response bodies in the format the Bark server returns.
const (
	// successBody is the body of an accepted push.
	successBody = `{"code":200,"message":"success","timestamp":1760000000}`
	// deviceErrorBody is the body of a push to an unknown device key.
	deviceErrorBody = `{"code":400,"message":"failed to get device token: failed to get [devicekey] device token from database","timestamp":1760000000}`
)

// errTransport is the transport failure returned by the failing HTTP client.
var errTransport = errors.New("transport failure")

// captureRequest reads the method, URL, Content-Type header, and body of req.
//
// Parameters:
//   - t: the test.
//   - req: the request to read.
//
// Returns:
//   - capturedRequest: the recorded request.
func captureRequest(t *testing.T, req *http.Request) capturedRequest {
	t.Helper()

	body, err := io.ReadAll(req.Body)
	require.NoError(t, err)

	return capturedRequest{
		method:      req.Method,
		url:         req.URL.String(),
		contentType: req.Header.Get("Content-Type"),
		body:        string(body),
	}
}

// newRespondingClient returns a mock HTTP client that expects one request,
// records it, and answers with the given status code and body.
//
// Parameters:
//   - t: the test that owns the mock.
//   - statusCode: the response status code.
//   - body: the response body.
//
// Returns:
//   - *mocks.MockHTTPClient: the client.
//   - *capturedRequest: the request, filled in once the client receives it.
func newRespondingClient(t *testing.T, statusCode int, body string) (*mocks.MockHTTPClient, *capturedRequest) {
	t.Helper()

	captured := &capturedRequest{}
	client := mocks.NewMockHTTPClient(t)
	client.EXPECT().Do(mock.Anything).RunAndReturn(func(req *http.Request) (*http.Response, error) {
		*captured = captureRequest(t, req)

		return &http.Response{
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

	assert.Equal(t, "bark", (&Service{}).GetID())
}

// TestInitialize verifies that Initialize applies the default props, keeps an
// HTTP client set before it runs, and rejects an invalid URL.
func TestInitialize(t *testing.T) {
	t.Parallel()

	t.Run("defaults and default client", func(t *testing.T) {
		t.Parallel()

		service := newTestService(t, "bark://:devicekey@hostname")

		assert.Equal(t, Config{Host: "hostname", DeviceKey: "devicekey", Scheme: "https"}, *service.Config)
		assert.IsType(t, &DefaultHTTPClient{}, service.HTTPClient)
	})

	t.Run("keeps an injected client", func(t *testing.T) {
		t.Parallel()

		client := mocks.NewMockHTTPClient(t)
		service := &Service{HTTPClient: client}
		require.NoError(t, service.Initialize(parseTestURL(t, "bark://:devicekey@hostname"), log.New(t.Output(), "", 0)))

		assert.Same(t, client, service.HTTPClient)
	})

	t.Run("missing device key", func(t *testing.T) {
		t.Parallel()

		err := (&Service{}).Initialize(parseTestURL(t, "bark://hostname"), log.New(t.Output(), "", 0))

		require.ErrorIs(t, err, ErrMissingDeviceKey)
	})
}

// TestSend verifies the request a send makes: a JSON POST to the push
// endpoint carrying the payload built from the config and params.
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
			name:     "required fields only",
			rawURL:   "bark://:devicekey@hostname",
			wantURL:  "https://hostname/push",
			wantBody: `{"body":"Message","device_key":"devicekey","title":"","badge":0}`,
		},
		{
			name: "configured props and path",
			rawURL: "bark://:devicekey@hostname:8080/bark?scheme=http&title=Title&sound=bell&badge=3" +
				"&icon=https%3A%2F%2Fexample.com%2Ficon.png&group=Group&url=https%3A%2F%2Fexample.com&category=Cat&copy=Copied",
			wantURL: "http://hostname:8080/bark/push",
			wantBody: `{"body":"Message","device_key":"devicekey","title":"Title","sound":"bell","badge":3,` +
				`"icon":"https://example.com/icon.png","group":"Group","url":"https://example.com","category":"Cat","copy":"Copied"}`,
		},
		{
			name:     "params",
			rawURL:   "bark://:devicekey@hostname?title=Configured",
			params:   &types.Params{"title": "Override", "sound": "alarm", "badge": "7"},
			wantURL:  "https://hostname/push",
			wantBody: `{"body":"Message","device_key":"devicekey","title":"Override","sound":"alarm","badge":7}`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			service := newTestService(t, tt.rawURL)
			client, captured := newRespondingClient(t, http.StatusOK, successBody)
			service.SetHTTPClient(client)

			require.NoError(t, service.Send("Message", tt.params))

			assert.Equal(t, http.MethodPost, captured.method)
			assert.Equal(t, tt.wantURL, captured.url)
			assert.Equal(t, "application/json", captured.contentType)
			assert.JSONEq(t, tt.wantBody, captured.body)
		})
	}
}

// TestSendParamsLeaveConfigUnchanged verifies that params apply to a copy of
// the config for one send, so the stored config and the next send keep the
// configured values.
func TestSendParamsLeaveConfigUnchanged(t *testing.T) {
	t.Parallel()

	service := newTestService(t, "bark://:devicekey@hostname?title=Configured")
	want := *service.Config

	client, captured := newRespondingClient(t, http.StatusOK, successBody)
	service.SetHTTPClient(client)

	require.NoError(t, service.Send("Message", &types.Params{"title": "Override", "scheme": "http"}))
	assert.Equal(t, "http://hostname/push", captured.url)
	assert.JSONEq(t, `{"body":"Message","device_key":"devicekey","title":"Override","badge":0}`, captured.body)
	assert.Equal(t, want, *service.Config)

	client, captured = newRespondingClient(t, http.StatusOK, successBody)
	service.SetHTTPClient(client)

	require.NoError(t, service.Send("Message", nil))
	assert.Equal(t, "https://hostname/push", captured.url)
	assert.JSONEq(t, `{"body":"Message","device_key":"devicekey","title":"Configured","badge":0}`, captured.body)
}

// TestSendItems verifies that the deprecated SendItems sends the items as one
// plain text message.
func TestSendItems(t *testing.T) {
	t.Parallel()

	service := newTestService(t, "bark://:devicekey@hostname")
	client, captured := newRespondingClient(t, http.StatusOK, successBody)
	service.SetHTTPClient(client)

	items := []types.MessageItem{{Text: "first"}, {Text: "second"}}
	require.NoError(t, service.SendItems(items, nil))

	assert.JSONEq(t, `{"body":"first\nsecond\n","device_key":"devicekey","title":"","badge":0}`, captured.body)
}

// TestSendServerErrors verifies how a send reports a response the Bark server
// rejects: the server's message as an *APIResponse, ErrUnexpectedStatus when
// the response has no message, and a decoding error for a body that is not
// JSON.
func TestSendServerErrors(t *testing.T) {
	t.Parallel()

	t.Run("server message", func(t *testing.T) {
		t.Parallel()

		service := newTestService(t, "bark://:devicekey@hostname")
		client, _ := newRespondingClient(t, http.StatusBadRequest, deviceErrorBody)
		service.SetHTTPClient(client)

		err := service.Send("Message", nil)

		apiErr, ok := errors.AsType[*APIResponse](err)
		require.True(t, ok, "error %v is not an *APIResponse", err)
		assert.Equal(t, int64(http.StatusBadRequest), apiErr.Code)
		assert.Contains(t, err.Error(), "failed to get device token")
	})

	t.Run("code without a message", func(t *testing.T) {
		t.Parallel()

		service := newTestService(t, "bark://:devicekey@hostname")
		client, _ := newRespondingClient(t, http.StatusOK, `{"code":500}`)
		service.SetHTTPClient(client)

		err := service.Send("Message", nil)

		require.ErrorIs(t, err, ErrUnexpectedStatus)
		assert.Contains(t, err.Error(), "500")
	})

	t.Run("body is not JSON", func(t *testing.T) {
		t.Parallel()

		service := newTestService(t, "bark://:devicekey@hostname")
		client, _ := newRespondingClient(t, http.StatusBadGateway, "<html>Bad Gateway</html>")
		service.SetHTTPClient(client)

		err := service.Send("Message", nil)

		require.Error(t, err)
		assert.Contains(t, err.Error(), "decoding response")
	})
}

// TestSendContextErrors verifies that a send wraps a transport error, reports
// the server message carried by a jsonclient.Error, rejects an unknown param
// before any request, and stops when the caller's context is canceled.
func TestSendContextErrors(t *testing.T) {
	t.Parallel()

	t.Run("transport error", func(t *testing.T) {
		t.Parallel()

		service := newTestService(t, "bark://:devicekey@hostname")
		client := mocks.NewMockHTTPClient(t)
		client.EXPECT().Do(mock.Anything).Return(nil, errTransport).Once()
		service.SetHTTPClient(client)

		err := service.SendContext(t.Context(), "Message", nil)

		require.ErrorIs(t, err, ErrFailedAPIRequest)
		require.ErrorIs(t, err, errTransport)
	})

	t.Run("jsonclient error with a server message", func(t *testing.T) {
		t.Parallel()

		service := newTestService(t, "bark://:devicekey@hostname")
		client := mocks.NewMockHTTPClient(t)
		client.EXPECT().Do(mock.Anything).
			Return(nil, jsonclient.Error{StatusCode: http.StatusBadRequest, Body: deviceErrorBody}).Once()
		service.SetHTTPClient(client)

		err := service.SendContext(t.Context(), "Message", nil)

		_, ok := errors.AsType[*APIResponse](err)
		require.True(t, ok, "error %v is not an *APIResponse", err)
		assert.Contains(t, err.Error(), "failed to get device token")
	})

	t.Run("jsonclient error without a JSON body", func(t *testing.T) {
		t.Parallel()

		service := newTestService(t, "bark://:devicekey@hostname")
		client := mocks.NewMockHTTPClient(t)
		client.EXPECT().Do(mock.Anything).
			Return(nil, jsonclient.Error{StatusCode: http.StatusBadGateway, Body: "Bad Gateway"}).Once()
		service.SetHTTPClient(client)

		err := service.SendContext(t.Context(), "Message", nil)

		require.ErrorIs(t, err, ErrFailedAPIRequest)
	})

	t.Run("unknown param", func(t *testing.T) {
		t.Parallel()

		service := newTestService(t, "bark://:devicekey@hostname")
		// The mock has no expectations, so a request fails the test.
		service.SetHTTPClient(mocks.NewMockHTTPClient(t))

		err := service.SendContext(t.Context(), "Message", &types.Params{"foo": "bar"})

		require.ErrorIs(t, err, ErrUpdateParamsFailed)
		require.ErrorIs(t, err, format.ErrInvalidConfigKey)
	})

	t.Run("canceled context", func(t *testing.T) {
		t.Parallel()

		ctx, cancel := context.WithCancel(t.Context())
		cancel()

		client := mocks.NewMockHTTPClient(t)
		client.EXPECT().Do(mock.Anything).RunAndReturn(func(req *http.Request) (*http.Response, error) {
			return nil, req.Context().Err()
		}).Once()

		service := newTestService(t, "bark://:devicekey@bark.example.invalid")
		service.SetHTTPClient(client)

		require.ErrorIs(t, service.SendContext(ctx, "message", nil), context.Canceled)
	})
}

// TestSendRedactsUnparsableURL verifies that a host that makes the API URL
// unparsable fails the send before any request, without the URL in the error.
func TestSendRedactsUnparsableURL(t *testing.T) {
	t.Parallel()

	service := newTestService(t, "bark://:devicekey@bark.example.com/SECRETpath")
	service.Config.Host = "bad host"
	// The mock has no expectations, so a request fails the test.
	service.SetHTTPClient(mocks.NewMockHTTPClient(t))

	err := service.Send("message", nil)

	_, isURLErr := errors.AsType[*url.Error](err)
	require.True(t, isURLErr, "error %v is not a *url.Error", err)
	assert.NotContains(t, err.Error(), "SECRETpath")
}

// TestSendDefaultClient verifies that a send reaches a plain HTTP server
// through the default client that Initialize sets and that a nil
// *http.Client selects.
func TestSendDefaultClient(t *testing.T) {
	t.Parallel()

	// tests lists how each case sets up the service's HTTP client.
	tests := []struct {
		configure func(*Service)
		name      string
	}{
		{name: "client set by Initialize"},
		{name: "nil *http.Client", configure: func(s *Service) { s.SetHTTPClient((*http.Client)(nil)) }},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			requests := make(chan capturedRequest, 1)

			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
				body, _ := io.ReadAll(req.Body)
				requests <- capturedRequest{method: req.Method, url: req.URL.String(), body: string(body)}

				_, _ = io.WriteString(w, successBody)
			}))
			t.Cleanup(server.Close)

			service := newTestService(t, "bark://:devicekey@"+server.Listener.Addr().String()+"?scheme=http")

			if tt.configure != nil {
				tt.configure(service)
			}

			require.NoError(t, service.Send("Message", nil))

			received := <-requests
			assert.Equal(t, http.MethodPost, received.method)
			assert.Equal(t, "/push", received.url)
			assert.JSONEq(t, `{"body":"Message","device_key":"devicekey","title":"","badge":0}`, received.body)
		})
	}
}

// TestDefaultHTTPClientDo verifies that the default client redacts the request
// URL from a transport error and keeps the error matchable as a *url.Error.
func TestDefaultHTTPClientDo(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.NotFoundHandler())
	serverURL := server.URL
	server.Close()

	req, err := http.NewRequestWithContext(t.Context(), http.MethodPost, serverURL+"/SECRETpath/push", http.NoBody)
	require.NoError(t, err)

	resp, err := NewDefaultHTTPClient().Do(req)
	if resp != nil {
		_ = resp.Body.Close()
	}

	_, isURLErr := errors.AsType[*url.Error](err)
	require.True(t, isURLErr, "error %v is not a *url.Error", err)
	assert.NotContains(t, err.Error(), "SECRETpath")
}

// TestSetHTTPClient verifies that SetHTTPClient stores a custom client and
// selects the default client for nil.
func TestSetHTTPClient(t *testing.T) {
	t.Parallel()

	// tests lists the clients to set and whether the default client should
	// replace them.
	tests := []struct {
		client      types.HTTPClient
		name        string
		wantDefault bool
	}{
		{name: "custom client", client: mocks.NewMockHTTPClient(t)},
		{name: "nil", client: nil, wantDefault: true},
		{name: "nil *http.Client", client: (*http.Client)(nil), wantDefault: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			service := &Service{}
			service.SetHTTPClient(tt.client)

			if tt.wantDefault {
				assert.IsType(t, &DefaultHTTPClient{}, service.HTTPClient)

				return
			}

			assert.Same(t, tt.client, service.HTTPClient)
		})
	}
}

// TestServiceTimeout verifies that the router budget for a send is the HTTP
// timeout.
func TestServiceTimeout(t *testing.T) {
	t.Parallel()

	assert.Equal(t, defaultHTTPTimeout, (&Service{}).ServiceTimeout(nil))
}
