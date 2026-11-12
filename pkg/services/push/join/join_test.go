package join

import (
	"context"
	"errors"
	"io"
	"log"
	"net/http"
	"net/url"
	"strings"
	"testing"
	"testing/iotest"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	"github.com/nicholas-fedor/shoutrrr/pkg/format"
	"github.com/nicholas-fedor/shoutrrr/pkg/types"
	"github.com/nicholas-fedor/shoutrrr/pkg/types/mocks"
)

// capturedRequest records the parts of a request that an HTTP client received.
type capturedRequest struct {
	// query is the parsed request query.
	query url.Values
	// method is the request method.
	method string
	// endpoint is the request URL without its query.
	endpoint string
	// contentType is the request's Content-Type header.
	contentType string
}

// acceptedBody is the reply body of a push that Join accepted.
const acceptedBody = `{"success":true,"userAuthError":false}`

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
func newRespondingClient(
	t *testing.T,
	status string,
	statusCode int,
	body string,
) (*mocks.MockHTTPClient, *capturedRequest) {
	t.Helper()

	captured := &capturedRequest{}
	client := mocks.NewMockHTTPClient(t)
	client.EXPECT().Do(mock.Anything).RunAndReturn(func(req *http.Request) (*http.Response, error) {
		endpoint := *req.URL
		endpoint.RawQuery = ""

		*captured = capturedRequest{
			query:       req.URL.Query(),
			method:      req.Method,
			endpoint:    endpoint.String(),
			contentType: req.Header.Get("Content-Type"),
		}

		return &http.Response{
			Status:     status,
			StatusCode: statusCode,
			Body:       io.NopCloser(strings.NewReader(body)),
		}, nil
	}).Once()

	return client, captured
}

// newCutOffClient returns a mock HTTP client that expects one request and
// answers with the given status and a body whose read fails with
// [context.DeadlineExceeded] partway through.
//
// Parameters:
//   - t: the test that owns the mock.
//   - status: the response status line, such as "200 OK".
//   - statusCode: the response status code.
//
// Returns:
//   - *mocks.MockHTTPClient: the client.
func newCutOffClient(t *testing.T, status string, statusCode int) *mocks.MockHTTPClient {
	t.Helper()

	client := mocks.NewMockHTTPClient(t)
	client.EXPECT().Do(mock.Anything).RunAndReturn(func(*http.Request) (*http.Response, error) {
		body := io.MultiReader(strings.NewReader(`{"success":fa`), iotest.ErrReader(context.DeadlineExceeded))

		return &http.Response{
			Status:     status,
			StatusCode: statusCode,
			Body:       io.NopCloser(body),
		}, nil
	}).Once()

	return client
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

	assert.Equal(t, "join", (&Service{}).GetID())
}

// TestInitialize verifies that Initialize parses the service URL and rejects a
// URL without devices.
func TestInitialize(t *testing.T) {
	t.Parallel()

	t.Run("valid URL", func(t *testing.T) {
		t.Parallel()

		service := newTestService(t, "join://Token:apikey@join?devices=dev1,dev2&title=Title")

		assert.Equal(t, &Config{APIKey: "apikey", Devices: []string{"dev1", "dev2"}, Title: "Title"}, service.Config)
	})

	t.Run("missing devices", func(t *testing.T) {
		t.Parallel()

		err := (&Service{}).Initialize(parseTestURL(t, "join://Token:apikey@join"), log.New(t.Output(), "", 0))

		require.ErrorIs(t, err, ErrDevicesMissing)
	})
}

// TestSend verifies the request a send makes.
//
// The request is a POST to the Join push endpoint with the API key, devices,
// and text in the query. The title and icon are added only when set, each
// independently of the other. Params override any config key, in any case.
func TestSend(t *testing.T) {
	t.Parallel()

	// tests lists service URLs and params with the query the send should
	// produce.
	tests := []struct {
		params    *types.Params
		name      string
		rawURL    string
		wantQuery url.Values
	}{
		{
			name:      "neither title nor icon",
			rawURL:    "join://Token:apikey@join?devices=dev1",
			wantQuery: url.Values{"apikey": {"apikey"}, "deviceIds": {"dev1"}, "text": {"Message"}},
		},
		{
			name:   "devices joined by commas",
			rawURL: "join://Token:apikey@join?devices=dev1,dev2",
			wantQuery: url.Values{
				"apikey": {"apikey"}, "deviceIds": {"dev1,dev2"}, "text": {"Message"},
			},
		},
		{
			name:   "configured title and icon",
			rawURL: "join://Token:apikey@join?devices=dev1&title=Configured&icon=https%3A%2F%2Fexample.com%2Fconfigured.png",
			wantQuery: url.Values{
				"apikey": {"apikey"}, "deviceIds": {"dev1"}, "text": {"Message"},
				"title": {"Configured"}, "icon": {"https://example.com/configured.png"},
			},
		},
		{
			name:   "icon param without a title",
			rawURL: "join://Token:apikey@join?devices=dev1",
			params: &types.Params{"icon": "https://example.com/icon.png"},
			wantQuery: url.Values{
				"apikey": {"apikey"}, "deviceIds": {"dev1"}, "text": {"Message"},
				"icon": {"https://example.com/icon.png"},
			},
		},
		{
			name:   "title param without an icon",
			rawURL: "join://Token:apikey@join?devices=dev1",
			params: &types.Params{"title": "Alert"},
			wantQuery: url.Values{
				"apikey": {"apikey"}, "deviceIds": {"dev1"}, "text": {"Message"}, "title": {"Alert"},
			},
		},
		{
			name:   "devices param",
			rawURL: "join://Token:apikey@join?devices=dev1",
			params: &types.Params{"devices": "dev2,dev3"},
			wantQuery: url.Values{
				"apikey": {"apikey"}, "deviceIds": {"dev2,dev3"}, "text": {"Message"},
			},
		},
		{
			name:   "param in another case",
			rawURL: "join://Token:apikey@join?devices=dev1",
			params: &types.Params{"Title": "Alert"},
			wantQuery: url.Values{
				"apikey": {"apikey"}, "deviceIds": {"dev1"}, "text": {"Message"}, "title": {"Alert"},
			},
		},
		{
			name:   "params override the configured title and icon",
			rawURL: "join://Token:apikey@join?devices=dev1&title=Configured&icon=configured",
			params: &types.Params{"title": "Alert", "icon": "https://example.com/icon.png"},
			wantQuery: url.Values{
				"apikey": {"apikey"}, "deviceIds": {"dev1"}, "text": {"Message"},
				"title": {"Alert"}, "icon": {"https://example.com/icon.png"},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			service := newTestService(t, tt.rawURL)
			client, captured := newRespondingClient(t, "200 OK", http.StatusOK, acceptedBody)
			service.SetHTTPClient(client)

			require.NoError(t, service.Send("Message", tt.params))

			assert.Equal(t, http.MethodPost, captured.method)
			assert.Equal(t, hookURL, captured.endpoint)
			assert.Equal(t, contentType, captured.contentType)
			assert.Equal(t, tt.wantQuery, captured.query)
		})
	}
}

// TestSendParams verifies that an unknown param or an empty devices param
// fails the send before any request, and that params leave the stored config
// unchanged.
func TestSendParams(t *testing.T) {
	t.Parallel()

	service := newTestService(t, "join://Token:apikey@join?devices=dev1&title=Configured")
	want := *service.Config

	// The mock has no expectations, so a request fails the test.
	service.SetHTTPClient(mocks.NewMockHTTPClient(t))
	require.ErrorIs(t, service.Send("Message", &types.Params{"titel": "Alert"}), format.ErrInvalidConfigKey)
	require.ErrorIs(t, service.Send("Message", &types.Params{"devices": ""}), ErrDevicesMissing)

	client, _ := newRespondingClient(t, "200 OK", http.StatusOK, acceptedBody)
	service.SetHTTPClient(client)
	require.NoError(t, service.Send("Message", &types.Params{"title": "Alert", "devices": "dev2"}))
	assert.Equal(t, want, *service.Config)
}

// TestSendChecksConfig verifies that a send with a config built in code fails
// before any request when reading a URL with the same values would fail.
func TestSendChecksConfig(t *testing.T) {
	t.Parallel()

	// tests lists config changes with the error the send must return.
	tests := []struct {
		change  func(config *Config)
		wantErr error
		name    string
	}{
		{name: "no devices", change: func(config *Config) { config.Devices = nil }, wantErr: ErrDevicesMissing},
		{name: "no API key", change: func(config *Config) { config.APIKey = "" }, wantErr: ErrAPIKeyMissing},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			service := newTestService(t, "join://Token:apikey@join?devices=dev1")
			// The mock has no expectations, so a request fails the test.
			service.SetHTTPClient(mocks.NewMockHTTPClient(t))
			tt.change(service.Config)

			require.ErrorIs(t, service.Send("Message", nil), tt.wantErr)
		})
	}
}

// TestSendContextErrors verifies that a send wraps a transport error without
// the API key and stops when the caller's context is canceled.
//
// A 200 reply cut off before its end fails with the read error, since the
// unread part could hold a refusal. A cut-off error status still fails with
// ErrSendFailed and its status.
func TestSendContextErrors(t *testing.T) {
	t.Parallel()

	t.Run("200 reply cut off", func(t *testing.T) {
		t.Parallel()

		service := newTestService(t, "join://Token:apikey@join?devices=dev1")
		service.SetHTTPClient(newCutOffClient(t, "200 OK", http.StatusOK))

		require.ErrorIs(t, service.SendContext(t.Context(), "message", nil), context.DeadlineExceeded)
	})

	t.Run("error status cut off", func(t *testing.T) {
		t.Parallel()

		service := newTestService(t, "join://Token:apikey@join?devices=dev1")
		service.SetHTTPClient(newCutOffClient(t, "503 Service Unavailable", http.StatusServiceUnavailable))

		err := service.SendContext(t.Context(), "message", nil)

		require.ErrorIs(t, err, ErrSendFailed)
		assert.Contains(t, err.Error(), "503 Service Unavailable")
	})

	t.Run("transport error", func(t *testing.T) {
		t.Parallel()

		service := newTestService(t, "join://Token:SECRETkey@join?devices=dev1")
		client := mocks.NewMockHTTPClient(t)
		client.EXPECT().Do(mock.Anything).RunAndReturn(func(req *http.Request) (*http.Response, error) {
			return nil, &url.Error{Op: "Post", URL: req.URL.String(), Err: errTransport}
		}).Once()
		service.SetHTTPClient(client)

		err := service.SendContext(t.Context(), "Message", nil)

		require.ErrorIs(t, err, errTransport)
		assert.NotContains(t, err.Error(), "SECRETkey")
	})

	t.Run("canceled context", func(t *testing.T) {
		t.Parallel()

		ctx, cancel := context.WithCancel(t.Context())
		cancel()

		client := mocks.NewMockHTTPClient(t)
		client.EXPECT().Do(mock.Anything).RunAndReturn(func(req *http.Request) (*http.Response, error) {
			return nil, req.Context().Err()
		}).Once()

		service := newTestService(t, "join://Token:apikey@join?devices=dev1")
		service.SetHTTPClient(client)

		require.ErrorIs(t, service.SendContext(ctx, "message", nil), context.Canceled)
	})
}

// TestSendResponses verifies how a send reads Join's reply.
//
// Join answers a push with HTTP 200 even when it refuses it, and reports in the
// JSON body whether it accepted it. A refused push fails with ErrSendFailed and
// Join's error message. A 200 reply whose body is not JSON leaves the status to
// decide.
func TestSendResponses(t *testing.T) {
	t.Parallel()

	// tests lists Join replies with the text a failed send's error must hold.
	// A reply without wantContains must give a successful send.
	tests := []struct {
		name         string
		status       string
		body         string
		wantContains []string
		statusCode   int
	}{
		{name: "accepted", status: "200 OK", statusCode: http.StatusOK, body: acceptedBody},
		{
			name:         "refused with a message",
			status:       "200 OK",
			statusCode:   http.StatusOK,
			body:         `{"success":false,"userAuthError":false,"errorMessage":"No device to send message to"}`,
			wantContains: []string{"No device to send message to", "dev1", "200 OK"},
		},
		{
			name:         "refused without a message",
			status:       "200 OK",
			statusCode:   http.StatusOK,
			body:         `{"success":false,"userAuthError":true}`,
			wantContains: []string{"200 OK", "dev1"},
		},
		{name: "body that is not JSON", status: "200 OK", statusCode: http.StatusOK, body: "<html>ok</html>"},
		{
			name:         "error status with a message",
			status:       "500 Internal Server Error",
			statusCode:   http.StatusInternalServerError,
			body:         `{"success":false,"errorMessage":"backend unavailable"}`,
			wantContains: []string{"500 Internal Server Error", "backend unavailable"},
		},
		{
			name:         "error status reporting success",
			status:       "502 Bad Gateway",
			statusCode:   http.StatusBadGateway,
			body:         acceptedBody,
			wantContains: []string{"502 Bad Gateway"},
		},
		{
			name:         "error status without JSON",
			status:       "401 Unauthorized",
			statusCode:   http.StatusUnauthorized,
			body:         "",
			wantContains: []string{"401 Unauthorized", "dev1"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			service := newTestService(t, "join://Token:apikey@join?devices=dev1")
			client, _ := newRespondingClient(t, tt.status, tt.statusCode, tt.body)
			service.SetHTTPClient(client)

			err := service.SendContext(t.Context(), "Message", nil)

			if len(tt.wantContains) == 0 {
				require.NoError(t, err)

				return
			}

			require.ErrorIs(t, err, ErrSendFailed)

			for _, want := range tt.wantContains {
				assert.Contains(t, err.Error(), want)
			}
		})
	}
}

// TestSendDefaultClient verifies that a send without a custom client, or with
// a nil *http.Client, uses a default client that honors the caller's context.
// The context is canceled before the send, so the transport returns before it
// dials the Join API.
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

			service := newTestService(t, "join://Token:SECRETkey@join?devices=dev1")

			if tt.configure != nil {
				tt.configure(service)
			}

			ctx, cancel := context.WithCancel(t.Context())
			cancel()

			err := service.SendContext(ctx, "Message", nil)

			require.ErrorIs(t, err, context.Canceled)
			assert.NotContains(t, err.Error(), "SECRETkey")
		})
	}
}

// TestDefaultHTTPClient verifies that the default client has no request
// timeout, since the send budget bounds each send, and sends through
// http.DefaultTransport, so it honors the proxy environment variables.
func TestDefaultHTTPClient(t *testing.T) {
	t.Parallel()

	service := newTestService(t, "join://Token:apikey@join?devices=dev1")

	httpClient, ok := service.client.Client().(*http.Client)
	require.True(t, ok)
	assert.Zero(t, httpClient.Timeout)
	assert.Nil(t, httpClient.Transport)
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

		assert.Same(t, client, service.client.Client())
	})

	t.Run("nil *http.Client", func(t *testing.T) {
		t.Parallel()

		service := &Service{}
		service.SetHTTPClient(mocks.NewMockHTTPClient(t))
		service.SetHTTPClient((*http.Client)(nil))

		assert.IsType(t, &http.Client{}, service.client.Client())
	})
}
