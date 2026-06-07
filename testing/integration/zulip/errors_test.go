package zulip_test

import (
	"errors"
	"net/http"
	"testing"
	"testing/synctest"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	"github.com/nicholas-fedor/shoutrrr/pkg/services/chat/zulip"
	"github.com/nicholas-fedor/shoutrrr/pkg/services/chat/zulip/mocks"
)

func TestServiceSendWithHTTPError(t *testing.T) {
	t.Parallel()
	synctest.Test(t, func(t *testing.T) {
		mockClient := mocks.NewMockHTTPClient(t)
		service := createTestService(
			t,
			"zulip://bot@example.com:secret-key@zulip.example.com?stream=general",
			mockClient,
		)

		tests := []struct {
			name       string
			statusCode int
			body       string
		}{
			{
				name:       "bad request",
				statusCode: http.StatusBadRequest,
				body:       `{"result": "error", "msg": "Invalid request"}`,
			},
			{
				name:       "unauthorized",
				statusCode: http.StatusUnauthorized,
				body:       `{"result": "error", "msg": "Invalid API key"}`,
			},
			{
				name:       "forbidden",
				statusCode: http.StatusForbidden,
				body:       `{"result": "error", "msg": "Access denied"}`,
			},
			{
				name:       "not found",
				statusCode: http.StatusNotFound,
				body:       `{"result": "error", "msg": "Not found"}`,
			},
			{
				name:       "server error",
				statusCode: http.StatusInternalServerError,
				body:       `{"result": "error", "msg": "Server error"}`,
			},
			{
				name:       "service unavailable",
				statusCode: http.StatusServiceUnavailable,
				body:       `{"result": "error", "msg": "Service unavailable"}`,
			},
		}

		for _, tt := range tests {
			mockClient.On("Do", mock.AnythingOfType("*http.Request")).
				Return(createMockResponse(tt.statusCode, tt.body), nil).
				Once()

			err := service.Send("Test message", nil)

			require.Error(t, err, "Expected error for status %d", tt.statusCode)
			require.ErrorIs(t, err, zulip.ErrResponseStatusFailure)
		}

		mockClient.AssertExpectations(t)
	})
}

func TestServiceSendWithNetworkError(t *testing.T) {
	t.Parallel()
	synctest.Test(t, func(t *testing.T) {
		mockClient := mocks.NewMockHTTPClient(t)
		service := createTestService(
			t,
			"zulip://bot@example.com:secret-key@zulip.example.com?stream=general",
			mockClient,
		)

		mockClient.On("Do", mock.AnythingOfType("*http.Request")).
			Return(nil, errors.New("connection refused")).
			Once()

		err := service.Send("Test message", nil)

		require.Error(t, err)
		assert.Contains(t, err.Error(), "making HTTP POST request")

		mockClient.AssertExpectations(t)
	})
}

func TestServiceSendWithTimeoutError(t *testing.T) {
	t.Parallel()
	synctest.Test(t, func(t *testing.T) {
		mockClient := mocks.NewMockHTTPClient(t)
		service := createTestService(
			t,
			"zulip://bot@example.com:secret-key@zulip.example.com?stream=general",
			mockClient,
		)

		mockClient.On("Do", mock.AnythingOfType("*http.Request")).
			Return(nil, errors.New("context deadline exceeded")).
			Once()

		err := service.Send("Test message", nil)

		require.Error(t, err)

		mockClient.AssertExpectations(t)
	})
}

func TestServiceSendWithInvalidHost(t *testing.T) {
	t.Parallel()
	synctest.Test(t, func(t *testing.T) {
		mockClient := mocks.NewMockHTTPClient(t)
		service := createTestService(
			t,
			"zulip://bot@example.com:secret-key@zulip.example.com?stream=general",
			mockClient,
		)

		service.Config.Host = "invalid host with spaces!"

		err := service.Send("Test message", nil)

		require.Error(t, err)
		require.ErrorIs(t, err, zulip.ErrInvalidHost)

		mockClient.AssertNotCalled(t, "Do", mock.AnythingOfType("*http.Request"))
	})
}

func TestServiceSendWithEmptyStream(t *testing.T) {
	t.Parallel()
	synctest.Test(t, func(t *testing.T) {
		mockClient := mocks.NewMockHTTPClient(t)
		service := createTestService(
			t,
			"zulip://bot@example.com:secret-key@zulip.example.com",
			mockClient,
		)

		mockClient.On("Do", mock.AnythingOfType("*http.Request")).
			Return(createMockResponse(http.StatusOK, `{"result": "success"}`), nil).
			Once()

		err := service.Send("Test message", nil)

		require.NoError(t, err)
		assertRequestContains(t, mockClient, "to=")

		mockClient.AssertExpectations(t)
	})
}

func TestServiceSendWithMalformedResponseBody(t *testing.T) {
	t.Parallel()
	synctest.Test(t, func(t *testing.T) {
		mockClient := mocks.NewMockHTTPClient(t)
		service := createTestService(
			t,
			"zulip://bot@example.com:secret-key@zulip.example.com?stream=general",
			mockClient,
		)

		mockClient.On("Do", mock.AnythingOfType("*http.Request")).
			Return(createMockResponse(http.StatusOK, "not valid json"), nil).
			Once()

		err := service.Send("Test message", nil)

		require.NoError(t, err)

		mockClient.AssertExpectations(t)
	})
}
