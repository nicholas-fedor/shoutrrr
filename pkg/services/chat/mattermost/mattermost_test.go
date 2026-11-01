package mattermost

import (
	"bytes"
	"context"
	"errors"
	"io"
	"log"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"sync"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	"github.com/nicholas-fedor/shoutrrr/pkg/format"
	"github.com/nicholas-fedor/shoutrrr/pkg/types"
	"github.com/nicholas-fedor/shoutrrr/pkg/types/mocks"
)

// capturedRequest records the parts of a request that an HTTP client or
// test server received.
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

// requestRecorder collects the requests a test server receives.
type requestRecorder struct {
	// mu guards requests.
	mu sync.Mutex
	// requests holds the requests in the order received.
	requests []capturedRequest
}

// errTransport is the transport failure returned by the failing HTTP client.
var errTransport = errors.New("transport failure")

// sent returns the requests received so far.
//
// Returns:
//   - []capturedRequest: the requests, in order.
func (r *requestRecorder) sent() []capturedRequest {
	r.mu.Lock()
	defer r.mu.Unlock()

	return append([]capturedRequest(nil), r.requests...)
}

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
		*captured = captureRequest(t, req)

		return &http.Response{
			Status:     status,
			StatusCode: statusCode,
			Body:       io.NopCloser(strings.NewReader("")),
		}, nil
	}).Once()

	return client, captured
}

// newRecordingServer returns a loopback test server that answers every
// request with 200 OK and records it.
//
// Parameters:
//   - t: the test that owns the server.
//
// Returns:
//   - *httptest.Server: the server, closed when the test ends.
//   - *requestRecorder: the recorded requests.
func newRecordingServer(t *testing.T) (*httptest.Server, *requestRecorder) {
	t.Helper()

	recorder := &requestRecorder{}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		body, _ := io.ReadAll(req.Body)

		recorder.mu.Lock()
		recorder.requests = append(recorder.requests, capturedRequest{
			method:      req.Method,
			url:         req.URL.String(),
			contentType: req.Header.Get("Content-Type"),
			body:        string(body),
		})
		recorder.mu.Unlock()

		w.WriteHeader(http.StatusOK)
	}))
	t.Cleanup(server.Close)

	return server, recorder
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

	assert.Equal(t, "mattermost", (&Service{}).GetID())
}

// TestInitialize verifies that Initialize parses the service URL and builds
// an HTTP client, and that it rejects a URL without a token.
func TestInitialize(t *testing.T) {
	t.Parallel()

	t.Run("valid URL", func(t *testing.T) {
		t.Parallel()

		service := newTestService(t, "mattermost://user@host:8080/token/channel?disabletls=yes&icon=tanabata_tree")

		assert.Equal(t, &Config{
			UserName:   "user",
			Icon:       "tanabata_tree",
			Channel:    "channel",
			Host:       "host:8080",
			Token:      "token",
			DisableTLS: true,
		}, service.Config)
	})

	t.Run("missing token", func(t *testing.T) {
		t.Parallel()

		service := &Service{}
		err := service.Initialize(parseTestURL(t, "mattermost://mattermost.my-domain.com"), log.New(t.Output(), "", 0))

		require.ErrorIs(t, err, ErrNotEnoughArguments)
	})
}

// TestSend verifies the request a send makes: a JSON POST to the hooks
// endpoint over https, or http when TLS is disabled, carrying the payload
// built from the config and params.
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
			name:     "no params",
			rawURL:   "mattermost://mattermost.host/token",
			wantURL:  "https://mattermost.host/hooks/token",
			wantBody: `{"text":"Message"}`,
		},
		{
			name:     "configured user name and channel",
			rawURL:   "mattermost://bot@mattermost.host/token/channel",
			wantURL:  "https://mattermost.host/hooks/token",
			wantBody: `{"text":"Message","username":"bot","channel":"channel"}`,
		},
		{
			name:     "disabletls and port",
			rawURL:   "mattermost://user@host:8080/token?disabletls=yes",
			wantURL:  "http://host:8080/hooks/token",
			wantBody: `{"text":"Message","username":"user"}`,
		},
		{
			name:     "user name and channel params",
			rawURL:   "mattermost://bot@mattermost.host/token/news",
			params:   &types.Params{"username": "override", "channel": "@alice"},
			wantURL:  "https://mattermost.host/hooks/token",
			wantBody: `{"text":"Message","username":"override","channel":"@alice"}`,
		},
		{
			name:     "disabletls param",
			rawURL:   "mattermost://host:8080/token",
			params:   &types.Params{"disabletls": "yes"},
			wantURL:  "http://host:8080/hooks/token",
			wantBody: `{"text":"Message"}`,
		},
		{
			name:     "title and icon params",
			rawURL:   "mattermost://mattermost.host/token?icon=tanabata_tree",
			params:   &types.Params{"title": "Title", "icon_url": "https://example.com/logo.png"},
			wantURL:  "https://mattermost.host/hooks/token",
			wantBody: `{"text":"Title\nMessage","icon_url":"https://example.com/logo.png"}`,
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
			assert.Equal(t, tt.wantURL, captured.url)
			assert.Equal(t, "application/json", captured.contentType)
			assert.JSONEq(t, tt.wantBody, captured.body)
		})
	}
}

// TestSendChecksConfig verifies that a send checks a config built in code as
// reading a URL does, so a config without a server host or a token fails
// before any request.
func TestSendChecksConfig(t *testing.T) {
	t.Parallel()

	// tests lists config changes with the error the send must return.
	tests := []struct {
		change  func(config *Config)
		wantErr error
		name    string
	}{
		{name: "zero config", change: func(config *Config) { *config = Config{} }, wantErr: ErrHostMissing},
		{name: "port without a host", change: func(config *Config) { config.Host = ":8065" }, wantErr: ErrHostMissing},
		{name: "no token", change: func(config *Config) { config.Token = "" }, wantErr: ErrNotEnoughArguments},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			service := newTestService(t, "mattermost://mattermost.example.com/token")
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

	service := newTestService(t, "mattermost://bot@mattermost.host/token/news?icon=tanabata_tree&title=Configured")
	want := *service.Config

	client, captured := newRespondingClient(t, "200 OK", http.StatusOK)
	service.SetHTTPClient(client)

	params := &types.Params{
		"title":      "Override",
		"icon":       "https://example.com/logo.png",
		"username":   "override",
		"channel":    "@alice",
		"disabletls": "yes",
	}
	require.NoError(t, service.Send("Message", params))
	assert.Equal(t, "http://mattermost.host/hooks/token", captured.url)
	assert.JSONEq(t, `{"text":"Override\nMessage","username":"override","channel":"@alice","icon_url":"https://example.com/logo.png"}`, captured.body)
	assert.Equal(t, want, *service.Config)

	client, captured = newRespondingClient(t, "200 OK", http.StatusOK)
	service.SetHTTPClient(client)

	require.NoError(t, service.Send("Message", nil))
	assert.Equal(t, "https://mattermost.host/hooks/token", captured.url)
	assert.JSONEq(t, `{"text":"Configured\nMessage","username":"bot","channel":"news","icon_emoji":"tanabata_tree"}`, captured.body)
}

// TestSendContextErrors verifies that a send fails with ErrSendFailed for an
// error status, wraps a transport error, rejects an unknown param before any
// request, and stops when the caller's context is canceled.
func TestSendContextErrors(t *testing.T) {
	t.Parallel()

	t.Run("error status", func(t *testing.T) {
		t.Parallel()

		service := newTestService(t, "mattermost://mattermost.host/token")
		client, _ := newRespondingClient(t, "403 Forbidden", http.StatusForbidden)
		service.SetHTTPClient(client)

		err := service.SendContext(context.Background(), "Message", nil)

		require.ErrorIs(t, err, ErrSendFailed)
		assert.Contains(t, err.Error(), "403 Forbidden")
	})

	t.Run("transport error", func(t *testing.T) {
		t.Parallel()

		service := newTestService(t, "mattermost://mattermost.host/token")
		client := mocks.NewMockHTTPClient(t)
		client.EXPECT().Do(mock.Anything).Return(nil, errTransport).Once()
		service.SetHTTPClient(client)

		require.ErrorIs(t, service.SendContext(context.Background(), "Message", nil), errTransport)
	})

	t.Run("unknown param", func(t *testing.T) {
		t.Parallel()

		service := newTestService(t, "mattermost://mockhost/mocktoken")
		service.SetHTTPClient(mocks.NewMockHTTPClient(t))

		err := service.SendContext(context.Background(), "Message", &types.Params{"foo": "bar"})

		require.ErrorIs(t, err, format.ErrInvalidConfigKey)
	})

	t.Run("canceled context", func(t *testing.T) {
		t.Parallel()

		ctx, cancel := context.WithCancel(context.Background())
		cancel()

		client := mocks.NewMockHTTPClient(t)
		client.EXPECT().Do(mock.Anything).RunAndReturn(func(req *http.Request) (*http.Response, error) {
			return nil, req.Context().Err()
		}).Once()

		service := newTestService(t, "mattermost://user@mattermost.example.invalid/token")
		service.SetHTTPClient(client)

		require.ErrorIs(t, service.SendContext(ctx, "message", nil), context.Canceled)
	})
}

// TestSendDefaultClient verifies that a send reaches a plain HTTP server
// through the default client, both when no client was set and when a nil
// *http.Client restores it.
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

			server, recorder := newRecordingServer(t)
			service := newTestService(t, "mattermost://"+server.Listener.Addr().String()+"/token?disabletls=yes")

			if tt.configure != nil {
				tt.configure(service)
			}

			require.NoError(t, service.Send("Message", nil))

			assert.Equal(t, []capturedRequest{{
				method:      http.MethodPost,
				url:         "/hooks/token",
				contentType: "application/json",
				body:        `{"text":"Message"}`,
			}}, recorder.sent())
		})
	}
}

// TestSetHTTPClient verifies that SetHTTPClient stores a custom client for
// later sends.
func TestSetHTTPClient(t *testing.T) {
	t.Parallel()

	client := mocks.NewMockHTTPClient(t)
	service := &Service{}
	service.SetHTTPClient(client)

	assert.Same(t, client, service.client.Client())
}

// TestSendWarnsPlaintext verifies that sends without TLS log the plaintext
// warning once, and again after the service is initialized again.
func TestSendWarnsPlaintext(t *testing.T) {
	t.Parallel()

	var logs bytes.Buffer

	rawURL := "mattermost://mattermost.example.com/token?disabletls=yes"

	client := mocks.NewMockHTTPClient(t)
	client.EXPECT().Do(mock.Anything).RunAndReturn(func(req *http.Request) (*http.Response, error) {
		return &http.Response{StatusCode: http.StatusOK, Body: http.NoBody, Request: req}, nil
	}).Times(3)

	service := &Service{}
	service.SetHTTPClient(client)
	require.NoError(t, service.Initialize(parseTestURL(t, rawURL), log.New(&logs, "", 0)))

	require.NoError(t, service.Send("first", nil))
	require.NoError(t, service.Send("second", nil))
	assert.Equal(t, 1, strings.Count(logs.String(), "WARN sending without TLS"))

	require.NoError(t, service.Initialize(parseTestURL(t, rawURL), log.New(&logs, "", 0)))
	require.NoError(t, service.Send("third", nil))
	assert.Equal(t, 2, strings.Count(logs.String(), "WARN sending without TLS"))
}

// TestSendSkipTLSVerify verifies sends to a server whose certificate cannot be
// verified. Without skiptlsverify the send fails. With it in the URL or the
// params, the send goes through the insecure default client and warns once
// over two sends. An injected client is used unchanged and never warned about.
func TestSendSkipTLSVerify(t *testing.T) {
	t.Parallel()

	const insecureWarning = "WARN skipping TLS certificate verification"

	// tests lists the URL query, the send params, and whether a client is
	// injected, with the expected outcome.
	tests := []struct {
		params       *types.Params
		name         string
		query        string
		injected     bool
		wantErr      bool
		wantWarnings int
	}{
		{name: "verified by default", wantErr: true},
		{name: "URL", query: "?skiptlsverify=yes", wantWarnings: 1},
		{name: "params", params: &types.Params{"skiptlsverify": "yes"}, wantWarnings: 1},
		{name: "injected client", query: "?skiptlsverify=yes", injected: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
				w.WriteHeader(http.StatusOK)
			}))
			t.Cleanup(server.Close)

			var logs strings.Builder

			service := &Service{}
			rawURL := "mattermost://" + server.Listener.Addr().String() + "/token" + tt.query
			require.NoError(t, service.Initialize(parseTestURL(t, rawURL), log.New(&logs, "", 0)))

			if tt.injected {
				service.SetHTTPClient(server.Client())
			}

			for range 2 {
				err := service.Send("Message", tt.params)
				require.Equal(t, tt.wantErr, err != nil, "send error: %v", err)
			}

			assert.Equal(t, tt.wantWarnings, strings.Count(logs.String(), insecureWarning))
		})
	}
}

// TestDefaultHTTPClient verifies that the default client has no request
// timeout, since the send budget bounds each send, and sends through
// http.DefaultTransport, so it honors the proxy environment variables.
func TestDefaultHTTPClient(t *testing.T) {
	t.Parallel()

	service := newTestService(t, "mattermost://mattermost.example.com/token")

	httpClient, ok := service.client.Client().(*http.Client)
	require.True(t, ok)
	assert.Zero(t, httpClient.Timeout)
	assert.Nil(t, httpClient.Transport)
}

// TestSetHTTPClientBeforeInitialize verifies that a client injected before
// Initialize is the one sends use.
func TestSetHTTPClientBeforeInitialize(t *testing.T) {
	t.Parallel()

	client, captured := newRespondingClient(t, "200 OK", http.StatusOK)
	service := &Service{}
	service.SetHTTPClient(client)
	require.NoError(t, service.Initialize(parseTestURL(t, "mattermost://mattermost.example.com/token"), log.New(t.Output(), "", 0)))

	require.NoError(t, service.Send("Message", nil))
	assert.Equal(t, http.MethodPost, captured.method)
}

// TestSendRedactsUnparsableURL verifies that a host that makes the webhook URL
// unparsable fails the send without the token in the error.
func TestSendRedactsUnparsableURL(t *testing.T) {
	t.Parallel()

	service := newTestService(t, "mattermost://mattermost.example.com/SECRETtoken")
	service.Config.Host = "bad host"
	// The mock has no expectations, so a request fails the test.
	service.SetHTTPClient(mocks.NewMockHTTPClient(t))

	err := service.Send("message", nil)

	_, isURLErr := errors.AsType[*url.Error](err)
	require.True(t, isURLErr, "error %v is not a *url.Error", err)
	assert.NotContains(t, err.Error(), "SECRETtoken")
}
