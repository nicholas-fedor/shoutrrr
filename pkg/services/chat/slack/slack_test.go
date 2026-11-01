package slack

import (
	"bytes"
	"context"
	"encoding/json/v2"
	"errors"
	"io"
	"log"
	"net/http"
	"net/url"
	"strings"
	"testing"
	"testing/synctest"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	"github.com/nicholas-fedor/shoutrrr/internal/httpbody"
	"github.com/nicholas-fedor/shoutrrr/pkg/format"
	"github.com/nicholas-fedor/shoutrrr/pkg/types"
	"github.com/nicholas-fedor/shoutrrr/pkg/types/mocks"
	"github.com/nicholas-fedor/shoutrrr/pkg/util/jsonclient"
)

// failingReader is a response body whose reads fail.
type failingReader struct{}

// recordedRequest is a request a recording client received.
type recordedRequest struct {
	// header is the request's header.
	header http.Header
	// method is the request method.
	method string
	// url is the request URL.
	url string
	// body is the request body.
	body []byte
}

// Service URLs and endpoints shared by the service tests.
const (
	// testWebhookServiceURL is a webhook service URL in the current format.
	testWebhookServiceURL = "slack://hook:AAAAAAAAA-BBBBBBBBB-123456789123456789123456@webhook"
	// testAPIServiceURL is a bot API service URL in the current format.
	testAPIServiceURL = "slack://xoxb:123456789012-1234567890123-4mt0t4l1YL3g1T5L4cK70k3N@C0123456789"
	// testWebhookURL is the webhook URL that testWebhookServiceURL posts to.
	testWebhookURL = "https://hooks.slack.com/services/AAAAAAAAA/BBBBBBBBB/123456789123456789123456"
)

// errTransport is the error a failing client returns.
var errTransport = errors.New("connection refused")

// newRecordingClient returns a client that records each request and answers
// it with status and body.
//
// Parameters:
//   - t: the test.
//   - status: the response status code.
//   - body: the response body.
//
// Returns:
//   - *mocks.MockHTTPClient: the client.
//   - *[]recordedRequest: the requests the client received.
func newRecordingClient(t *testing.T, status int, body string) (*mocks.MockHTTPClient, *[]recordedRequest) {
	t.Helper()

	var requests []recordedRequest

	client := mocks.NewMockHTTPClient(t)
	client.EXPECT().Do(mock.Anything).RunAndReturn(func(req *http.Request) (*http.Response, error) {
		payload, err := io.ReadAll(req.Body)
		if err != nil {
			return nil, err
		}

		requests = append(requests, recordedRequest{header: req.Header.Clone(), method: req.Method, url: req.URL.String(), body: payload})

		return &http.Response{
			StatusCode: status,
			Status:     http.StatusText(status),
			Header:     http.Header{"Content-Type": {"application/json"}},
			Body:       io.NopCloser(strings.NewReader(body)),
			Request:    req,
		}, nil
	}).Maybe()

	return client, &requests
}

// newTestService returns a service initialized from rawURL that sends
// through client and logs to logs.
//
// Parameters:
//   - t: the test.
//   - rawURL: the service URL.
//   - client: the HTTP client to send through.
//   - logs: where the service logs.
//
// Returns:
//   - *Service: the service.
func newTestService(t *testing.T, rawURL string, client types.HTTPClient, logs io.Writer) *Service {
	t.Helper()

	serviceURL, err := url.Parse(rawURL)
	require.NoError(t, err)

	service := &Service{}
	require.NoError(t, service.Initialize(serviceURL, log.New(logs, "", 0)))
	service.SetHTTPClient(client)

	return service
}

// TestGetID verifies the service identifier.
func TestGetID(t *testing.T) {
	t.Parallel()

	assert.Equal(t, "slack", (&Service{}).GetID())
}

// TestInitialize verifies that Initialize parses the URL into the config and
// that the service's default client has a timeout.
func TestInitialize(t *testing.T) {
	t.Parallel()

	serviceURL, err := url.Parse(testAPIServiceURL + "?title=Alert")
	require.NoError(t, err)

	service := &Service{}
	require.NoError(t, service.Initialize(serviceURL, nil))

	assert.Equal(t, "C0123456789", service.Config.Channel)
	assert.Equal(t, "Alert", service.Config.Title)
	assert.Equal(t, "xoxb:123456789012-1234567890123-4mt0t4l1YL3g1T5L4cK70k3N", service.Config.Token.String())

	client, ok := service.client.Client().(*http.Client)
	require.True(t, ok, "the default client should be an *http.Client")
	assert.Zero(t, client.Timeout, "the send budget bounds each send")
}

// TestInitializeErrors verifies that a token with a part that is too short or
// missing, or an unknown query key, fails initialization.
func TestInitializeErrors(t *testing.T) {
	t.Parallel()

	// tests lists invalid service URLs with the error each should produce.
	tests := []struct {
		wantErr error
		name    string
		rawURL  string
	}{
		{name: "short part A", rawURL: "slack://lol@12345678/123456789/123456789123456789123456", wantErr: ErrInvalidToken},
		{name: "short part B", rawURL: "slack://lol@123456789/12345678/123456789123456789123456", wantErr: ErrInvalidToken},
		{name: "short part C", rawURL: "slack://123456789/123456789/12345678912345678912345", wantErr: ErrInvalidToken},
		{name: "missing part A", rawURL: "slack://lol@/123456789/123456789123456789123456", wantErr: ErrInvalidToken},
		{name: "missing part B", rawURL: "slack://lol@123456789//123456789", wantErr: ErrInvalidToken},
		{name: "missing part C", rawURL: "slack://lol@123456789/123456789/", wantErr: ErrInvalidToken},
		{name: "unknown key", rawURL: testWebhookServiceURL + "?bass=dirty", wantErr: format.ErrInvalidConfigKey},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			serviceURL, err := url.Parse(tt.rawURL)
			require.NoError(t, err)

			require.ErrorIs(t, (&Service{}).Initialize(serviceURL, nil), tt.wantErr)
		})
	}
}

// TestSendWebhook verifies that a webhook send posts the JSON payload to the
// webhook URL and succeeds on an "ok" or empty 200 response.
func TestSendWebhook(t *testing.T) {
	t.Parallel()

	for _, body := range []string{"ok", ""} {
		t.Run(body, func(t *testing.T) {
			t.Parallel()

			client, requests := newRecordingClient(t, http.StatusOK, body)
			service := newTestService(t, testWebhookServiceURL+"?botname=testbot&color=3f00fe&title=Alert", client, io.Discard)

			require.NoError(t, service.Send("Line 1\nLine 2", nil))

			require.Len(t, *requests, 1)

			request := (*requests)[0]
			assert.Equal(t, http.MethodPost, request.method)
			assert.Equal(t, testWebhookURL, request.url)
			assert.Equal(t, "application/json", request.header.Get("Content-Type"))
			assert.Empty(t, request.header.Get("Authorization"))
			assert.JSONEq(t,
				`{"text":"Alert","username":"testbot","attachments":[{"text":"Line 1","color":"3f00fe"},{"text":"Line 2","color":"3f00fe"}]}`,
				string(request.body))
		})
	}
}

// TestSendWebhookErrors verifies that a webhook send fails with the error
// code the webhook returns, the status of an empty error response, the
// transport error without the webhook secret, or a body read error. An error
// code in a body over [httpbody.MaxSize] is still reported.
func TestSendWebhookErrors(t *testing.T) {
	t.Parallel()

	// tests lists webhook responses with the error each send should produce.
	tests := []struct {
		wantErr  error
		name     string
		body     string
		wantText string
		status   int
	}{
		{name: "error code", status: http.StatusBadRequest, body: "invalid_payload", wantErr: ErrWebhookResponseFailure, wantText: "invalid_payload"},
		{name: "error code with OK status", status: http.StatusOK, body: "no_text", wantErr: ErrWebhookResponseFailure, wantText: "no_text"},
		{name: "empty error response", status: http.StatusInternalServerError, body: "", wantErr: ErrWebhookStatusFailure, wantText: "Internal Server Error"},
		{
			name:     "oversized error code",
			status:   http.StatusBadRequest,
			body:     "invalid_payload" + strings.Repeat(" ", httpbody.MaxSize),
			wantErr:  ErrWebhookResponseFailure,
			wantText: "invalid_payload",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			client, _ := newRecordingClient(t, tt.status, tt.body)

			err := newTestService(t, testWebhookServiceURL, client, io.Discard).Send("Message", nil)

			require.ErrorIs(t, err, tt.wantErr)
			require.ErrorContains(t, err, tt.wantText)
		})
	}

	t.Run("transport error", func(t *testing.T) {
		t.Parallel()

		client := mocks.NewMockHTTPClient(t)
		client.EXPECT().Do(mock.Anything).RunAndReturn(func(req *http.Request) (*http.Response, error) {
			return nil, &url.Error{Op: "Post", URL: req.URL.String(), Err: errTransport}
		}).Once()

		err := newTestService(t, testWebhookServiceURL, client, io.Discard).Send("Message", nil)

		require.ErrorIs(t, err, errTransport)
		assert.NotContains(t, err.Error(), "123456789123456789123456")
	})

	t.Run("body read error", func(t *testing.T) {
		t.Parallel()

		client := mocks.NewMockHTTPClient(t)
		client.EXPECT().Do(mock.Anything).Return(&http.Response{
			StatusCode: http.StatusOK,
			Body:       io.NopCloser(failingReader{}),
		}, nil).Once()

		err := newTestService(t, testWebhookServiceURL, client, io.Discard).Send("Message", nil)

		require.ErrorIs(t, err, errTransport)
		require.ErrorContains(t, err, "failed to read response body")
	})
}

// TestSendWebhookTimeout verifies that a webhook request that never answers
// ends when the send budget runs out.
func TestSendWebhookTimeout(t *testing.T) {
	t.Parallel()

	synctest.Test(t, func(t *testing.T) {
		client := mocks.NewMockHTTPClient(t)
		client.EXPECT().Do(mock.Anything).RunAndReturn(func(req *http.Request) (*http.Response, error) {
			<-req.Context().Done()

			return nil, req.Context().Err()
		}).Once()

		service := newTestService(t, testWebhookServiceURL, client, io.Discard)
		start := time.Now()

		err := service.Send("Message", nil)

		require.ErrorIs(t, err, context.DeadlineExceeded)
		assert.Equal(t, service.ServiceTimeout(nil), time.Since(start))
	})
}

// TestSendAPI verifies that an API send posts the JSON payload with the
// channel to chat.postMessage, authorized with the bot token, and logs a
// warning the API returns with a successful response.
func TestSendAPI(t *testing.T) {
	t.Parallel()

	var logs bytes.Buffer

	client, requests := newRecordingClient(t, http.StatusOK, `{"ok":true,"warning":"missing_charset"}`)
	service := newTestService(t, testAPIServiceURL+"?icon=:robot_face:&thread_ts=1503435956.000247", client, &logs)

	require.NoError(t, service.Send("Message", nil))

	require.Len(t, *requests, 1)

	request := (*requests)[0]
	assert.Equal(t, http.MethodPost, request.method)
	assert.Equal(t, apiPostMessage, request.url)
	assert.Equal(t, "Bearer xoxb-123456789012-1234567890123-4mt0t4l1YL3g1T5L4cK70k3N", request.header.Get("Authorization"))
	assert.Equal(t, "application/json", request.header.Get("Content-Type"))
	assert.JSONEq(t,
		`{"text":"","channel":"C0123456789","thread_ts":"1503435956.000247","icon_emoji":":robot_face:","attachments":[{"text":"Message"}]}`,
		string(request.body))
	assert.Equal(t, "WARN API returned a warning warning=missing_charset\n", logs.String())
}

// TestSendAPIErrors verifies that an API send fails with the error code the
// API returns, an unknown error when it returns none, the HTTP status of a
// failed request, and the transport error.
func TestSendAPIErrors(t *testing.T) {
	t.Parallel()

	// tests lists API responses with the error each send should produce.
	tests := []struct {
		wantErr  error
		name     string
		body     string
		wantText string
		status   int
	}{
		{name: "error code", status: http.StatusOK, body: `{"ok":false,"error":"channel_not_found"}`, wantErr: ErrAPIResponseFailure, wantText: "channel_not_found"},
		{name: "no error code", status: http.StatusOK, body: `{"ok":false}`, wantErr: ErrUnknownAPIError},
		{
			name: "HTTP error", status: http.StatusTooManyRequests, body: `{"ok":false,"error":"ratelimited"}`,
			wantErr: jsonclient.ErrUnexpectedStatus, wantText: "Too Many Requests",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			client, _ := newRecordingClient(t, tt.status, tt.body)

			err := newTestService(t, testAPIServiceURL, client, io.Discard).Send("Message", nil)

			require.Error(t, err)

			if tt.wantErr != nil {
				require.ErrorIs(t, err, tt.wantErr)
			}

			require.ErrorContains(t, err, tt.wantText)
		})
	}

	t.Run("transport error", func(t *testing.T) {
		t.Parallel()

		client := mocks.NewMockHTTPClient(t)
		client.EXPECT().Do(mock.Anything).Return(nil, errTransport).Once()

		err := newTestService(t, testAPIServiceURL, client, io.Discard).Send("Message", nil)

		require.ErrorIs(t, err, errTransport)
		assert.NotContains(t, err.Error(), "4mt0t4l1YL3g1T5L4cK70k3N")
	})
}

// TestSendParams verifies that params apply to a single send without changing
// the service config, and that an unknown param fails the send.
func TestSendParams(t *testing.T) {
	t.Parallel()

	client, requests := newRecordingClient(t, http.StatusOK, "ok")
	service := newTestService(t, testWebhookServiceURL+"?title=Alert", client, io.Discard)
	before := *service.Config

	require.NoError(t, service.Send("Message", &types.Params{"title": "Override", "color": "good"}))

	require.Len(t, *requests, 1)

	var payload MessagePayload

	require.NoError(t, json.Unmarshal((*requests)[0].body, &payload))
	assert.Equal(t, "Override", payload.Text)
	assert.Equal(t, "good", payload.Attachments[0].Color)
	assert.Equal(t, before, *service.Config)

	err := service.Send("Message", &types.Params{"bass": "dirty"})

	require.ErrorContains(t, err, "updating config from params")
	assert.Len(t, *requests, 1)
}

// TestSendChecksConfig verifies that a send with a config built in code
// without a token fails before any request, as reading a URL without one does.
func TestSendChecksConfig(t *testing.T) {
	t.Parallel()

	service := newTestService(t, testWebhookServiceURL, mocks.NewMockHTTPClient(t), io.Discard)
	service.Config = &Config{}

	require.ErrorIs(t, service.Send("Message", nil), ErrInvalidToken)
}

// TestSendContextCanceled verifies that a canceled context stops the request
// through either path.
func TestSendContextCanceled(t *testing.T) {
	t.Parallel()

	for _, rawURL := range []string{testAPIServiceURL, testWebhookServiceURL} {
		t.Run(rawURL, func(t *testing.T) {
			t.Parallel()

			ctx, cancel := context.WithCancel(t.Context())
			cancel()

			client := mocks.NewMockHTTPClient(t)
			client.EXPECT().Do(mock.Anything).RunAndReturn(func(req *http.Request) (*http.Response, error) {
				return nil, req.Context().Err()
			}).Once()

			err := newTestService(t, rawURL, client, io.Discard).SendContext(ctx, "Message", nil)

			require.ErrorIs(t, err, context.Canceled)
		})
	}
}

// TestSetHTTPClient verifies that the injected client is used, and that a nil
// client, including a nil *http.Client, selects the default client.
func TestSetHTTPClient(t *testing.T) {
	t.Parallel()

	client := mocks.NewMockHTTPClient(t)
	service := newTestService(t, testWebhookServiceURL, client, io.Discard)
	assert.Same(t, client, service.client.Client())

	service.SetHTTPClient(nil)

	defaultClient := service.client.Client()
	assert.IsType(t, &http.Client{}, defaultClient)

	service.SetHTTPClient(client)
	service.SetHTTPClient((*http.Client)(nil))
	assert.Same(t, defaultClient, service.client.Client())
}

// Read fails with errTransport.
//
// Parameters:
//   - []byte: unused.
//
// Returns:
//   - int: always 0.
//   - error: always errTransport.
func (failingReader) Read([]byte) (int, error) {
	return 0, errTransport
}
