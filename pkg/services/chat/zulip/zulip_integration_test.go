package zulip_test

import (
	"context"
	"errors"
	"io"
	"log"
	"net"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"sync"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/nicholas-fedor/shoutrrr/pkg/services/chat/zulip"
	"github.com/nicholas-fedor/shoutrrr/pkg/types"
)

// zulipRequest is a request that the fake Zulip server received.
type zulipRequest struct {
	// form is the decoded form body.
	form url.Values
	// method is the HTTP method.
	method string
	// path is the request path.
	path string
	// contentType is the Content-Type header.
	contentType string
	// user is the basic auth user name.
	user string
	// password is the basic auth password.
	password string
}

// cannedResponse is the fake Zulip server's answer to every request to one
// endpoint.
type cannedResponse struct {
	// body is the response body.
	body string
	// statusCode is the response status code.
	statusCode int
}

// fakeZulip is a loopback TLS server that stands in for a Zulip server. It
// records the register and message requests and answers each endpoint with a
// fixed response.
type fakeZulip struct {
	// server is the loopback TLS server.
	server *httptest.Server
	// registers are the register requests, in order.
	registers []zulipRequest
	// messages are the message requests, in order.
	messages []zulipRequest
	// register is the answer to every register request.
	register cannedResponse
	// message is the answer to every message request.
	message cannedResponse
	// mu guards registers and messages.
	mu sync.Mutex
}

// limitSend is one send in a server limits test, with the error it should
// produce.
type limitSend struct {
	// wantErr is the error the send should produce, or nil.
	wantErr error
	// params are the send params.
	params *types.Params
	// message is the message to send.
	message string
}

// Values shared by the integration tests.
const (
	// testBotMail is the bot e-mail address in the test URLs.
	testBotMail = "bot@example.com"
	// testBotKey is the bot API key in the test URLs.
	testBotKey = "SECRETbotKey"
	// formContentType is the content type of Zulip API requests.
	formContentType = "application/x-www-form-urlencoded"
	// registerPath is the path of the Zulip register endpoint.
	registerPath = "/api/v1/register"
	// messagesPath is the path of the Zulip messages endpoint.
	messagesPath = "/api/v1/messages"
	// sentBody is Zulip's response to a sent message.
	sentBody = `{"result":"success","msg":"","id":42}`
	// defaultLimitsBody is a register response with Zulip's default limits.
	defaultLimitsBody = `{"result":"success","msg":"","queue_id":"1760500000:1","max_message_length":10000,"max_topic_length":60}`
)

// Responses shared by the integration tests.
var (
	// sent is the response to a sent message.
	sent = cannedResponse{statusCode: http.StatusOK, body: sentBody}
	// defaultLimits is the register response with Zulip's default limits.
	defaultLimits = cannedResponse{statusCode: http.StatusOK, body: defaultLimitsBody}
)

// Messages returns the message requests received so far.
//
// errConnectionRefused is the error the refused dials report.
var errConnectionRefused = errors.New("connection refused")

// Returns:
//   - []zulipRequest: the requests, in order.
func (f *fakeZulip) Messages() []zulipRequest {
	f.mu.Lock()
	defer f.mu.Unlock()

	return append([]zulipRequest(nil), f.messages...)
}

// Registers returns the register requests received so far.
//
// Returns:
//   - []zulipRequest: the requests, in order.
func (f *fakeZulip) Registers() []zulipRequest {
	f.mu.Lock()
	defer f.mu.Unlock()

	return append([]zulipRequest(nil), f.registers...)
}

// ServeHTTP records a register or message request and answers with the
// endpoint's canned response, or 404 for any other path.
//
// Parameters:
//   - w: the response writer.
//   - r: the request.
func (f *fakeZulip) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseForm(); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)

		return
	}

	user, password, _ := r.BasicAuth()
	request := zulipRequest{
		form:        r.PostForm,
		method:      r.Method,
		path:        r.URL.Path,
		contentType: r.Header.Get("Content-Type"),
		user:        user,
		password:    password,
	}

	var response cannedResponse

	f.mu.Lock()

	switch r.URL.Path {
	case registerPath:
		f.registers = append(f.registers, request)
		response = f.register
	case messagesPath:
		f.messages = append(f.messages, request)
		response = f.message
	default:
		response = cannedResponse{statusCode: http.StatusNotFound, body: `{"result":"error","msg":"Not found"}`}
	}

	f.mu.Unlock()

	w.WriteHeader(response.statusCode)
	_, _ = io.WriteString(w, response.body)
}

// newFakeZulip starts a fake Zulip server that answers register requests
// with register and message requests with message, and stops when the test
// ends.
//
// Parameters:
//   - t: the test.
//   - register: the answer to every register request.
//   - message: the answer to every message request.
//
// Returns:
//   - *fakeZulip: the fake server.
func newFakeZulip(t *testing.T, register, message cannedResponse) *fakeZulip {
	t.Helper()

	fake := &fakeZulip{register: register, message: message}
	fake.server = httptest.NewTLSServer(fake)
	t.Cleanup(fake.server.Close)

	return fake
}

// serviceURL returns a Zulip service URL with the test bot credentials for
// the host of server.
//
// Parameters:
//   - server: the server the URL points at.
//   - query: the URL query, including the leading "?", or empty.
//
// Returns:
//   - string: the service URL.
func serviceURL(server *httptest.Server, query string) string {
	return "zulip://" + testBotMail + ":" + testBotKey + "@" + server.Listener.Addr().String() + query
}

// refusingClient returns an HTTP client whose every dial is refused, as by a
// host where nothing listens, so no connection leaves the process.
//
// Returns:
//   - *http.Client: the client.
func refusingClient() *http.Client {
	return &http.Client{Transport: &http.Transport{
		DialContext: func(context.Context, string, string) (net.Conn, error) {
			return nil, &net.OpError{Op: "dial", Net: "tcp", Err: errConnectionRefused}
		},
	}}
}

// newTestService returns a service initialized from rawURL that sends
// through client and logs to the test output.
//
// Parameters:
//   - t: the test.
//   - rawURL: the service URL.
//   - client: the HTTP client.
//
// Returns:
//   - *zulip.Service: the service.
func newTestService(t *testing.T, rawURL string, client *http.Client) *zulip.Service {
	t.Helper()

	parsed, err := url.Parse(rawURL)
	require.NoError(t, err)

	service := &zulip.Service{}
	require.NoError(t, service.Initialize(parsed, log.New(t.Output(), "", 0)))
	service.SetHTTPClient(client)

	return service
}

// TestIntegrationInitializeErrors verifies that Initialize rejects a service
// URL without a bot e-mail address or host.
func TestIntegrationInitializeErrors(t *testing.T) {
	t.Parallel()

	// tests lists service URLs with the error Initialize should return.
	tests := []struct {
		wantErr error
		name    string
		rawURL  string
	}{
		{name: "no bot e-mail address", rawURL: "zulip://:" + testBotKey + "@zulip.example.com", wantErr: zulip.ErrMissingBotMail},
		{name: "no host", rawURL: "zulip://" + testBotMail + ":" + testBotKey + "@", wantErr: zulip.ErrMissingHost},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			parsed, err := url.Parse(tt.rawURL)
			require.NoError(t, err)

			err = (&zulip.Service{}).Initialize(parsed, log.New(t.Output(), "", 0))
			require.ErrorIs(t, err, tt.wantErr)
		})
	}
}

// TestIntegrationSend verifies that a send initialized from a service URL
// asks the server for its limits and then posts one message over HTTPS with
// the bot credentials as basic auth, carrying the message type, recipients,
// topic, and content built from the URL and params.
func TestIntegrationSend(t *testing.T) {
	t.Parallel()

	// tests lists service URL queries, params, and messages with the message
	// form each send should post.
	tests := []struct {
		params   *types.Params
		wantForm url.Values
		name     string
		query    string
		message  string
	}{
		{
			name:     "channel message from the URL",
			query:    "?stream=general&topic=announcements",
			message:  "Hello, World!",
			wantForm: url.Values{"type": {"channel"}, "to": {"general"}, "topic": {"announcements"}, "content": {"Hello, World!"}},
		},
		{
			name:     "URL topic with an encoded space",
			query:    "?stream=general&topic=hello+world",
			message:  "Test message",
			wantForm: url.Values{"type": {"channel"}, "to": {"general"}, "topic": {"hello world"}, "content": {"Test message"}},
		},
		{
			name:     "stream and topic from params",
			params:   &types.Params{"stream": "general", "topic": "announcements"},
			message:  "Test message",
			wantForm: url.Values{"type": {"channel"}, "to": {"general"}, "topic": {"announcements"}, "content": {"Test message"}},
		},
		{
			name:     "title param as the topic",
			query:    "?stream=general",
			params:   &types.Params{"title": "Fallback Topic"},
			message:  "body content",
			wantForm: url.Values{"type": {"channel"}, "to": {"general"}, "topic": {"Fallback Topic"}, "content": {"body content"}},
		},
		{
			name:     "title before the message with a topic",
			query:    "?stream=general&topic=explicit",
			params:   &types.Params{"title": "Notification Title"},
			message:  "body content",
			wantForm: url.Values{"type": {"channel"}, "to": {"general"}, "topic": {"explicit"}, "content": {"Notification Title\n\nbody content"}},
		},
		{
			name:     "no topic with non-ASCII and markup content",
			query:    "?stream=general",
			message:  "Hello 世界 🌍 <script>alert('xss')</script>",
			wantForm: url.Values{"type": {"channel"}, "to": {"general"}, "content": {"Hello 世界 🌍 <script>alert('xss')</script>"}},
		},
		{
			name:     "empty message",
			query:    "?stream=general",
			wantForm: url.Values{"type": {"channel"}, "to": {"general"}, "content": {""}},
		},
		{
			name:    "direct message read by the sender",
			params:  &types.Params{"type": "direct", "to": "user1@example.com,user2@example.com", "read_by_sender": "true"},
			message: "Direct hello",
			wantForm: url.Values{
				"type":           {"direct"},
				"to":             {`["user1@example.com","user2@example.com"]`},
				"content":        {"Direct hello"},
				"read_by_sender": {"true"},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			fake := newFakeZulip(t, defaultLimits, sent)
			service := newTestService(t, serviceURL(fake.server, tt.query), fake.server.Client())

			require.NoError(t, service.Send(tt.message, tt.params))

			registers := fake.Registers()
			require.Len(t, registers, 1)
			assert.Equal(t, zulipRequest{
				form:        url.Values{"fetch_event_types": {`["realm"]`}},
				method:      http.MethodPost,
				path:        registerPath,
				contentType: formContentType,
				user:        testBotMail,
				password:    testBotKey,
			}, registers[0])

			messages := fake.Messages()
			require.Len(t, messages, 1)
			assert.Equal(t, zulipRequest{
				form:        tt.wantForm,
				method:      http.MethodPost,
				path:        messagesPath,
				contentType: formContentType,
				user:        testBotMail,
				password:    testBotKey,
			}, messages[0])
		})
	}
}

// TestIntegrationSendServerLimits verifies that sends use the message and
// topic limits from the server's register response, keep the default limits
// of 10000 message and 60 topic characters when the register request fails,
// and ask again on each send until the server answers.
func TestIntegrationSendServerLimits(t *testing.T) {
	t.Parallel()

	// tests lists register responses with the sends to make in order and the
	// register and message requests those sends should make.
	tests := []struct {
		name          string
		register      cannedResponse
		sends         []limitSend
		wantRegisters int
		wantMessages  int
	}{
		{
			name: "limits from the server",
			register: cannedResponse{
				statusCode: http.StatusOK,
				body:       `{"result":"success","msg":"","max_message_length":20000,"max_topic_length":120}`,
			},
			sends: []limitSend{
				{message: strings.Repeat("a", 15000)},
				{message: strings.Repeat("a", 20001), wantErr: zulip.ErrMessageTooLong},
				{message: "Hello", params: &types.Params{"topic": strings.Repeat("a", 120)}},
				{message: "Hello", params: &types.Params{"topic": strings.Repeat("a", 121)}, wantErr: zulip.ErrTopicTooLong},
			},
			wantRegisters: 1,
			wantMessages:  2,
		},
		{
			name:     "default limits after an error status",
			register: cannedResponse{statusCode: http.StatusServiceUnavailable},
			sends: []limitSend{
				{message: strings.Repeat("a", 10000)},
				{message: strings.Repeat("a", 10001), wantErr: zulip.ErrMessageTooLong},
				{message: "body content", params: &types.Params{"title": strings.Repeat("a", 61)}, wantErr: zulip.ErrTopicTooLong},
			},
			wantRegisters: 3,
			wantMessages:  1,
		},
		{
			name:     "default limits after a malformed response",
			register: cannedResponse{statusCode: http.StatusOK, body: "not valid json"},
			sends: []limitSend{
				{message: "Test message"},
				{message: strings.Repeat("a", 10001), wantErr: zulip.ErrMessageTooLong},
			},
			wantRegisters: 2,
			wantMessages:  1,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			fake := newFakeZulip(t, tt.register, sent)
			service := newTestService(t, serviceURL(fake.server, "?stream=general"), fake.server.Client())

			for i, send := range tt.sends {
				err := service.Send(send.message, send.params)

				if send.wantErr == nil {
					require.NoError(t, err, "send %d", i)

					continue
				}

				require.ErrorIs(t, err, send.wantErr, "send %d", i)
			}

			assert.Len(t, fake.Registers(), tt.wantRegisters)
			assert.Len(t, fake.Messages(), tt.wantMessages)
		})
	}
}

// TestIntegrationSendErrorStatus verifies that a send reports an error status
// from the messages endpoint as ErrResponseStatusFailure with the status and
// any response body, without the bot API key.
func TestIntegrationSendErrorStatus(t *testing.T) {
	t.Parallel()

	// tests lists message responses with the error each send should report.
	tests := []struct {
		name      string
		response  cannedResponse
		wantError string
	}{
		{
			name:      "bad request",
			response:  cannedResponse{statusCode: http.StatusBadRequest, body: `{"result":"error","msg":"Invalid request","code":"BAD_REQUEST"}`},
			wantError: `failed to send zulip message: response status code unexpected: 400, body: {"result":"error","msg":"Invalid request","code":"BAD_REQUEST"}`,
		},
		{
			name:      "unauthorized",
			response:  cannedResponse{statusCode: http.StatusUnauthorized, body: `{"result":"error","msg":"Invalid API key","code":"INVALID_API_KEY"}`},
			wantError: `failed to send zulip message: response status code unexpected: 401, body: {"result":"error","msg":"Invalid API key","code":"INVALID_API_KEY"}`,
		},
		{
			name:      "service unavailable without a body",
			response:  cannedResponse{statusCode: http.StatusServiceUnavailable},
			wantError: "failed to send zulip message: response status code unexpected: 503",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			fake := newFakeZulip(t, defaultLimits, tt.response)
			service := newTestService(t, serviceURL(fake.server, "?stream=general"), fake.server.Client())

			err := service.Send("Test message", nil)

			require.ErrorIs(t, err, zulip.ErrResponseStatusFailure)
			require.EqualError(t, err, tt.wantError)
			assert.Len(t, fake.Messages(), 1)
		})
	}
}

// TestIntegrationSendConnectionFailure verifies that a send to a server that
// refuses the connection reports the failed message request without the bot
// credentials.
func TestIntegrationSendConnectionFailure(t *testing.T) {
	t.Parallel()

	rawURL := "zulip://" + testBotMail + ":" + testBotKey + "@zulip.example.com?stream=general"

	err := newTestService(t, rawURL, refusingClient()).Send("Test message", nil)

	require.ErrorContains(t, err, "making HTTP POST request")
	assert.NotContains(t, err.Error(), testBotKey)
	assert.NotContains(t, err.Error(), url.QueryEscape(testBotMail))
}

// TestIntegrationSendContextCanceled verifies that canceling the context of
// a send while the server holds the message request ends the send with
// context.Canceled.
func TestIntegrationSendContextCanceled(t *testing.T) {
	t.Parallel()

	received := make(chan struct{})
	server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == registerPath {
			_, _ = io.WriteString(w, defaultLimitsBody)

			return
		}

		// The server watches the connection for the client going away only
		// after the handler reads the whole body.
		_, _ = io.Copy(io.Discard, r.Body)

		close(received)
		<-r.Context().Done()
	}))
	t.Cleanup(server.Close)

	service := newTestService(t, serviceURL(server, "?stream=general"), server.Client())

	ctx, cancel := context.WithCancel(t.Context())
	t.Cleanup(cancel)

	done := make(chan error, 1)

	go func() { done <- service.SendContext(ctx, "Canceled message", nil) }()

	select {
	case <-received:
	case err := <-done:
		require.FailNow(t, "the send returned before the server received the message request", "error: %v", err)
	}

	cancel()

	require.ErrorIs(t, <-done, context.Canceled)
}
