package signalgrid

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

	"github.com/nicholas-fedor/shoutrrr/internal/meta"
	"github.com/nicholas-fedor/shoutrrr/pkg/format"
	"github.com/nicholas-fedor/shoutrrr/pkg/types"
	"github.com/nicholas-fedor/shoutrrr/pkg/types/mocks"
	"github.com/nicholas-fedor/shoutrrr/pkg/util"
)

// capturedRequest records the parts of a request that an HTTP client received.
type capturedRequest struct {
	// form is the parsed form body.
	form url.Values
	// method is the request method.
	method string
	// url is the full request URL.
	url string
	// contentType is the request's Content-Type header.
	contentType string
	// userAgent is the request's User-Agent header.
	userAgent string
}

// successBody is the body the Push API returns for an accepted notification.
const successBody = `{"ruuid":"bc0e212110067ff2995abff087a4a58c8d843ae9","text":"OK","code":"200","node":"dp03","time":"440ms"}`

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

		form, err := url.ParseQuery(string(requestBody))
		require.NoError(t, err)

		*captured = capturedRequest{
			form:        form,
			method:      req.Method,
			url:         req.URL.String(),
			contentType: req.Header.Get("Content-Type"),
			userAgent:   req.Header.Get("User-Agent"),
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

	assert.Equal(t, "signalgrid", (&Service{}).GetID())
}

// TestInitialize verifies that Initialize applies the default props, keeps an
// injected client, and rejects a URL without a client key.
func TestInitialize(t *testing.T) {
	t.Parallel()

	t.Run("valid URL", func(t *testing.T) {
		t.Parallel()

		service := newTestService(t, "signalgrid://abc123@channel")

		assert.Equal(t, &Config{ClientKey: "abc123", Channel: "channel", Type: TypeINFO}, service.Config)
	})

	t.Run("keeps an injected client", func(t *testing.T) {
		t.Parallel()

		client := mocks.NewMockHTTPClient(t)
		service := &Service{}
		service.SetHTTPClient(client)
		require.NoError(t, service.Initialize(parseTestURL(t, "signalgrid://key@channel"), log.New(t.Output(), "", 0)))

		assert.Same(t, client, service.client.Client())
	})

	t.Run("missing client key", func(t *testing.T) {
		t.Parallel()

		err := (&Service{}).Initialize(parseTestURL(t, "signalgrid://channel"), log.New(t.Output(), "", 0))

		require.ErrorIs(t, err, ErrClientKeyMissing)
	})
}

// TestSend verifies the request a send makes: a form POST to the Push API with
// the client key, channel, body, and type, and the title and critical fields
// only when set.
func TestSend(t *testing.T) {
	t.Parallel()

	// tests lists service URLs and params with the form the send should
	// produce.
	tests := []struct {
		params   *types.Params
		name     string
		rawURL   string
		wantForm url.Values
	}{
		{
			name:   "defaults",
			rawURL: "signalgrid://clientkey@channeltoken",
			wantForm: url.Values{
				"client_key": {"clientkey"}, "channel": {"channeltoken"}, "body": {"Host unreachable"}, "type": {"INFO"},
			},
		},
		{
			name:   "configured title, type, and critical",
			rawURL: "signalgrid://clientkey@channeltoken?title=Server%20Down&type=CRIT&critical=true",
			wantForm: url.Values{
				"client_key": {"clientkey"}, "channel": {"channeltoken"}, "body": {"Host unreachable"}, "type": {"CRIT"},
				"title": {"Server Down"}, "critical": {"true"},
			},
		},
		{
			name:   "params",
			rawURL: "signalgrid://clientkey@channeltoken",
			params: &types.Params{"title": "Override", "type": "WARN", "critical": "Yes"},
			wantForm: url.Values{
				"client_key": {"clientkey"}, "channel": {"channeltoken"}, "body": {"Host unreachable"}, "type": {"WARN"},
				"title": {"Override"}, "critical": {"true"},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			service := newTestService(t, tt.rawURL)
			client, captured := newRespondingClient(t, "200 OK", http.StatusOK, successBody)
			service.SetHTTPClient(client)

			require.NoError(t, service.Send("Host unreachable", tt.params))

			assert.Equal(t, http.MethodPost, captured.method)
			assert.Equal(t, apiURL, captured.url)
			assert.Equal(t, contentType, captured.contentType)
			assert.Equal(t, meta.UserAgent(), captured.userAgent)
			assert.Equal(t, tt.wantForm, captured.form)
		})
	}
}

// TestSendChecksConfig verifies that a send checks a config built in code as
// reading a URL does. A config without a client key or a channel token fails
// before any request. So does a channel token with a colon or brackets.
func TestSendChecksConfig(t *testing.T) {
	t.Parallel()

	// tests lists config changes with the error the send must return.
	tests := []struct {
		change  func(config *Config)
		wantErr error
		name    string
	}{
		{name: "zero config", change: func(config *Config) { *config = Config{} }, wantErr: ErrClientKeyMissing},
		{name: "no channel", change: func(config *Config) { config.Channel = "" }, wantErr: ErrChannelMissing},
		{name: "channel with a port", change: func(config *Config) { config.Channel = "channel:443" }, wantErr: util.ErrInvalidHost},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			service := newTestService(t, "signalgrid://key@channel")
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

	service := newTestService(t, "signalgrid://clientkey@channeltoken")
	want := *service.Config

	client, _ := newRespondingClient(t, "200 OK", http.StatusOK, successBody)
	service.SetHTTPClient(client)

	require.NoError(t, service.Send("first", &types.Params{"title": "Override", "critical": "Yes"}))
	assert.Equal(t, want, *service.Config)

	client, captured := newRespondingClient(t, "200 OK", http.StatusOK, successBody)
	service.SetHTTPClient(client)

	require.NoError(t, service.Send("second", nil))
	assert.Equal(t, url.Values{
		"client_key": {"clientkey"}, "channel": {"channeltoken"}, "body": {"second"}, "type": {"INFO"},
	}, captured.form)
}

// TestSendContextErrors verifies that a send fails with ErrSendFailed and
// ErrUnexpectedStatus for an error status, wraps a transport error, rejects an
// unknown param before any request, and stops when the caller's context is
// canceled.
func TestSendContextErrors(t *testing.T) {
	t.Parallel()

	// statusTests lists error responses with the status they report.
	statusTests := []struct {
		name       string
		status     string
		body       string
		statusCode int
	}{
		{name: "server error", status: "500 Internal Server Error", statusCode: http.StatusInternalServerError, body: "boom"},
		{name: "unauthorized", status: "401 Unauthorized", statusCode: http.StatusUnauthorized, body: "denied"},
		{name: "redirect", status: "302 Found", statusCode: http.StatusFound, body: ""},
	}

	for _, tt := range statusTests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			service := newTestService(t, "signalgrid://clientkey@channeltoken")
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

		service := newTestService(t, "signalgrid://clientkey@channeltoken")
		client := mocks.NewMockHTTPClient(t)
		client.EXPECT().Do(mock.Anything).Return(nil, errTransport).Once()
		service.SetHTTPClient(client)

		err := service.SendContext(t.Context(), "fail", nil)

		require.ErrorIs(t, err, ErrSendFailed)
		require.ErrorIs(t, err, errTransport)
	})

	t.Run("unknown param", func(t *testing.T) {
		t.Parallel()

		service := newTestService(t, "signalgrid://clientkey@channeltoken")
		// The mock has no expectations, so a request fails the test.
		service.SetHTTPClient(mocks.NewMockHTTPClient(t))

		err := service.SendContext(t.Context(), "Message", &types.Params{"foo": "bar"})

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

		service := newTestService(t, "signalgrid://clientkey@channel")
		service.SetHTTPClient(client)

		require.ErrorIs(t, service.SendContext(ctx, "message", nil), context.Canceled)
	})
}

// TestSendDefaultClient verifies that a send with a nil *http.Client uses a
// default client that honors the caller's context. The context is canceled
// before the send, so the transport returns before it dials the Push API.
func TestSendDefaultClient(t *testing.T) {
	t.Parallel()

	service := newTestService(t, "signalgrid://SECRETkey@channel")
	service.SetHTTPClient((*http.Client)(nil))

	ctx, cancel := context.WithCancel(t.Context())
	cancel()

	err := service.SendContext(ctx, "Message", nil)

	require.ErrorIs(t, err, context.Canceled)
	assert.NotContains(t, err.Error(), "SECRETkey")
}

// TestDefaultHTTPClient verifies that the default client has no request
// timeout, since the send budget bounds each send, and sends through
// http.DefaultTransport, so it honors the proxy environment variables.
func TestDefaultHTTPClient(t *testing.T) {
	t.Parallel()

	service := newTestService(t, "signalgrid://abc123@channel")

	httpClient, ok := service.client.Client().(*http.Client)
	require.True(t, ok)
	assert.Zero(t, httpClient.Timeout)
	assert.Nil(t, httpClient.Transport)
}

// TestSetHTTPClient verifies that SetHTTPClient makes sends use a custom
// client, and that a nil *http.Client restores the default client.
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

// TestSendLevelType verifies that the level param sets the type, debug and
// info to INFO, warning to WARN, and error to CRIT, and that a type from the
// service URL or the send params, or an unknown level, leaves it as it is.
func TestSendLevelType(t *testing.T) {
	t.Parallel()

	const rawURL = "signalgrid://clientkey@channeltoken"

	tests := map[string]struct {
		rawURL   string
		params   *types.Params
		wantType string
	}{
		"debug":                   {rawURL: rawURL, params: &types.Params{"level": "Debug"}, wantType: "INFO"},
		"info":                    {rawURL: rawURL, params: &types.Params{"level": "Info"}, wantType: "INFO"},
		"warning":                 {rawURL: rawURL, params: &types.Params{"level": "Warning"}, wantType: "WARN"},
		"error":                   {rawURL: rawURL, params: &types.Params{"level": "Error"}, wantType: "CRIT"},
		"type in the URL":         {rawURL: rawURL + "?type=SUCCESS", params: &types.Params{"level": "Error"}, wantType: "SUCCESS"},
		"default type in the URL": {rawURL: rawURL + "?type=INFO", params: &types.Params{"level": "Error"}, wantType: "INFO"},
		"type param":              {rawURL: rawURL, params: &types.Params{"level": "Error", "type": "INFO"}, wantType: "INFO"},
		"unknown level":           {rawURL: rawURL, params: &types.Params{"level": "critical"}, wantType: "INFO"},
		"no params":               {rawURL: rawURL, params: nil, wantType: "INFO"},
	}

	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			service := newTestService(t, tc.rawURL)
			client, captured := newRespondingClient(t, "200 OK", http.StatusOK, successBody)
			service.SetHTTPClient(client)

			require.NoError(t, service.Send("Host unreachable", tc.params))

			assert.Equal(t, tc.wantType, captured.form.Get("type"))
		})
	}
}
