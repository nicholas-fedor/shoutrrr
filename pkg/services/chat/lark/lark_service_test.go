package lark

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json/jsontext"
	"encoding/json/v2"
	"errors"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"testing"
	"testing/iotest"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	"github.com/nicholas-fedor/shoutrrr/pkg/format"
	"github.com/nicholas-fedor/shoutrrr/pkg/types"
	"github.com/nicholas-fedor/shoutrrr/pkg/types/mocks"
)

// capturedRequest records the parts of a request the mock HTTP client received.
type capturedRequest struct {
	// method is the request method.
	method string
	// url is the full request URL.
	url string
	// contentType is the request's Content-Type header.
	contentType string
	// body is the raw request body.
	body []byte
}

// roundTripFunc adapts a function to [http.RoundTripper].
type roundTripFunc func(*http.Request) (*http.Response, error)

// testWebhookURL is the API URL that fullURL posts to.
const testWebhookURL = "https://open.larksuite.com/open-apis/bot/v2/hook/token"

// Errors the fake transports and response bodies return.
var (
	// errNetwork is the transport error the network failure test returns.
	errNetwork = errors.New("network error")
	// errRead is the error the unreadable response body returns.
	errRead = errors.New("read failed")
)

// RoundTrip calls f with the request.
//
// Parameters:
//   - req: the outgoing request.
//
// Returns:
//   - *http.Response: the response from f.
//   - error: the error from f.
func (f roundTripFunc) RoundTrip(req *http.Request) (*http.Response, error) {
	return f(req)
}

// newResponse returns an HTTP response with the given status and body.
//
// Parameters:
//   - status: the HTTP status code.
//   - body: the response body.
//
// Returns:
//   - *http.Response: the response.
func newResponse(status int, body string) *http.Response {
	return &http.Response{
		StatusCode: status,
		Status:     fmt.Sprintf("%d %s", status, http.StatusText(status)),
		Header:     http.Header{"Content-Type": []string{"application/json"}},
		Body:       io.NopCloser(strings.NewReader(body)),
	}
}

// newTestService returns a service initialized from rawURL with a logger that
// writes to the test output.
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

// newRespondingClient returns a mock HTTP client that expects one request,
// records it, and answers with the given status and body.
//
// Parameters:
//   - t: the test that owns the mock.
//   - status: the HTTP status code to answer with.
//   - body: the response body to answer with.
//
// Returns:
//   - *mocks.MockHTTPClient: the client.
//   - *capturedRequest: the recorded request, filled in when the request arrives.
func newRespondingClient(t *testing.T, status int, body string) (*mocks.MockHTTPClient, *capturedRequest) {
	t.Helper()

	captured := &capturedRequest{}
	client := mocks.NewMockHTTPClient(t)
	client.EXPECT().Do(mock.Anything).RunAndReturn(func(req *http.Request) (*http.Response, error) {
		reqBody, err := io.ReadAll(req.Body)
		if err != nil {
			return nil, err
		}

		captured.method = req.Method
		captured.url = req.URL.String()
		captured.contentType = req.Header.Get("Content-Type")
		captured.body = reqBody

		return newResponse(status, body), nil
	}).Once()

	return client, captured
}

// decodeBody decodes the captured request body as a [RequestBody].
//
// Parameters:
//   - t: the test.
//   - captured: the recorded request.
//
// Returns:
//   - RequestBody: the decoded payload.
func decodeBody(t *testing.T, captured *capturedRequest) RequestBody {
	t.Helper()

	var body RequestBody
	require.NoError(t, json.Unmarshal(captured.body, &body))

	return body
}

// expectedSign computes the Lark signature for timestamp and secret: the
// base64 HMAC-SHA256 of empty data keyed with "timestamp\nsecret".
//
// Parameters:
//   - timestamp: the Unix timestamp in seconds.
//   - secret: the bot secret.
//
// Returns:
//   - string: the signature.
func expectedSign(timestamp int64, secret string) string {
	mac := hmac.New(sha256.New, []byte(strconv.FormatInt(timestamp, 10)+"\n"+secret))

	return base64.StdEncoding.EncodeToString(mac.Sum(nil))
}

// TestServiceGetID verifies that the service reports the lark scheme.
func TestServiceGetID(t *testing.T) {
	t.Parallel()

	assert.Equal(t, Scheme, (&Service{}).GetID())
}

// TestServiceTimeout verifies that the service reports the default HTTP timeout.
func TestServiceTimeout(t *testing.T) {
	t.Parallel()

	assert.Equal(t, defaultTime, (&Service{}).ServiceTimeout(nil))
}

// TestServiceInitializeRejectsInvalidURLs verifies that Initialize returns the
// config errors for an unknown host and a missing path.
func TestServiceInitializeRejectsInvalidURLs(t *testing.T) {
	t.Parallel()

	// tests lists invalid service URLs with the error each should produce.
	tests := []struct {
		name    string
		rawURL  string
		wantErr error
	}{
		{name: "invalid host", rawURL: "lark://invalid.com/token", wantErr: ErrInvalidHost},
		{name: "no path", rawURL: "lark://open.larksuite.com", wantErr: ErrNoPath},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			err := (&Service{}).Initialize(parseTestURL(t, tt.rawURL), log.New(t.Output(), "", 0))
			require.ErrorIs(t, err, tt.wantErr)
		})
	}
}

// TestSetHTTPClient verifies that the service uses an injected
// client and falls back to a default client without a request timeout when no
// client, a nil interface, or a nil *http.Client is set.
func TestSetHTTPClient(t *testing.T) {
	t.Parallel()

	t.Run("custom client", func(t *testing.T) {
		t.Parallel()

		client := &http.Client{}
		service := newTestService(t, fullURL)
		service.SetHTTPClient(client)

		assert.Same(t, client, service.client.Client())
	})

	// tests lists ways of leaving the client unset, each of which selects the
	// default client.
	tests := []struct {
		name  string
		setup func(*Service)
	}{
		{name: "never set", setup: func(*Service) {}},
		{name: "nil interface", setup: func(s *Service) {
			s.SetHTTPClient(&http.Client{})
			s.SetHTTPClient(nil)
		}},
		{name: "nil http client", setup: func(s *Service) {
			s.SetHTTPClient(&http.Client{})
			s.SetHTTPClient((*http.Client)(nil))
		}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			service := newTestService(t, fullURL)
			tt.setup(service)

			client, ok := service.client.Client().(*http.Client)
			require.True(t, ok, "the default client should be an *http.Client")
			assert.Zero(t, client.Timeout, "the send budget bounds each send")
			assert.Same(t, client, service.client.Client())
		})
	}
}

// TestServiceSendRejectsInvalidInput verifies that Send validates the request
// body size, params, host, and path before making any request.
func TestServiceSendRejectsInvalidInput(t *testing.T) {
	t.Parallel()

	// tests lists invalid sends with the error each should produce.
	tests := []struct {
		name    string
		config  Config
		message string
		params  *types.Params
		wantErr error
	}{
		{
			name:    "message too large",
			config:  Config{Host: larkHost, Path: "token"},
			message: strings.Repeat("a", 20*1024),
			wantErr: ErrLargeMessage,
		},
		{
			name:    "invalid param",
			config:  Config{Host: larkHost, Path: "token"},
			message: "test message",
			params:  &types.Params{"invalid": "value"},
			wantErr: format.ErrInvalidConfigKey,
		},
		{
			name:    "invalid host",
			config:  Config{Host: "invalid.com", Path: "token"},
			message: "test message",
			wantErr: ErrInvalidHost,
		},
		{
			name:    "no path",
			config:  Config{Host: larkHost},
			message: "test message",
			wantErr: ErrNoPath,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			config := tt.config
			service := &Service{Config: &config, pkr: format.NewPropKeyResolver(&config)}
			service.SetHTTPClient(mocks.NewMockHTTPClient(t))

			err := service.Send(tt.message, tt.params)
			require.ErrorIs(t, err, tt.wantErr)
		})
	}
}

// TestServiceSendBodySizeLimit verifies that Lark's 20 KB limit applies to the
// encoded request body, so the title, the link, and JSON escapes count toward
// it, and that a body of exactly the limit is sent.
func TestServiceSendBodySizeLimit(t *testing.T) {
	t.Parallel()

	const limit = 20 * 1024

	service := newTestService(t, fullURL)
	// An empty text is left out of the body, so a one-byte message measures
	// the body around the text.
	base, err := service.preparePayload("a", service.Config, nil)
	require.NoError(t, err)

	// fill is the text message that makes the body exactly the limit.
	fill := strings.Repeat("a", limit-len(base)+1)

	// tests lists sends with whether the body stays within the limit.
	tests := []struct {
		params  *types.Params
		name    string
		message string
		wantErr bool
	}{
		{name: "body at the limit", message: fill},
		{name: "body one byte over", message: fill + "a", wantErr: true},
		{name: "title over the limit", message: fill, params: &types.Params{"title": "Title"}, wantErr: true},
		{
			name:    "link over the limit",
			message: fill[:len(fill)-200],
			params:  &types.Params{"title": "T", "link": "https://example.com/" + strings.Repeat("l", 200)},
			wantErr: true,
		},
		{name: "escaped quotes over the limit", message: strings.Repeat(`"`, len(fill)/2+1), wantErr: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			service := newTestService(t, fullURL)

			if tt.wantErr {
				// The mock has no expectations, so a request fails the test.
				service.SetHTTPClient(mocks.NewMockHTTPClient(t))
				require.ErrorIs(t, service.Send(tt.message, tt.params), ErrLargeMessage)

				return
			}

			client, captured := newRespondingClient(t, http.StatusOK, `{"code":0,"msg":"success"}`)
			service.SetHTTPClient(client)

			require.NoError(t, service.Send(tt.message, tt.params))
			assert.Len(t, captured.body, limit)
		})
	}
}

// TestServiceSendPayload verifies the request the service sends for text
// messages, titled post messages, and post messages with a link.
func TestServiceSendPayload(t *testing.T) {
	t.Parallel()

	// tests lists sends with the message type and post content each should
	// produce. A nil wantPost means a text message.
	tests := []struct {
		name        string
		params      *types.Params
		wantMsgType MsgType
		wantPost    *Post
	}{
		{
			name:        "text message",
			wantMsgType: MsgTypeText,
		},
		{
			name:        "link without title stays a text message",
			params:      &types.Params{"link": "https://example.com"},
			wantMsgType: MsgTypeText,
		},
		{
			name:        "post message with title",
			params:      &types.Params{"title": "title"},
			wantMsgType: MsgTypePost,
			wantPost: &Post{En: &Message{
				Title:   "title",
				Content: [][]Item{{{Tag: TagValueTextContent, Text: "message"}}},
			}},
		},
		{
			name:        "post message with title and link",
			params:      &types.Params{"title": "title", "link": "https://example.com"},
			wantMsgType: MsgTypePost,
			wantPost: &Post{En: &Message{
				Title: "title",
				Content: [][]Item{
					{{Tag: TagValueTextContent, Text: "message"}},
					{{Tag: TagValueLink, Text: "More Info", Link: "https://example.com"}},
				},
			}},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			client, captured := newRespondingClient(t, http.StatusOK, `{"code":0,"msg":"success"}`)
			service := newTestService(t, fullURL)
			service.SetHTTPClient(client)

			require.NoError(t, service.Send("message", tt.params))

			assert.Equal(t, http.MethodPost, captured.method)
			assert.Equal(t, testWebhookURL, captured.url)
			assert.Equal(t, "application/json", captured.contentType)

			body := decodeBody(t, captured)
			assert.Equal(t, tt.wantMsgType, body.MsgType)

			if tt.wantPost == nil {
				assert.Equal(t, "message", body.Content.Text)
				assert.Nil(t, body.Content.Post)

				return
			}

			assert.Empty(t, body.Content.Text)
			assert.Equal(t, tt.wantPost, body.Content.Post)
		})
	}
}

// TestServiceSendWireFormat verifies the JSON field names of a post message
// with a link, as the Lark API expects them.
func TestServiceSendWireFormat(t *testing.T) {
	t.Parallel()

	client, captured := newRespondingClient(t, http.StatusOK, `{"code":0,"msg":"success"}`)
	service := newTestService(t, "lark://open.feishu.cn/token")
	service.SetHTTPClient(client)

	require.NoError(t, service.Send("message", &types.Params{"title": "title", "link": "https://example.com"}))

	assert.Equal(t, "https://open.feishu.cn/open-apis/bot/v2/hook/token", captured.url)
	assert.JSONEq(t, `{
		"msg_type": "post",
		"content": {
			"post": {
				"en_us": {
					"title": "title",
					"content": [
						[{"tag": "text", "text": "message"}],
						[{"tag": "a", "text": "More Info", "href": "https://example.com"}]
					]
				}
			}
		}
	}`, string(captured.body))
}

// TestServiceSendSignature verifies that a send with a secret carries the
// current timestamp and the signature computed from that timestamp and the
// secret, and that a send without a secret carries neither.
func TestServiceSendSignature(t *testing.T) {
	t.Parallel()

	t.Run("with secret", func(t *testing.T) {
		t.Parallel()

		client, captured := newRespondingClient(t, http.StatusOK, `{"code":0,"msg":"success"}`)
		service := newTestService(t, fullURL)
		service.SetHTTPClient(client)

		before := time.Now().Unix()

		require.NoError(t, service.Send("message", nil))

		after := time.Now().Unix()

		body := decodeBody(t, captured)
		timestamp, err := strconv.ParseInt(body.Timestamp, 10, 64)
		require.NoError(t, err)

		assert.GreaterOrEqual(t, timestamp, before)
		assert.LessOrEqual(t, timestamp, after)
		assert.Equal(t, expectedSign(timestamp, "sss"), body.Sign)
	})

	t.Run("without secret", func(t *testing.T) {
		t.Parallel()

		client, captured := newRespondingClient(t, http.StatusOK, `{"code":0,"msg":"success"}`)
		service := newTestService(t, "lark://open.larksuite.com/token")
		service.SetHTTPClient(client)

		require.NoError(t, service.Send("message", nil))

		var fields map[string]any
		require.NoError(t, json.Unmarshal(captured.body, &fields))
		assert.NotContains(t, fields, "timestamp")
		assert.NotContains(t, fields, "sign")
	})
}

// TestServiceSendResponseErrors verifies that Send reports HTTP error
// statuses and error codes in the response body as ErrSendFailed.
func TestServiceSendResponseErrors(t *testing.T) {
	t.Parallel()

	// tests lists responses with the text each error should contain.
	tests := []struct {
		name         string
		status       int
		body         string
		wantContains string
	}{
		{name: "non-zero response code", status: http.StatusOK, body: `{"code":1,"msg":"some error"}`, wantContains: "server returned code 1: some error"},
		{name: "bad request status", status: http.StatusBadRequest, body: "bad request", wantContains: "unexpected status 400"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			client, _ := newRespondingClient(t, tt.status, tt.body)
			service := newTestService(t, fullURL)
			service.SetHTTPClient(client)

			err := service.Send("message", nil)
			require.ErrorIs(t, err, ErrSendFailed)
			assert.ErrorContains(t, err, tt.wantContains)
		})
	}
}

// TestServiceSendInvalidJSONResponse verifies that Send returns the JSON
// syntax error for a response body that is not JSON.
func TestServiceSendInvalidJSONResponse(t *testing.T) {
	t.Parallel()

	client, _ := newRespondingClient(t, http.StatusOK, "some response")
	service := newTestService(t, fullURL)
	service.SetHTTPClient(client)

	err := service.Send("message", nil)

	var syntaxErr *jsontext.SyntacticError
	require.ErrorAs(t, err, &syntaxErr)
}

// TestServiceSendUnreadableResponse verifies that Send returns the error from
// reading the response body.
func TestServiceSendUnreadableResponse(t *testing.T) {
	t.Parallel()

	client := mocks.NewMockHTTPClient(t)
	client.EXPECT().Do(mock.Anything).RunAndReturn(func(*http.Request) (*http.Response, error) {
		resp := newResponse(http.StatusOK, "")
		resp.Body = io.NopCloser(iotest.ErrReader(errRead))

		return resp, nil
	}).Once()

	service := newTestService(t, fullURL)
	service.SetHTTPClient(client)

	err := service.Send("message", nil)
	require.ErrorIs(t, err, errRead)
	assert.ErrorContains(t, err, "reading response body")
}

// TestServiceSendNetworkError verifies that a transport failure is reported as
// ErrSendFailed with its cause and without the webhook token.
func TestServiceSendNetworkError(t *testing.T) {
	t.Parallel()

	client := &http.Client{Transport: roundTripFunc(func(*http.Request) (*http.Response, error) {
		return nil, errNetwork
	})}

	service := newTestService(t, fullURL)
	service.SetHTTPClient(client)

	err := service.Send("message", nil)
	require.ErrorIs(t, err, ErrSendFailed)
	require.ErrorContains(t, err, "network error")
	assert.NotContains(t, err.Error(), "/hook/token")
}

// TestServiceSendLogging verifies that a successful send logs the host but
// neither the webhook token nor the request body.
func TestServiceSendLogging(t *testing.T) {
	t.Parallel()

	client, _ := newRespondingClient(t, http.StatusOK, `{"code":0,"msg":"success"}`)
	logs := &strings.Builder{}

	service := &Service{}
	require.NoError(t, service.Initialize(parseTestURL(t, fullURL), log.New(logs, "", 0)))
	service.SetHTTPClient(client)

	require.NoError(t, service.Send("message", nil))

	assert.Contains(t, logs.String(), "open.larksuite.com")
	assert.NotContains(t, logs.String(), "token")
	assert.NotContains(t, logs.String(), "msg_type")
}

// TestServiceSendContextCanceled verifies that SendContext stops the request
// when the caller's context is canceled and returns the context's error.
func TestServiceSendContextCanceled(t *testing.T) {
	t.Parallel()

	ctx, cancel := context.WithCancel(t.Context())
	cancel()

	client := mocks.NewMockHTTPClient(t)
	client.EXPECT().Do(mock.Anything).RunAndReturn(func(req *http.Request) (*http.Response, error) {
		return nil, req.Context().Err()
	}).Once()

	service := newTestService(t, fullURL)
	service.SetHTTPClient(client)

	require.ErrorIs(t, service.SendContext(ctx, "message", nil), context.Canceled)
}

// TestServiceDoSendMissingHost verifies that doSend rejects a config without a
// host. SendContext checks the host first, so doSend is called directly.
func TestServiceDoSendMissingHost(t *testing.T) {
	t.Parallel()

	service := &Service{}
	service.SetHTTPClient(mocks.NewMockHTTPClient(t))

	err := service.doSend(t.Context(), &Config{Path: "token"}, "message", nil)
	require.ErrorIs(t, err, ErrMissingHost)
}

// TestServiceSendRequestInvalidURL verifies that sendRequest returns the
// request construction error for a URL that cannot be parsed.
func TestServiceSendRequestInvalidURL(t *testing.T) {
	t.Parallel()

	service := &Service{}
	service.SetHTTPClient(mocks.NewMockHTTPClient(t))

	err := service.sendRequest(t.Context(), "https://open.larksuite.com/\x7f", []byte("{}"))

	var urlErr *url.Error
	require.ErrorAs(t, err, &urlErr)
	assert.ErrorContains(t, err, "creating HTTP request")
}

// TestServiceSendRedactsUnparsableURL verifies that a token that makes the
// webhook URL unparsable fails the send without the token in the error.
func TestServiceSendRedactsUnparsableURL(t *testing.T) {
	t.Parallel()

	service := newTestService(t, "lark://open.larksuite.com/SECRETtoken%25zz")
	// The mock has no expectations, so a request fails the test.
	service.SetHTTPClient(mocks.NewMockHTTPClient(t))

	err := service.Send("message", nil)

	_, isURLErr := errors.AsType[*url.Error](err)
	require.True(t, isURLErr, "error %v is not a *url.Error", err)
	assert.NotContains(t, err.Error(), "SECRETtoken")
}
