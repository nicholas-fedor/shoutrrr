package twilio_test

import (
	"errors"
	"net/http"
	"testing"
	"testing/synctest"

	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

func TestSendWithHTTPError(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		tests := []struct {
			name       string
			statusCode int
			response   string
		}{
			{
				"bad request",
				http.StatusBadRequest,
				`{"code": 21211, "message": "The 'To' number is not a valid phone number.", "status": 400}`,
			},
			{
				"unauthorized",
				http.StatusUnauthorized,
				`{"code": 20003, "message": "Authenticate", "status": 401}`,
			},
			{
				"forbidden",
				http.StatusForbidden,
				`{"code": 20003, "message": "Forbidden", "status": 403}`,
			},
			{
				"not found",
				http.StatusNotFound,
				`{"code": 20404, "message": "Not Found", "status": 404}`,
			},
			{
				"internal server error",
				http.StatusInternalServerError,
				"",
			},
		}

		for _, tt := range tests {
			mockClient := &MockHTTPClient{}
			service := createTestService(t, validTwilioURL, mockClient)

			mockClient.On("Do", mock.Anything).
				Return(createMockResponse(tt.statusCode, tt.response), nil).
				Once()

			err := service.Send("Test message", nil)
			require.Error(t, err, "Expected error for %s", tt.name)

			mockClient.AssertExpectations(t)
		}
	})
}

func TestSendWithNetworkError(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		mockClient := &MockHTTPClient{}
		service := createTestService(t, validTwilioURL, mockClient)

		mockClient.On("Do", mock.Anything).
			Return(nil, errors.New("connection refused")).
			Once()

		err := service.Send("Test message", nil)
		require.Error(t, err)
		require.Contains(t, err.Error(), "connection refused")

		mockClient.AssertExpectations(t)
	})
}

func TestSendWithAPIErrorParsing(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		mockClient := &MockHTTPClient{}
		service := createTestService(t, validTwilioURL, mockClient)

		mockClient.On("Do", mock.Anything).
			Return(createMockResponse(
				http.StatusBadRequest,
				`{"code": 21211, "message": "The 'To' number is not a valid phone number.", "status": 400}`,
			), nil).
			Once()

		err := service.Send("Test message", nil)
		require.Error(t, err)
		require.Contains(t, err.Error(), "The 'To' number is not a valid phone number.")
		require.Contains(t, err.Error(), "21211")

		mockClient.AssertExpectations(t)
	})
}

func TestSendWithMalformedAPIErrorResponse(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		mockClient := &MockHTTPClient{}
		service := createTestService(t, validTwilioURL, mockClient)

		mockClient.On("Do", mock.Anything).
			Return(createMockResponse(http.StatusBadRequest, "not valid json"), nil).
			Once()

		err := service.Send("Test message", nil)
		require.Error(t, err)

		mockClient.AssertExpectations(t)
	})
}

func TestSendPartialFailure(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		mockClient := &MockHTTPClient{}
		service := createTestService(
			t,
			"twilio://ACXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXX:authToken@+15551234567/+15559876543/+15551111111",
			mockClient,
		)

		// First recipient succeeds, second fails
		mockClient.On("Do", mock.Anything).
			Return(createMockResponse(http.StatusCreated, `{"sid": "SM123"}`), nil).
			Once()
		mockClient.On("Do", mock.Anything).
			Return(nil, errors.New("network error")).
			Once()

		err := service.Send("Test message", nil)
		require.Error(t, err)

		mockClient.AssertNumberOfCalls(t, "Do", 2)
		mockClient.AssertExpectations(t)
	})
}

func TestSendWithEmptyErrorResponseBody(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		mockClient := &MockHTTPClient{}
		service := createTestService(t, validTwilioURL, mockClient)

		mockClient.On("Do", mock.Anything).
			Return(createMockResponse(http.StatusBadRequest, ""), nil).
			Once()

		err := service.Send("Test message", nil)
		require.Error(t, err)

		mockClient.AssertExpectations(t)
	})
}
