package gotify

import (
	"bufio"
	"bytes"
	"context"
	"crypto/tls"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"net"
	"net/http"
	"net/http/httptest"
	"net/url"
	"slices"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"testing/synctest"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	"github.com/nicholas-fedor/shoutrrr/pkg/format"
	"github.com/nicholas-fedor/shoutrrr/pkg/types"
	typesmocks "github.com/nicholas-fedor/shoutrrr/pkg/types/mocks"
)

// capturedRequest records the parts of a request that an HTTP client received.
type capturedRequest struct {
	// method is the request method.
	method string
	// url is the full request URL.
	url string
	// contentType is the request's Content-Type header.
	contentType string
	// gotifyKey is the request's X-Gotify-Key header.
	gotifyKey string
	// body is the request body.
	body string
}

// cannedResponse is what the recording client returns for one request.
type cannedResponse struct {
	// err is a transport error returned instead of a response.
	err error
	// body is the response body.
	body string
	// statusCode is the response status code.
	statusCode int
}

// fixedTransportManager creates default clients that send through a fixed
// transport.
type fixedTransportManager struct {
	// transport is the transport every client sends through.
	transport *http.Transport
}

// failingURLBuilder fails to build every message endpoint.
type failingURLBuilder struct {
	// err is the error BuildURL returns.
	err error
}

// Values shared by the service tests.
const (
	// sentBody is Gotify's response to a created message.
	sentBody = `{"id":25,"appid":5,"message":"Hello","title":"Shoutrrr notification","priority":0,"date":"2023-12-25T10:00:00Z"}`
	// unauthorizedBody is Gotify's response to an invalid token.
	unauthorizedBody = `{"error":"Unauthorized","errorCode":401,"errorDescription":"you need to provide a valid access token or user credentials to access this api"}`
	// testEndpoint is the message endpoint for testServiceURL.
	testEndpoint = "https://" + testHost + "/message?token=" + testToken
)

// Values shared by the service tests.
var (
	// errTransport is the transport failure returned by a failing client.
	errTransport = errors.New("transport failure")
	// sent is the response to a created message.
	sent = cannedResponse{statusCode: http.StatusOK, body: sentBody}
)

// newRecordingClient returns a mock HTTP client that expects one request per
// response, records each request, and answers them in order.
//
// Parameters:
//   - t: the test that owns the mock.
//   - responses: the responses to return, in request order.
//
// Returns:
//   - *typesmocks.MockHTTPClient: the client.
//   - func() []capturedRequest: returns the requests received so far.
func newRecordingClient(t *testing.T, responses ...cannedResponse) (*typesmocks.MockHTTPClient, func() []capturedRequest) {
	t.Helper()

	var (
		mu       sync.Mutex
		captured []capturedRequest
	)

	client := typesmocks.NewMockHTTPClient(t)
	client.EXPECT().Do(mock.Anything).RunAndReturn(func(req *http.Request) (*http.Response, error) {
		body, err := io.ReadAll(req.Body)
		require.NoError(t, err)

		mu.Lock()
		defer mu.Unlock()

		captured = append(captured, capturedRequest{
			method:      req.Method,
			url:         req.URL.String(),
			contentType: req.Header.Get("Content-Type"),
			gotifyKey:   req.Header.Get("X-Gotify-Key"),
			body:        string(body),
		})

		response := responses[len(captured)-1]
		if response.err != nil {
			return nil, &url.Error{Op: "Post", URL: req.URL.String(), Err: response.err}
		}

		return &http.Response{
			StatusCode: response.statusCode,
			Status:     fmt.Sprintf("%d %s", response.statusCode, http.StatusText(response.statusCode)),
			Body:       io.NopCloser(strings.NewReader(response.body)),
		}, nil
	}).Times(len(responses))

	return client, func() []capturedRequest {
		mu.Lock()
		defer mu.Unlock()

		return slices.Clone(captured)
	}
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

// assertPayload asserts that body is the JSON message payload want. The
// comparison ignores the priority field unless want sets it.
//
// Parameters:
//   - t: the test.
//   - want: the expected JSON payload.
//   - body: the request body.
func assertPayload(t *testing.T, want, body string) {
	t.Helper()

	var wantFields, gotFields map[string]any

	require.NoError(t, json.Unmarshal([]byte(want), &wantFields))
	require.NoError(t, json.Unmarshal([]byte(body), &gotFields))

	if _, ok := wantFields["priority"]; !ok {
		delete(gotFields, "priority")
	}

	assert.Equal(t, wantFields, gotFields)
}

// TestGetID verifies that the service identifies itself by its scheme.
func TestGetID(t *testing.T) {
	t.Parallel()

	assert.Equal(t, "gotify", (&Service{}).GetID())
}

// TestServiceTimeout verifies that the router gives Gotify the default send
// budget.
func TestServiceTimeout(t *testing.T) {
	t.Parallel()

	assert.Equal(t, types.DefaultSendTimeout, (&Service{}).ServiceTimeout(nil))
}

// TestInitialize verifies that Initialize parses the service URL over the
// default title and sets up the default helpers and client.
func TestInitialize(t *testing.T) {
	t.Parallel()

	service := newTestService(t, testServiceURL+"?priority=3")

	assert.Equal(t, &Config{Host: testHost, Token: testToken, Title: "Shoutrrr notification", Priority: 3}, service.Config)
	assert.IsType(t, &DefaultHTTPClientManager{}, service.httpClientManager)
	assert.IsType(t, &DefaultURLBuilder{}, service.urlBuilder)
	assert.IsType(t, &DefaultPayloadBuilder{}, service.payloadBuilder)
	assert.IsType(t, &DefaultValidator{}, service.validator)
	assert.IsType(t, &DefaultSender{}, service.sender)
	assert.IsType(t, &http.Client{}, service.httpClient)

	err := (&Service{}).Initialize(parseTestURL(t, testServiceURL+"?extras=%7Binvalid"), log.New(t.Output(), "", 0))
	require.ErrorIs(t, err, ErrExtrasParseFailed)
}

// TestSend verifies the request a send makes: a JSON POST to the message
// endpoint with the token in the query or a header, carrying the message,
// title, priority, extras, and date from the URL and params.
func TestSend(t *testing.T) {
	t.Parallel()

	// tests lists sends with the request each should produce.
	tests := []struct {
		params   *types.Params
		name     string
		rawURL   string
		message  string
		wantURL  string
		wantKey  string
		wantBody string
	}{
		{
			name:     "message only",
			rawURL:   testServiceURL,
			message:  "Hello",
			wantURL:  testEndpoint,
			wantBody: `{"message":"Hello","title":"Shoutrrr notification","extras":null}`,
		},
		{
			name:     "title and priority params",
			rawURL:   testServiceURL,
			params:   &types.Params{"title": "Backup", "priority": "5"},
			message:  "Hello",
			wantURL:  testEndpoint,
			wantBody: `{"message":"Hello","title":"Backup","priority":5,"extras":null}`,
		},
		{
			name:     "priority in the URL",
			rawURL:   testServiceURL + "?priority=-2",
			message:  "Hello",
			wantURL:  testEndpoint,
			wantBody: `{"message":"Hello","title":"Shoutrrr notification","priority":-2,"extras":null}`,
		},
		{
			name:     "extras param",
			rawURL:   testServiceURL,
			params:   &types.Params{"extras": `{"client::display":{"contentType":"text/markdown"}}`},
			message:  "**Backup** done",
			wantURL:  testEndpoint,
			wantBody: `{"message":"**Backup** done","title":"Shoutrrr notification","extras":{"client::display":{"contentType":"text/markdown"}}}`,
		},
		{
			name:     "extras in the URL",
			rawURL:   testServiceURL + "?extras=" + url.QueryEscape(`{"client::display":{"contentType":"text/plain"}}`),
			message:  "Hello",
			wantURL:  testEndpoint,
			wantBody: `{"message":"Hello","title":"Shoutrrr notification","extras":{"client::display":{"contentType":"text/plain"}}}`,
		},
		{
			name:     "extras param over the URL extras",
			rawURL:   testServiceURL + "?extras=" + url.QueryEscape(`{"client::display":{"contentType":"text/plain"}}`),
			params:   &types.Params{"extras": `{"client::display":{"contentType":"text/markdown"}}`},
			message:  "Hello",
			wantURL:  testEndpoint,
			wantBody: `{"message":"Hello","title":"Shoutrrr notification","extras":{"client::display":{"contentType":"text/markdown"}}}`,
		},
		{
			name:     "invalid extras param",
			rawURL:   testServiceURL + "?extras=" + url.QueryEscape(`{"client::display":{"contentType":"text/plain"}}`),
			params:   &types.Params{"extras": "{invalid"},
			message:  "Hello",
			wantURL:  testEndpoint,
			wantBody: `{"message":"Hello","title":"Shoutrrr notification","extras":{"client::display":{"contentType":"text/plain"}}}`,
		},
		{
			name:     "date param",
			rawURL:   testServiceURL,
			params:   &types.Params{"date": "2023-12-25T10:00:00+02:00"},
			message:  "Hello",
			wantURL:  testEndpoint,
			wantBody: `{"message":"Hello","title":"Shoutrrr notification","date":"2023-12-25T08:00:00Z","extras":null}`,
		},
		{
			name:     "invalid date param",
			rawURL:   testServiceURL,
			params:   &types.Params{"date": "tomorrow"},
			message:  "Hello",
			wantURL:  testEndpoint,
			wantBody: `{"message":"Hello","title":"Shoutrrr notification","extras":null}`,
		},
		{
			name:     "special characters",
			rawURL:   testServiceURL,
			message:  "Disk <90%> & \"temp\" ✓\nline two",
			wantURL:  testEndpoint,
			wantBody: `{"message":"Disk <90%> & \"temp\" ✓\nline two","title":"Shoutrrr notification","extras":null}`,
		},
		{
			name:     "token in a header",
			rawURL:   testServiceURL + "?useheader=yes",
			message:  "Hello",
			wantURL:  "https://" + testHost + "/message",
			wantKey:  testToken,
			wantBody: `{"message":"Hello","title":"Shoutrrr notification","extras":null}`,
		},
		{
			name:     "TLS disabled",
			rawURL:   testServiceURL + "?disabletls=yes",
			message:  "Hello",
			wantURL:  "http://" + testHost + "/message?token=" + testToken,
			wantBody: `{"message":"Hello","title":"Shoutrrr notification","extras":null}`,
		},
		{
			name:     "Gotify 3 token",
			rawURL:   "gotify://" + testHost + "/" + enhancedToken,
			message:  "Hello",
			wantURL:  "https://" + testHost + "/message?token=" + enhancedToken,
			wantBody: `{"message":"Hello","title":"Shoutrrr notification","extras":null}`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			service := newTestService(t, tt.rawURL)
			client, captured := newRecordingClient(t, sent)
			service.SetHTTPClient(client)

			require.NoError(t, service.Send(tt.message, tt.params))

			requests := captured()
			require.Len(t, requests, 1)
			assert.Equal(t, http.MethodPost, requests[0].method)
			assert.Equal(t, tt.wantURL, requests[0].url)
			assert.Equal(t, "application/json", requests[0].contentType)
			assert.Equal(t, tt.wantKey, requests[0].gotifyKey)
			assertPayload(t, tt.wantBody, requests[0].body)
		})
	}
}

// TestSendLogsDroppedParams verifies that a send logs an invalid date or
// extras param that it leaves out.
func TestSendLogsDroppedParams(t *testing.T) {
	t.Parallel()

	var logs bytes.Buffer

	service := &Service{}
	require.NoError(t, service.Initialize(parseTestURL(t, testServiceURL), log.New(&logs, "", 0)))

	client, _ := newRecordingClient(t, sent)
	service.SetHTTPClient(client)

	require.NoError(t, service.Send("Hello", &types.Params{"date": "tomorrow", "extras": "{invalid"}))
	assert.Equal(t,
		"Invalid date format: invalid date format\nFailed to parse extras from params: failed to unmarshal extras JSON\n",
		logs.String())
}

// TestSendValidation verifies that a send rejects an empty message, an
// invalid token, an out of range priority, and an unknown param before any
// request.
func TestSendValidation(t *testing.T) {
	t.Parallel()

	// tests lists sends with the error each should produce.
	tests := []struct {
		wantErr error
		params  *types.Params
		name    string
		rawURL  string
		message string
	}{
		{name: "empty message", rawURL: testServiceURL, wantErr: ErrEmptyMessage},
		{name: "invalid token", rawURL: "gotify://" + testHost + "/Aaa", message: "Hello", wantErr: ErrInvalidToken},
		{name: "no token", rawURL: "gotify://" + testHost, message: "Hello", wantErr: ErrInvalidToken},
		{name: "priority too high", rawURL: testServiceURL, params: &types.Params{"priority": "11"}, message: "Hello", wantErr: ErrInvalidPriority},
		{name: "priority too low in the URL", rawURL: testServiceURL + "?priority=-3", message: "Hello", wantErr: ErrInvalidPriority},
		{name: "unknown param", rawURL: testServiceURL, params: &types.Params{"unknown": "value"}, message: "Hello", wantErr: format.ErrInvalidConfigKey},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			service := newTestService(t, tt.rawURL)
			// The mock has no expectations, so a request fails the test.
			service.SetHTTPClient(typesmocks.NewMockHTTPClient(t))

			require.ErrorIs(t, service.Send(tt.message, tt.params), tt.wantErr)
		})
	}
}

// TestSendWithoutConfig verifies that a send on a service whose config was
// cleared fails before any request.
func TestSendWithoutConfig(t *testing.T) {
	t.Parallel()

	service := newTestService(t, testServiceURL)
	// The mock has no expectations, so a request fails the test.
	service.SetHTTPClient(typesmocks.NewMockHTTPClient(t))
	service.Config = nil

	require.ErrorIs(t, service.Send("Hello", nil), ErrServiceNotInitialized)
}

// TestSendErrors verifies that a failed request fails the send with
// [ErrSendFailed], keeping the server's error and the token out of the error.
func TestSendErrors(t *testing.T) {
	t.Parallel()

	// tests lists URLs and responses with the error each should match.
	tests := []struct {
		wantErr   error
		name      string
		rawURL    string
		response  cannedResponse
		wantInErr string
	}{
		{
			name:      "error response",
			rawURL:    testServiceURL,
			response:  cannedResponse{statusCode: http.StatusUnauthorized, body: unauthorizedBody},
			wantInErr: "server responded with Unauthorized (401)",
		},
		{
			name:      "error response with the token in a header",
			rawURL:    testServiceURL + "?useheader=yes",
			response:  cannedResponse{statusCode: http.StatusUnauthorized, body: unauthorizedBody},
			wantInErr: "server responded with Unauthorized (401)",
		},
		{name: "transport error", rawURL: testServiceURL, response: cannedResponse{err: errTransport}, wantErr: errTransport},
		{name: "transport error with the token in a header", rawURL: testServiceURL + "?useheader=yes", response: cannedResponse{err: errTransport}, wantErr: errTransport},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			service := newTestService(t, tt.rawURL)
			client, _ := newRecordingClient(t, tt.response)
			service.SetHTTPClient(client)

			err := service.Send("Hello", nil)

			require.ErrorIs(t, err, ErrSendFailed)

			if tt.wantErr != nil {
				require.ErrorIs(t, err, tt.wantErr)
			}

			assert.Contains(t, err.Error(), tt.wantInErr)
			assert.NotContains(t, err.Error(), testToken)
		})
	}
}

// TestSendLeavesConfigUnchanged verifies that params apply to one send only
// and leave the caller's params unchanged.
func TestSendLeavesConfigUnchanged(t *testing.T) {
	t.Parallel()

	service := newTestService(t, testServiceURL)
	client, captured := newRecordingClient(t, sent, sent)
	service.SetHTTPClient(client)

	want := *service.Config
	params := types.Params{"title": "Backup", "priority": "5", "extras": `{"a":1}`, "useheader": "yes"}

	require.NoError(t, service.Send("Hello", &params))
	require.NoError(t, service.Send("Hello", nil))

	assert.Equal(t, want, *service.Config)
	assert.Equal(t, types.Params{"title": "Backup", "priority": "5", "extras": `{"a":1}`, "useheader": "yes"}, params)

	requests := captured()
	require.Len(t, requests, 2)
	assert.Equal(t, testEndpoint, requests[1].url)
	assertPayload(t, `{"message":"Hello","title":"Shoutrrr notification","extras":null}`, requests[1].body)
}

// TestSendContextCancellation verifies that the caller's context bounds the
// request, with the token in the query or a header.
func TestSendContextCancellation(t *testing.T) {
	t.Parallel()

	for _, rawURL := range []string{testServiceURL, testServiceURL + "?useheader=yes"} {
		t.Run(rawURL, func(t *testing.T) {
			t.Parallel()

			ctx, cancel := context.WithCancel(t.Context())
			cancel()

			client := typesmocks.NewMockHTTPClient(t)
			client.EXPECT().Do(mock.Anything).RunAndReturn(func(req *http.Request) (*http.Response, error) {
				return nil, req.Context().Err()
			}).Once()

			service := newTestService(t, rawURL)
			service.SetHTTPClient(client)

			require.ErrorIs(t, service.SendContext(ctx, "Hello", nil), context.Canceled)
		})
	}
}

// TestSendConcurrently verifies that concurrent sends through one service
// each make their request.
func TestSendConcurrently(t *testing.T) {
	t.Parallel()

	const sends = 8

	responses := make([]cannedResponse, sends)
	for i := range responses {
		responses[i] = sent
	}

	service := newTestService(t, testServiceURL)
	client, captured := newRecordingClient(t, responses...)
	service.SetHTTPClient(client)

	var wg sync.WaitGroup
	for i := range sends {
		wg.Go(func() {
			assert.NoError(t, service.Send(fmt.Sprintf("Hello %d", i), nil))
		})
	}

	wg.Wait()

	assert.Len(t, captured(), sends)
}

// TestSetHTTPClient verifies that SetHTTPClient stores a custom client and
// clears it for nil and a nil *http.Client, so the next send creates the
// default client.
func TestSetHTTPClient(t *testing.T) {
	t.Parallel()

	// tests lists constructors for the clients to set, which build any mock
	// with the subtest's t, and whether the stored client should be cleared.
	tests := []struct {
		client    func(t *testing.T) types.HTTPClient
		name      string
		wantClear bool
	}{
		{name: "custom client", client: func(t *testing.T) types.HTTPClient {
			t.Helper()

			return typesmocks.NewMockHTTPClient(t)
		}},
		{name: "nil", client: func(*testing.T) types.HTTPClient { return nil }, wantClear: true},
		{name: "nil *http.Client", client: func(*testing.T) types.HTTPClient { return (*http.Client)(nil) }, wantClear: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			client := tt.client(t)
			service := newTestService(t, testServiceURL)
			service.SetHTTPClient(client)

			if tt.wantClear {
				assert.Nil(t, service.httpClient)
				assert.Nil(t, service.client)

				return
			}

			assert.Same(t, client, service.httpClient)
			assert.NotNil(t, service.client)
		})
	}
}

// TestSendDefaultClient verifies that a send without a custom client, or
// after a nil client is set, uses a default client that honors the caller's
// context and keeps the token out of its errors. The context is canceled
// before the send, so the transport returns before it dials.
func TestSendDefaultClient(t *testing.T) {
	t.Parallel()

	// tests lists how each case sets up the service's HTTP client.
	tests := []struct {
		configure func(*Service)
		name      string
	}{
		{name: "no client set", configure: func(*Service) {}},
		{name: "nil client", configure: func(s *Service) { s.SetHTTPClient(nil) }},
		{name: "nil *http.Client", configure: func(s *Service) { s.SetHTTPClient((*http.Client)(nil)) }},
	}

	for _, tt := range tests {
		for _, rawURL := range []string{testServiceURL, testServiceURL + "?useheader=yes"} {
			t.Run(tt.name+" "+rawURL, func(t *testing.T) {
				t.Parallel()

				service := newTestService(t, rawURL)
				tt.configure(service)

				ctx, cancel := context.WithCancel(t.Context())
				cancel()

				err := service.SendContext(ctx, "Hello", nil)

				require.ErrorIs(t, err, context.Canceled)
				assert.NotContains(t, err.Error(), testToken)
			})
		}
	}
}

// newGotifyServer returns a loopback server that accepts messages with
// testToken in the query or the X-Gotify-Key header, counting them.
//
// Parameters:
//   - t: the test.
//   - useTLS: whether the server uses TLS with a self-signed certificate.
//
// Returns:
//   - *httptest.Server: the started server, closed when the test ends.
//   - *atomic.Int32: the number of accepted messages.
func newGotifyServer(t *testing.T, useTLS bool) (*httptest.Server, *atomic.Int32) {
	t.Helper()

	var accepted atomic.Int32

	server := httptest.NewUnstartedServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		token := r.URL.Query().Get("token")
		if token == "" {
			token = r.Header.Get("X-Gotify-Key")
		}

		if r.Method != http.MethodPost || r.URL.Path != "/message" || token != testToken {
			w.WriteHeader(http.StatusUnauthorized)
			_, _ = io.WriteString(w, unauthorizedBody)

			return
		}

		accepted.Add(1)

		_, _ = io.WriteString(w, sentBody)
	}))
	// The server logs rejected TLS handshakes to the test output.
	server.Config.ErrorLog = log.New(t.Output(), "", 0)

	if useTLS {
		server.StartTLS()
	} else {
		server.Start()
	}

	t.Cleanup(server.Close)

	return server, &accepted
}

// TestSendDefaultClientLoopback verifies the default client against loopback
// servers: plain HTTP with TLS disabled, with the token in the query or a
// header, and HTTPS with a self-signed certificate, which fails verification
// unless insecureskipverify is set.
func TestSendDefaultClientLoopback(t *testing.T) {
	t.Parallel()

	t.Run("plain HTTP", func(t *testing.T) {
		t.Parallel()

		server, accepted := newGotifyServer(t, false)
		rawURL := "gotify://" + server.Listener.Addr().String() + "/" + testToken + "?disabletls=yes"

		require.NoError(t, newTestService(t, rawURL).Send("Hello", nil))
		require.NoError(t, newTestService(t, rawURL+"&useheader=yes").Send("Hello", nil))
		assert.Equal(t, int32(2), accepted.Load())
	})

	t.Run("self-signed HTTPS", func(t *testing.T) {
		t.Parallel()

		server, accepted := newGotifyServer(t, true)
		rawURL := "gotify://" + server.Listener.Addr().String() + "/" + testToken

		_, isCertErr := errors.AsType[*tls.CertificateVerificationError](newTestService(t, rawURL).Send("Hello", nil))
		assert.True(t, isCertErr, "error is not a certificate verification error")

		require.NoError(t, newTestService(t, rawURL+"?insecureskipverify=yes").Send("Hello", nil))
		assert.Equal(t, int32(1), accepted.Load())
	})
}

// TestSendTimeout verifies that the default client gives up on a server that
// does not answer within its 10 second timeout.
func TestSendTimeout(t *testing.T) {
	t.Parallel()

	synctest.Test(t, func(t *testing.T) {
		serverConn, clientConn := net.Pipe()

		t.Cleanup(func() {
			_ = serverConn.Close()
			_ = clientConn.Close()
		})

		service := newTestService(t, "gotify://"+testHost+"/"+testToken)
		service.httpClientManager = &fixedTransportManager{transport: &http.Transport{
			DialTLSContext: func(context.Context, string, string) (net.Conn, error) {
				return clientConn, nil
			},
		}}
		service.SetHTTPClient(nil)

		// The server reads the request and never answers.
		go func() { _, _ = http.ReadRequest(bufio.NewReader(serverConn)) }()

		start := time.Now()
		err := service.Send("Hello", nil)

		require.ErrorIs(t, err, context.DeadlineExceeded)
		assert.Equal(t, 10*time.Second, time.Since(start))
	})
}

// TestSendURLBuilderError verifies that a send fails before any request when
// the message endpoint cannot be built.
func TestSendURLBuilderError(t *testing.T) {
	t.Parallel()

	errBuild := errors.New("build failure")

	service := newTestService(t, testServiceURL)
	service.urlBuilder = failingURLBuilder{err: errBuild}
	// The mock has no expectations, so a request fails the test.
	service.SetHTTPClient(typesmocks.NewMockHTTPClient(t))

	require.ErrorIs(t, service.Send("Hello", nil), errBuild)
}

// TestFilterParams verifies that filterParams copies every param except
// extras and handles nil params.
func TestFilterParams(t *testing.T) {
	t.Parallel()

	params := types.Params{"title": "Backup", "extras": "{}", "priority": "5"}

	assert.Equal(t, types.Params{"title": "Backup", "priority": "5"}, filterParams(&params))
	assert.Equal(t, types.Params{"title": "Backup", "extras": "{}", "priority": "5"}, params)
	assert.Equal(t, types.Params{}, filterParams(nil))
}

// CreateClient returns a default client that sends through httpTransport.
//
// Parameters:
//   - httpTransport: the transport for the client.
//
// Returns:
//   - *http.Client: the client.
func (m *fixedTransportManager) CreateClient(httpTransport *http.Transport) *http.Client {
	return (&DefaultHTTPClientManager{}).CreateClient(httpTransport)
}

// CreateTransport returns the fixed transport.
//
// Returns:
//   - *http.Transport: the transport.
func (m *fixedTransportManager) CreateTransport(*Config) *http.Transport {
	return m.transport
}

// BuildURL returns the builder's error.
//
// Returns:
//   - string: an empty URL.
//   - error: the builder's error.
func (b failingURLBuilder) BuildURL(*Config) (string, error) {
	return "", b.err
}
