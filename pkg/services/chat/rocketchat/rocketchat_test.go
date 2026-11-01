package rocketchat

import (
	"context"
	"encoding/json/v2"
	"errors"
	"io"
	"log"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"testing/iotest"
	"testing/synctest"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	"github.com/nicholas-fedor/shoutrrr/pkg/format"
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
// without filling the deprecated client field, keeps a client set before it,
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
		assert.Nil(t, service.Client, "Initialize leaves the deprecated client field unset")
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

		assert.Same(t, client, service.client.Prefer(service.Client))
	})
}

// TestSendRequest verifies the method, URL, headers, and payload a send posts.
// Params can override the configured username and channel. A disabletls param
// sends over HTTP, and the reserved title param is accepted.
func TestSendRequest(t *testing.T) {
	t.Parallel()

	const httpsURL = "https://rocketchat.example.com:5055/hooks/tokenA/tokenB"

	// tests lists params with the URL and payload the send should post.
	tests := []struct {
		name     string
		params   *types.Params
		wantURL  string
		wantBody JSON
	}{
		{
			name:     "config values",
			wantURL:  httpsURL,
			wantBody: JSON{Text: "test message", UserName: "defaultUser", Channel: "#defaultChannel"},
		},
		{
			name: "params override username and channel",
			params: &types.Params{
				"username": "overrideUser",
				"channel":  "#overrideChannel",
			},
			wantURL:  httpsURL,
			wantBody: JSON{Text: "test message", UserName: "overrideUser", Channel: "#overrideChannel"},
		},
		{
			name:     "disabletls param sends over HTTP",
			params:   &types.Params{"disabletls": "yes", "username": "overrideUser"},
			wantURL:  "http://rocketchat.example.com:5055/hooks/tokenA/tokenB",
			wantBody: JSON{Text: "test message", UserName: "overrideUser", Channel: "#defaultChannel"},
		},
		{
			name:     "reserved title param accepted",
			params:   &types.Params{"title": "Ignored"},
			wantURL:  httpsURL,
			wantBody: JSON{Text: "test message", UserName: "defaultUser", Channel: "#defaultChannel"},
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
			assert.Equal(t, tt.wantURL, captured.url)
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
		assert.True(t, strings.HasPrefix(err.Error(), "posting to rocketchat.example.com:5055: "), err.Error())
		assert.NotContains(t, err.Error(), "\n")
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

// TestSendUnknownParam verifies that a send param the config does not declare
// fails the send before any request, as it does for other services.
func TestSendUnknownParam(t *testing.T) {
	t.Parallel()

	var captured capturedRequest

	service := newTestService(t, newCapturingClient(t, http.StatusOK, "", &captured))

	require.ErrorIs(t, service.Send("test message", &types.Params{"disabeltls": "yes"}), format.ErrInvalidConfigKey)
	assert.Empty(t, captured.url, "no request should be sent")
}

// TestSendChecksConfig verifies that a send checks a config built in code as
// reading a URL does. A config without a host, an integration ID, or a token
// fails before any request. So does a config with a slash in its channel.
func TestSendChecksConfig(t *testing.T) {
	t.Parallel()

	// tests lists config changes with the error the send must return.
	tests := []struct {
		change  func(config *Config)
		wantErr error
		name    string
	}{
		{name: "zero config", change: func(config *Config) { *config = Config{} }, wantErr: ErrHostMissing},
		{name: "no integration ID", change: func(config *Config) { config.TokenA = "" }, wantErr: ErrNotEnoughArguments},
		{name: "no token", change: func(config *Config) { config.TokenB = "" }, wantErr: ErrNotEnoughArguments},
		{name: "channel with a slash", change: func(config *Config) { config.Channel = "#team/general" }, wantErr: ErrInvalidChannel},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			var captured capturedRequest

			service := newTestService(t, newCapturingClient(t, http.StatusOK, "", &captured))
			tt.change(service.Config)

			require.ErrorIs(t, service.Send("test message", nil), tt.wantErr)
			assert.Empty(t, captured.url, "no request should be sent")
		})
	}
}

// TestSendPlaintextWarning verifies that sends over HTTP log the shared
// plaintext warning once. It covers a client injected through SetHTTPClient,
// because the URL decides the protocol. The warning appears again after the
// service is initialized again.
func TestSendPlaintextWarning(t *testing.T) {
	t.Parallel()

	var (
		captured capturedRequest
		logs     strings.Builder
	)

	service := &Service{}
	rawURL := "rocketchat://rocketchat.example.com/" + testTokenA + "/" + testTokenB + "?disabletls=yes"
	require.NoError(t, service.Initialize(parseURL(t, rawURL), log.New(&logs, "", 0)))
	service.SetHTTPClient(newCapturingClient(t, http.StatusOK, "", &captured))

	require.NoError(t, service.Send("test message", nil))
	require.NoError(t, service.Send("test message", nil))

	assert.Equal(t, "http://rocketchat.example.com/hooks/tokenA/tokenB", captured.url)
	assert.Equal(t, []string{"WARN sending without TLS"}, warningLines(logs.String()))

	require.NoError(t, service.Initialize(parseURL(t, rawURL), log.New(&logs, "", 0)))
	require.NoError(t, service.Send("test message", nil))

	assert.Equal(t, []string{"WARN sending without TLS", "WARN sending without TLS"}, warningLines(logs.String()))
}

// TestSendSkipTLSVerify verifies sends to a server whose certificate cannot be
// verified. Without skiptlsverify the send fails. With it in the URL or the
// params, the send goes through the insecure default client and warns once
// over two sends. A client set through the deprecated Client field is used
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
		{name: "verified by default", wantErr: true},
		{name: "URL", query: "?skiptlsverify=yes", wantWarnings: []string{insecureWarning}},
		{name: "params", params: &types.Params{"skiptlsverify": "yes"}, wantWarnings: []string{insecureWarning}},
		{name: "deprecated client field", query: "?skiptlsverify=yes", clientField: true},
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
			rawURL := "rocketchat://" + server.Listener.Addr().String() + "/" + testTokenA + "/" + testTokenB + tt.query
			require.NoError(t, service.Initialize(parseURL(t, rawURL), log.New(&logs, "", 0)))

			if tt.clientField {
				service.Client = server.Client()
			}

			for range 2 {
				err := service.Send("test message", tt.params)
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

// TestSetHTTPClient verifies that SetHTTPClient stores a client in place of
// the deprecated Client field, and that a nil client of either kind restores
// the default client after an *http.Client was set.
func TestSetHTTPClient(t *testing.T) {
	t.Parallel()

	t.Run("custom client clears the field", func(t *testing.T) {
		t.Parallel()

		custom := mocks.NewMockHTTPClient(t)
		service := &Service{Client: &http.Client{}}

		service.SetHTTPClient(custom)

		assert.Nil(t, service.Client, "SetHTTPClient clears the deprecated client field")
		assert.Same(t, custom, service.client.Prefer(service.Client))
	})

	// nilClients lists the nil clients that restore the default client.
	nilClients := []struct {
		client types.HTTPClient
		name   string
	}{
		{name: "nil interface", client: nil},
		{name: "nil *http.Client", client: (*http.Client)(nil)},
	}

	for _, tt := range nilClients {
		t.Run(tt.name+" restores the default client", func(t *testing.T) {
			t.Parallel()

			serviceURL, err := url.Parse("rocketchat://rocketchat.example.com/" + testTokenA + "/" + testTokenB)
			require.NoError(t, err)

			service := &Service{}
			require.NoError(t, service.Initialize(serviceURL, log.New(t.Output(), "", 0)))

			service.SetHTTPClient(&http.Client{Transport: roundTripFunc(func(*http.Request) (*http.Response, error) {
				t.Error("the replaced client was used")

				return nil, errTransport
			})})
			service.SetHTTPClient(tt.client)

			// The context ends before the send, so the default client returns
			// before it dials.
			ctx, cancel := context.WithCancel(t.Context())
			cancel()

			require.ErrorIs(t, service.SendContext(ctx, "message", nil), context.Canceled)
		})
	}
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

			return &http.Response{StatusCode: http.StatusOK, Body: http.NoBody, Request: req}, nil
		})

		service := newTestService(t, nil)
		service.SetHTTPClient(client)

		start := time.Now()

		require.NoError(t, service.Send("message", nil))
		assert.Equal(t, start.Add(service.ServiceTimeout(nil)), deadline, "a send without a deadline gets the budget")

		ctx, cancel := context.WithTimeout(t.Context(), time.Hour)
		defer cancel()

		require.NoError(t, service.SendContext(ctx, "message", nil))
		assert.Equal(t, start.Add(time.Hour), deadline, "the caller's deadline applies")
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
		{
			name:   "tokens with URL delimiters",
			config: &Config{Host: "rocketchat.example.com", TokenA: "a/b", TokenB: "c?d#e%f"},
			want:   "https://rocketchat.example.com/hooks/a%2Fb/c%3Fd%23e%25f",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			assert.Equal(t, tt.want, buildURL(tt.config))
		})
	}
}

// TestSendRedactsUnparsableURL verifies that a host that makes the webhook URL
// unparsable fails the send without either token in the error.
func TestSendRedactsUnparsableURL(t *testing.T) {
	t.Parallel()

	service := newTestService(t, nil)
	service.Config.Host = "bad host"
	service.Config.TokenA = "SECRETtokenA"
	service.Config.TokenB = "SECRETtokenB"
	// The mock has no expectations, so a request fails the test.
	service.SetHTTPClient(mocks.NewMockHTTPClient(t))

	err := service.Send("message", nil)

	_, isURLErr := errors.AsType[*url.Error](err)
	require.True(t, isURLErr, "error %v is not a *url.Error", err)
	assert.NotContains(t, err.Error(), "SECRETtokenA")
	assert.NotContains(t, err.Error(), "SECRETtokenB")
}
