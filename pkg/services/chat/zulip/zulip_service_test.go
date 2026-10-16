package zulip

import (
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
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	"github.com/nicholas-fedor/shoutrrr/pkg/format"
	"github.com/nicholas-fedor/shoutrrr/pkg/services/chat/zulip/mocks"
	"github.com/nicholas-fedor/shoutrrr/pkg/types"
)

// capturedRequest records the parts of a request that an HTTP client received.
type capturedRequest struct {
	// form is the decoded form body.
	form url.Values
	// deadline is the request context's deadline, or the zero time.
	deadline time.Time
	// method is the request method.
	method string
	// url is the full request URL, including the credentials.
	url string
	// contentType is the request's Content-Type header.
	contentType string
}

// cannedResponse is what the fake Zulip client returns for one request.
type cannedResponse struct {
	// err is a transport error returned instead of a response.
	err error
	// body is the response body.
	body string
	// statusCode is the response status code.
	statusCode int
}

// fakeZulip records the register and message requests a fake Zulip client
// receives.
type fakeZulip struct {
	// registers are the register requests, in order.
	registers []capturedRequest
	// messages are the message requests, in order.
	messages []capturedRequest
	// mu guards registers and messages.
	mu sync.Mutex
}

// Responses shared by the service tests.
const (
	// sentBody is Zulip's response to a sent message.
	sentBody = `{"result":"success","msg":"","id":42,"message_url":"https://zulip.example.com/#narrow/near/42"}`
	// defaultLimitsBody is a register response with Zulip's default limits.
	defaultLimitsBody = `{"result":"success","msg":"","queue_id":"1760500000:1","max_message_length":10000,"max_topic_length":60}`
	// messagesURL is the messages endpoint for the test credentials and host.
	messagesURL = "https://bot%40example.com:" + testBotKey + "@" + testHost + "/api/v1/messages"
)

// Values shared by the service tests.
var (
	// errTransport is the transport failure returned by a failing client.
	errTransport = errors.New("transport failure")
	// sent is the response to a sent message.
	sent = cannedResponse{statusCode: http.StatusOK, body: sentBody}
	// defaultLimits is the register response with Zulip's default limits.
	defaultLimits = cannedResponse{statusCode: http.StatusOK, body: defaultLimitsBody}
)

// newFakeClient returns a mock HTTP client that answers register requests
// with register, and message requests with messages in order, recording
// each request. It fails the test on a request to any other endpoint or on
// more message requests than messages.
//
// Parameters:
//   - t: the test that owns the mock.
//   - register: the response to every register request.
//   - messages: the responses to the message requests, in order.
//
// Returns:
//   - *mocks.MockHTTPClient: the client.
//   - *fakeZulip: the recorded requests.
func newFakeClient(t *testing.T, register cannedResponse, messages ...cannedResponse) (*mocks.MockHTTPClient, *fakeZulip) {
	t.Helper()

	fake := &fakeZulip{}
	client := mocks.NewMockHTTPClient(t)
	client.EXPECT().Do(mock.Anything).RunAndReturn(func(req *http.Request) (*http.Response, error) {
		body, err := io.ReadAll(req.Body)
		require.NoError(t, err)

		form, err := url.ParseQuery(string(body))
		require.NoError(t, err)

		deadline, _ := req.Context().Deadline()
		request := capturedRequest{
			form:        form,
			deadline:    deadline,
			method:      req.Method,
			url:         req.URL.String(),
			contentType: req.Header.Get("Content-Type"),
		}

		fake.mu.Lock()
		defer fake.mu.Unlock()

		var response cannedResponse

		switch req.URL.Path {
		case "/api/v1/register":
			fake.registers = append(fake.registers, request)
			response = register
		case "/api/v1/messages":
			fake.messages = append(fake.messages, request)
			require.LessOrEqual(t, len(fake.messages), len(messages), "unexpected message request")
			response = messages[len(fake.messages)-1]
		default:
			t.Errorf("unexpected request to %s", req.URL.Path)

			return nil, errTransport
		}

		if response.err != nil {
			return nil, &url.Error{Op: "Post", URL: req.URL.String(), Err: response.err}
		}

		return &http.Response{StatusCode: response.statusCode, Body: io.NopCloser(strings.NewReader(response.body))}, nil
	}).Maybe()

	return client, fake
}

// newTestService returns a service initialized from rawURL that logs to the
// test output and sends through client.
//
// Parameters:
//   - t: the test.
//   - rawURL: the service URL.
//   - client: the HTTP client for the service.
//
// Returns:
//   - *Service: the service.
func newTestService(t *testing.T, rawURL string, client types.HTTPClient) *Service {
	t.Helper()

	service := &Service{}
	service.SetHTTPClient(client)
	require.NoError(t, service.Initialize(parseTestURL(t, rawURL), log.New(t.Output(), "", 0)))

	return service
}

// TestGetID verifies that the service identifies itself by its scheme.
func TestGetID(t *testing.T) {
	t.Parallel()

	assert.Equal(t, "zulip", (&Service{}).GetID())
}

// TestServiceTimeout verifies that the router budget covers the register
// request and the message request.
func TestServiceTimeout(t *testing.T) {
	t.Parallel()

	assert.Equal(t, 40*time.Second, (&Service{}).ServiceTimeout(nil))
}

// TestInitialize verifies that Initialize parses the service URL, starts with
// the default limits, and creates the default client unless one was set.
func TestInitialize(t *testing.T) {
	t.Parallel()

	service := &Service{}
	require.NoError(t, service.Initialize(parseTestURL(t, testServiceURL), log.New(t.Output(), "", 0)))

	assert.Equal(t, &Config{BotMail: testBotMail, BotKey: testBotKey, Host: testHost, Stream: "general", Topic: "alerts"}, service.Config)
	assert.Equal(t, 10000, service.contentMaxSize)
	assert.Equal(t, 60, service.topicMaxLength)

	client, ok := service.HTTPClient.(*DefaultHTTPClient)
	require.True(t, ok, "default client is %T", service.HTTPClient)
	assert.Equal(t, defaultHTTPTimeout, client.client.Timeout)

	t.Run("keeps a custom client", func(t *testing.T) {
		t.Parallel()

		custom := mocks.NewMockHTTPClient(t)
		service := newTestService(t, testServiceURL, custom)

		assert.Same(t, custom, service.HTTPClient)
	})

	t.Run("invalid URL", func(t *testing.T) {
		t.Parallel()

		err := (&Service{}).Initialize(parseTestURL(t, "zulip://"+testBotMail+"@"+testHost), log.New(t.Output(), "", 0))

		require.ErrorIs(t, err, ErrMissingAPIKey)
	})
}

// TestSend verifies the message request a send makes: a form POST to the
// messages endpoint with the bot credentials, carrying the message type,
// recipients, topic, and content built from the URL and params, after a
// register request for the server limits.
func TestSend(t *testing.T) {
	t.Parallel()

	// tests lists sends with the message request URL and form each should
	// produce.
	tests := []struct {
		params   *types.Params
		name     string
		rawURL   string
		message  string
		wantURL  string
		wantForm url.Values
	}{
		{
			name:     "channel message",
			rawURL:   testServiceURL,
			message:  "Hello",
			wantURL:  messagesURL,
			wantForm: url.Values{"type": {"channel"}, "to": {"general"}, "topic": {"alerts"}, "content": {"Hello"}},
		},
		{
			name:     "stream and topic params",
			rawURL:   testServiceURL,
			params:   &types.Params{"stream": "ops", "topic": "disks"},
			message:  "Hello",
			wantURL:  messagesURL,
			wantForm: url.Values{"type": {"channel"}, "to": {"ops"}, "topic": {"disks"}, "content": {"Hello"}},
		},
		{
			name:     "title as the topic",
			rawURL:   "zulip://" + testBotMail + ":" + testBotKey + "@" + testHost + "?stream=general",
			params:   &types.Params{"title": "Disk full"},
			message:  "Hello",
			wantURL:  messagesURL,
			wantForm: url.Values{"type": {"channel"}, "to": {"general"}, "topic": {"Disk full"}, "content": {"Hello"}},
		},
		{
			name:     "title before the message",
			rawURL:   testServiceURL + "&title=Disk%20full",
			message:  "Hello",
			wantURL:  messagesURL,
			wantForm: url.Values{"type": {"channel"}, "to": {"general"}, "topic": {"alerts"}, "content": {"Disk full\n\nHello"}},
		},
		{
			name:     "empty title param",
			rawURL:   testServiceURL,
			params:   &types.Params{"title": ""},
			message:  "Hello",
			wantURL:  messagesURL,
			wantForm: url.Values{"type": {"channel"}, "to": {"general"}, "topic": {"alerts"}, "content": {"Hello"}},
		},
		{
			name:     "read by sender param",
			rawURL:   testServiceURL,
			params:   &types.Params{"read_by_sender": "yes"},
			message:  "Hello",
			wantURL:  messagesURL,
			wantForm: url.Values{"type": {"channel"}, "to": {"general"}, "topic": {"alerts"}, "content": {"Hello"}, "read_by_sender": {"true"}},
		},
		{
			name:     "direct message",
			rawURL:   testServiceURL,
			params:   &types.Params{"type": "direct", "to": "alice@example.com,bob@example.com"},
			message:  "Hello",
			wantURL:  messagesURL,
			wantForm: url.Values{"type": {"direct"}, "to": {`["alice@example.com","bob@example.com"]`}, "content": {"Hello"}},
		},
		{
			name:     "direct message to the stream recipients",
			rawURL:   "zulip://" + testBotMail + ":" + testBotKey + "@" + testHost + "?type=direct&stream=alice@example.com",
			message:  "Hello",
			wantURL:  messagesURL,
			wantForm: url.Values{"type": {"direct"}, "to": {`["alice@example.com"]`}, "content": {"Hello"}},
		},
		{
			name:     "host with a port",
			rawURL:   "zulip://" + testBotMail + ":" + testBotKey + "@" + testHost + ":8443?stream=general",
			message:  "Hello",
			wantURL:  "https://bot%40example.com:" + testBotKey + "@" + testHost + ":8443/api/v1/messages",
			wantForm: url.Values{"type": {"channel"}, "to": {"general"}, "content": {"Hello"}},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			client, fake := newFakeClient(t, defaultLimits, sent)
			service := newTestService(t, tt.rawURL, client)

			require.NoError(t, service.Send(tt.message, tt.params))

			require.Len(t, fake.registers, 1)
			register := fake.registers[0]
			assert.Equal(t, http.MethodPost, register.method)
			assert.Equal(t, strings.Replace(tt.wantURL, "/api/v1/messages", "/api/v1/register", 1), register.url)
			assert.Equal(t, "application/x-www-form-urlencoded", register.contentType)
			assert.Equal(t, url.Values{"fetch_event_types": {`["realm"]`}}, register.form)
			assert.WithinDuration(t, time.Now().Add(registerTimeout), register.deadline, time.Second)

			require.Len(t, fake.messages, 1)
			request := fake.messages[0]
			assert.Equal(t, http.MethodPost, request.method)
			assert.Equal(t, tt.wantURL, request.url)
			assert.Equal(t, "application/x-www-form-urlencoded", request.contentType)
			assert.Equal(t, tt.wantForm, request.form)
		})
	}
}

// TestSendFetchesLimitsOnce verifies that only the first send asks the
// server for its limits.
func TestSendFetchesLimitsOnce(t *testing.T) {
	t.Parallel()

	client, fake := newFakeClient(t, defaultLimits, sent, sent)
	service := newTestService(t, testServiceURL, client)

	require.NoError(t, service.Send("Hello", nil))
	require.NoError(t, service.Send("Hello again", nil))

	assert.Len(t, fake.registers, 1)
	assert.Len(t, fake.messages, 2)
}

// TestSendServerLimits verifies that the topic and message limits come from
// the register response, with the topic counted in characters, and that a
// send at a limit is accepted.
func TestSendServerLimits(t *testing.T) {
	t.Parallel()

	limits := cannedResponse{statusCode: http.StatusOK, body: `{"result":"success","msg":"","max_message_length":5000,"max_topic_length":30}`}

	// tests lists sends with the error each should produce, or nil.
	tests := []struct {
		wantErr   error
		params    *types.Params
		name      string
		message   string
		wantError string
	}{
		{name: "topic at the limit", params: &types.Params{"topic": strings.Repeat("日", 30)}, message: "Hello"},
		{
			name:      "topic over the limit",
			params:    &types.Params{"topic": strings.Repeat("日", 31)},
			message:   "Hello",
			wantErr:   ErrTopicTooLong,
			wantError: "topic exceeds max length: 30 characters, got 31",
		},
		{name: "message at the limit", message: strings.Repeat("a", 5000)},
		{
			name:      "message over the limit",
			message:   strings.Repeat("a", 5001),
			wantErr:   ErrMessageTooLong,
			wantError: "message exceeds max size: 5000 bytes, got 5001 bytes",
		},
		{
			name:      "title before the message counts",
			params:    &types.Params{"title": "Disk"},
			message:   strings.Repeat("a", 4995),
			wantErr:   ErrMessageTooLong,
			wantError: "message exceeds max size: 5000 bytes, got 5001 bytes",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			var responses []cannedResponse
			if tt.wantErr == nil {
				responses = append(responses, sent)
			}

			client, fake := newFakeClient(t, limits, responses...)
			service := newTestService(t, testServiceURL, client)

			err := service.Send(tt.message, tt.params)

			assert.Len(t, fake.registers, 1)

			if tt.wantErr == nil {
				require.NoError(t, err)
				assert.Len(t, fake.messages, 1)

				return
			}

			require.ErrorIs(t, err, tt.wantErr)
			require.EqualError(t, err, tt.wantError)
			assert.Empty(t, fake.messages)
		})
	}
}

// TestSendDefaultLimits verifies that the default limits of 60 topic
// characters and 10000 message bytes apply when the register request fails or
// its response has no usable limits.
func TestSendDefaultLimits(t *testing.T) {
	t.Parallel()

	// registers lists register responses that leave the default limits.
	registers := []struct {
		name     string
		response cannedResponse
	}{
		{
			name:     "error status",
			response: cannedResponse{statusCode: http.StatusServiceUnavailable, body: `{"result":"error","max_message_length":5,"max_topic_length":5}`},
		},
		{name: "transport error", response: cannedResponse{err: errTransport}},
		{name: "not JSON", response: cannedResponse{statusCode: http.StatusOK, body: "<html>"}},
		{name: "no limits", response: cannedResponse{statusCode: http.StatusOK, body: `{"result":"success","msg":""}`}},
		{name: "zero limits", response: cannedResponse{statusCode: http.StatusOK, body: `{"result":"success","max_message_length":0,"max_topic_length":0}`}},
	}

	for _, register := range registers {
		t.Run(register.name, func(t *testing.T) {
			t.Parallel()

			client, fake := newFakeClient(t, register.response, sent, sent)
			service := newTestService(t, testServiceURL, client)

			err := service.Send("Hello", &types.Params{"topic": strings.Repeat("a", 61)})
			require.ErrorIs(t, err, ErrTopicTooLong)
			require.EqualError(t, err, "topic exceeds max length: 60 characters, got 61")

			err = service.Send(strings.Repeat("a", 10001), nil)
			require.ErrorIs(t, err, ErrMessageTooLong)
			require.EqualError(t, err, "message exceeds max size: 10000 bytes, got 10001 bytes")

			require.NoError(t, service.Send(strings.Repeat("a", 10000), &types.Params{"topic": strings.Repeat("日", 60)}))
			require.NoError(t, service.Send("Hello", &types.Params{"topic": strings.Repeat("a", 60)}))

			assert.Len(t, fake.registers, 1)
			assert.Len(t, fake.messages, 2)
		})
	}
}

// TestSendValidation verifies that a send rejects an invalid message type,
// host, or param before any request, and a missing recipient before the
// message request.
func TestSendValidation(t *testing.T) {
	t.Parallel()

	// rejected lists sends that fail before any request.
	rejected := []struct {
		wantErr error
		params  *types.Params
		name    string
		host    string
	}{
		{name: "invalid message type", params: &types.Params{"type": "group"}, wantErr: ErrInvalidMessageType},
		{name: "unknown param", params: &types.Params{"unknown": "value"}, wantErr: format.ErrInvalidConfigKey},
		{name: "invalid read by sender param", params: &types.Params{"read_by_sender": "maybe"}, wantErr: format.ErrInvalidBoolValue},
		{name: "port out of range", host: testHost + ":70000", wantErr: ErrInvalidHost},
		{name: "port zero", host: testHost + ":0", wantErr: ErrInvalidHost},
		{name: "host with a space", host: "zulip example.com", wantErr: ErrInvalidHost},
	}

	for _, tt := range rejected {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			// The mock has no expectations, so a request fails the test.
			service := newTestService(t, testServiceURL, mocks.NewMockHTTPClient(t))
			if tt.host != "" {
				service.Config.Host = tt.host
			}

			require.ErrorIs(t, service.Send("Hello", tt.params), tt.wantErr)
		})
	}

	// unaddressed lists sends without a recipient.
	unaddressed := []struct {
		params *types.Params
		name   string
		rawURL string
	}{
		{name: "channel message without a stream", rawURL: "zulip://" + testBotMail + ":" + testBotKey + "@" + testHost},
		{name: "direct message without recipients", rawURL: "zulip://" + testBotMail + ":" + testBotKey + "@" + testHost + "?type=direct"},
		{name: "stream cleared by a param", rawURL: testServiceURL, params: &types.Params{"stream": ""}},
	}

	for _, tt := range unaddressed {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			client, fake := newFakeClient(t, defaultLimits)
			service := newTestService(t, tt.rawURL, client)

			require.ErrorIs(t, service.Send("Hello", tt.params), ErrMissingRecipient)
			assert.Empty(t, fake.messages)
		})
	}
}

// TestSendErrors verifies that a send reports an error status from the
// messages endpoint with the response body, cut at 4 KiB, and a transport
// error without the bot credentials.
func TestSendErrors(t *testing.T) {
	t.Parallel()

	// tests lists message responses with the error each should produce.
	tests := []struct {
		wantErr   error
		name      string
		response  cannedResponse
		wantError string
	}{
		{
			name:      "API error",
			response:  cannedResponse{statusCode: http.StatusBadRequest, body: `{"result":"error","msg":"Channel 'nope' does not exist","code":"STREAM_DOES_NOT_EXIST"}`},
			wantErr:   ErrResponseStatusFailure,
			wantError: `failed to send zulip message: response status code unexpected: 400, body: {"result":"error","msg":"Channel 'nope' does not exist","code":"STREAM_DOES_NOT_EXIST"}`,
		},
		{
			name:      "error status without a body",
			response:  cannedResponse{statusCode: http.StatusUnauthorized},
			wantErr:   ErrResponseStatusFailure,
			wantError: "failed to send zulip message: response status code unexpected: 401",
		},
		{
			name:      "long error body",
			response:  cannedResponse{statusCode: http.StatusBadGateway, body: strings.Repeat("a", maxErrorBodySize) + "tail"},
			wantErr:   ErrResponseStatusFailure,
			wantError: "failed to send zulip message: response status code unexpected: 502, body: " + strings.Repeat("a", maxErrorBodySize) + " [truncated]",
		},
		{
			name:      "transport error",
			response:  cannedResponse{err: errTransport},
			wantErr:   errTransport,
			wantError: `making HTTP POST request: Post "https://zulip.example.com": transport failure`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			client, _ := newFakeClient(t, defaultLimits, tt.response)
			service := newTestService(t, testServiceURL, client)

			err := service.Send("Hello", nil)

			require.ErrorIs(t, err, tt.wantErr)
			assert.EqualError(t, err, tt.wantError)
		})
	}
}

// TestSendLeavesConfigUnchanged verifies that params apply to one send only
// and leave the caller's params unchanged.
func TestSendLeavesConfigUnchanged(t *testing.T) {
	t.Parallel()

	client, fake := newFakeClient(t, defaultLimits, sent, sent)
	service := newTestService(t, testServiceURL, client)
	want := service.Config.Clone()
	params := types.Params{"type": "direct", "to": "alice@example.com", "title": "Disk", "read_by_sender": "yes"}

	require.NoError(t, service.Send("Hello", &params))
	require.NoError(t, service.Send("Hello", nil))

	assert.Equal(t, want, service.Config)
	assert.Equal(t, types.Params{"type": "direct", "to": "alice@example.com", "title": "Disk", "read_by_sender": "yes"}, params)
	require.Len(t, fake.messages, 2)
	assert.Equal(t, url.Values{"type": {"channel"}, "to": {"general"}, "topic": {"alerts"}, "content": {"Hello"}}, fake.messages[1].form)
}

// TestSendContextCancellation verifies that the caller's context bounds the
// requests of a send.
func TestSendContextCancellation(t *testing.T) {
	t.Parallel()

	ctx, cancel := context.WithCancel(t.Context())
	cancel()

	client := mocks.NewMockHTTPClient(t)
	client.EXPECT().Do(mock.Anything).RunAndReturn(func(req *http.Request) (*http.Response, error) {
		return nil, req.Context().Err()
	}).Twice()

	service := newTestService(t, testServiceURL, client)

	require.ErrorIs(t, service.SendContext(ctx, "Hello", nil), context.Canceled)
}

// TestSendWithContext verifies that the deprecated SendWithContext sends like
// SendContext.
func TestSendWithContext(t *testing.T) {
	t.Parallel()

	client, fake := newFakeClient(t, defaultLimits, sent)
	service := newTestService(t, testServiceURL, client)

	require.NoError(t, service.SendWithContext(t.Context(), "Hello", nil))
	assert.Len(t, fake.messages, 1)
}

// TestSendDefaultClient verifies that a send without a custom client, or
// after a nil client is set, uses a default client that honors the caller's
// context and keeps the bot credentials out of its errors. The context is
// canceled before the send, so the transport returns before it dials.
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
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			service := &Service{}
			require.NoError(t, service.Initialize(parseTestURL(t, testServiceURL), log.New(t.Output(), "", 0)))
			tt.configure(service)

			ctx, cancel := context.WithCancel(t.Context())
			cancel()

			err := service.SendContext(ctx, "Hello", nil)

			require.ErrorIs(t, err, context.Canceled)
			assert.NotContains(t, err.Error(), testBotKey)
		})
	}
}

// TestDefaultHTTPClientDo verifies that the default client performs a
// request, and wraps a failed request without the URL's credentials.
func TestDefaultHTTPClientDo(t *testing.T) {
	t.Parallel()

	const secretBotMail = "SECRETbot@example.com"

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		user, pass, _ := r.BasicAuth()
		if user != secretBotMail || pass != testBotKey {
			w.WriteHeader(http.StatusUnauthorized)

			return
		}

		_, _ = io.WriteString(w, sentBody)
	}))
	t.Cleanup(server.Close)

	serverURL := parseTestURL(t, server.URL)
	serverURL.User = url.UserPassword(secretBotMail, testBotKey)
	serverURL.Path = "/api/v1/messages"

	client := NewDefaultHTTPClient()

	req, err := http.NewRequestWithContext(t.Context(), http.MethodPost, serverURL.String(), http.NoBody)
	require.NoError(t, err)

	resp, err := client.Do(req)
	require.NoError(t, err)
	assert.Equal(t, http.StatusOK, resp.StatusCode)
	require.NoError(t, resp.Body.Close())

	server.Close()

	req, err = http.NewRequestWithContext(t.Context(), http.MethodPost, serverURL.String(), http.NoBody)
	require.NoError(t, err)

	resp, err = client.Do(req)
	require.Error(t, err)
	assert.Nil(t, resp)
	assert.True(t, strings.HasPrefix(err.Error(), "performing HTTP request: "), err.Error())
	assert.NotContains(t, err.Error(), "SECRETbot")
	assert.NotContains(t, err.Error(), testBotKey)
}

// TestSetHTTPClient verifies that SetHTTPClient stores a custom client and
// restores the default client for nil and a nil *http.Client.
func TestSetHTTPClient(t *testing.T) {
	t.Parallel()

	// tests lists constructors for the clients to set, which build any mock
	// with the subtest's t, and whether the default client should be stored.
	tests := []struct {
		client      func(t *testing.T) types.HTTPClient
		name        string
		wantDefault bool
	}{
		{name: "custom client", client: func(t *testing.T) types.HTTPClient {
			t.Helper()

			return mocks.NewMockHTTPClient(t)
		}},
		{name: "custom *http.Client", client: func(*testing.T) types.HTTPClient { return &http.Client{} }},
		{name: "nil", client: func(*testing.T) types.HTTPClient { return nil }, wantDefault: true},
		{name: "nil *http.Client", client: func(*testing.T) types.HTTPClient { return (*http.Client)(nil) }, wantDefault: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			client := tt.client(t)
			service := &Service{HTTPClient: mocks.NewMockHTTPClient(t)}
			service.SetHTTPClient(client)

			if tt.wantDefault {
				assert.IsType(t, &DefaultHTTPClient{}, service.HTTPClient)

				return
			}

			assert.Same(t, client, service.HTTPClient)
		})
	}
}

// TestValidateHost verifies that a host name with an optional port from 1 to
// 65535 is accepted and that other values are rejected.
func TestValidateHost(t *testing.T) {
	t.Parallel()

	for _, valid := range []string{testHost, testHost + ":1", testHost + ":65535", "chat.zulip-cloud.example.co"} {
		require.NoError(t, validateHost(valid), valid)
	}

	for _, invalid := range []string{"", testHost + ":0", testHost + ":65536", testHost + ":", testHost + ":http", "zulip example.com", "-zulip.example.com"} {
		require.ErrorIs(t, validateHost(invalid), ErrInvalidHost, invalid)
	}
}

// TestDoSendRedactsUnparsableURL verifies that a host that makes the request
// URL unparsable fails before any request, without the bot credentials in the
// error.
func TestDoSendRedactsUnparsableURL(t *testing.T) {
	t.Parallel()

	// The mock has no expectations, so a request fails the test.
	service := &Service{HTTPClient: mocks.NewMockHTTPClient(t)}
	config := &Config{BotMail: "SECRETbot@example.com", BotKey: testBotKey, Host: "bad host", Stream: "general", Topic: "alerts"}

	err := service.doSend(t.Context(), config, "Hello")

	_, isURLErr := errors.AsType[*url.Error](err)
	require.True(t, isURLErr, "error %v is not a *url.Error", err)
	assert.NotContains(t, err.Error(), "SECRETbot")
	assert.NotContains(t, err.Error(), testBotKey)
}
