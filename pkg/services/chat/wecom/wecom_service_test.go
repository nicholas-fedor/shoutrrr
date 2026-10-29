package wecom

import (
	"context"
	"encoding/json/jsontext"
	"encoding/json/v2"
	"errors"
	"fmt"
	"io"
	"log"
	"net/http"
	"strings"
	"testing"
	"testing/iotest"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	"github.com/nicholas-fedor/shoutrrr/pkg/format"
	"github.com/nicholas-fedor/shoutrrr/pkg/types"
	"github.com/nicholas-fedor/shoutrrr/pkg/types/mocks"
)

// roundTripFunc is an [http.RoundTripper] backed by a function, so a test can
// answer requests from an [http.Client] without a network.
type roundTripFunc func(req *http.Request) (*http.Response, error)

// capturedRequest records the request a fake transport received.
type capturedRequest struct {
	// method is the request method.
	method string
	// url is the full request URL.
	url string
	// contentType is the request's Content-Type header.
	contentType string
	// body is the request body.
	body []byte
}

// testPostURL is the webhook API URL for testKey.
const testPostURL = "https://qyapi.weixin.qq.com/cgi-bin/webhook/send?key=" + testKey

// Errors the fake transports return.
var (
	// errNetwork is the transport error for a failed request.
	errNetwork = errors.New("network error")
	// errRead is the error a response body returns when read.
	errRead = errors.New("read error")
)

// RoundTrip answers req by calling f.
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

// newResponse returns an HTTP response with the given status code and body.
//
// Parameters:
//   - code: the HTTP status code.
//   - body: the response body.
//
// Returns:
//   - *http.Response: the response.
func newResponse(code int, body string) *http.Response {
	return &http.Response{
		Status:     fmt.Sprintf("%d %s", code, http.StatusText(code)),
		StatusCode: code,
		Header:     http.Header{},
		Body:       io.NopCloser(strings.NewReader(body)),
	}
}

// newRecordingClient returns an HTTP client that answers every request with
// resp and records the last request it received.
//
// Parameters:
//   - t: the test.
//   - resp: the response to return.
//
// Returns:
//   - *http.Client: the client.
//   - *capturedRequest: the recorded request, filled in when a request is sent.
func newRecordingClient(t *testing.T, resp *http.Response) (*http.Client, *capturedRequest) {
	t.Helper()

	captured := &capturedRequest{}
	client := &http.Client{Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
		body, err := io.ReadAll(req.Body)
		if err != nil {
			return nil, fmt.Errorf("reading request body: %w", err)
		}

		captured.method = req.Method
		captured.url = req.URL.String()
		captured.contentType = req.Header.Get("Content-Type")
		captured.body = body

		return resp, nil
	})}

	return client, captured
}

// newTestService returns a service initialized from fullURL that sends
// through client and logs to logger.
//
// Parameters:
//   - t: the test.
//   - client: the HTTP client to inject.
//   - logger: the logger for the service.
//
// Returns:
//   - *Service: the service.
func newTestService(t *testing.T, client types.HTTPClient, logger types.StdLogger) *Service {
	t.Helper()

	service := &Service{}
	require.NoError(t, service.Initialize(parseTestURL(t, fullURL), logger))
	service.SetHTTPClient(client)

	return service
}

// TestGetID verifies that the service reports the wecom scheme as its ID.
func TestGetID(t *testing.T) {
	t.Parallel()

	assert.Equal(t, Scheme, (&Service{}).GetID())
}

// TestServiceTimeout verifies that the service reports its default HTTP
// timeout.
func TestServiceTimeout(t *testing.T) {
	t.Parallel()

	assert.Equal(t, defaultTime, (&Service{}).ServiceTimeout(nil))
}

// TestInitialize verifies that Initialize sets the key from the URL and
// rejects empty and malformed keys without echoing them.
func TestInitialize(t *testing.T) {
	t.Parallel()

	// tests lists service URLs with the key or sentinel error each should
	// produce.
	tests := []struct {
		name    string
		rawURL  string
		wantKey string
		wantErr error
	}{
		{name: "valid key", rawURL: fullURL, wantKey: testKey},
		{name: "invalid key format", rawURL: "wecom://invalid.key", wantErr: ErrInvalidKey},
		{name: "empty key", rawURL: "wecom://", wantErr: ErrEmptyKey},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			service := &Service{}
			err := service.Initialize(parseTestURL(t, tt.rawURL), log.New(t.Output(), "", 0))

			if tt.wantErr != nil {
				require.ErrorIs(t, err, tt.wantErr)
				assert.NotContains(t, err.Error(), "invalid.key")

				return
			}

			require.NoError(t, err)
			assert.Equal(t, tt.wantKey, service.Config.Key)
		})
	}
}

// TestSetHTTPClient verifies which client a send uses after SetHTTPClient,
// including the default client for a nil interface and a nil *http.Client.
func TestSetHTTPClient(t *testing.T) {
	t.Parallel()

	t.Run("custom client", func(t *testing.T) {
		t.Parallel()

		custom := mocks.NewMockHTTPClient(t)
		service := newTestService(t, nil, log.New(t.Output(), "", 0))
		service.SetHTTPClient(custom)

		assert.Same(t, custom, service.client.Client())
	})

	// nilClients lists the nil clients that restore the default client.
	nilClients := []struct {
		name   string
		client types.HTTPClient
	}{
		{name: "nil interface", client: nil},
		{name: "nil *http.Client", client: (*http.Client)(nil)},
	}

	for _, tt := range nilClients {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			service := newTestService(t, nil, log.New(t.Output(), "", 0))
			defaultClient := service.client.Client()

			service.SetHTTPClient(mocks.NewMockHTTPClient(t))
			service.SetHTTPClient(tt.client)

			assert.Same(t, defaultClient, service.client.Client())
		})
	}
}

// TestDefaultHTTPClient verifies that a service without a custom client uses
// one default client without a request timeout, since the send budget bounds
// each send.
func TestDefaultHTTPClient(t *testing.T) {
	t.Parallel()

	service := newTestService(t, nil, log.New(t.Output(), "", 0))

	client, ok := service.client.Client().(*http.Client)
	require.True(t, ok, "the default client should be an *http.Client")
	assert.Zero(t, client.Timeout)
	assert.Same(t, client, service.client.Client())
}

// TestSendSuccess verifies the request a successful send makes for plain
// messages, messages at the size limit, and mention params, and that the log
// output holds neither the webhook key nor the request body.
func TestSendSuccess(t *testing.T) {
	t.Parallel()

	// tests lists messages and params with the payload the send should post.
	tests := []struct {
		name     string
		message  string
		params   *types.Params
		wantBody RequestBody
	}{
		{
			name:     "plain message",
			message:  "message",
			wantBody: RequestBody{MsgType: "text", Text: TextContent{Content: "message"}},
		},
		{
			name:     "message at the size limit",
			message:  strings.Repeat("0", 2048),
			wantBody: RequestBody{MsgType: "text", Text: TextContent{Content: strings.Repeat("0", 2048)}},
		},
		{
			name:     "multi-byte message within the size limit",
			message:  strings.Repeat("测", 682),
			wantBody: RequestBody{MsgType: "text", Text: TextContent{Content: strings.Repeat("测", 682)}},
		},
		{
			name:    "mentioned users",
			message: "message",
			params:  &types.Params{"mentioned_list": "@all"},
			wantBody: RequestBody{MsgType: "text", Text: TextContent{
				Content:       "message",
				MentionedList: []string{"@all"},
			}},
		},
		{
			name:    "mentioned mobile numbers",
			message: "message",
			params:  &types.Params{"mentioned_mobile_list": "13800001111"},
			wantBody: RequestBody{MsgType: "text", Text: TextContent{
				Content:             "message",
				MentionedMobileList: []string{"13800001111"},
			}},
		},
		{
			name:    "several mentioned users",
			message: "message",
			params:  &types.Params{"mentioned_list": "wangqing,@all"},
			wantBody: RequestBody{MsgType: "text", Text: TextContent{
				Content:       "message",
				MentionedList: []string{"wangqing", "@all"},
			}},
		},
		{
			name:    "several mentioned mobile numbers with spaces",
			message: "message",
			params:  &types.Params{"mentioned_mobile_list": " 13800001111 , 13900002222 "},
			wantBody: RequestBody{MsgType: "text", Text: TextContent{
				Content:             "message",
				MentionedMobileList: []string{"13800001111", "13900002222"},
			}},
		},
		{
			name:    "empty mention entries",
			message: "message",
			params:  &types.Params{"mentioned_list": "wangqing,, ,", "mentioned_mobile_list": " , "},
			wantBody: RequestBody{MsgType: "text", Text: TextContent{
				Content:       "message",
				MentionedList: []string{"wangqing"},
			}},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			client, captured := newRecordingClient(t, newResponse(http.StatusOK, `{"errcode":0,"errmsg":"ok"}`))
			logs := &strings.Builder{}
			service := newTestService(t, client, log.New(logs, "", 0))

			require.NoError(t, service.Send(tt.message, tt.params))

			assert.Equal(t, http.MethodPost, captured.method)
			assert.Equal(t, testPostURL, captured.url)
			assert.Equal(t, "application/json", captured.contentType)

			var body RequestBody
			require.NoError(t, json.Unmarshal(captured.body, &body))
			assert.Equal(t, tt.wantBody, body)

			assert.Equal(t, "DEBUG notification sent\n", logs.String())
			assert.NotContains(t, logs.String(), "693axxx6")
			assert.NotContains(t, logs.String(), "msgtype")
		})
	}
}

// TestSendResponseFailures verifies the errors a send returns when the
// request fails or the API reports a failure, and that none of them contain
// the webhook key.
func TestSendResponseFailures(t *testing.T) {
	t.Parallel()

	// tests lists transport behaviors with the error each send should return.
	tests := []struct {
		name      string
		transport roundTripFunc
		wantErr   error
		wantMsg   string
	}{
		{
			name: "network failure",
			transport: func(*http.Request) (*http.Response, error) {
				return nil, errNetwork
			},
			wantErr: errNetwork,
			wantMsg: "network error",
		},
		{
			name: "HTTP 400 status",
			transport: func(*http.Request) (*http.Response, error) {
				return newResponse(http.StatusBadRequest, "bad request"), nil
			},
			wantErr: ErrSendFailed,
			wantMsg: "unexpected status 400",
		},
		{
			name: "non-zero error code",
			transport: func(*http.Request) (*http.Response, error) {
				return newResponse(http.StatusOK, `{"errcode":1,"errmsg":"some error"}`), nil
			},
			wantErr: ErrSendFailed,
			wantMsg: "server returned error code 1: some error",
		},
		{
			name: "unreadable response body",
			transport: func(*http.Request) (*http.Response, error) {
				resp := newResponse(http.StatusOK, "")
				resp.Body = io.NopCloser(iotest.ErrReader(errRead))

				return resp, nil
			},
			wantErr: errRead,
			wantMsg: "reading response body",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			service := newTestService(t, &http.Client{Transport: tt.transport}, log.New(t.Output(), "", 0))

			err := service.Send("message", nil)

			require.ErrorIs(t, err, tt.wantErr)
			require.ErrorContains(t, err, tt.wantMsg)
			assert.NotContains(t, err.Error(), "693axxx6")
		})
	}
}

// TestSendInvalidJSONResponse verifies that a send fails with a JSON syntax
// error when the API answers with a body that is not JSON.
func TestSendInvalidJSONResponse(t *testing.T) {
	t.Parallel()

	client, _ := newRecordingClient(t, newResponse(http.StatusOK, "some response"))
	service := newTestService(t, client, log.New(t.Output(), "", 0))

	err := service.Send("message", nil)

	var syntaxErr *jsontext.SyntacticError
	require.ErrorAs(t, err, &syntaxErr)
	assert.ErrorContains(t, err, "unmarshaling response")
}

// TestSendValidationErrors verifies that a send rejects an oversized message,
// an unknown param, a missing key, and a key that cannot form a URL, all
// before any request is made.
func TestSendValidationErrors(t *testing.T) {
	t.Parallel()

	// tests lists configs, messages, and params with the error each send
	// should return.
	tests := []struct {
		name    string
		config  Config
		message string
		params  *types.Params
		wantErr error
		wantMsg string
	}{
		{
			name:    "message over the size limit",
			config:  Config{Key: testKey},
			message: strings.Repeat("0", 2049),
			wantErr: ErrLargeMessage,
		},
		{
			name:    "multi-byte message over the size limit",
			config:  Config{Key: testKey},
			message: strings.Repeat("测", 683),
			wantErr: ErrLargeMessage,
		},
		{
			name:    "unknown param",
			config:  Config{Key: testKey},
			message: "test message",
			params:  &types.Params{"invalid": "value"},
			wantErr: format.ErrInvalidConfigKey,
			wantMsg: "not a valid config key: invalid",
		},
		{
			name:    "missing key",
			config:  Config{},
			message: "test message",
			wantErr: ErrKeyRequired,
		},
		{
			name:    "key param with a control character",
			config:  Config{Key: testKey},
			message: "test message",
			params:  &types.Params{"key": "bad\nkey"},
			wantErr: ErrInvalidKey,
		},
		{
			name:    "key param that adds a query parameter",
			config:  Config{Key: testKey},
			message: "test message",
			params:  &types.Params{"key": "key&debug=1"},
			wantErr: ErrInvalidKey,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			config := tt.config
			service := &Service{Config: &config, pkr: format.NewPropKeyResolver(&config)}
			service.SetLogger(log.New(t.Output(), "", 0))
			service.SetHTTPClient(mocks.NewMockHTTPClient(t))

			err := service.Send(tt.message, tt.params)

			require.ErrorIs(t, err, tt.wantErr)

			if tt.wantMsg != "" {
				require.ErrorContains(t, err, tt.wantMsg)
			}
		})
	}
}

// TestSendContextCanceled verifies that SendContext passes the caller's
// context to the request and returns its error when it is canceled.
func TestSendContextCanceled(t *testing.T) {
	t.Parallel()

	ctx, cancel := context.WithCancel(t.Context())
	cancel()

	client := mocks.NewMockHTTPClient(t)
	client.EXPECT().Do(mock.Anything).RunAndReturn(func(req *http.Request) (*http.Response, error) {
		return nil, req.Context().Err()
	}).Once()

	service := newTestService(t, client, log.New(t.Output(), "", 0))

	require.ErrorIs(t, service.SendContext(ctx, "message", nil), context.Canceled)
}

// TestSendRejectsInvalidKeyParam verifies that a key param with characters a
// webhook key cannot have fails the send before any request, without the key
// in the error.
func TestSendRejectsInvalidKeyParam(t *testing.T) {
	t.Parallel()

	// The mock has no expectations, so a request fails the test.
	service := newTestService(t, mocks.NewMockHTTPClient(t), log.New(t.Output(), "", 0))

	err := service.Send("message", &types.Params{"key": "SECRETkey\x7f"})

	require.ErrorIs(t, err, ErrInvalidKey)
	assert.NotContains(t, err.Error(), "SECRETkey")
}
