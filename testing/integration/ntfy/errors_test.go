package ntfy_test

import (
	"errors"
	"net/http"
	"testing"
	"testing/synctest"

	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

func TestSendWithHTTPError(t *testing.T) {
	t.Parallel()
	synctest.Test(t, func(t *testing.T) {
		mockClient := &MockHTTPClient{}
		service := createTestService(
			t,
			validNtfyURL,
			mockClient,
		)

		mockClient.On("Do", mock.Anything).
			Return(nil, errors.New("connection refused")).
			Once()

		err := service.Send("Test message", nil)
		require.Error(t, err)
		require.Contains(t, err.Error(), "posting to ntfy API")

		mockClient.AssertExpectations(t)
	})
}

func TestSendWith400BadRequest(t *testing.T) {
	t.Parallel()
	synctest.Test(t, func(t *testing.T) {
		mockClient := &MockHTTPClient{}
		service := createTestService(
			t,
			validNtfyURL,
			mockClient,
		)

		mockClient.On("Do", mock.Anything).
			Return(createMockResponse(http.StatusBadRequest, `{"code":400,"error":"invalid request","link":"https://docs.ntfy.sh"}`), nil).
			Once()

		err := service.Send("Test message", nil)
		require.Error(t, err)
		require.Contains(t, err.Error(), "invalid request")

		mockClient.AssertExpectations(t)
	})
}

func TestSendWith401Unauthorized(t *testing.T) {
	t.Parallel()
	synctest.Test(t, func(t *testing.T) {
		mockClient := &MockHTTPClient{}
		service := createTestService(
			t,
			validNtfyURL,
			mockClient,
		)

		mockClient.On("Do", mock.Anything).
			Return(createMockResponse(http.StatusUnauthorized, `{"code":401,"error":"unauthorized"}`), nil).
			Once()

		err := service.Send("Test message", nil)
		require.Error(t, err)
		require.Contains(t, err.Error(), "unauthorized")

		mockClient.AssertExpectations(t)
	})
}

func TestSendWith403Forbidden(t *testing.T) {
	t.Parallel()
	synctest.Test(t, func(t *testing.T) {
		mockClient := &MockHTTPClient{}
		service := createTestService(
			t,
			validNtfyURL,
			mockClient,
		)

		mockClient.On("Do", mock.Anything).
			Return(createMockResponse(http.StatusForbidden, `{"code":403,"error":"forbidden"}`), nil).
			Once()

		err := service.Send("Test message", nil)
		require.Error(t, err)
		require.Contains(t, err.Error(), "forbidden")

		mockClient.AssertExpectations(t)
	})
}

func TestSendWith404NotFound(t *testing.T) {
	t.Parallel()
	synctest.Test(t, func(t *testing.T) {
		mockClient := &MockHTTPClient{}
		service := createTestService(
			t,
			validNtfyURL,
			mockClient,
		)

		mockClient.On("Do", mock.Anything).
			Return(createMockResponse(http.StatusNotFound, `{"code":404,"error":"not found"}`), nil).
			Once()

		err := service.Send("Test message", nil)
		require.Error(t, err)
		require.Contains(t, err.Error(), "not found")

		mockClient.AssertExpectations(t)
	})
}

func TestSendWith500ServerError(t *testing.T) {
	t.Parallel()
	synctest.Test(t, func(t *testing.T) {
		mockClient := &MockHTTPClient{}
		service := createTestService(
			t,
			validNtfyURL,
			mockClient,
		)

		mockClient.On("Do", mock.Anything).
			Return(createMockResponse(http.StatusInternalServerError, `{"code":500,"error":"internal server error"}`), nil).
			Once()

		err := service.Send("Test message", nil)
		require.Error(t, err)
		require.Contains(t, err.Error(), "internal server error")

		mockClient.AssertExpectations(t)
	})
}

func TestSendWithMalformedAPIErrorResponse(t *testing.T) {
	t.Parallel()
	synctest.Test(t, func(t *testing.T) {
		mockClient := &MockHTTPClient{}
		service := createTestService(
			t,
			validNtfyURL,
			mockClient,
		)

		mockClient.On("Do", mock.Anything).
			Return(createMockResponse(http.StatusBadRequest, `not json`), nil).
			Once()

		err := service.Send("Test message", nil)
		require.Error(t, err)
		require.Contains(t, err.Error(), "posting to ntfy API")

		mockClient.AssertExpectations(t)
	})
}

func TestSendWithEmptyErrorBody(t *testing.T) {
	t.Parallel()
	synctest.Test(t, func(t *testing.T) {
		mockClient := &MockHTTPClient{}
		service := createTestService(
			t,
			validNtfyURL,
			mockClient,
		)

		mockClient.On("Do", mock.Anything).
			Return(createMockResponse(http.StatusInternalServerError, ``), nil).
			Once()

		err := service.Send("Test message", nil)
		require.Error(t, err)
		require.Contains(t, err.Error(), "posting to ntfy API")

		mockClient.AssertExpectations(t)
	})
}

func TestSendWithTimeout(t *testing.T) {
	t.Parallel()
	synctest.Test(t, func(t *testing.T) {
		mockClient := &MockHTTPClient{}
		service := createTestService(
			t,
			validNtfyURL,
			mockClient,
		)

		mockClient.On("Do", mock.Anything).
			Return(nil, errors.New("deadline exceeded")).
			Once()

		err := service.Send("Test message", nil)
		require.Error(t, err)
		require.Contains(t, err.Error(), "posting to ntfy API")

		mockClient.AssertExpectations(t)
	})
}

func TestSendWithNilParams(t *testing.T) {
	t.Parallel()
	synctest.Test(t, func(t *testing.T) {
		mockClient := &MockHTTPClient{}
		service := createTestService(
			t,
			validNtfyURL,
			mockClient,
		)

		mockClient.On("Do", mock.Anything).
			Return(createMockResponse(http.StatusOK, `{"code":200,"error":"OK"}`), nil).
			Once()

		err := service.Send("Message with nil params", nil)
		require.NoError(t, err)

		mockClient.AssertExpectations(t)
	})
}

func TestSendWithEmptyParams(t *testing.T) {
	t.Parallel()
	synctest.Test(t, func(t *testing.T) {
		mockClient := &MockHTTPClient{}
		service := createTestService(
			t,
			validNtfyURL,
			mockClient,
		)

		mockClient.On("Do", mock.Anything).
			Return(createMockResponse(http.StatusOK, `{"code":200,"error":"OK"}`), nil).
			Once()

		params := createTestParams()
		err := service.Send("Message with empty params", params)
		require.NoError(t, err)

		mockClient.AssertExpectations(t)
	})
}

func TestSendWithStructuredAPIError(t *testing.T) {
	t.Parallel()
	synctest.Test(t, func(t *testing.T) {
		mockClient := &MockHTTPClient{}
		service := createTestService(
			t,
			validNtfyURL,
			mockClient,
		)

		mockClient.On("Do", mock.Anything).
			Return(createMockResponse(http.StatusBadRequest, `{"code":400,"error":"invalid request","link":"https://docs.ntfy.sh"}`), nil).
			Once()

		err := service.Send("Test message", nil)
		require.Error(t, err)
		require.Contains(t, err.Error(), "server response")
		require.Contains(t, err.Error(), "invalid request")
		require.Contains(t, err.Error(), "400")
		require.Contains(t, err.Error(), "https://docs.ntfy.sh")

		mockClient.AssertExpectations(t)
	})
}
