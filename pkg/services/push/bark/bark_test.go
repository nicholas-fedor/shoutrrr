package bark

import (
	"bytes"
	"context"
	"errors"
	"io"
	"log"
	"net/http"
	"net/http/httptest"
	"net/url"
	"slices"
	"strings"
	"testing"
	"testing/synctest"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	"github.com/nicholas-fedor/shoutrrr/internal/httpbody"
	"github.com/nicholas-fedor/shoutrrr/pkg/format"
	"github.com/nicholas-fedor/shoutrrr/pkg/services/push/bark/mocks"
	"github.com/nicholas-fedor/shoutrrr/pkg/types"
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

// TestInitialize verifies that Initialize applies the default props, leaves
// the deprecated client field unset, keeps a client set before it runs, and
// rejects an invalid URL.
func TestInitialize(t *testing.T) {
	t.Parallel()

	t.Run("defaults", func(t *testing.T) {
		t.Parallel()

		service := newTestService(t, "bark://:devicekey@hostname")

		assert.Equal(t, Config{Host: "hostname", DeviceKey: "devicekey", Scheme: "https"}, *service.Config)
		assert.Nil(t, service.HTTPClient, "Initialize leaves the deprecated client field unset")
	})

	t.Run("keeps an injected client", func(t *testing.T) {
		t.Parallel()

		client := mocks.NewMockHTTPClient(t)
		service := &Service{}
		service.SetHTTPClient(client)
		require.NoError(t, service.Initialize(parseTestURL(t, "bark://:devicekey@hostname"), log.New(t.Output(), "", 0)))

		assert.Same(t, client, service.client.Prefer(service.HTTPClient))
	})

	t.Run("invalid scheme", func(t *testing.T) {
		t.Parallel()

		err := (&Service{}).Initialize(parseTestURL(t, "bark://:devicekey@hostname?scheme=ftp"), log.New(t.Output(), "", 0))

		require.ErrorIs(t, err, ErrInvalidScheme)
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

// TestSendChecksConfig verifies that a send checks a config built in code as
// reading a URL does, so a config without a device key or a host fails before
// any request.
func TestSendChecksConfig(t *testing.T) {
	t.Parallel()

	// tests lists config changes with the error the send must return.
	tests := []struct {
		change  func(config *Config)
		wantErr error
		name    string
	}{
		{name: "zero config", change: func(config *Config) { *config = Config{} }, wantErr: ErrMissingDeviceKey},
		{name: "no host", change: func(config *Config) { config.Host = "" }, wantErr: ErrMissingHost},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			service := newTestService(t, "bark://:devicekey@hostname")
			// The mock has no expectations, so a request fails the test.
			service.SetHTTPClient(mocks.NewMockHTTPClient(t))
			tt.change(service.Config)

			require.ErrorIs(t, service.Send("Message", nil), tt.wantErr)
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

// TestSendSchemePrecedence verifies that disabletls decides the protocol of a
// send over the deprecated scheme. A disabletls param overrides a stored http
// scheme, a scheme param of either protocol applies when the params do not set
// disabletls, and disabletls wins when the params set both.
func TestSendSchemePrecedence(t *testing.T) {
	t.Parallel()

	// tests lists service URLs and params with the request URL to expect.
	tests := []struct {
		params  *types.Params
		name    string
		rawURL  string
		wantURL string
	}{
		{
			name:    "disabletls param overrides the stored scheme",
			rawURL:  "bark://:devicekey@hostname?scheme=http",
			params:  &types.Params{"disabletls": "no"},
			wantURL: "https://hostname/push",
		},
		{name: "stored scheme", rawURL: "bark://:devicekey@hostname?scheme=http", wantURL: "http://hostname/push"},
		{
			name:    "https scheme param overrides the stored scheme",
			rawURL:  "bark://:devicekey@hostname?scheme=http",
			params:  &types.Params{"scheme": "https"},
			wantURL: "https://hostname/push",
		},
		{name: "disabletls param", rawURL: "bark://:devicekey@hostname", params: &types.Params{"disabletls": "yes"}, wantURL: "http://hostname/push"},
		{name: "scheme param", rawURL: "bark://:devicekey@hostname", params: &types.Params{"scheme": "http"}, wantURL: "http://hostname/push"},
		{
			name:    "disabletls param wins over a scheme param",
			rawURL:  "bark://:devicekey@hostname",
			params:  &types.Params{"scheme": "http", "disabletls": "no"},
			wantURL: "https://hostname/push",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			service := newTestService(t, tt.rawURL)
			client, captured := newRespondingClient(t, http.StatusOK, successBody)
			service.SetHTTPClient(client)

			require.NoError(t, service.Send("Message", tt.params))

			assert.Equal(t, tt.wantURL, captured.url)
		})
	}
}

// TestSendWarnings verifies that the plaintext warning, the deprecated scheme
// key, and the deprecated Scheme field are each warned about once over two
// sends, and appear again after the service is initialized again.
func TestSendWarnings(t *testing.T) {
	t.Parallel()

	var logs bytes.Buffer

	service := &Service{}
	require.NoError(t, service.Initialize(parseTestURL(t, "bark://:devicekey@hostname"), log.New(&logs, "", 0)))

	client := mocks.NewMockHTTPClient(t)
	client.EXPECT().Do(mock.Anything).RunAndReturn(func(*http.Request) (*http.Response, error) {
		return &http.Response{StatusCode: http.StatusOK, Body: io.NopCloser(strings.NewReader(successBody))}, nil
	}).Times(3)
	service.SetHTTPClient(client)
	service.Config.Scheme = "http"

	params := &types.Params{"scheme": "http"}
	require.NoError(t, service.Send("Message", params))
	require.NoError(t, service.Send("Message", params))

	want := []string{
		"WARN config field is deprecated field=Scheme replacement=DisableTLS",
		`WARN config key is deprecated key=scheme replacement=disabletls`,
		"WARN sending without TLS",
	}
	assert.Equal(t, want, warningLines(logs.String()))

	require.NoError(t, service.Initialize(parseTestURL(t, "bark://:devicekey@hostname?scheme=http"), log.New(&logs, "", 0)))
	require.NoError(t, service.Send("Message", nil))

	again := []string{`WARN config key is deprecated key=scheme replacement=disabletls`, "WARN sending without TLS"}
	assert.Equal(t, slices.Concat(want, again), warningLines(logs.String()))
}

// TestSendSkipTLSVerify verifies sends to a server whose certificate cannot be
// verified. Without skiptlsverify the send fails. With it in the URL or the
// params, the send goes through the insecure default client and warns once
// over two sends. A client set through the deprecated HTTPClient field is used
// unchanged and never warned about.
func TestSendSkipTLSVerify(t *testing.T) {
	t.Parallel()

	const insecureWarning = "WARN skipping TLS certificate verification"

	// tests lists the URL query, the send params, and whether the deprecated
	// client field is set, with the expected outcome.
	tests := []struct {
		params       *types.Params
		name         string
		query        string
		clientField  bool
		wantErr      bool
		wantWarnings []string
	}{
		{name: "verified by default", query: "", params: nil, clientField: false, wantErr: true, wantWarnings: nil},
		{name: "URL", query: "?skiptlsverify=yes", params: nil, clientField: false, wantErr: false, wantWarnings: []string{insecureWarning}},
		{
			name: "params", query: "", params: &types.Params{"skiptlsverify": "yes"}, clientField: false,
			wantErr: false, wantWarnings: []string{insecureWarning},
		},
		{name: "deprecated client field", query: "?skiptlsverify=yes", params: nil, clientField: true, wantErr: false, wantWarnings: nil},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
				_, _ = io.WriteString(w, successBody)
			}))
			t.Cleanup(server.Close)

			var logs bytes.Buffer

			service := &Service{}
			rawURL := "bark://:devicekey@" + server.Listener.Addr().String() + tt.query
			require.NoError(t, service.Initialize(parseTestURL(t, rawURL), log.New(&logs, "", 0)))

			if tt.clientField {
				service.HTTPClient = server.Client()
			}

			for range 2 {
				err := service.Send("Message", tt.params)
				require.Equal(t, tt.wantErr, err != nil, "send error: %v", err)
			}

			assert.Equal(t, tt.wantWarnings, warningLines(logs.String()))
		})
	}
}

// warningLines returns the logged lines that are warnings.
//
// Parameters:
//   - logs: the log output.
//
// Returns:
//   - []string: the warning lines in order.
func warningLines(logs string) []string {
	var lines []string

	for line := range strings.Lines(logs) {
		if strings.HasPrefix(line, "WARN ") {
			lines = append(lines, strings.TrimSuffix(line, "\n"))
		}
	}

	return lines
}

// TestSendBudget verifies that a send without a caller deadline is bounded by
// the budget that ServiceTimeout reports, and that a caller's deadline is
// kept as it is.
func TestSendBudget(t *testing.T) {
	t.Parallel()

	synctest.Test(t, func(t *testing.T) {
		var deadline time.Time

		client := mocks.NewMockHTTPClient(t)
		client.EXPECT().Do(mock.Anything).RunAndReturn(func(req *http.Request) (*http.Response, error) {
			deadline, _ = req.Context().Deadline()

			return &http.Response{StatusCode: http.StatusOK, Body: io.NopCloser(strings.NewReader(successBody))}, nil
		})

		service := newTestService(t, "bark://:devicekey@hostname")
		service.SetHTTPClient(client)

		start := time.Now()

		require.NoError(t, service.Send("Message", nil))
		assert.Equal(t, start.Add(defaultHTTPTimeout), deadline, "a send without a deadline gets the budget")

		ctx, cancel := context.WithTimeout(t.Context(), time.Hour)
		defer cancel()

		require.NoError(t, service.SendContext(ctx, "Message", nil))
		assert.Equal(t, start.Add(time.Hour), deadline, "the caller's deadline applies")
	})
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
// rejects. The server's message comes back as an *APIResponse, and a response
// without a message as ErrUnexpectedStatus. An error status whose body is not
// JSON, or whose JSON body reports success, gives ErrUnexpectedStatus with the
// HTTP status. A success status with a body that is not JSON gives a decoding
// error, and a body over [httpbody.MaxSize] gives [httpbody.ErrTooLarge].
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

	t.Run("error status with a body that is not JSON", func(t *testing.T) {
		t.Parallel()

		service := newTestService(t, "bark://:devicekey@hostname")
		client, _ := newRespondingClient(t, http.StatusBadGateway, "<html>Bad Gateway</html>")
		service.SetHTTPClient(client)

		err := service.Send("Message", nil)

		require.ErrorIs(t, err, ErrUnexpectedStatus)
		assert.Contains(t, err.Error(), "502")
	})

	t.Run("error status with a JSON body that reports success", func(t *testing.T) {
		t.Parallel()

		service := newTestService(t, "bark://:devicekey@hostname")
		client, _ := newRespondingClient(t, http.StatusBadGateway, successBody)
		service.SetHTTPClient(client)

		err := service.Send("Message", nil)

		require.ErrorIs(t, err, ErrUnexpectedStatus)
		assert.Contains(t, err.Error(), "502")
	})

	t.Run("success status with a body that is not JSON", func(t *testing.T) {
		t.Parallel()

		service := newTestService(t, "bark://:devicekey@hostname")
		client, _ := newRespondingClient(t, http.StatusOK, "<html>OK</html>")
		service.SetHTTPClient(client)

		err := service.Send("Message", nil)

		require.Error(t, err)
		assert.Contains(t, err.Error(), "decoding response")
	})

	t.Run("body too large", func(t *testing.T) {
		t.Parallel()

		service := newTestService(t, "bark://:devicekey@hostname")
		body := `{"code":200,"message":"success"}` + strings.Repeat(" ", httpbody.MaxSize)
		client, _ := newRespondingClient(t, http.StatusOK, body)
		service.SetHTTPClient(client)

		err := service.Send("Message", nil)

		require.ErrorIs(t, err, httpbody.ErrTooLarge)
	})
}

// TestSendContextErrors verifies that a send wraps a transport error, rejects
// an unknown param or an invalid scheme param before any request, and stops
// when the caller's context is canceled.
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

	t.Run("unknown param", func(t *testing.T) {
		t.Parallel()

		service := newTestService(t, "bark://:devicekey@hostname")
		// The mock has no expectations, so a request fails the test.
		service.SetHTTPClient(mocks.NewMockHTTPClient(t))

		err := service.SendContext(t.Context(), "Message", &types.Params{"foo": "bar"})

		require.ErrorIs(t, err, ErrUpdateParamsFailed)
		require.ErrorIs(t, err, format.ErrInvalidConfigKey)
	})

	t.Run("invalid stored scheme", func(t *testing.T) {
		t.Parallel()

		service := newTestService(t, "bark://:devicekey@hostname")
		service.Config.Scheme = "ftp"
		// The mock has no expectations, so a request fails the test.
		service.SetHTTPClient(mocks.NewMockHTTPClient(t))

		err := service.SendContext(t.Context(), "Message", nil)

		require.ErrorIs(t, err, ErrInvalidScheme)
	})

	t.Run("invalid scheme param", func(t *testing.T) {
		t.Parallel()

		service := newTestService(t, "bark://:devicekey@hostname")
		// The mock has no expectations, so a request fails the test.
		service.SetHTTPClient(mocks.NewMockHTTPClient(t))

		err := service.SendContext(t.Context(), "Message", &types.Params{"scheme": "ftp"})

		require.ErrorIs(t, err, ErrInvalidScheme)
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
// through the default client, without a client set and after a nil
// *http.Client is set.
func TestSendDefaultClient(t *testing.T) {
	t.Parallel()

	// tests lists how each case sets up the service's HTTP client.
	tests := []struct {
		configure func(*Service)
		name      string
	}{
		{name: "no client set"},
		{name: "nil *http.Client", configure: func(s *Service) { s.SetHTTPClient((*http.Client)(nil)) }},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			requests := make(chan capturedRequest, 1)

			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
				body, _ := io.ReadAll(req.Body)

				// A non-blocking send keeps an extra request from stalling
				// server.Close, which waits for running handlers.
				select {
				case requests <- capturedRequest{method: req.Method, url: req.URL.String(), body: string(body)}:
				default:
					t.Errorf("unexpected extra request: %s %s", req.Method, req.URL)
				}

				_, _ = io.WriteString(w, successBody)
			}))
			t.Cleanup(server.Close)

			service := newTestService(t, "bark://:devicekey@"+server.Listener.Addr().String()+"?disabletls=yes")

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

// TestDefaultHTTPClientDo verifies that the default client sends a request,
// redacts the request URL from a transport error and keeps the error
// matchable as a *url.Error, and works as a zero value.
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

	t.Run("success", func(t *testing.T) {
		t.Parallel()

		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			_, _ = io.WriteString(w, successBody)
		}))
		t.Cleanup(server.Close)

		req, err := http.NewRequestWithContext(t.Context(), http.MethodPost, server.URL+"/push", http.NoBody)
		require.NoError(t, err)

		resp, err := NewDefaultHTTPClient().Do(req)
		require.NoError(t, err)
		assert.Equal(t, http.StatusOK, resp.StatusCode)
		require.NoError(t, resp.Body.Close())
	})

	t.Run("zero value", func(t *testing.T) {
		t.Parallel()

		ctx, cancel := context.WithCancel(t.Context())
		cancel()

		req, err := http.NewRequestWithContext(ctx, http.MethodPost, serverURL+"/push", http.NoBody)
		require.NoError(t, err)

		_, err = (&DefaultHTTPClient{}).Do(req)
		require.ErrorIs(t, err, context.Canceled)
	})
}

// TestSetHTTPClient verifies that SetHTTPClient stores a custom client in
// place of the deprecated client field, and selects the default client for
// nil.
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

			service := &Service{HTTPClient: mocks.NewMockHTTPClient(t)}
			service.SetHTTPClient(tt.client)

			assert.Nil(t, service.HTTPClient, "SetHTTPClient clears the deprecated client field")

			if tt.wantDefault {
				assert.IsType(t, &http.Client{}, service.client.Prefer(service.HTTPClient))

				return
			}

			assert.Same(t, tt.client, service.client.Prefer(service.HTTPClient))
		})
	}
}

// TestServiceTimeout verifies that the router budget for a send is the HTTP
// timeout.
func TestServiceTimeout(t *testing.T) {
	t.Parallel()

	assert.Equal(t, defaultHTTPTimeout, (&Service{}).ServiceTimeout(nil))
}
