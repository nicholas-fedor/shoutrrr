package ntfy

import (
	"bytes"
	"context"
	"crypto/tls"
	"errors"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/http/httptest"
	"net/url"
	"slices"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	"github.com/nicholas-fedor/shoutrrr/internal/meta"
	"github.com/nicholas-fedor/shoutrrr/pkg/format"
	"github.com/nicholas-fedor/shoutrrr/pkg/types"
	"github.com/nicholas-fedor/shoutrrr/pkg/types/mocks"
	"github.com/nicholas-fedor/shoutrrr/pkg/util"
	"github.com/nicholas-fedor/shoutrrr/pkg/util/jsonclient"
)

// capturedRequest records the parts of a request that an HTTP client received.
type capturedRequest struct {
	// header is the request's headers.
	header http.Header
	// method is the request method.
	method string
	// url is the full request URL.
	url string
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

// Values shared by the service tests.
const (
	// publishedBody is ntfy's response to a published message.
	publishedBody = `{"id":"sPs71M8A2T","time":1760500000,"expires":1760543200,"event":"message","topic":"alerts","message":"Hello"}`
	// forbiddenBody is ntfy's response to a publish without access to the topic.
	forbiddenBody = `{"code":40301,"http":403,"error":"forbidden","link":"https://ntfy.sh/docs/publish/#authentication"}`
)

// Values shared by the service tests.
var (
	// errTransport is the transport failure returned by a failing client.
	errTransport = errors.New("transport failure")
	// published is the response to a published message.
	published = cannedResponse{statusCode: http.StatusOK, body: publishedBody}
)

// newRecordingClient returns a mock HTTP client that expects one request per
// response, records each request, and answers them in order.
//
// Parameters:
//   - t: the test that owns the mock.
//   - responses: the responses to return, in request order.
//
// Returns:
//   - *mocks.MockHTTPClient: the client.
//   - func() []capturedRequest: returns the requests received so far.
func newRecordingClient(t *testing.T, responses ...cannedResponse) (*mocks.MockHTTPClient, func() []capturedRequest) {
	t.Helper()

	var (
		mu       sync.Mutex
		captured []capturedRequest
	)

	client := mocks.NewMockHTTPClient(t)
	client.EXPECT().Do(mock.Anything).RunAndReturn(func(req *http.Request) (*http.Response, error) {
		body, err := io.ReadAll(req.Body)
		require.NoError(t, err)

		mu.Lock()
		defer mu.Unlock()

		captured = append(captured, capturedRequest{
			header: req.Header.Clone(),
			method: req.Method,
			url:    req.URL.String(),
			body:   string(body),
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

// requestHeader returns the headers every publish carries, with the given
// headers added.
//
// Parameters:
//   - extra: header names and values to add, in pairs.
//
// Returns:
//   - http.Header: the headers.
func requestHeader(extra ...string) http.Header {
	header := http.Header{
		"Content-Type": {"text/plain; charset=utf-8"},
		"User-Agent":   {meta.UserAgent()},
		"Priority":     {"Default"},
	}

	for i := 0; i+1 < len(extra); i += 2 {
		header.Set(extra[i], extra[i+1])
	}

	return header
}

// assertDefaultClient asserts that client is a default client without a
// timeout of its own. The default client sends through http.DefaultTransport,
// and the insecure one through a proxy-aware transport that requires TLS 1.2
// and skips certificate verification.
//
// Parameters:
//   - t: the test.
//   - client: the client to check.
//   - wantSkip: whether the client should skip certificate verification.
func assertDefaultClient(t *testing.T, client types.HTTPClient, wantSkip bool) {
	t.Helper()

	httpClient, ok := client.(*http.Client)
	require.True(t, ok, "client is %T", client)
	assert.Zero(t, httpClient.Timeout)

	if !wantSkip {
		assert.Nil(t, httpClient.Transport, "the default client should send through http.DefaultTransport")

		return
	}

	transport, ok := httpClient.Transport.(*http.Transport)
	require.True(t, ok, "transport is %T", httpClient.Transport)
	require.NotNil(t, transport.TLSClientConfig)
	assert.Equal(t, uint16(tls.VersionTLS12), transport.TLSClientConfig.MinVersion)
	assert.True(t, transport.TLSClientConfig.InsecureSkipVerify)
	assert.NotNil(t, transport.Proxy)
}

// TestGetID verifies that the service identifies itself by its scheme.
func TestGetID(t *testing.T) {
	t.Parallel()

	assert.Equal(t, "ntfy", (&Service{}).GetID())
}

// TestServiceTimeout verifies that the router gives ntfy the default send
// budget.
func TestServiceTimeout(t *testing.T) {
	t.Parallel()

	assert.Equal(t, types.DefaultSendTimeout, (&Service{}).ServiceTimeout(nil))
}

// TestInitialize verifies that Initialize parses the service URL over the
// defaults, and that it warns only about the deprecated keys the URL uses. An
// http scheme is kept as written and turns TLS off.
func TestInitialize(t *testing.T) {
	t.Parallel()

	// tests lists URLs with the scheme, publish URL, and log each should
	// produce.
	tests := []struct {
		name       string
		rawURL     string
		wantScheme string
		wantAPIURL string
		wantLog    string
	}{
		{name: "defaults", rawURL: testServiceURL, wantScheme: "https", wantAPIURL: testEndpoint},
		{name: "TLS disabled", rawURL: testServiceURL + "?disabletls=yes", wantScheme: "https", wantAPIURL: "http://ntfy.sh/" + testTopic},
		{name: "verification skipped", rawURL: testServiceURL + "?skiptlsverify=yes", wantScheme: "https", wantAPIURL: testEndpoint},
		{
			name:       "deprecated verification key",
			rawURL:     testServiceURL + "?disabletlsverification=yes",
			wantScheme: "https",
			wantAPIURL: testEndpoint,
			wantLog:    `WARN config key is deprecated key=disabletlsverification replacement=skiptlsverify` + "\n",
		},
		{
			name:       "disabletls over the deprecated scheme",
			rawURL:     testServiceURL + "?scheme=http&disabletls=no",
			wantScheme: "https",
			wantAPIURL: testEndpoint,
			wantLog:    `WARN config key is deprecated key=scheme replacement=disabletls` + "\n",
		},
		{
			name:       "deprecated scheme",
			rawURL:     testServiceURL + "?scheme=http",
			wantScheme: "http",
			wantAPIURL: "http://ntfy.sh/" + testTopic,
			wantLog:    `WARN config key is deprecated key=scheme replacement=disabletls` + "\n",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			var logs bytes.Buffer

			service := &Service{}
			require.NoError(t, service.Initialize(parseTestURL(t, tt.rawURL), log.New(&logs, "", 0)))

			assert.Equal(t, tt.wantScheme, service.Config.Scheme)
			assert.Equal(t, tt.wantAPIURL, service.Config.GetAPIURL())
			assertDefaultClient(t, service.client.Client(), false)
			assert.Equal(t, tt.wantLog, logs.String())
		})
	}

	t.Run("missing topic", func(t *testing.T) {
		t.Parallel()

		err := (&Service{}).Initialize(parseTestURL(t, "ntfy://ntfy.sh"), log.New(t.Output(), "", 0))

		require.ErrorIs(t, err, ErrTopicRequired)
	})

	t.Run("invalid scheme", func(t *testing.T) {
		t.Parallel()

		err := (&Service{}).Initialize(parseTestURL(t, testServiceURL+"?scheme=ftp"), log.New(t.Output(), "", 0))

		require.ErrorIs(t, err, ErrInvalidScheme)
	})
}

// TestInitializeKeepsCustomClient verifies that a custom client set before
// the first Initialize is kept across a second one.
func TestInitializeKeepsCustomClient(t *testing.T) {
	t.Parallel()

	client := mocks.NewMockHTTPClient(t)
	service := &Service{}
	service.SetHTTPClient(client)
	require.NoError(t, service.Initialize(parseTestURL(t, testServiceURL), log.New(t.Output(), "", 0)))
	require.NoError(t, service.Initialize(parseTestURL(t, testServiceURL+"?skiptlsverify=yes"), log.New(t.Output(), "", 0)))

	assert.Same(t, client, service.client.Client())
}

// TestSend verifies the request a send makes: a POST of the message to the
// topic URL, with the settings from the URL and params as ntfy headers.
func TestSend(t *testing.T) {
	t.Parallel()

	// tests lists sends with the request each should produce.
	tests := []struct {
		params     *types.Params
		wantHeader http.Header
		name       string
		rawURL     string
		message    string
		wantURL    string
	}{
		{name: "message only", rawURL: testServiceURL, message: "Hello", wantURL: testEndpoint, wantHeader: requestHeader()},
		{name: "empty message", rawURL: testServiceURL, wantURL: testEndpoint, wantHeader: requestHeader()},
		{
			name: "message settings",
			rawURL: testServiceURL + "?title=Backup&tags=warning,skull&priority=high&click=https://example.com" +
				"&attach=https://example.com/log.txt&filename=log.txt&delay=30m&email=ops@example.com&icon=https://example.com/icon.png" +
				"&actions=view, Open, https://example.com;http, Ping, https://example.com/ping",
			message: "Hello",
			wantURL: testEndpoint,
			wantHeader: requestHeader(
				"Title", "Backup", "Tags", "warning,skull", "Priority", "High", "Click", "https://example.com",
				"Attach", "https://example.com/log.txt", "Filename", "log.txt", "Delay", "30m", "Email", "ops@example.com",
				"X-Icon", "https://example.com/icon.png", "Actions", "view, Open, https://example.com;http, Ping, https://example.com/ping",
			),
		},
		{
			name:       "params",
			rawURL:     testServiceURL + "?title=Backup",
			params:     &types.Params{"title": "Restore", "priority": "5", "tags": "white_check_mark"},
			message:    "Hello",
			wantURL:    testEndpoint,
			wantHeader: requestHeader("Title", "Restore", "Priority", "Max", "Tags", "white_check_mark"),
		},
		{
			name:       "markdown",
			rawURL:     testServiceURL + "?markdown=yes",
			message:    "**Hello**",
			wantURL:    testEndpoint,
			wantHeader: requestHeader("Content-Type", "text/markdown"),
		},
		{
			name:       "cache and Firebase off",
			rawURL:     testServiceURL + "?cache=no&firebase=no",
			message:    "Hello",
			wantURL:    testEndpoint,
			wantHeader: requestHeader("Cache", "no", "Firebase", "no"),
		},
		{
			name:       "username and password",
			rawURL:     "ntfy://user:pass@ntfy.sh/" + testTopic,
			message:    "Hello",
			wantURL:    testEndpoint,
			wantHeader: requestHeader("Authorization", "Basic dXNlcjpwYXNz"),
		},
		{
			name:       "username only",
			rawURL:     "ntfy://user@ntfy.sh/" + testTopic,
			message:    "Hello",
			wantURL:    testEndpoint,
			wantHeader: requestHeader("Authorization", "Basic dXNlcjo="),
		},
		{
			name:       "password only",
			rawURL:     "ntfy://:pass@ntfy.sh/" + testTopic,
			message:    "Hello",
			wantURL:    testEndpoint,
			wantHeader: requestHeader("Authorization", "Basic OnBhc3M="),
		},
		{
			name:       "token",
			rawURL:     testServiceURL + "?token=" + testToken,
			message:    "Hello",
			wantURL:    testEndpoint,
			wantHeader: requestHeader("Authorization", "Bearer "+testToken),
		},
		{
			name:       "token over username and password",
			rawURL:     "ntfy://user:pass@ntfy.sh/" + testTopic + "?token=" + testToken,
			message:    "Hello",
			wantURL:    testEndpoint,
			wantHeader: requestHeader("Authorization", "Bearer "+testToken),
		},
		{
			name:       "TLS disabled",
			rawURL:     "ntfy://ntfy.example.com:8080/" + testTopic + "?disabletls=yes",
			message:    "Hello",
			wantURL:    "http://ntfy.example.com:8080/" + testTopic,
			wantHeader: requestHeader(),
		},
		{
			name:       "nested topic",
			rawURL:     "ntfy://ntfy.example.com/ntfy/" + testTopic,
			message:    "Hello",
			wantURL:    "https://ntfy.example.com/ntfy/" + testTopic,
			wantHeader: requestHeader(),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			service := newTestService(t, tt.rawURL)
			client, captured := newRecordingClient(t, published)
			service.SetHTTPClient(client)

			require.NoError(t, service.Send(tt.message, tt.params))

			requests := captured()
			require.Len(t, requests, 1)
			assert.Equal(t, http.MethodPost, requests[0].method)
			assert.Equal(t, tt.wantURL, requests[0].url)
			assert.Equal(t, tt.wantHeader, requests[0].header)
			assert.Equal(t, tt.message, requests[0].body)
		})
	}
}

// TestSendLevelPriority verifies that the level param sets the priority, debug
// to low, info to default, warning to high, and error to max, and that a
// priority from the service URL or the send params, or an unknown level,
// leaves the priority as it is.
func TestSendLevelPriority(t *testing.T) {
	t.Parallel()

	tests := map[string]struct {
		rawURL       string
		params       *types.Params
		wantPriority string
	}{
		"debug":                       {rawURL: testServiceURL, params: &types.Params{"level": "Debug"}, wantPriority: "Low"},
		"info":                        {rawURL: testServiceURL, params: &types.Params{"level": "Info"}, wantPriority: "Default"},
		"warning":                     {rawURL: testServiceURL, params: &types.Params{"level": "Warning"}, wantPriority: "High"},
		"error":                       {rawURL: testServiceURL, params: &types.Params{"level": "Error"}, wantPriority: "Max"},
		"level in another case":       {rawURL: testServiceURL, params: &types.Params{"level": "error"}, wantPriority: "Max"},
		"priority in the URL":         {rawURL: testServiceURL + "?priority=low", params: &types.Params{"level": "Error"}, wantPriority: "Low"},
		"default priority in the URL": {rawURL: testServiceURL + "?priority=default", params: &types.Params{"level": "Error"}, wantPriority: "Default"},
		"priority param":              {rawURL: testServiceURL, params: &types.Params{"level": "Error", "priority": "default"}, wantPriority: "Default"},
		"unknown level":               {rawURL: testServiceURL, params: &types.Params{"level": "critical"}, wantPriority: "Default"},
		"no level":                    {rawURL: testServiceURL, params: &types.Params{"title": "Hello"}, wantPriority: "Default"},
	}

	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			service := newTestService(t, tc.rawURL)
			client, captured := newRecordingClient(t, published)
			service.SetHTTPClient(client)

			require.NoError(t, service.Send("Hello", tc.params))

			requests := captured()
			require.Len(t, requests, 1)
			assert.Equal(t, tc.wantPriority, requests[0].header.Get("Priority"))
		})
	}
}

// TestSendLogs verifies that a send logs a request that succeeded, and
// reports a failed request through its error only.
func TestSendLogs(t *testing.T) {
	t.Parallel()

	var logs bytes.Buffer

	service := &Service{}
	require.NoError(t, service.Initialize(parseTestURL(t, testServiceURL), log.New(&logs, "", 0)))

	client, _ := newRecordingClient(t, published, cannedResponse{statusCode: http.StatusForbidden, body: forbiddenBody})
	service.SetHTTPClient(client)

	require.NoError(t, service.Send("Hello", nil))
	require.Error(t, service.Send("Hello", nil))
	assert.Equal(t, "DEBUG notification sent\n", logs.String())
}

// TestSendErrors verifies that a send reports ntfy's error response, other
// error statuses, and transport errors without the credentials, and rejects
// an invalid param before any request.
func TestSendErrors(t *testing.T) {
	t.Parallel()

	// tests lists responses with the error each should produce.
	tests := []struct {
		wantErr   error
		name      string
		response  cannedResponse
		wantError string
	}{
		{
			name:      "error response",
			response:  cannedResponse{statusCode: http.StatusForbidden, body: forbiddenBody},
			wantError: "failed to send ntfy notification: server response: forbidden (40301), see: https://ntfy.sh/docs/publish/#authentication",
		},
		{
			name:      "status without an error body",
			response:  cannedResponse{statusCode: http.StatusBadGateway, body: "<html>Bad Gateway</html>"},
			wantErr:   jsonclient.ErrUnexpectedStatus,
			wantError: "failed to send ntfy notification: posting to ntfy API: got unexpected HTTP status: 502 Bad Gateway",
		},
		{
			name:     "transport error",
			response: cannedResponse{err: errTransport},
			wantErr:  errTransport,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			service := newTestService(t, "ntfy://user:SECRETpass@ntfy.sh/"+testTopic+"?token=SECRETtoken")
			client, _ := newRecordingClient(t, tt.response)
			service.SetHTTPClient(client)

			err := service.Send("Hello", nil)

			require.Error(t, err)

			if tt.wantErr != nil {
				require.ErrorIs(t, err, tt.wantErr)
			}

			if tt.wantError != "" {
				require.EqualError(t, err, tt.wantError)
			}

			assert.NotContains(t, err.Error(), "SECRET")
		})
	}

	t.Run("API error type", func(t *testing.T) {
		t.Parallel()

		service := newTestService(t, testServiceURL)
		client, _ := newRecordingClient(t, cannedResponse{statusCode: http.StatusTooManyRequests, body: `{"code":42901,"http":429,"error":"limit reached: too many requests"}`})
		service.SetHTTPClient(client)

		apiErr, ok := errors.AsType[*apiResponseError](service.Send("Hello", nil))
		require.True(t, ok, "error is not an *apiResponseError")
		assert.Equal(t, &apiResponseError{Code: 42901, Message: "limit reached: too many requests"}, apiErr)
	})

	t.Run("invalid param", func(t *testing.T) {
		t.Parallel()

		service := newTestService(t, testServiceURL)
		// The mock has no expectations, so a request fails the test.
		service.SetHTTPClient(mocks.NewMockHTTPClient(t))

		require.ErrorIs(t, service.Send("Hello", &types.Params{"unknown": "value"}), format.ErrInvalidConfigKey)
	})
}

// TestSendChecksConfig verifies that a send checks a config built in code as
// reading a URL does. A config without a valid topic fails before any request.
// So does a host that is not a hostname or a bracketed IPv6 address.
func TestSendChecksConfig(t *testing.T) {
	t.Parallel()

	// tests lists config changes with the error the send must return.
	tests := []struct {
		change  func(config *Config)
		wantErr error
		name    string
	}{
		{name: "zero config", change: func(config *Config) { *config = Config{} }, wantErr: ErrTopicRequired},
		{name: "invalid topic", change: func(config *Config) { config.Topic = "a topic" }, wantErr: ErrInvalidTopic},
		{name: "unbracketed IPv6 host", change: func(config *Config) { config.Host = "::1" }, wantErr: util.ErrInvalidHost},
		{name: "non-numeric port", change: func(config *Config) { config.Host = "ntfy.sh:abc" }, wantErr: util.ErrInvalidPort},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			service := newTestService(t, "ntfy://ntfy.sh/"+testTopic)
			// The mock has no expectations, so a request fails the test.
			service.SetHTTPClient(mocks.NewMockHTTPClient(t))
			tt.change(service.Config)

			require.ErrorIs(t, service.Send("Message", nil), tt.wantErr)
		})
	}
}

// TestSendLeavesConfigUnchanged verifies that params and their headers apply
// to one send only and leave the caller's params unchanged.
func TestSendLeavesConfigUnchanged(t *testing.T) {
	t.Parallel()

	service := newTestService(t, testServiceURL)
	client, captured := newRecordingClient(t, published, published)
	service.SetHTTPClient(client)

	want := *service.Config
	params := types.Params{"title": "Backup", "priority": "max", "tags": "skull", "markdown": "yes"}

	require.NoError(t, service.Send("Hello", &params))
	require.NoError(t, service.Send("Hello", nil))

	assert.Equal(t, want, *service.Config)
	assert.Equal(t, types.Params{"title": "Backup", "priority": "max", "tags": "skull", "markdown": "yes"}, params)

	requests := captured()
	require.Len(t, requests, 2)
	assert.Equal(t, requestHeader(), requests[1].header)
}

// TestSendConcurrently verifies that concurrent sends with different params
// each carry only their own headers.
func TestSendConcurrently(t *testing.T) {
	t.Parallel()

	const sends = 8

	responses := make([]cannedResponse, sends)
	for i := range responses {
		responses[i] = published
	}

	service := newTestService(t, testServiceURL)
	client, captured := newRecordingClient(t, responses...)
	service.SetHTTPClient(client)

	var wg sync.WaitGroup
	for i := range sends {
		wg.Go(func() {
			assert.NoError(t, service.Send(strconv.Itoa(i), &types.Params{"title": fmt.Sprintf("Title %d", i)}))
		})
	}

	wg.Wait()

	requests := captured()
	require.Len(t, requests, sends)

	for _, request := range requests {
		assert.Equal(t, requestHeader("Title", "Title "+request.body), request.header)
	}
}

// TestSendContextCancellation verifies that the caller's context bounds the
// request.
func TestSendContextCancellation(t *testing.T) {
	t.Parallel()

	ctx, cancel := context.WithCancel(t.Context())
	cancel()

	client := mocks.NewMockHTTPClient(t)
	client.EXPECT().Do(mock.Anything).RunAndReturn(func(req *http.Request) (*http.Response, error) {
		return nil, req.Context().Err()
	}).Once()

	service := newTestService(t, testServiceURL)
	service.SetHTTPClient(client)

	require.ErrorIs(t, service.SendContext(ctx, "Hello", nil), context.Canceled)
}

// TestSetHTTPClient verifies that SetHTTPClient stores a custom client, and
// that nil and a nil *http.Client restore the default client, before and
// after Initialize.
func TestSetHTTPClient(t *testing.T) {
	t.Parallel()

	// tests lists the clients that restore the default.
	tests := []struct {
		client types.HTTPClient
		name   string
	}{
		{name: "nil", client: nil},
		{name: "nil *http.Client", client: (*http.Client)(nil)},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			service := newTestService(t, testServiceURL+"?skiptlsverify=yes")

			custom := mocks.NewMockHTTPClient(t)
			service.SetHTTPClient(custom)
			assert.Same(t, custom, service.client.Client())

			service.SetHTTPClient(tt.client)
			assertDefaultClient(t, service.client.Client(), false)
			assertDefaultClient(t, service.client.InsecureClient(nil), true)
		})
	}

	t.Run("before Initialize", func(t *testing.T) {
		t.Parallel()

		service := &Service{}
		service.SetHTTPClient(nil)

		require.NoError(t, service.Initialize(parseTestURL(t, testServiceURL), log.New(t.Output(), "", 0)))
		assertDefaultClient(t, service.client.Client(), false)
	})
}

// TestSendDefaultClient verifies that a send through the default client
// honors the caller's context and keeps the credentials out of its errors.
// The context is canceled before the send, so the transport returns before it
// dials.
func TestSendDefaultClient(t *testing.T) {
	t.Parallel()

	service := newTestService(t, "ntfy://user:SECRETpass@ntfy.sh/"+testTopic+"?token=SECRETtoken")

	ctx, cancel := context.WithCancel(t.Context())
	cancel()

	err := service.SendContext(ctx, "Hello", nil)

	require.ErrorIs(t, err, context.Canceled)
	assert.NotContains(t, err.Error(), "SECRET")
}

// newNtfyServer returns a loopback server that accepts messages for testTopic,
// counting them.
//
// Parameters:
//   - t: the test.
//   - useTLS: whether the server uses TLS with a self-signed certificate.
//
// Returns:
//   - *httptest.Server: the started server, closed when the test ends.
//   - *atomic.Int32: the number of accepted messages.
func newNtfyServer(t *testing.T, useTLS bool) (*httptest.Server, *atomic.Int32) {
	t.Helper()

	var accepted atomic.Int32

	server := httptest.NewUnstartedServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost || r.URL.Path != "/"+testTopic {
			w.WriteHeader(http.StatusNotFound)
			_, _ = io.WriteString(w, `{"code":40401,"http":404,"error":"page not found"}`)

			return
		}

		accepted.Add(1)

		_, _ = io.WriteString(w, publishedBody)
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

// TestSendDefaultClientLoopback verifies the default clients against loopback
// servers.
//
//   - Plain HTTP works with TLS disabled, or with the deprecated http scheme.
//   - HTTPS with a self-signed certificate fails verification, unless the URL,
//     the deprecated key, the params, or the deprecated field skip it.
func TestSendDefaultClientLoopback(t *testing.T) {
	t.Parallel()

	t.Run("plain HTTP", func(t *testing.T) {
		t.Parallel()

		server, accepted := newNtfyServer(t, false)

		rawURL := "ntfy://" + server.Listener.Addr().String() + "/" + testTopic

		require.NoError(t, newTestService(t, rawURL+"?disabletls=yes").Send("Hello", nil))
		require.NoError(t, newTestService(t, rawURL+"?scheme=http").Send("Hello", nil))
		require.NoError(t, newTestService(t, rawURL).Send("Hello", &types.Params{"scheme": "http"}))

		deprecatedField := newTestService(t, rawURL)
		deprecatedField.Config.Scheme = "http"
		require.NoError(t, deprecatedField.Send("Hello", nil))

		assert.Equal(t, int32(4), accepted.Load())
	})

	t.Run("self-signed HTTPS", func(t *testing.T) {
		t.Parallel()

		server, accepted := newNtfyServer(t, true)
		rawURL := "ntfy://" + server.Listener.Addr().String() + "/" + testTopic

		_, isCertErr := errors.AsType[*tls.CertificateVerificationError](newTestService(t, rawURL).Send("Hello", nil))
		assert.True(t, isCertErr, "error is not a certificate verification error")

		require.NoError(t, newTestService(t, rawURL+"?skiptlsverify=yes").Send("Hello", nil))
		require.NoError(t, newTestService(t, rawURL+"?disabletlsverification=yes").Send("Hello", nil))

		deprecatedField := newTestService(t, rawURL)
		deprecatedField.Config.DisableTLSVerification = true
		require.NoError(t, deprecatedField.Send("Hello", nil))

		assert.Equal(t, int32(3), accepted.Load())
	})
}

// TestSendTLSParams verifies that the disabletls and skiptlsverify send
// params apply to one send. disabletls sends over HTTP, and skiptlsverify
// skips certificate verification for that send, while sends without the param
// keep the stored setting. A custom client is used unchanged.
func TestSendTLSParams(t *testing.T) {
	t.Parallel()

	t.Run("disabletls", func(t *testing.T) {
		t.Parallel()

		server, accepted := newNtfyServer(t, false)
		service := newTestService(t, "ntfy://"+server.Listener.Addr().String()+"/"+testTopic)

		require.NoError(t, service.Send("Hello", &types.Params{"disabletls": "yes"}))
		require.Error(t, service.Send("Hello", nil))
		assert.Equal(t, int32(1), accepted.Load())
	})

	t.Run("skiptlsverify", func(t *testing.T) {
		t.Parallel()

		server, accepted := newNtfyServer(t, true)
		service := newTestService(t, "ntfy://"+server.Listener.Addr().String()+"/"+testTopic)

		require.NoError(t, service.Send("Hello", &types.Params{"skiptlsverify": "yes"}))

		_, isCertErr := errors.AsType[*tls.CertificateVerificationError](service.Send("Hello", nil))
		assert.True(t, isCertErr, "error is not a certificate verification error")
		assert.Equal(t, int32(1), accepted.Load())
	})

	t.Run("skiptlsverify turned back on", func(t *testing.T) {
		t.Parallel()

		server, accepted := newNtfyServer(t, true)
		service := newTestService(t, "ntfy://"+server.Listener.Addr().String()+"/"+testTopic+"?skiptlsverify=yes")

		_, isCertErr := errors.AsType[*tls.CertificateVerificationError](service.Send("Hello", &types.Params{"skiptlsverify": "no"}))
		assert.True(t, isCertErr, "error is not a certificate verification error")
		require.NoError(t, service.Send("Hello", nil))
		assert.Equal(t, int32(1), accepted.Load())
	})

	t.Run("disabletls turned back on", func(t *testing.T) {
		t.Parallel()

		server, accepted := newNtfyServer(t, true)
		service := newTestService(t, "ntfy://"+server.Listener.Addr().String()+"/"+testTopic+"?disabletls=yes&skiptlsverify=yes")

		require.NoError(t, service.Send("Hello", &types.Params{"disabletls": "no"}))
		require.Error(t, service.Send("Hello", nil))
		assert.Equal(t, int32(1), accepted.Load())
	})

	t.Run("invalid scheme param", func(t *testing.T) {
		t.Parallel()

		service := newTestService(t, testServiceURL)

		require.ErrorIs(t, service.Send("Hello", &types.Params{"scheme": "ftp"}), ErrInvalidScheme)
	})

	t.Run("custom client", func(t *testing.T) {
		t.Parallel()

		service := newTestService(t, testServiceURL)
		client, captured := newRecordingClient(t, published)
		service.SetHTTPClient(client)

		require.NoError(t, service.Send("Hello", &types.Params{"skiptlsverify": "yes"}))
		assert.Len(t, captured(), 1)
	})
}

// TestSendSchemePrecedence verifies that disabletls decides the protocol of a
// send over the deprecated scheme. A disabletls param overrides a stored http
// scheme, a scheme param of either protocol applies when the params do not set
// disabletls, and disabletls wins when the params set both.
func TestSendSchemePrecedence(t *testing.T) {
	t.Parallel()

	tests := []struct {
		params  *types.Params
		name    string
		rawURL  string
		wantURL string
	}{
		{
			name:    "disabletls param overrides the stored scheme",
			rawURL:  testServiceURL + "?scheme=http",
			params:  &types.Params{"disabletls": "no"},
			wantURL: testEndpoint,
		},
		{name: "stored scheme", rawURL: testServiceURL + "?scheme=http", wantURL: "http://ntfy.sh/" + testTopic},
		{
			name:    "https scheme param overrides the stored scheme",
			rawURL:  testServiceURL + "?scheme=http",
			params:  &types.Params{"scheme": "https"},
			wantURL: testEndpoint,
		},
		{name: "stored disabletls over a stored scheme", rawURL: testServiceURL + "?scheme=http&disabletls=no", wantURL: testEndpoint},
		{
			name:    "scheme param",
			rawURL:  testServiceURL,
			params:  &types.Params{"scheme": "http"},
			wantURL: "http://ntfy.sh/" + testTopic,
		},
		{
			name:    "disabletls param wins over a scheme param",
			rawURL:  testServiceURL,
			params:  &types.Params{"scheme": "http", "disabletls": "no"},
			wantURL: testEndpoint,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			service := newTestService(t, tt.rawURL)
			client, captured := newRecordingClient(t, published)
			service.SetHTTPClient(client)

			require.NoError(t, service.Send("Hello", tt.params))

			requests := captured()
			require.Len(t, requests, 1)
			assert.Equal(t, tt.wantURL, requests[0].url)
		})
	}
}

// TestSendClient verifies that sends that verify certificates use the default
// client, and that sends that skip it share the insecure default client, even
// when they run concurrently. The first insecure send logs the shared warning
// once. An injected client is used for both and never warned about.
func TestSendClient(t *testing.T) {
	t.Parallel()

	var logs bytes.Buffer

	service := &Service{}
	require.NoError(t, service.Initialize(parseTestURL(t, testServiceURL), log.New(&logs, "", 0)))

	assert.Same(t, service.client.Client(), service.sendClient(t.Context(), service.Config))

	insecure := *service.Config
	insecure.SkipTLSVerify = true

	clients := make([]types.HTTPClient, 8)

	var wg sync.WaitGroup
	for i := range clients {
		wg.Go(func() { clients[i] = service.sendClient(t.Context(), &insecure) })
	}

	wg.Wait()

	for _, client := range clients {
		assert.Same(t, clients[0], client)
	}

	assertDefaultClient(t, clients[0], true)
	assert.Equal(t, "WARN skipping TLS certificate verification\n", logs.String())

	var injectedLogs bytes.Buffer

	injected := &Service{}
	require.NoError(t, injected.Initialize(parseTestURL(t, testServiceURL), log.New(&injectedLogs, "", 0)))

	custom := mocks.NewMockHTTPClient(t)
	injected.SetHTTPClient(custom)
	assert.Same(t, custom, injected.sendClient(t.Context(), &insecure))
	assert.Empty(t, injectedLogs.String())
}

// TestSendWarnings verifies that the plaintext warning, the deprecated param
// keys, and the deprecated fields are each warned about once over two sends,
// and appear again after the service is initialized again.
func TestSendWarnings(t *testing.T) {
	t.Parallel()

	var logs bytes.Buffer

	service := &Service{}
	require.NoError(t, service.Initialize(parseTestURL(t, testServiceURL), log.New(&logs, "", 0)))

	client, _ := newRecordingClient(t, published, published, published)
	service.SetHTTPClient(client)
	service.Config.DisableTLSVerification = true
	service.Config.Scheme = "http"

	params := &types.Params{"disabletlsverification": "yes", "scheme": "http"}
	require.NoError(t, service.Send("Hello", params))
	require.NoError(t, service.Send("Hello", params))

	want := []string{
		"WARN config field is deprecated field=DisableTLSVerification replacement=SkipTLSVerify",
		"WARN config field is deprecated field=Scheme replacement=DisableTLS",
		`WARN config key is deprecated key=disabletlsverification replacement=skiptlsverify`,
		`WARN config key is deprecated key=scheme replacement=disabletls`,
		"WARN sending without TLS",
	}
	assert.Equal(t, want, warningLines(logs.String()))

	require.NoError(t, service.Initialize(parseTestURL(t, testServiceURL+"?disabletls=yes"), log.New(&logs, "", 0)))
	require.NoError(t, service.Send("Hello", nil))
	assert.Equal(t, slices.Concat(want, []string{"WARN sending without TLS"}), warningLines(logs.String()))
}

// warningLines returns the logged lines that are warnings, leaving out the
// log of each request that succeeded.
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

// TestAddHeaderIfNotEmpty verifies that a header is added only for a
// non-empty value, alongside any existing values.
func TestAddHeaderIfNotEmpty(t *testing.T) {
	t.Parallel()

	header := http.Header{}

	addHeaderIfNotEmpty(&header, "Title", "")
	assert.Empty(t, header)

	addHeaderIfNotEmpty(&header, "Tags", "a")
	addHeaderIfNotEmpty(&header, "Tags", "b")
	assert.Equal(t, http.Header{"Tags": {"a", "b"}}, header)
}
