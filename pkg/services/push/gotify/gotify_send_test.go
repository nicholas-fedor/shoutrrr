package gotify

import (
	"context"
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

	typesmocks "github.com/nicholas-fedor/shoutrrr/pkg/types/mocks"
	"github.com/nicholas-fedor/shoutrrr/pkg/util/jsonclient"
)

// headerAuth is the header that sends the token in a request.
var headerAuth = http.Header{"X-Gotify-Key": {testToken}}

// testRequest is the message request the sender tests send.
var testRequest = &MessageRequest{Message: "Hello", Title: "Backup", Priority: 5}

// TestSendRequestContext verifies the request the sender makes: a JSON POST
// to the URL with the given headers, and that a created message is a
// success.
func TestSendRequestContext(t *testing.T) {
	t.Parallel()

	// tests lists headers with the X-Gotify-Key header each should send.
	tests := []struct {
		headers http.Header
		name    string
		wantKey string
	}{
		{name: "token in the query"},
		{name: "token in a header", headers: headerAuth, wantKey: testToken},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			client, captured := newRecordingClient(t, sent)

			require.NoError(t, (&DefaultSender{}).SendRequestContext(t.Context(), client, testEndpoint, testRequest, tt.headers))

			requests := captured()
			require.Len(t, requests, 1)
			assert.Equal(t, http.MethodPost, requests[0].method)
			assert.Equal(t, testEndpoint, requests[0].url)
			assert.Equal(t, "application/json", requests[0].contentType)
			assert.Equal(t, tt.wantKey, requests[0].gotifyKey)
			assert.JSONEq(t, `{"message":"Hello","title":"Backup","priority":5,"extras":null}`, requests[0].body)
		})
	}
}

// TestSendRequest verifies that SendRequest sends like SendRequestContext.
func TestSendRequest(t *testing.T) {
	t.Parallel()

	client, captured := newRecordingClient(t, sent)

	require.NoError(t, (&DefaultSender{}).SendRequest(client, testEndpoint, testRequest, nil))
	assert.Len(t, captured(), 1)
}

// TestSendRequestErrorResponses verifies the error for each failed response,
// with the token in the query and in a header: the server's error, labeled as
// a client or server error by status, when the body has one, the status
// otherwise, and a parse error for a success body that is not JSON. The
// service adds [ErrSendFailed], so the sender's errors leave it out.
func TestSendRequestErrorResponses(t *testing.T) {
	t.Parallel()

	// tests lists responses and headers with the error each should produce.
	tests := []struct {
		wantErr   error
		headers   http.Header
		name      string
		response  cannedResponse
		wantError string
	}{
		{
			name:      "error response",
			response:  cannedResponse{statusCode: http.StatusUnauthorized, body: unauthorizedBody},
			wantError: "client error (401): server responded with Unauthorized (401): you need to provide a valid access token or user credentials to access this api",
		},
		{
			name:      "server error response",
			response:  cannedResponse{statusCode: http.StatusInternalServerError, body: `{"error":"Internal Server Error","errorCode":500,"errorDescription":"database is locked"}`},
			wantError: "server error (500): server responded with Internal Server Error (500): database is locked",
		},
		{
			name:      "status without an error body",
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
			name:      "success body not JSON",
			response:  cannedResponse{statusCode: http.StatusOK, body: "OK"},
			wantError: "invalid character 'O' looking for beginning of value",
		},
		{
			name:      "client error response with the token in a header",
			headers:   headerAuth,
			response:  cannedResponse{statusCode: http.StatusUnauthorized, body: unauthorizedBody},
			wantError: "client error (401): server responded with Unauthorized (401): you need to provide a valid access token or user credentials to access this api",
		},
		{
			name:      "bad request response with the token in a header",
			headers:   headerAuth,
			response:  cannedResponse{statusCode: http.StatusBadRequest, body: `{"error":"Bad Request","errorCode":400,"errorDescription":"Field 'message' is required"}`},
			wantError: "client error (400): server responded with Bad Request (400): Field 'message' is required",
		},
		{
			name:      "server error response with the token in a header",
			headers:   headerAuth,
			response:  cannedResponse{statusCode: http.StatusInternalServerError, body: `{"error":"Internal Server Error","errorCode":500,"errorDescription":"database is locked"}`},
			wantError: "server error (500): server responded with Internal Server Error (500): database is locked",
		},
		{
			name:      "client status without an error body with the token in a header",
			headers:   headerAuth,
			response:  cannedResponse{statusCode: http.StatusNotFound, body: "404 page not found"},
			wantErr:   ErrUnexpectedStatus,
			wantError: "got unexpected HTTP status: 404 Not Found",
		},
		{
			name:      "server status without an error body with the token in a header",
			headers:   headerAuth,
			response:  cannedResponse{statusCode: http.StatusServiceUnavailable},
			wantErr:   ErrUnexpectedStatus,
			wantError: "got unexpected HTTP status: 503 Service Unavailable",
		},
		{
			name:      "JSON body without an error with the token in a header",
			headers:   headerAuth,
			response:  cannedResponse{statusCode: http.StatusBadRequest, body: `{}`},
			wantErr:   ErrUnexpectedStatus,
			wantError: "got unexpected HTTP status: 400 Bad Request",
		},
		{
			name:     "success body not JSON with the token in a header",
			headers:  headerAuth,
			response: cannedResponse{statusCode: http.StatusOK, body: "OK"},
			wantErr:  ErrParseResponse,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			client, _ := newRecordingClient(t, tt.response)

			err := (&DefaultSender{}).SendRequestContext(t.Context(), client, testEndpoint, testRequest, tt.headers)

			require.Error(t, err)
			require.NotErrorIs(t, err, ErrSendFailed)

			if tt.wantErr != nil {
				require.ErrorIs(t, err, tt.wantErr)
			}

			if tt.wantError != "" {
				assert.EqualError(t, err, tt.wantError)
			}
		})
	}
}

// TestSendRequestFailures verifies that a transport error, a request that
// cannot be built, a body that cannot be read, and a request that cannot be
// encoded fail the send, without the token in the error.
func TestSendRequestFailures(t *testing.T) {
	t.Parallel()

	for _, headers := range []http.Header{nil, headerAuth} {
		t.Run("transport error "+headers.Get("X-Gotify-Key"), func(t *testing.T) {
			t.Parallel()

			client, _ := newRecordingClient(t, cannedResponse{err: errTransport})

			err := (&DefaultSender{}).SendRequestContext(t.Context(), client, testEndpoint, testRequest, headers)

			require.ErrorIs(t, err, errTransport)
			assert.NotContains(t, err.Error(), testToken)
		})

		t.Run("unparsable URL "+headers.Get("X-Gotify-Key"), func(t *testing.T) {
			t.Parallel()

			// The mock has no expectations, so a request fails the test.
			client := typesmocks.NewMockHTTPClient(t)

			err := (&DefaultSender{}).SendRequestContext(t.Context(), client, "https://bad host/message?token="+testToken, testRequest, headers)

			_, isURLErr := errors.AsType[*url.Error](err)
			require.True(t, isURLErr, "error %v is not a *url.Error", err)
			assert.NotContains(t, err.Error(), testToken)
		})

		t.Run("request not JSON "+headers.Get("X-Gotify-Key"), func(t *testing.T) {
			t.Parallel()

			// The mock has no expectations, so a request fails the test.
			client := typesmocks.NewMockHTTPClient(t)
			request := &MessageRequest{Message: "Hello", Extras: map[string]any{"channel": make(chan int)}}

			require.Error(t, (&DefaultSender{}).SendRequestContext(t.Context(), client, testEndpoint, request, headers))
		})
	}

	t.Run("sentinel errors with the token in a header", func(t *testing.T) {
		t.Parallel()

		client, _ := newRecordingClient(t, cannedResponse{err: errTransport})
		request := &MessageRequest{Message: "Hello", Extras: map[string]any{"channel": make(chan int)}}
		sender := &DefaultSender{}

		require.ErrorIs(t, sender.SendRequestContext(t.Context(), client, testEndpoint, testRequest, headerAuth), ErrSendRequest)
		require.ErrorIs(t, sender.SendRequestContext(t.Context(), client, testEndpoint, request, headerAuth), ErrMarshalRequest)
		require.ErrorIs(t, sender.SendRequestContext(t.Context(), client, "https://bad host/message", testRequest, headerAuth), ErrCreateRequest)
	})

	t.Run("body read error with the token in a header", func(t *testing.T) {
		t.Parallel()

		errRead := errors.New("connection reset")

		client := typesmocks.NewMockHTTPClient(t)
		client.EXPECT().Do(mock.Anything).Return(&http.Response{
			StatusCode: http.StatusOK,
			Body:       io.NopCloser(iotest.ErrReader(errRead)),
		}, nil).Once()

		err := (&DefaultSender{}).SendRequestContext(t.Context(), client, testEndpoint, testRequest, headerAuth)

		require.ErrorIs(t, err, ErrReadResponse)
		require.ErrorIs(t, err, errRead)
	})
}

// TestSendRequestHeaders verifies that the request carries the JSON content
// type and every value of each given header.
func TestSendRequestHeaders(t *testing.T) {
	t.Parallel()

	var got http.Header

	client := typesmocks.NewMockHTTPClient(t)
	client.EXPECT().Do(mock.Anything).RunAndReturn(func(req *http.Request) (*http.Response, error) {
		got = req.Header.Clone()

		return &http.Response{StatusCode: http.StatusOK, Body: io.NopCloser(strings.NewReader(sentBody))}, nil
	}).Once()

	headers := http.Header{"X-Gotify-Key": {testToken}, "X-Trace": {"a", "b"}}

	require.NoError(t, (&DefaultSender{}).SendRequestContext(context.Background(), client, testEndpoint, testRequest, headers))
	assert.Equal(t, http.Header{"Content-Type": {"application/json"}, "X-Gotify-Key": {testToken}, "X-Trace": {"a", "b"}}, got)
}
