package teams

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"log"
	"net/http"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	"github.com/nicholas-fedor/shoutrrr/pkg/format"
	"github.com/nicholas-fedor/shoutrrr/pkg/services/chat/teams/mocks"
	"github.com/nicholas-fedor/shoutrrr/pkg/types"
)

// capturedRequest records the parts of a request the mock HTTP client received.
type capturedRequest struct {
	// method is the HTTP method.
	method string
	// url is the request URL.
	url string
	// contentType is the Content-Type header.
	contentType string
	// payload is the decoded request body.
	payload adaptivePayload
}

// closeTracker is a response body that records whether it was closed.
type closeTracker struct {
	io.Reader

	// closed reports whether Close was called.
	closed bool
}

// roundTripFunc adapts a function to [http.RoundTripper].
type roundTripFunc func(*http.Request) (*http.Response, error)

// errTransport is the transport error that the failing test clients return.
var errTransport = errors.New("transport failed")

// Close records that the body was closed.
//
// Returns:
//   - error: always nil.
func (c *closeTracker) Close() error {
	c.closed = true

	return nil
}

// RoundTrip calls the function.
//
// Parameters:
//   - req: the request.
//
// Returns:
//   - *http.Response: the function's response.
//   - error: the function's error.
func (f roundTripFunc) RoundTrip(req *http.Request) (*http.Response, error) {
	return f(req)
}

// newTestService returns a service initialized from rawURL that sends through
// client and logs to the test output.
//
// Parameters:
//   - t: the test.
//   - rawURL: the Teams service URL.
//   - client: the HTTP client.
//
// Returns:
//   - *Service: the service.
func newTestService(t *testing.T, rawURL string, client types.HTTPClient) *Service {
	t.Helper()

	service := &Service{}
	require.NoError(t, service.Initialize(mustParseURL(t, rawURL), log.New(t.Output(), "", 0)))
	service.SetHTTPClient(client)

	return service
}

// newResponse returns a response with the given status code and an empty body.
//
// Parameters:
//   - statusCode: the HTTP status code.
//
// Returns:
//   - *http.Response: the response.
func newResponse(statusCode int) *http.Response {
	return &http.Response{
		StatusCode: statusCode,
		Status:     http.StatusText(statusCode),
		Body:       io.NopCloser(strings.NewReader("")),
	}
}

// newCapturingClient returns a mock HTTP client that expects one request,
// decodes it into the returned capturedRequest, and answers 200 OK.
//
// Parameters:
//   - t: the test that owns the mock.
//
// Returns:
//   - *mocks.MockHTTPClient: the client.
//   - *capturedRequest: the captured request, filled in when the request arrives.
func newCapturingClient(t *testing.T) (*mocks.MockHTTPClient, *capturedRequest) {
	t.Helper()

	captured := &capturedRequest{}
	client := mocks.NewMockHTTPClient(t)
	client.EXPECT().Do(mock.Anything).RunAndReturn(func(req *http.Request) (*http.Response, error) {
		captured.method = req.Method
		captured.url = req.URL.String()
		captured.contentType = req.Header.Get("Content-Type")

		if err := json.NewDecoder(req.Body).Decode(&captured.payload); err != nil {
			return nil, err
		}

		return newResponse(http.StatusOK), nil
	}).Once()

	return client, captured
}

// wantPayload returns the Adaptive Card payload that a send with body produces.
//
// Parameters:
//   - body: the card body blocks.
//
// Returns:
//   - adaptivePayload: the payload.
func wantPayload(body []adaptiveBlock) adaptivePayload {
	return adaptivePayload{
		Type: "message",
		Attachments: []adaptiveAttachment{
			{
				ContentType: "application/vnd.microsoft.card.adaptive",
				ContentURL:  nil,
				Content: adaptiveCardContent{
					Schema:  "http://adaptivecards.io/schemas/adaptive-card.json",
					Type:    "AdaptiveCard",
					Version: adaptiveCardVersion,
					Body:    body,
				},
			},
		},
	}
}

// TestServiceGetID verifies that GetID returns the teams scheme.
func TestServiceGetID(t *testing.T) {
	t.Parallel()

	assert.Equal(t, "teams", (&Service{}).GetID())
}

// TestServiceServiceTimeout verifies that ServiceTimeout returns the default
// HTTP timeout.
func TestServiceServiceTimeout(t *testing.T) {
	t.Parallel()

	assert.Equal(t, defaultHTTPTimeout, (&Service{}).ServiceTimeout(nil))
}

// TestServiceInitialize verifies that Initialize reads the config from the
// service URL and installs the default HTTP client.
func TestServiceInitialize(t *testing.T) {
	t.Parallel()

	service := &Service{}
	require.NoError(t, service.Initialize(mustParseURL(t, testServiceURL+"&title=Alert&color=red"), log.New(t.Output(), "", 0)))

	require.NotNil(t, service.Config)
	assert.Equal(t, Config{Host: testWorkflowURL, Title: "Alert", Color: "red"}, *service.Config)

	client, ok := service.httpClient.(*defaultHTTPClient)
	require.True(t, ok)

	httpClient, ok := client.client.(*http.Client)
	require.True(t, ok)
	assert.Equal(t, defaultHTTPTimeout, httpClient.Timeout)
}

// TestServiceSetHTTPClient verifies which client SetHTTPClient installs for
// nil, *http.Client, and custom clients.
func TestServiceSetHTTPClient(t *testing.T) {
	t.Parallel()

	custom := &http.Client{}

	// tests lists clients passed to SetHTTPClient with the inner *http.Client
	// that the installed default wrapper should hold. A nil want means the
	// wrapper holds a new client with the default timeout.
	tests := []struct {
		name   string
		client types.HTTPClient
		want   *http.Client
	}{
		{name: "nil selects the default client", client: nil},
		{name: "nil *http.Client selects the default client", client: (*http.Client)(nil)},
		{name: "*http.Client is wrapped", client: custom, want: custom},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			service := &Service{}
			service.SetHTTPClient(tt.client)

			wrapper, ok := service.httpClient.(*defaultHTTPClient)
			require.True(t, ok)

			inner, ok := wrapper.client.(*http.Client)
			require.True(t, ok)

			if tt.want != nil {
				assert.Same(t, tt.want, inner)

				return
			}

			assert.Equal(t, defaultHTTPTimeout, inner.Timeout)
		})
	}

	t.Run("custom client is installed as is", func(t *testing.T) {
		t.Parallel()

		client := mocks.NewMockHTTPClient(t)
		service := &Service{}
		service.SetHTTPClient(client)

		assert.Same(t, client, service.httpClient)
	})
}

// TestServiceSendStatus verifies that Send accepts 200 OK and 202 Accepted,
// returns ErrSendFailedStatus for any other status, and closes the response
// body either way.
func TestServiceSendStatus(t *testing.T) {
	t.Parallel()

	// tests lists response status codes and whether Send should fail.
	tests := []struct {
		name       string
		statusCode int
		wantErr    bool
	}{
		{name: "200 OK", statusCode: http.StatusOK},
		{name: "202 Accepted", statusCode: http.StatusAccepted},
		{name: "204 No Content", statusCode: http.StatusNoContent, wantErr: true},
		{name: "400 Bad Request", statusCode: http.StatusBadRequest, wantErr: true},
		{name: "500 Internal Server Error", statusCode: http.StatusInternalServerError, wantErr: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			body := &closeTracker{Reader: strings.NewReader("")}
			client := mocks.NewMockHTTPClient(t)
			client.EXPECT().Do(mock.Anything).Return(&http.Response{
				StatusCode: tt.statusCode,
				Status:     http.StatusText(tt.statusCode),
				Body:       body,
			}, nil).Once()

			err := newTestService(t, testServiceURL, client).Send("Message", nil)

			if tt.wantErr {
				require.ErrorIs(t, err, ErrSendFailedStatus)
				require.ErrorContains(t, err, http.StatusText(tt.statusCode))
			} else {
				require.NoError(t, err)
			}

			assert.True(t, body.closed)
		})
	}
}

// TestServiceSendTransportError verifies that Send wraps a client error in
// ErrSendFailed and keeps the cause matchable.
func TestServiceSendTransportError(t *testing.T) {
	t.Parallel()

	client := mocks.NewMockHTTPClient(t)
	client.EXPECT().Do(mock.Anything).Return(nil, errTransport).Once()

	err := newTestService(t, testServiceURL, client).Send("Message", nil)

	require.ErrorIs(t, err, ErrSendFailed)
	assert.ErrorIs(t, err, errTransport)
}

// TestServiceSendWithHTTPClient verifies that Send posts through an injected
// *http.Client and succeeds on its 200 OK response.
func TestServiceSendWithHTTPClient(t *testing.T) {
	t.Parallel()

	var requested []string

	client := &http.Client{Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
		requested = append(requested, req.URL.String())

		return newResponse(http.StatusOK), nil
	})}

	require.NoError(t, newTestService(t, testServiceURL, client).Send("Message", nil))
	assert.Equal(t, []string{testWorkflowURL}, requested)
}

// TestServiceSendRedactsTransportError verifies that a transport error from an
// injected *http.Client reports only the webhook's scheme and host, not the
// path or signature query.
func TestServiceSendRedactsTransportError(t *testing.T) {
	t.Parallel()

	calls := 0
	client := &http.Client{Transport: roundTripFunc(func(*http.Request) (*http.Response, error) {
		calls++

		return nil, errTransport
	})}

	err := newTestService(t, testServiceURL, client).Send("Message", nil)

	require.ErrorIs(t, err, ErrSendFailed)
	require.ErrorIs(t, err, errTransport)
	assert.Equal(t, 1, calls)
	require.ErrorContains(t, err, "https://prod-00.westus.logic.azure.com:443")
	assert.NotContains(t, err.Error(), "abc123")
	assert.NotContains(t, err.Error(), "sig=")
}

// TestServiceSendRequest verifies the request that Send posts: the method,
// URL, content type, and the Adaptive Card payload built from the message,
// title, and color, with blank lines dropped.
func TestServiceSendRequest(t *testing.T) {
	t.Parallel()

	// tests lists service URL suffixes, send params, and messages with the
	// card body each send should post.
	tests := []struct {
		name    string
		query   string
		params  *types.Params
		message string
		want    []adaptiveBlock
	}{
		{
			name:    "single line without title",
			message: "Message",
			want:    []adaptiveBlock{{Type: "TextBlock", Text: "Message", Wrap: true}},
		},
		{
			name:    "blank lines are dropped",
			message: "Line 1\n\n   \nLine 2",
			want: []adaptiveBlock{
				{Type: "TextBlock", Text: "Line 1", Wrap: true},
				{Type: "TextBlock", Text: "Line 2", Wrap: true},
			},
		},
		{
			name:    "empty message has an empty body",
			message: "",
			want:    []adaptiveBlock{},
		},
		{
			name:    "title with mapped color",
			query:   "&title=Alert&color=red",
			message: "Message",
			want: []adaptiveBlock{
				{Type: "TextBlock", Text: "Alert", Color: "attention", Weight: "Bolder", Size: "Medium"},
				{Type: "TextBlock", Text: "Message", Wrap: true},
			},
		},
		{
			name:    "title without color uses the default color",
			query:   "&title=Alert",
			message: "Message",
			want: []adaptiveBlock{
				{Type: "TextBlock", Text: "Alert", Color: "default", Weight: "Bolder", Size: "Medium"},
				{Type: "TextBlock", Text: "Message", Wrap: true},
			},
		},
		{
			name:    "color without title adds no title block",
			query:   "&color=red",
			message: "Message",
			want:    []adaptiveBlock{{Type: "TextBlock", Text: "Message", Wrap: true}},
		},
		{
			name:    "params override title and color",
			query:   "&title=Config&color=red",
			params:  &types.Params{"title": "Param", "color": "green"},
			message: "Message",
			want: []adaptiveBlock{
				{Type: "TextBlock", Text: "Param", Color: "good", Weight: "Bolder", Size: "Medium"},
				{Type: "TextBlock", Text: "Message", Wrap: true},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			client, captured := newCapturingClient(t)
			service := newTestService(t, testServiceURL+tt.query, client)
			configBefore := *service.Config

			require.NoError(t, service.Send(tt.message, tt.params))

			assert.Equal(t, http.MethodPost, captured.method)
			assert.Equal(t, testWorkflowURL, captured.url)
			assert.Equal(t, "application/json", captured.contentType)
			assert.Equal(t, wantPayload(tt.want), captured.payload)
			assert.Equal(t, configBefore, *service.Config, "params must not change the service config")
		})
	}
}

// TestServiceSendContextCanceled verifies that SendContext passes the caller's
// context to the request, so a canceled context ends the send with
// context.Canceled.
func TestServiceSendContextCanceled(t *testing.T) {
	t.Parallel()

	ctx, cancel := context.WithCancel(t.Context())
	cancel()

	client := mocks.NewMockHTTPClient(t)
	client.EXPECT().Do(mock.Anything).RunAndReturn(func(req *http.Request) (*http.Response, error) {
		return nil, req.Context().Err()
	}).Once()

	err := newTestService(t, testServiceURL, client).SendContext(ctx, "message", nil)

	require.ErrorIs(t, err, context.Canceled)
	assert.ErrorIs(t, err, ErrSendFailed)
}

// TestServiceSendErrorsBeforeRequest verifies the errors that Send returns
// without making a request: a missing config or host, an invalid webhook URL,
// an unknown param, and a URL that cannot form a request.
func TestServiceSendErrorsBeforeRequest(t *testing.T) {
	t.Parallel()

	// tests lists service URLs and params with the error Send should return.
	// An empty rawURL sends from an uninitialized service. The mock client has
	// no expectations, so any request fails the test.
	tests := []struct {
		name    string
		rawURL  string
		params  *types.Params
		wantErr error
	}{
		{name: "uninitialized service", wantErr: ErrMissingHost},
		{name: "empty host", rawURL: "teams://?title=Alert", wantErr: ErrMissingHost},
		{
			name:    "invalid webhook URL",
			rawURL:  "teams://?host=https%3A%2F%2Fexample.com%2Fwebhook",
			wantErr: ErrInvalidWebhookURL,
		},
		{
			name:    "invalid webhook URL from params",
			rawURL:  testServiceURL,
			params:  &types.Params{"host": "https://example.com/webhook"},
			wantErr: ErrInvalidWebhookURL,
		},
		{
			name:    "unknown param",
			rawURL:  testServiceURL,
			params:  &types.Params{"bogus": "value"},
			wantErr: format.ErrInvalidConfigKey,
		},
		{
			name:    "webhook URL that cannot form a request",
			rawURL:  testServiceURL,
			params:  &types.Params{"host": "https://prod-00.westus.logic.azure.com/workflows/%zz"},
			wantErr: ErrSendFailed,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			service := &Service{}
			if tt.rawURL != "" {
				service = newTestService(t, tt.rawURL, mocks.NewMockHTTPClient(t))
			}

			err := service.Send("Message", tt.params)
			require.ErrorIs(t, err, tt.wantErr)
		})
	}
}

// TestColorToEnum verifies how colorToEnum maps user color names, in any
// case, to Adaptive Card color values.
func TestColorToEnum(t *testing.T) {
	t.Parallel()

	// tests lists user color names with the Adaptive Card color each maps to.
	tests := []struct {
		color string
		want  string
	}{
		{color: "red", want: "attention"},
		{color: "attention", want: "attention"},
		{color: "danger", want: "attention"},
		{color: "RED", want: "attention"},
		{color: "orange", want: "warning"},
		{color: "yellow", want: "warning"},
		{color: "warning", want: "warning"},
		{color: "warn", want: "warning"},
		{color: "green", want: "good"},
		{color: "good", want: "good"},
		{color: "success", want: "good"},
		{color: "blue", want: "accent"},
		{color: "Accent", want: "accent"},
		{color: "dark", want: "dark"},
		{color: "light", want: "light"},
		{color: "default", want: "default"},
		{color: "", want: "default"},
		{color: "purple", want: "default"},
	}

	for _, tt := range tests {
		t.Run(tt.color, func(t *testing.T) {
			t.Parallel()

			assert.Equal(t, tt.want, colorToEnum(tt.color))
		})
	}
}
