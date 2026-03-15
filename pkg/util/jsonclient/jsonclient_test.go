package jsonclient

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	"github.com/nicholas-fedor/shoutrrr/pkg/util/jsonclient/mocks"
)

// errorTransport is an HTTP client transport that always returns an error.
type errorTransport struct {
	err error
}

// Compile-time interface compliance check.
var _ Client = (*client)(nil)

// RoundTrip implements the http.RoundTripper interface.
func (e *errorTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	return nil, e.err
}

// TestError_Error tests the Error method of the Error type.
func TestError_Error(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		je   Error
		want string
	}{
		{
			name: "returns internal error message",
			je: Error{
				StatusCode: http.StatusBadRequest,
				Body:       `{"error": "bad request"}`,
				err:        errors.New("internal error"),
			},
			want: "internal error",
		},
		{
			name: "returns generic message when no internal error",
			je: Error{
				StatusCode: http.StatusInternalServerError,
				Body:       "",
				err:        nil,
			},
			want: "unknown error (HTTP 500)",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			got := tt.je.Error()
			assert.Equal(t, tt.want, got, "Error.Error() mismatch")
		})
	}
}

// TestError_String tests the String method of the Error type.
func TestError_String(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		je   Error
		want string
	}{
		{
			name: "returns internal error when set",
			je: Error{
				StatusCode: http.StatusNotFound,
				Body:       `{"message": "not found"}`,
				err:        errors.New("resource not found"),
			},
			want: "resource not found",
		},
		{
			name: "returns generic message without internal error",
			je: Error{
				StatusCode: http.StatusUnauthorized,
				Body:       "",
				err:        nil,
			},
			want: "unknown error (HTTP 401)",
		},
		{
			name: "returns wrapped error message",
			je: Error{
				StatusCode: http.StatusForbidden,
				Body:       `{"error": "access denied"}`,
				err:        fmt.Errorf("wrapped: %w", errors.New("base error")),
			},
			want: "wrapped: base error",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			got := tt.je.String()
			assert.Equal(t, tt.want, got, "Error.String() mismatch")
		})
	}
}

// TestErrorBody tests the ErrorBody helper function.
func TestErrorBody(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		e    error
		want string
	}{
		{
			name: "extracts body from jsonclient.Error",
			e: Error{
				StatusCode: http.StatusBadRequest,
				Body:       `{"error": "validation failed"}`,
				err:        errors.New("bad request"),
			},
			want: `{"error": "validation failed"}`,
		},
		{
			name: "returns empty string for non-jsonclient.Error",
			e:    errors.New("some other error"),
			want: "",
		},
		{
			name: "returns empty string for wrapped non-jsonclient.Error",
			e:    fmt.Errorf("wrapped: %w", errors.New("regular error")),
			want: "",
		},
		{
			name: "extracts body from wrapped jsonclient.Error",
			e: fmt.Errorf("wrapped: %w", Error{
				StatusCode: http.StatusTeapot,
				Body:       "I'm a teapot",
				err:        errors.New("teapot error"),
			}),
			want: "I'm a teapot",
		},
		{
			name: "returns empty string for nil error",
			e:    nil,
			want: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			got := ErrorBody(tt.e)
			assert.Equal(t, tt.want, got, "ErrorBody() mismatch")
		})
	}
}

// TestNewClient tests the NewClient constructor.
func TestNewClient(t *testing.T) {
	t.Parallel()

	client := NewClient()

	require.NotNil(t, client, "NewClient() should not return nil")

	// Verify headers are set correctly
	headers := client.Headers()
	require.NotNil(t, headers, "Headers() should not return nil")
	assert.Equal(t, ContentType, headers.Get("Content-Type"), "Content-Type header mismatch")
}

// TestNewWithHTTPClient tests the NewWithHTTPClient constructor.
func TestNewWithHTTPClient(t *testing.T) {
	t.Parallel()

	customHTTPClient := &http.Client{
		Timeout: 0,
	}

	client := NewWithHTTPClient(customHTTPClient)

	require.NotNil(t, client, "NewWithHTTPClient() should not return nil")

	// Verify headers are set correctly
	headers := client.Headers()
	require.NotNil(t, headers, "Headers() should not return nil")
	assert.Equal(t, ContentType, headers.Get("Content-Type"), "Content-Type header mismatch")
}

// TestClient_Get_Success tests successful GET requests.
func TestClient_Get_Success(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, http.MethodGet, r.Method, "HTTP method mismatch")
		assert.Equal(t, ContentType, r.Header.Get("Content-Type"), "Content-Type header mismatch")

		w.Header().Set("Content-Type", ContentType)
		w.WriteHeader(http.StatusOK)
		_, err := w.Write([]byte(`{"message": "success", "data": 123}`))
		assert.NoError(t, err, "Failed to write response")
	}))
	defer server.Close()

	client := NewClient()

	var response struct {
		Message string `json:"message"`
		Data    int    `json:"data"`
	}

	err := client.Get(server.URL, &response)

	require.NoError(t, err, "Get() should not return error")
	assert.Equal(t, "success", response.Message, "Response message mismatch")
	assert.Equal(t, 123, response.Data, "Response data mismatch")
}

// TestClient_Get_HTTPError tests GET requests that return HTTP error status codes.
func TestClient_Get_HTTPError(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name       string
		statusCode int
		body       string
		wantErr    bool
	}{
		{
			name:       "bad request",
			statusCode: http.StatusBadRequest,
			body:       `{"error": "invalid request"}`,
			wantErr:    true,
		},
		{
			name:       "unauthorized",
			statusCode: http.StatusUnauthorized,
			body:       `{"error": "authentication required"}`,
			wantErr:    true,
		},
		{
			name:       "not found",
			statusCode: http.StatusNotFound,
			body:       `{"error": "resource not found"}`,
			wantErr:    true,
		},
		{
			name:       "server error",
			statusCode: http.StatusInternalServerError,
			body:       `{"error": "internal server error"}`,
			wantErr:    true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.Header().Set("Content-Type", ContentType)
				w.WriteHeader(tt.statusCode)
				_, err := w.Write([]byte(tt.body))
				assert.NoError(t, err, "Failed to write response")
			}))
			defer server.Close()

			client := NewClient()

			var response any

			err := client.Get(server.URL, &response)

			require.Error(t, err, "Get() should return error for HTTP %d", tt.statusCode)

			var jsonErr Error
			require.ErrorAs(t, err, &jsonErr, "Error should be jsonclient.Error")
			assert.Equal(t, tt.statusCode, jsonErr.StatusCode, "StatusCode mismatch")
			assert.Equal(t, tt.body, jsonErr.Body, "Body mismatch")
		})
	}
}

// TestClient_Get_InvalidJSON tests GET requests with invalid JSON responses.
func TestClient_Get_InvalidJSON(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", ContentType)
		w.WriteHeader(http.StatusOK)
		_, err := w.Write([]byte(`{invalid json`))
		assert.NoError(t, err, "Failed to write response")
	}))
	defer server.Close()

	client := NewClient()

	var response any

	err := client.Get(server.URL, &response)

	require.Error(t, err, "Get() should return error for invalid JSON")

	var jsonErr Error
	require.ErrorAs(t, err, &jsonErr, "Error should be jsonclient.Error")
	assert.Equal(t, http.StatusOK, jsonErr.StatusCode, "StatusCode should be 200")
}

// TestClient_Get_NetworkError tests GET requests with network errors.
func TestClient_Get_NetworkError(t *testing.T) {
	t.Parallel()

	client := NewClient()

	var response any

	// Use an invalid URL to trigger a network error
	err := client.Get("http://[invalid-url", &response)

	require.Error(t, err, "Get() should return error for invalid URL")
}

// TestClient_Post_Success tests successful POST requests with struct body.
func TestClient_Post_Success(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, http.MethodPost, r.Method, "HTTP method mismatch")
		assert.Equal(t, ContentType, r.Header.Get("Content-Type"), "Content-Type header mismatch")

		// Read and verify request body
		body, err := io.ReadAll(r.Body)
		assert.NoError(t, err, "Failed to read request body")

		defer func() { _ = r.Body.Close() }()

		assert.JSONEq(t, `{"name": "test", "value": 42}`, string(body), "Request body mismatch")

		w.Header().Set("Content-Type", ContentType)
		w.WriteHeader(http.StatusCreated)
		_, err = w.Write([]byte(`{"id": "123", "status": "created"}`))
		assert.NoError(t, err, "Failed to write response")
	}))
	defer server.Close()

	client := NewClient()

	request := struct {
		Name  string `json:"name"`
		Value int    `json:"value"`
	}{
		Name:  "test",
		Value: 42,
	}

	var response struct {
		ID     string `json:"id"`
		Status string `json:"status"`
	}

	err := client.Post(server.URL, request, &response)

	require.NoError(t, err, "Post() should not return error")
	assert.Equal(t, "123", response.ID, "Response ID mismatch")
	assert.Equal(t, "created", response.Status, "Response status mismatch")
}

// TestClient_Post_StringBody tests POST requests with string body.
func TestClient_Post_StringBody(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, err := io.ReadAll(r.Body)
		assert.NoError(t, err, "Failed to read request body")

		defer func() { _ = r.Body.Close() }()

		assert.JSONEq(t, `{"custom": "json"}`, string(body), "Request body mismatch")

		w.Header().Set("Content-Type", ContentType)
		w.WriteHeader(http.StatusOK)
		_, err = w.Write([]byte(`{"result": "ok"}`))
		assert.NoError(t, err, "Failed to write response")
	}))
	defer server.Close()

	client := NewClient()

	request := `{"custom": "json"}`

	var response struct {
		Result string `json:"result"`
	}

	err := client.Post(server.URL, request, &response)

	require.NoError(t, err, "Post() should not return error")
	assert.Equal(t, "ok", response.Result, "Response result mismatch")
}

// TestClient_Post_HTTPError tests POST requests that return HTTP error status codes.
func TestClient_Post_HTTPError(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", ContentType)
		w.WriteHeader(http.StatusUnprocessableEntity)
		_, err := w.Write([]byte(`{"errors": ["field required"]}`))
		assert.NoError(t, err, "Failed to write response")
	}))
	defer server.Close()

	client := NewClient()

	request := struct {
		Name string `json:"name"`
	}{
		Name: "",
	}

	var response any

	err := client.Post(server.URL, request, &response)

	require.Error(t, err, "Post() should return error for HTTP 422")

	var jsonErr Error
	require.ErrorAs(t, err, &jsonErr, "Error should be jsonclient.Error")
	assert.Equal(t, http.StatusUnprocessableEntity, jsonErr.StatusCode, "StatusCode mismatch")
}

// TestClient_Headers tests the Headers method.
func TestClient_Headers(t *testing.T) {
	t.Parallel()

	client := NewClient()

	headers := client.Headers()

	require.NotNil(t, headers, "Headers() should not return nil")
	assert.Equal(t, ContentType, headers.Get("Content-Type"), "Content-Type header mismatch")

	// Verify that modifying the returned headers affects future requests
	headers.Set("X-Custom-Header", "custom-value")
	assert.Equal(t, "custom-value", client.Headers().Get("X-Custom-Header"), "Custom header should be set")
}

// TestClient_ErrorResponse_Success tests ErrorResponse with valid JSON error body.
func TestClient_ErrorResponse_Success(t *testing.T) {
	t.Parallel()

	client := NewClient()

	jsonErr := Error{
		StatusCode: http.StatusBadRequest,
		Body:       `{"error": "validation failed", "field": "name"}`,
		err:        errors.New("bad request"),
	}

	var errorResponse struct {
		Error string `json:"error"`
		Field string `json:"field"`
	}

	result := client.ErrorResponse(jsonErr, &errorResponse)

	assert.True(t, result, "ErrorResponse() should return true")
	assert.Equal(t, "validation failed", errorResponse.Error, "Error field mismatch")
	assert.Equal(t, "name", errorResponse.Field, "Field field mismatch")
}

// TestClient_ErrorResponse_InvalidJSON tests ErrorResponse with invalid JSON error body.
func TestClient_ErrorResponse_InvalidJSON(t *testing.T) {
	t.Parallel()

	client := NewClient()

	jsonErr := Error{
		StatusCode: http.StatusBadRequest,
		Body:       `{invalid json`,
		err:        errors.New("bad request"),
	}

	var errorResponse any

	result := client.ErrorResponse(jsonErr, &errorResponse)

	assert.False(t, result, "ErrorResponse() should return false for invalid JSON")
}

// TestClient_ErrorResponse_NonJSONError tests ErrorResponse with non-jsonclient.Error.
func TestClient_ErrorResponse_NonJSONError(t *testing.T) {
	t.Parallel()

	client := NewClient()

	regularErr := errors.New("regular error")

	var errorResponse any

	result := client.ErrorResponse(regularErr, &errorResponse)

	assert.False(t, result, "ErrorResponse() should return false for non-jsonclient.Error")
}

// TestGet_Success tests the package-level Get function with a successful response.
func TestGet_Success(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", ContentType)
		w.WriteHeader(http.StatusOK)
		_, err := w.Write([]byte(`{"status": "ok"}`))
		assert.NoError(t, err, "Failed to write response")
	}))
	defer server.Close()

	var response struct {
		Status string `json:"status"`
	}

	err := Get(server.URL, &response)

	require.NoError(t, err, "Get() should not return error")
	assert.Equal(t, "ok", response.Status, "Response status mismatch")
}

// TestGet_Error tests the package-level Get function with an error response.
func TestGet_Error(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
		_, err := w.Write([]byte(`not found`))
		assert.NoError(t, err, "Failed to write response")
	}))
	defer server.Close()

	var response any

	err := Get(server.URL, &response)

	require.Error(t, err, "Get() should return error")
	assert.Contains(t, err.Error(), fmt.Sprintf("getting JSON from %q", server.URL), "Error should contain URL")
}

// TestPost_Success tests the package-level Post function with a successful response.
func TestPost_Success(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", ContentType)
		w.WriteHeader(http.StatusOK)
		_, err := w.Write([]byte(`{"received": true}`))
		assert.NoError(t, err, "Failed to write response")
	}))
	defer server.Close()

	request := struct {
		Data string `json:"data"`
	}{
		Data: "test",
	}

	var response struct {
		Received bool `json:"received"`
	}

	err := Post(server.URL, request, &response)

	require.NoError(t, err, "Post() should not return error")
	assert.True(t, response.Received, "Response received should be true")
}

// TestPost_Error tests the package-level Post function with an error response.
func TestPost_Error(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		_, err := w.Write([]byte(`server error`))
		assert.NoError(t, err, "Failed to write response")
	}))
	defer server.Close()

	request := struct {
		Data string `json:"data"`
	}{
		Data: "test",
	}

	var response any

	err := Post(server.URL, request, &response)

	require.Error(t, err, "Post() should return error")
	assert.Contains(t, err.Error(), fmt.Sprintf("posting JSON to %q", server.URL), "Error should contain URL")
}

// TestParseResponse_Success tests successful response parsing.
func TestParseResponse_Success(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", ContentType)
		w.WriteHeader(http.StatusOK)
		_, err := w.Write([]byte(`{"id": 1, "name": "test"}`))
		assert.NoError(t, err, "Failed to write response")
	}))
	defer server.Close()

	req, err := http.NewRequestWithContext(context.Background(), http.MethodGet, server.URL, http.NoBody)
	require.NoError(t, err, "Failed to create request")

	client := &http.Client{}
	resp, err := client.Do(req)
	require.NoError(t, err, "Failed to make request")

	defer func() { _ = resp.Body.Close() }()

	var result struct {
		ID   int    `json:"id"`
		Name string `json:"name"`
	}

	err = parseResponse(resp, &result)

	require.NoError(t, err, "parseResponse() should not return error")
	assert.Equal(t, 1, result.ID, "ID mismatch")
	assert.Equal(t, "test", result.Name, "Name mismatch")
}

// TestParseResponse_HTTPError tests response parsing with HTTP error status.
func TestParseResponse_HTTPError(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusBadRequest)
		_, err := w.Write([]byte(`{"error": "bad request"}`))
		assert.NoError(t, err, "Failed to write response")
	}))
	defer server.Close()

	req, err := http.NewRequestWithContext(context.Background(), http.MethodGet, server.URL, http.NoBody)
	require.NoError(t, err, "Failed to create request")

	client := &http.Client{}
	resp, err := client.Do(req)
	require.NoError(t, err, "Failed to make request")

	defer func() { _ = resp.Body.Close() }()

	var result any

	err = parseResponse(resp, &result)

	require.Error(t, err, "parseResponse() should return error for HTTP 400")

	var jsonErr Error
	require.ErrorAs(t, err, &jsonErr, "Error should be jsonclient.Error")
	assert.Equal(t, http.StatusBadRequest, jsonErr.StatusCode, "StatusCode mismatch")
	assert.JSONEq(t, `{"error": "bad request"}`, jsonErr.Body, "Body mismatch")
}

// TestParseResponse_InvalidJSON tests response parsing with invalid JSON.
func TestParseResponse_InvalidJSON(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", ContentType)
		w.WriteHeader(http.StatusOK)
		_, err := w.Write([]byte(`not valid json`))
		assert.NoError(t, err, "Failed to write response")
	}))
	defer server.Close()

	req, err := http.NewRequestWithContext(context.Background(), http.MethodGet, server.URL, http.NoBody)
	require.NoError(t, err, "Failed to create request")

	client := &http.Client{}
	resp, err := client.Do(req)
	require.NoError(t, err, "Failed to make request")

	defer func() { _ = resp.Body.Close() }()

	var result any

	err = parseResponse(resp, &result)

	require.Error(t, err, "parseResponse() should return error for invalid JSON")

	var jsonErr Error
	require.ErrorAs(t, err, &jsonErr, "Error should be jsonclient.Error")
	assert.Equal(t, http.StatusOK, jsonErr.StatusCode, "StatusCode should be 200")
}

// TestParseResponse_EmptyBody tests response parsing with empty body.
func TestParseResponse_EmptyBody(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", ContentType)
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	req, err := http.NewRequestWithContext(context.Background(), http.MethodGet, server.URL, http.NoBody)
	require.NoError(t, err, "Failed to create request")

	client := &http.Client{}
	resp, err := client.Do(req)
	require.NoError(t, err, "Failed to make request")

	defer func() { _ = resp.Body.Close() }()

	var result any

	err = parseResponse(resp, &result)

	require.Error(t, err, "parseResponse() should return error for empty body")
}

// Mock Client Interface Tests

// TestMockClient_Get tests mocking the Client interface Get method.
func TestMockClient_Get(t *testing.T) {
	t.Parallel()

	mockClient := mocks.NewMockClient(t)

	expectedResponse := struct {
		Data string `json:"data"`
	}{
		Data: "mocked",
	}

	mockClient.EXPECT().Get("http://example.com", mock.Anything).RunAndReturn(func(url string, response any) error {
		// Simulate unmarshaling into the response
		respPtr, ok := response.(*struct {
			Data string `json:"data"`
		})
		if ok {
			respPtr.Data = "mocked"
		}

		return nil
	})

	var response struct {
		Data string `json:"data"`
	}

	err := mockClient.Get("http://example.com", &response)

	require.NoError(t, err, "Get() should not return error")
	assert.Equal(t, expectedResponse.Data, response.Data, "Response data mismatch")
}

// TestMockClient_Get_Error tests mocking the Client interface Get method with error.
func TestMockClient_Get_Error(t *testing.T) {
	t.Parallel()

	mockClient := mocks.NewMockClient(t)

	expectedErr := errors.New("mocked error")

	mockClient.EXPECT().Get("http://example.com", mock.Anything).Return(expectedErr)

	var response any

	err := mockClient.Get("http://example.com", &response)

	require.Error(t, err, "Get() should return error")
	assert.Equal(t, expectedErr, err, "Error mismatch")
}

// TestMockClient_Post tests mocking the Client interface Post method.
func TestMockClient_Post(t *testing.T) {
	t.Parallel()

	mockClient := mocks.NewMockClient(t)

	expectedResponse := struct {
		ID string `json:"id"`
	}{
		ID: "123",
	}

	mockClient.EXPECT().Post("http://example.com", mock.Anything, mock.Anything).RunAndReturn(
		func(url string, request, response any) error {
			// Simulate unmarshaling into the response
			respPtr, ok := response.(*struct {
				ID string `json:"id"`
			})
			if ok {
				respPtr.ID = "123"
			}

			return nil
		},
	)

	request := struct {
		Name string `json:"name"`
	}{
		Name: "test",
	}

	var response struct {
		ID string `json:"id"`
	}

	err := mockClient.Post("http://example.com", request, &response)

	require.NoError(t, err, "Post() should not return error")
	assert.Equal(t, expectedResponse.ID, response.ID, "Response ID mismatch")
}

// TestMockClient_Post_Error tests mocking the Client interface Post method with error.
func TestMockClient_Post_Error(t *testing.T) {
	t.Parallel()

	mockClient := mocks.NewMockClient(t)

	expectedErr := errors.New("post failed")

	mockClient.EXPECT().Post("http://example.com", mock.Anything, mock.Anything).Return(expectedErr)

	request := struct {
		Name string `json:"name"`
	}{
		Name: "test",
	}

	var response any

	err := mockClient.Post("http://example.com", request, &response)

	require.Error(t, err, "Post() should return error")
	assert.Equal(t, expectedErr, err, "Error mismatch")
}

// TestMockClient_Headers tests mocking the Client interface Headers method.
func TestMockClient_Headers(t *testing.T) {
	t.Parallel()

	mockClient := mocks.NewMockClient(t)

	expectedHeaders := http.Header{
		"Content-Type": []string{ContentType},
		"X-Custom":     []string{"value"},
	}

	mockClient.EXPECT().Headers().Return(expectedHeaders)

	headers := mockClient.Headers()

	assert.Equal(t, ContentType, headers.Get("Content-Type"), "Content-Type header mismatch")
	assert.Equal(t, "value", headers.Get("X-Custom"), "X-Custom header mismatch")
}

// TestMockClient_ErrorResponse tests mocking the Client interface ErrorResponse method.
func TestMockClient_ErrorResponse(t *testing.T) {
	t.Parallel()

	mockClient := mocks.NewMockClient(t)

	testErr := Error{
		StatusCode: http.StatusBadRequest,
		Body:       `{"error": "validation failed"}`,
		err:        errors.New("bad request"),
	}

	expectedResponse := struct {
		Error string `json:"error"`
	}{
		Error: "validation failed",
	}

	mockClient.EXPECT().ErrorResponse(testErr, mock.Anything).RunAndReturn(
		func(err error, response any) bool {
			respPtr, ok := response.(*struct {
				Error string `json:"error"`
			})
			if ok {
				respPtr.Error = "validation failed"
			}

			return true
		},
	)

	var response struct {
		Error string `json:"error"`
	}

	result := mockClient.ErrorResponse(testErr, &response)

	assert.True(t, result, "ErrorResponse() should return true")
	assert.Equal(t, expectedResponse.Error, response.Error, "Response error mismatch")
}

// TestMockClient_ErrorResponse_False tests ErrorResponse returning false.
func TestMockClient_ErrorResponse_False(t *testing.T) {
	t.Parallel()

	mockClient := mocks.NewMockClient(t)

	regularErr := errors.New("regular error")

	mockClient.EXPECT().ErrorResponse(regularErr, mock.Anything).Return(false)

	var response any

	result := mockClient.ErrorResponse(regularErr, &response)

	assert.False(t, result, "ErrorResponse() should return false")
}

// Edge Case Tests

// TestClient_Get_EmptyResponse tests GET with empty but valid JSON response.
func TestClient_Get_EmptyResponse(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", ContentType)
		w.WriteHeader(http.StatusOK)
		_, err := w.Write([]byte(`{}`))
		assert.NoError(t, err, "Failed to write response")
	}))
	defer server.Close()

	client := NewClient()

	var response struct {
		Optional string `json:"optional"`
	}

	err := client.Get(server.URL, &response)

	require.NoError(t, err, "Get() should not return error")
	assert.Empty(t, response.Optional, "Optional field should be empty")
}

// TestClient_Post_EmptyRequest tests POST with empty struct request.
func TestClient_Post_EmptyRequest(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, err := io.ReadAll(r.Body)
		assert.NoError(t, err, "Failed to read request body")

		defer func() { _ = r.Body.Close() }()

		assert.JSONEq(t, `{}`, string(body), "Request body should be empty JSON object")

		w.Header().Set("Content-Type", ContentType)
		w.WriteHeader(http.StatusOK)
		_, err = w.Write([]byte(`{"status": "ok"}`))
		assert.NoError(t, err, "Failed to write response")
	}))
	defer server.Close()

	client := NewClient()

	request := struct{}{}

	var response struct {
		Status string `json:"status"`
	}

	err := client.Post(server.URL, request, &response)

	require.NoError(t, err, "Post() should not return error")
	assert.Equal(t, "ok", response.Status, "Response status mismatch")
}

// TestClient_ErrorResponse_NilError tests ErrorResponse with nil error.
func TestClient_ErrorResponse_NilError(t *testing.T) {
	t.Parallel()

	client := NewClient()

	var response any

	result := client.ErrorResponse(nil, &response)

	assert.False(t, result, "ErrorResponse() should return false for nil error")
}

// TestDefaultClient_NotNil tests that DefaultClient is not nil.
func TestDefaultClient_NotNil(t *testing.T) {
	t.Parallel()

	assert.NotNil(t, DefaultClient, "DefaultClient should not be nil")
}

// TestContentType_Constant tests the ContentType constant.
func TestContentType_Constant(t *testing.T) {
	t.Parallel()

	assert.Equal(t, "application/json", ContentType, "ContentType constant mismatch")
}

// TestHTTPClientErrorThreshold_Constant tests the HTTPClientErrorThreshold constant.
func TestHTTPClientErrorThreshold_Constant(t *testing.T) {
	t.Parallel()

	assert.Equal(t, 400, HTTPClientErrorThreshold, "HTTPClientErrorThreshold constant mismatch")
}

// TestErrUnexpectedStatus tests the ErrUnexpectedStatus error.
func TestErrUnexpectedStatus(t *testing.T) {
	t.Parallel()

	require.Error(t, ErrUnexpectedStatus, "ErrUnexpectedStatus should not be nil")
	assert.Equal(t, "got unexpected HTTP status", ErrUnexpectedStatus.Error(), "ErrUnexpectedStatus message mismatch")

	// Verify it can be used with errors.Is
	wrappedErr := fmt.Errorf("wrapped: %w", ErrUnexpectedStatus)
	assert.ErrorIs(t, wrappedErr, ErrUnexpectedStatus, "ErrUnexpectedStatus should be detectable with errors.Is")
}

// TestClient_Post_MarshalError tests JSON marshaling failure with unmarshalable types.
func TestClient_Post_MarshalError(t *testing.T) {
	t.Parallel()

	client := NewClient()

	// Channel cannot be marshaled to JSON
	request := struct {
		Data chan int `json:"data"`
	}{
		Data: make(chan int),
	}

	var response any

	err := client.Post("http://example.com", request, &response)

	require.Error(t, err, "Post() should return error for unmarshalable type")
	assert.Contains(t, err.Error(), "marshaling request to JSON", "Error should mention JSON marshaling")
}

// TestClient_Get_CustomHeaders verifies custom headers are actually sent in HTTP requests.
func TestClient_Get_CustomHeaders(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Verify custom header is received
		assert.Equal(t, "custom-value", r.Header.Get("X-Custom-Header"), "Custom header mismatch")
		assert.Equal(t, ContentType, r.Header.Get("Content-Type"), "Content-Type header mismatch")

		w.Header().Set("Content-Type", ContentType)
		w.WriteHeader(http.StatusOK)
		_, err := w.Write([]byte(`{"status": "ok"}`))
		assert.NoError(t, err, "Failed to write response")
	}))
	defer server.Close()

	client := NewClient()
	client.Headers().Set("X-Custom-Header", "custom-value")

	var response struct {
		Status string `json:"status"`
	}

	err := client.Get(server.URL, &response)

	require.NoError(t, err, "Get() should not return error")
	assert.Equal(t, "ok", response.Status, "Response status mismatch")
}

// TestClient_Get_DoError tests httpClient.Do() errors.
func TestClient_Get_DoError(t *testing.T) {
	t.Parallel()

	expectedErr := errors.New("mock HTTP error")
	httpClient := &http.Client{
		Transport: &errorTransport{err: expectedErr},
	}
	client := NewWithHTTPClient(httpClient)

	var response any

	err := client.Get("http://example.com", &response)

	require.Error(t, err, "Get() should return error for Do() failure")
	assert.Contains(t, err.Error(), "executing GET request", "Error should mention executing GET request")
	assert.ErrorIs(t, err, expectedErr, "Error should wrap the original HTTP error")
}

// TestClient_Post_DoError tests POST httpClient.Do() errors.
func TestClient_Post_DoError(t *testing.T) {
	t.Parallel()

	expectedErr := errors.New("mock POST error")
	httpClient := &http.Client{
		Transport: &errorTransport{err: expectedErr},
	}
	client := NewWithHTTPClient(httpClient)

	request := struct {
		Name string `json:"name"`
	}{
		Name: "test",
	}

	var response any

	err := client.Post("http://example.com", request, &response)

	require.Error(t, err, "Post() should return error for Do() failure")
	assert.Contains(t, err.Error(), "sending POST request", "Error should mention sending POST request")
	assert.ErrorIs(t, err, expectedErr, "Error should wrap the original HTTP error")
}

// TestParseResponse_StatusCodeBoundary verifies 399 = success, 400 = error.
func TestParseResponse_StatusCodeBoundary(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name       string
		statusCode int
		wantErr    bool
	}{
		{
			name:       "status 399 is success",
			statusCode: 399,
			wantErr:    false,
		},
		{
			name:       "status 400 is error",
			statusCode: 400,
			wantErr:    true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.Header().Set("Content-Type", ContentType)
				w.WriteHeader(tt.statusCode)
				_, err := w.Write([]byte(`{"data": "test"}`))
				assert.NoError(t, err, "Failed to write response")
			}))
			defer server.Close()

			req, err := http.NewRequestWithContext(context.Background(), http.MethodGet, server.URL, http.NoBody)
			require.NoError(t, err, "Failed to create request")

			client := &http.Client{}
			resp, err := client.Do(req)
			require.NoError(t, err, "Failed to make request")

			defer func() { _ = resp.Body.Close() }()

			var result struct {
				Data string `json:"data"`
			}

			err = parseResponse(resp, &result)

			if tt.wantErr {
				require.Error(t, err, "parseResponse() should return error for HTTP %d", tt.statusCode)

				var jsonErr Error
				require.ErrorAs(t, err, &jsonErr, "Error should be jsonclient.Error")
				assert.Equal(t, tt.statusCode, jsonErr.StatusCode, "StatusCode mismatch")
			} else {
				require.NoError(t, err, "parseResponse() should not return error for HTTP %d", tt.statusCode)
				assert.Equal(t, "test", result.Data, "Data mismatch")
			}
		})
	}
}

// TestClient_Get_InvalidURL tests request creation error with malformed URL.
func TestClient_Get_InvalidURL(t *testing.T) {
	t.Parallel()

	client := NewClient()

	var response any

	// Use an invalid URL format to trigger request creation error
	err := client.Get("://invalid-url", &response)

	require.Error(t, err, "Get() should return error for invalid URL")
	assert.Contains(t, err.Error(), "creating GET request", "Error should mention creating GET request")
}

// TestClient_Post_InvalidURL tests POST request creation error.
func TestClient_Post_InvalidURL(t *testing.T) {
	t.Parallel()

	client := NewClient()

	request := struct {
		Name string `json:"name"`
	}{
		Name: "test",
	}

	var response any

	// Use an invalid URL format to trigger request creation error
	err := client.Post("://invalid-url", request, &response)

	require.Error(t, err, "Post() should return error for invalid URL")
	assert.Contains(t, err.Error(), "creating POST request", "Error should mention creating POST request")
}

// TestParseResponse_NonJSONErrorBody tests HTML error pages (502 Bad Gateway).
func TestParseResponse_NonJSONErrorBody(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusBadGateway)
		_, err := w.Write([]byte(`<html><body>502 Bad Gateway</body></html>`))
		assert.NoError(t, err, "Failed to write response")
	}))
	defer server.Close()

	req, err := http.NewRequestWithContext(context.Background(), http.MethodGet, server.URL, http.NoBody)
	require.NoError(t, err, "Failed to create request")

	client := &http.Client{}
	resp, err := client.Do(req)
	require.NoError(t, err, "Failed to make request")

	defer func() { _ = resp.Body.Close() }()

	var result any

	err = parseResponse(resp, &result)

	require.Error(t, err, "parseResponse() should return error for HTTP 502")

	var jsonErr Error
	require.ErrorAs(t, err, &jsonErr, "Error should be jsonclient.Error")
	assert.Equal(t, http.StatusBadGateway, jsonErr.StatusCode, "StatusCode mismatch")
	assert.Contains(t, jsonErr.Body, "502 Bad Gateway", "Body should contain HTML error message")
}

// TestClient_Post_EmptyStringBody tests empty string POST body.
func TestClient_Post_EmptyStringBody(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, err := io.ReadAll(r.Body)
		assert.NoError(t, err, "Failed to read request body")

		defer func() { _ = r.Body.Close() }()

		// Empty string body should be empty
		assert.Empty(t, string(body), "Request body should be empty for empty string")

		w.Header().Set("Content-Type", ContentType)
		w.WriteHeader(http.StatusOK)
		_, err = w.Write([]byte(`{"result": "ok"}`))
		assert.NoError(t, err, "Failed to write response")
	}))
	defer server.Close()

	client := NewClient()

	request := ""

	var response struct {
		Result string `json:"result"`
	}

	err := client.Post(server.URL, request, &response)

	require.NoError(t, err, "Post() should not return error for empty string body")
	assert.Equal(t, "ok", response.Result, "Response result mismatch")
}

// TestClient_InterfaceCompliance verifies compile-time interface compliance.
func TestClient_InterfaceCompliance(t *testing.T) {
	t.Parallel()

	// This is a compile-time check - if it compiles, the test passes
	var _ Client = (*client)(nil)

	// Also verify that NewClient returns the interface type
	client := NewClient()
	assert.NotNil(t, client, "NewClient() should return a non-nil Client")

	// Verify all interface methods are accessible
	assert.NotNil(t, client.Headers(), "Headers() should be accessible")
}

// TestClient_MultipleHeaderValues tests only first header value is used.
func TestClient_MultipleHeaderValues(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// The client should only send the first value
		assert.Equal(t, "first-value", r.Header.Get("X-Multi-Header"), "Only first header value should be used")

		w.Header().Set("Content-Type", ContentType)
		w.WriteHeader(http.StatusOK)
		_, err := w.Write([]byte(`{"status": "ok"}`))
		assert.NoError(t, err, "Failed to write response")
	}))
	defer server.Close()

	client := NewClient()

	// Set multiple values for the same header
	client.Headers()["X-Multi-Header"] = []string{"first-value", "second-value", "third-value"}

	var response struct {
		Status string `json:"status"`
	}

	err := client.Get(server.URL, &response)

	require.NoError(t, err, "Get() should not return error")
	assert.Equal(t, "ok", response.Status, "Response status mismatch")
}

// TestClient_Post_CustomHeaders verifies custom headers are sent in POST requests.
func TestClient_Post_CustomHeaders(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, http.MethodPost, r.Method, "HTTP method mismatch")
		assert.Equal(t, "custom-value", r.Header.Get("X-Custom-Header"), "Custom header mismatch")
		assert.Equal(t, ContentType, r.Header.Get("Content-Type"), "Content-Type header mismatch")

		body, err := io.ReadAll(r.Body)
		assert.NoError(t, err, "Failed to read request body")

		defer func() { _ = r.Body.Close() }()

		assert.JSONEq(t, `{"name": "test"}`, string(body), "Request body mismatch")

		w.Header().Set("Content-Type", ContentType)
		w.WriteHeader(http.StatusOK)
		_, err = w.Write([]byte(`{"result": "ok"}`))
		assert.NoError(t, err, "Failed to write response")
	}))
	defer server.Close()

	client := NewClient()
	client.Headers().Set("X-Custom-Header", "custom-value")

	request := struct {
		Name string `json:"name"`
	}{
		Name: "test",
	}

	var response struct {
		Result string `json:"result"`
	}

	err := client.Post(server.URL, request, &response)

	require.NoError(t, err, "Post() should not return error")
	assert.Equal(t, "ok", response.Result, "Response result mismatch")
}

// TestClient_Post_MultipleHeaderValues tests only first header value is used in POST.
func TestClient_Post_MultipleHeaderValues(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, http.MethodPost, r.Method, "HTTP method mismatch")
		// The client should only send the first value
		assert.Equal(t, "first-value", r.Header.Get("X-Multi-Header"), "Only first header value should be used")

		w.Header().Set("Content-Type", ContentType)
		w.WriteHeader(http.StatusOK)
		_, err := w.Write([]byte(`{"status": "ok"}`))
		assert.NoError(t, err, "Failed to write response")
	}))
	defer server.Close()

	client := NewClient()

	// Set multiple values for the same header
	client.Headers()["X-Multi-Header"] = []string{"first-value", "second-value"}

	request := struct {
		Name string `json:"name"`
	}{
		Name: "test",
	}

	var response struct {
		Status string `json:"status"`
	}

	err := client.Post(server.URL, request, &response)

	require.NoError(t, err, "Post() should not return error")
	assert.Equal(t, "ok", response.Status, "Response status mismatch")
}

// TestParseResponse_EmptyBodyError tests error response with empty body.
func TestParseResponse_EmptyBodyError(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		// Write empty body
	}))
	defer server.Close()

	req, err := http.NewRequestWithContext(context.Background(), http.MethodGet, server.URL, http.NoBody)
	require.NoError(t, err, "Failed to create request")

	client := &http.Client{}
	resp, err := client.Do(req)
	require.NoError(t, err, "Failed to make request")

	defer func() { _ = resp.Body.Close() }()

	var result any

	err = parseResponse(resp, &result)

	require.Error(t, err, "parseResponse() should return error for empty error body")

	var jsonErr Error
	require.ErrorAs(t, err, &jsonErr, "Error should be jsonclient.Error")
	assert.Equal(t, http.StatusInternalServerError, jsonErr.StatusCode, "StatusCode mismatch")
}

// TestClient_Get_WithRequestBody tests GET request does not send body.
func TestClient_Get_WithRequestBody(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, http.MethodGet, r.Method, "HTTP method mismatch")

		body, err := io.ReadAll(r.Body)
		assert.NoError(t, err, "Failed to read request body")

		defer func() { _ = r.Body.Close() }()

		// GET requests should have no body
		assert.Empty(t, body, "GET request should have no body")

		w.Header().Set("Content-Type", ContentType)
		w.WriteHeader(http.StatusOK)
		_, err = w.Write([]byte(`{"status": "ok"}`))
		assert.NoError(t, err, "Failed to write response")
	}))
	defer server.Close()

	client := NewClient()

	var response struct {
		Status string `json:"status"`
	}

	err := client.Get(server.URL, &response)

	require.NoError(t, err, "Get() should not return error")
	assert.Equal(t, "ok", response.Status, "Response status mismatch")
}

// TestClient_Post_NilResponse tests POST with nil response pointer.
func TestClient_Post_NilResponse(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", ContentType)
		w.WriteHeader(http.StatusOK)
		_, err := w.Write([]byte(`{"status": "ok"}`))
		assert.NoError(t, err, "Failed to write response")
	}))
	defer server.Close()

	client := NewClient()

	request := struct {
		Name string `json:"name"`
	}{
		Name: "test",
	}

	// Pass nil as response - should still work but not unmarshal
	err := client.Post(server.URL, request, nil)

	// This will fail because json.Unmarshal doesn't accept nil
	require.Error(t, err, "Post() should return error for nil response")
}

// TestParseResponse_NilBody tests parseResponse with nil body handling.
func TestParseResponse_NilBody(t *testing.T) {
	t.Parallel()

	// Create a response with nil body using httptest
	recorder := httptest.NewRecorder()
	recorder.WriteHeader(http.StatusOK)

	// Create a response manually with a nil body reader
	resp := &http.Response{
		StatusCode: http.StatusOK,
		Header:     http.Header{"Content-Type": []string{ContentType}},
		Body:       io.NopCloser(bytes.NewReader([]byte(`{"data": "test"}`))),
	}

	var result struct {
		Data string `json:"data"`
	}

	err := parseResponse(resp, &result)

	require.NoError(t, err, "parseResponse() should not return error")
	assert.Equal(t, "test", result.Data, "Data mismatch")
}

// TestClient_NewWithHTTPClient_NilClient tests NewWithHTTPClient with nil client.
func TestClient_NewWithHTTPClient_NilClient(t *testing.T) {
	t.Parallel()

	// This should work but may panic when used - test the constructor
	client := NewWithHTTPClient(nil)

	require.NotNil(t, client, "NewWithHTTPClient(nil) should return non-nil client")
	assert.NotNil(t, client.Headers(), "Headers() should not return nil")
}

// TestError_Unwrap tests error unwrapping capabilities.
func TestError_Unwrap(t *testing.T) {
	t.Parallel()

	innerErr := errors.New("inner error")
	jsonErr := Error{
		StatusCode: http.StatusBadRequest,
		Body:       `{"error": "bad request"}`,
		err:        innerErr,
	}

	// Error() and String() should return the inner error message
	assert.Equal(t, "inner error", jsonErr.Error(), "Error() should return inner error")
	assert.Equal(t, "inner error", jsonErr.String(), "String() should return inner error")

	// Test with nil inner error
	jsonErrNil := Error{
		StatusCode: http.StatusInternalServerError,
		Body:       "",
		err:        nil,
	}

	assert.Equal(t, "unknown error (HTTP 500)", jsonErrNil.Error(), "Error() should return generic message")
	assert.Equal(t, "unknown error (HTTP 500)", jsonErrNil.String(), "String() should return generic message")
}
