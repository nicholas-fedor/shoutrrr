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

// errTransport is the transport failure returned by the failing HTTP client.
var errTransport = errors.New("transport failure")

// newRespondingClient returns a mock HTTP client that expects one request,
// records it, and answers with the given status.
//
// Parameters:
//   - t: the test that owns the mock.
//   - status: the response status line, such as "200 OK".
//   - statusCode: the response status code.
//
// Returns:
//   - *mocks.MockHTTPClient: the client.
//   - *capturedRequest: the request, filled in once the client receives it.
func newRespondingClient(t *testing.T, status string, statusCode int) (*mocks.MockHTTPClient, *capturedRequest) {
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
			Body:       io.NopCloser(strings.NewReader("")),
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

// TestSend verifies the request a send makes: a POST to the Join push endpoint
// with the API key, devices, and text in the query, and the title and icon
// only when set, independently of each other. Params override any config key,
// in any case.
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
			client, captured := newRespondingClient(t, "200 OK", http.StatusOK)
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

	client, _ := newRespondingClient(t, "200 OK", http.StatusOK)
	service.SetHTTPClient(client)
	require.NoError(t, service.Send("Message", &types.Params{"title": "Alert", "devices": "dev2"}))
	assert.Equal(t, want, *service.Config)
}

// TestSendContextErrors verifies that a send fails with ErrSendFailed for an
// error status, wraps a transport error without the API key, and stops when
// the caller's context is canceled.
func TestSendContextErrors(t *testing.T) {
	t.Parallel()

	t.Run("error status", func(t *testing.T) {
		t.Parallel()

		service := newTestService(t, "join://Token:apikey@join?devices=dev1")
		client, _ := newRespondingClient(t, "401 Unauthorized", http.StatusUnauthorized)
		service.SetHTTPClient(client)

		err := service.SendContext(t.Context(), "Message", nil)

		require.ErrorIs(t, err, ErrSendFailed)
		assert.Contains(t, err.Error(), "401 Unauthorized")
		assert.Contains(t, err.Error(), "dev1")
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
