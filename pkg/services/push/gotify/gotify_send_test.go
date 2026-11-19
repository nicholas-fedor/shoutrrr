package gotify

import (
	"errors"
	"io"
	"net/http"
	"net/url"
	"strings"
	"testing"
	"testing/iotest"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	"github.com/nicholas-fedor/shoutrrr/internal/httpbody"
	typesmocks "github.com/nicholas-fedor/shoutrrr/pkg/types/mocks"
	"github.com/nicholas-fedor/shoutrrr/pkg/util/jsonclient"
)

// testRequest is the message request the send tests post.
var testRequest = &MessageRequest{Message: "Hello", Title: "Backup", Priority: new(5)}

// TestSendRequest verifies the request a send makes: a JSON POST to the URL,
// with the token in the X-Gotify-Key header only under header
// authentication, and that a created message is a success.
func TestSendRequest(t *testing.T) {
	t.Parallel()

	// tests lists service URLs with the headers each request should carry.
	tests := []struct {
		wantHeader http.Header
		name       string
		serviceURL string
	}{
		{
			name:       "token in a header",
			serviceURL: testServiceURL,
			wantHeader: http.Header{"Content-Type": {"application/json"}, "X-Gotify-Key": {testToken}},
		},
		{
			name:       "token in the query",
			serviceURL: testServiceURL + "?useheader=no",
			wantHeader: http.Header{"Content-Type": {"application/json"}},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			service := newTestService(t, tt.serviceURL)

			var got *http.Request

			client := typesmocks.NewMockHTTPClient(t)
			client.EXPECT().Do(mock.Anything).RunAndReturn(func(req *http.Request) (*http.Response, error) {
				got = req

				return &http.Response{StatusCode: http.StatusOK, Body: io.NopCloser(strings.NewReader(sentBody))}, nil
			}).Once()

			require.NoError(t, service.sendRequest(t.Context(), client, service.Config, testEndpoint, testRequest))

			body, err := io.ReadAll(got.Body)
			require.NoError(t, err)
			assert.Equal(t, http.MethodPost, got.Method)
			assert.Equal(t, testEndpoint, got.URL.String())
			assert.Equal(t, tt.wantHeader, got.Header)
			assert.JSONEq(t, `{"message":"Hello","title":"Backup","priority":5}`, string(body))
		})
	}
}

// TestSendRequestErrorResponses verifies the error for each failed response.
// The server's error comes back labeled as a client or server error by status
// when the body of an error status has one. Any other status outside 200 to
// 299, a redirect included, gives [jsonclient.ErrUnexpectedStatus], and a
// success body that is not JSON gives the decoding error. A body over
// [httpbody.MaxSize] still yields the server's error, and fails a success
// status as too large. Every error wraps [ErrSendFailed].
func TestSendRequestErrorResponses(t *testing.T) {
	t.Parallel()

	// tests lists responses with the error each should produce, after the
	// ErrSendFailed text.
	tests := []struct {
		wantErr   error
		name      string
		wantError string
		response  cannedResponse
	}{
		{
			name:      "client error response",
			response:  cannedResponse{statusCode: http.StatusUnauthorized, body: unauthorizedBody},
			wantError: "client error (401): server responded with Unauthorized (401): you need to provide a valid access token or user credentials to access this api",
		},
		{
			name:      "bad request response",
			response:  cannedResponse{statusCode: http.StatusBadRequest, body: `{"error":"Bad Request","errorCode":400,"errorDescription":"Field 'message' is required"}`},
			wantError: "client error (400): server responded with Bad Request (400): Field 'message' is required",
		},
		{
			name:      "server error response",
			response:  cannedResponse{statusCode: http.StatusInternalServerError, body: `{"error":"Internal Server Error","errorCode":500,"errorDescription":"database is locked"}`},
			wantError: "server error (500): server responded with Internal Server Error (500): database is locked",
		},
		{
			name:      "client status without an error body",
			response:  cannedResponse{statusCode: http.StatusNotFound, body: "404 page not found"},
			wantErr:   jsonclient.ErrUnexpectedStatus,
			wantError: "got unexpected HTTP status: 404 Not Found",
		},
		{
			name:      "server status without an error body",
			response:  cannedResponse{statusCode: http.StatusBadGateway, body: "<html>Bad Gateway</html>"},
			wantErr:   jsonclient.ErrUnexpectedStatus,
			wantError: "got unexpected HTTP status: 502 Bad Gateway",
		},
		{
			name:      "JSON body without an error",
			response:  cannedResponse{statusCode: http.StatusBadRequest, body: `{}`},
			wantErr:   jsonclient.ErrUnexpectedStatus,
			wantError: "got unexpected HTTP status: 400 Bad Request",
		},
		{
			name:      "redirect with a success body",
			response:  cannedResponse{statusCode: http.StatusTemporaryRedirect, body: sentBody},
			wantErr:   jsonclient.ErrUnexpectedStatus,
			wantError: "got unexpected HTTP status: 307 Temporary Redirect",
		},
		{
			name:      "redirect with an error body",
			response:  cannedResponse{statusCode: http.StatusTemporaryRedirect, body: unauthorizedBody},
			wantErr:   jsonclient.ErrUnexpectedStatus,
			wantError: "got unexpected HTTP status: 307 Temporary Redirect",
		},
		{
			name:      "success body not JSON",
			response:  cannedResponse{statusCode: http.StatusOK, body: "OK"},
			wantError: "jsontext: invalid character 'O' at start of value",
		},
		{
			name: "oversized server error response",
			response: cannedResponse{
				statusCode: http.StatusInternalServerError,
				body:       `{"error":"Internal Server Error","errorCode":500,"errorDescription":"database is locked"}` + strings.Repeat(" ", httpbody.MaxSize),
			},
			wantError: "server error (500): server responded with Internal Server Error (500): database is locked",
		},
		{
			name:     "oversized success body",
			response: cannedResponse{statusCode: http.StatusOK, body: sentBody + strings.Repeat(" ", httpbody.MaxSize)},
			wantErr:  httpbody.ErrTooLarge,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			service := newTestService(t, testServiceURL)
			client, _ := newRecordingClient(t, tt.response)

			err := service.sendRequest(t.Context(), client, service.Config, testEndpoint, testRequest)

			require.ErrorIs(t, err, ErrSendFailed)

			if tt.wantErr != nil {
				require.ErrorIs(t, err, tt.wantErr)
			}

			if tt.wantError != "" {
				assert.EqualError(t, err, ErrSendFailed.Error()+": "+tt.wantError)
			}
		})
	}
}

// TestSendRequestFailures verifies that each of these failures fails the send
// with [ErrSendFailed]:
//   - a transport error
//   - a request that cannot be built
//   - a request that cannot be encoded
//   - a body that cannot be read
//
// The error of a transport failure or an unbuildable request leaves out the
// token.
func TestSendRequestFailures(t *testing.T) {
	t.Parallel()

	t.Run("transport error", func(t *testing.T) {
		t.Parallel()

		service := newTestService(t, testServiceURL)
		client, _ := newRecordingClient(t, cannedResponse{err: errTransport})

		err := service.sendRequest(t.Context(), client, service.Config, testEndpoint, testRequest)

		require.ErrorIs(t, err, ErrSendFailed)
		require.ErrorIs(t, err, errTransport)
		assert.NotContains(t, err.Error(), testToken)
	})

	t.Run("unparsable URL", func(t *testing.T) {
		t.Parallel()

		service := newTestService(t, testServiceURL+"?useheader=no")
		// The mock has no expectations, so a request fails the test.
		client := typesmocks.NewMockHTTPClient(t)

		err := service.sendRequest(t.Context(), client, service.Config, "https://bad host/message?token="+testToken, testRequest)

		require.ErrorIs(t, err, ErrSendFailed)

		_, isURLErr := errors.AsType[*url.Error](err)
		require.True(t, isURLErr, "error %v is not a *url.Error", err)
		assert.NotContains(t, err.Error(), testToken)
	})

	t.Run("request not JSON", func(t *testing.T) {
		t.Parallel()

		service := newTestService(t, testServiceURL)
		// The mock has no expectations, so a request fails the test.
		client := typesmocks.NewMockHTTPClient(t)
		request := &MessageRequest{Message: "Hello", Extras: map[string]any{"channel": make(chan int)}}

		require.ErrorIs(t, service.sendRequest(t.Context(), client, service.Config, testEndpoint, request), ErrSendFailed)
	})

	t.Run("body read error", func(t *testing.T) {
		t.Parallel()

		errRead := errors.New("connection reset")

		service := newTestService(t, testServiceURL)
		client := typesmocks.NewMockHTTPClient(t)
		client.EXPECT().Do(mock.Anything).Return(&http.Response{
			StatusCode: http.StatusOK,
			Body:       io.NopCloser(iotest.ErrReader(errRead)),
		}, nil).Once()

		err := service.sendRequest(t.Context(), client, service.Config, testEndpoint, testRequest)

		require.ErrorIs(t, err, ErrSendFailed)
		require.ErrorIs(t, err, errRead)
	})
}
