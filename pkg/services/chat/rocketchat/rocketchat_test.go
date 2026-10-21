package rocketchat

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"log"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"testing/iotest"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	"github.com/nicholas-fedor/shoutrrr/pkg/types"
	"github.com/nicholas-fedor/shoutrrr/pkg/types/mocks"
)

// roundTripFunc adapts a function to http.RoundTripper.
type roundTripFunc func(*http.Request) (*http.Response, error)

// capturedRequest records the parts of a request the send tests check.
type capturedRequest struct {
	// method is the HTTP method.
	method string
	// url is the full request URL.
	url string
	// contentType is the Content-Type header.
	contentType string
	// body is the request body.
	body string
}

// Test token values shared by the service and config tests.
const (
	// testTokenA is the first webhook token.
	testTokenA = "tokenA"
	// testTokenB is the second webhook token.
	testTokenB = "tokenB"
)

// Errors the fake HTTP clients and response bodies return.
var (
	// errTransport is the transport failure the fake HTTP clients return.
	errTransport = errors.New("transport failed")
	// errBodyRead is the failure the fake response bodies return when read.
	errBodyRead = errors.New("body read failed")
)

// RoundTrip calls f with the request.
//
// Parameters:
//   - req: the outgoing request.
//
// Returns:
//   - *http.Response: the response f returns.
//   - error: the error f returns.
func (f roundTripFunc) RoundTrip(req *http.Request) (*http.Response, error) {
	return f(req)
}

// newCapturingClient returns an HTTP client that records each request into
// captured and answers it with status and body.
//
// Parameters:
//   - t: the test.
//   - status: the response status code.
//   - body: the response body.
//   - captured: where the request is recorded.
//
// Returns:
//   - *http.Client: the client.
func newCapturingClient(t *testing.T, status int, body string, captured *capturedRequest) *http.Client {
	t.Helper()

	return &http.Client{Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
		reqBody, err := io.ReadAll(req.Body)
		if err != nil {
			return nil, err
		}

		*captured = capturedRequest{
			method:      req.Method,
			url:         req.URL.String(),
			contentType: req.Header.Get("Content-Type"),
			body:        string(reqBody),
		}

		return &http.Response{
			StatusCode: status,
			Body:       io.NopCloser(strings.NewReader(body)),
			Request:    req,
		}, nil
	})}
}

// newTestService returns a service for host rocketchat.example.com:5055 with
// the test tokens, sending through client and logging to the test output.
//
// Parameters:
//   - t: the test.
//   - client: the internal HTTP client.
//
// Returns:
//   - *Service: the service.
func newTestService(t *testing.T, client *http.Client) *Service {
	t.Helper()

	service := &Service{
		Config: &Config{
			Host:   "rocketchat.example.com",
			Port:   "5055",
			TokenA: testTokenA,
			TokenB: testTokenB,
		},
		Client: client,
	}
	service.SetLogger(log.New(t.Output(), "", 0))

	return service
}

// TestGetID verifies that the service reports its scheme as its ID.
func TestGetID(t *testing.T) {
	t.Parallel()

	assert.Equal(t, Scheme, (&Service{}).GetID())
}

// TestInitialize verifies that Initialize parses a full URL into the config
// and rejects a URL without tokens.
func TestInitialize(t *testing.T) {
	t.Parallel()

	t.Run("valid URL", func(t *testing.T) {
		t.Parallel()

		serviceURL, err := url.Parse(
			"rocketchat://testUser@rocketchat.my-domain.com:5055/" + testTokenA + "/" + testTokenB + "/#testChannel",
		)
		require.NoError(t, err)

		service := &Service{}
		require.NoError(t, service.Initialize(serviceURL, log.New(t.Output(), "", 0)))

		require.NotNil(t, service.Config)
		assert.NotNil(t, service.Client)
		assert.Equal(t, &Config{
			Host:     "rocketchat.my-domain.com",
			Port:     "5055",
			UserName: "testUser",
			TokenA:   testTokenA,
			TokenB:   testTokenB,
			Channel:  "#testChannel",
		}, service.Config)
	})

	t.Run("missing tokens", func(t *testing.T) {
		t.Parallel()

		serviceURL, err := url.Parse("rocketchat://rocketchat.my-domain.com")
		require.NoError(t, err)

		service := &Service{}
		err = service.Initialize(serviceURL, log.New(t.Output(), "", 0))
		require.ErrorIs(t, err, ErrNotEnoughArguments)
	})

	t.Run("keeps a preset client", func(t *testing.T) {
		t.Parallel()

		serviceURL, err := url.Parse("rocketchat://rocketchat.my-domain.com/" + testTokenA + "/" + testTokenB)
		require.NoError(t, err)

		client := &http.Client{}
		service := &Service{Client: client}
		require.NoError(t, service.Initialize(serviceURL, log.New(t.Output(), "", 0)))

		assert.Same(t, client, service.Client)
	})
}

// TestSendRequest verifies the method, URL, headers, and payload a send posts,
// with and without params overriding the configured username and channel.
func TestSendRequest(t *testing.T) {
	t.Parallel()

	// tests lists params with the payload the send should post.
	tests := []struct {
		name     string
		params   *types.Params
		wantBody JSON
	}{
		{
			name:     "config values",
			wantBody: JSON{Text: "test message", UserName: "defaultUser", Channel: "#defaultChannel"},
		},
		{
			name: "params override username and channel",
			params: &types.Params{
				"username": "overrideUser",
				"channel":  "#overrideChannel",
			},
			wantBody: JSON{Text: "test message", UserName: "overrideUser", Channel: "#overrideChannel"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			var captured capturedRequest

			service := newTestService(t, newCapturingClient(t, http.StatusOK, "", &captured))
			service.Config.UserName = "defaultUser"
			service.Config.Channel = "#defaultChannel"

			require.NoError(t, service.Send("test message", tt.params))

			assert.Equal(t, http.MethodPost, captured.method)
			assert.Equal(t, "https://rocketchat.example.com:5055/hooks/tokenA/tokenB", captured.url)
			assert.Equal(t, "application/json", captured.contentType)

			var got JSON
			require.NoError(t, json.Unmarshal([]byte(captured.body), &got))
			assert.Equal(t, tt.wantBody, got)
		})
	}
}

// TestSendErrors verifies the errors a send returns for a rejected request, an
// unreadable error response, a transport failure, and an invalid host.
func TestSendErrors(t *testing.T) {
	t.Parallel()

	t.Run("non-OK status includes code and body", func(t *testing.T) {
		t.Parallel()

		var captured capturedRequest

		service := newTestService(t, newCapturingClient(t, http.StatusBadRequest, "bad request", &captured))

		err := service.Send("test message", nil)
		require.ErrorIs(t, err, ErrNotificationFailed)
		assert.EqualError(t, err, "notification failed: 400 bad request")
	})

	t.Run("unreadable error body reports the status code", func(t *testing.T) {
		t.Parallel()

		service := newTestService(t, &http.Client{Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
			return &http.Response{
				StatusCode: http.StatusInternalServerError,
				Body:       io.NopCloser(iotest.ErrReader(errBodyRead)),
				Request:    req,
			}, nil
		})})

		err := service.Send("test message", nil)
		require.ErrorIs(t, err, ErrNotificationFailed)
		assert.EqualError(t, err, "notification failed: 500")
	})

	t.Run("transport failure", func(t *testing.T) {
		t.Parallel()

		client := mocks.NewMockHTTPClient(t)
		client.EXPECT().Do(mock.Anything).Return(nil, errTransport).Once()

		service := newTestService(t, &http.Client{})
		service.SetHTTPClient(client)

		err := service.Send("test message", nil)
		require.ErrorIs(t, err, errTransport)
		assert.Contains(t, err.Error(), "posting to URL")
		assert.Contains(t, err.Error(), "HOST: rocketchat.example.com")
		assert.Contains(t, err.Error(), "PORT: 5055")
	})

	t.Run("invalid host fails before sending", func(t *testing.T) {
		t.Parallel()

		client := mocks.NewMockHTTPClient(t)

		service := newTestService(t, &http.Client{})
		service.Config.Host = "bad host"
		service.SetHTTPClient(client)

		err := service.Send("test message", nil)
		_, ok := errors.AsType[*url.Error](err)
		require.True(t, ok, "expected a *url.Error, got %v", err)
		assert.True(t, strings.HasPrefix(err.Error(), "creating request: "), err.Error())
	})
}

// TestSendContextCanceled verifies that a canceled caller context stops the
// request and its error matches context.Canceled.
func TestSendContextCanceled(t *testing.T) {
	t.Parallel()

	ctx, cancel := context.WithCancel(t.Context())
	cancel()

	client := mocks.NewMockHTTPClient(t)
	client.EXPECT().Do(mock.Anything).RunAndReturn(func(req *http.Request) (*http.Response, error) {
		return nil, req.Context().Err()
	}).Once()

	serviceURL, err := url.Parse("rocketchat://rocketchat.example.invalid/token/channel")
	require.NoError(t, err)

	service := &Service{}
	require.NoError(t, service.Initialize(serviceURL, log.New(t.Output(), "", 0)))
	service.SetHTTPClient(client)

	require.ErrorIs(t, service.SendContext(ctx, "message", nil), context.Canceled)
}

// TestSendTLSServer verifies a send end to end against an in-process TLS
// server on loopback, for an accepted and a rejected request.
func TestSendTLSServer(t *testing.T) {
	t.Parallel()

	// tests lists server responses with the error the send should return.
	tests := []struct {
		name    string
		status  int
		body    string
		wantErr string
	}{
		{name: "accepted", status: http.StatusOK},
		{name: "rejected", status: http.StatusBadRequest, body: "bad request", wantErr: "notification failed: 400 bad request"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			var gotPath string

			server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				gotPath = r.URL.Path

				w.WriteHeader(tt.status)
				_, _ = w.Write([]byte(tt.body))
			}))
			t.Cleanup(server.Close)

			serverURL, err := url.Parse(server.URL)
			require.NoError(t, err)

			service := newTestService(t, server.Client())
			service.Config.Host = serverURL.Hostname()
			service.Config.Port = serverURL.Port()

			err = service.Send("test message", nil)
			if tt.wantErr == "" {
				require.NoError(t, err)
			} else {
				require.ErrorIs(t, err, ErrNotificationFailed)
				require.EqualError(t, err, tt.wantErr)
			}

			assert.Equal(t, "/hooks/tokenA/tokenB", gotPath)
		})
	}
}

// TestSetHTTPClient verifies which client a service uses after SetHTTPClient
// receives a typed nil, a nil interface, an *http.Client, or another client.
func TestSetHTTPClient(t *testing.T) {
	t.Parallel()

	t.Run("typed nil selects the internal client", func(t *testing.T) {
		t.Parallel()

		internal := &http.Client{}
		service := &Service{Client: internal}

		var nilClient *http.Client
		service.SetHTTPClient(nilClient)

		assert.Nil(t, service.httpClient)
		assert.Same(t, internal, service.httpClientOrDefault())
	})

	t.Run("nil interface selects the internal client", func(t *testing.T) {
		t.Parallel()

		internal := &http.Client{}
		service := &Service{Client: internal, httpClient: mocks.NewMockHTTPClient(t)}

		service.SetHTTPClient(nil)

		assert.Nil(t, service.httpClient)
		assert.Same(t, internal, service.httpClientOrDefault())
	})

	t.Run("http client replaces the internal client", func(t *testing.T) {
		t.Parallel()

		custom := &http.Client{}
		service := &Service{Client: &http.Client{}}

		service.SetHTTPClient(custom)

		assert.Same(t, custom, service.Client)
		assert.Same(t, custom, service.httpClientOrDefault())
	})

	t.Run("other client keeps the internal client", func(t *testing.T) {
		t.Parallel()

		internal := &http.Client{}
		custom := mocks.NewMockHTTPClient(t)
		service := &Service{Client: internal}

		service.SetHTTPClient(custom)

		assert.Same(t, internal, service.Client)
		assert.Same(t, custom, service.httpClientOrDefault())
	})
}

// TestBuildURL verifies the webhook URL built from the host, optional port,
// and tokens.
func TestBuildURL(t *testing.T) {
	t.Parallel()

	// tests lists configs with the webhook URL they should produce.
	tests := []struct {
		name   string
		config *Config
		want   string
	}{
		{
			name:   "without port",
			config: &Config{Host: "rocketchat.example.com", TokenA: "a", TokenB: "b"},
			want:   "https://rocketchat.example.com/hooks/a/b",
		},
		{
			name:   "with port",
			config: &Config{Host: "rocketchat.example.com", Port: "5055", TokenA: "a", TokenB: "b"},
			want:   "https://rocketchat.example.com:5055/hooks/a/b",
		},
		{
			name:   "IPv6 host without port",
			config: &Config{Host: "::1", TokenA: "a", TokenB: "b"},
			want:   "https://[::1]/hooks/a/b",
		},
		{
			name:   "IPv6 host with a zone",
			config: &Config{Host: "fe80::1%en0", Port: "8443", TokenA: "a", TokenB: "b"},
			want:   "https://[fe80::1%25en0]:8443/hooks/a/b",
		},
		{
			name:   "IPv6 host with port",
			config: &Config{Host: "::1", Port: "8443", TokenA: "a", TokenB: "b"},
			want:   "https://[::1]:8443/hooks/a/b",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			assert.Equal(t, tt.want, buildURL(tt.config))
		})
	}
}

// TestSendRedactsUnparsableURL verifies that tokens that make the webhook URL
// unparsable fail the send without either token in the error.
func TestSendRedactsUnparsableURL(t *testing.T) {
	t.Parallel()

	serviceURL, err := url.Parse("rocketchat://rocketchat.example.com/SECRETtokenA%25zz/SECRETtokenB")
	require.NoError(t, err)

	service := &Service{}
	require.NoError(t, service.Initialize(serviceURL, log.New(t.Output(), "", 0)))
	// The mock has no expectations, so a request fails the test.
	service.SetHTTPClient(mocks.NewMockHTTPClient(t))

	err = service.Send("message", nil)

	_, isURLErr := errors.AsType[*url.Error](err)
	require.True(t, isURLErr, "error %v is not a *url.Error", err)
	assert.NotContains(t, err.Error(), "SECRETtokenA")
	assert.NotContains(t, err.Error(), "SECRETtokenB")
}
