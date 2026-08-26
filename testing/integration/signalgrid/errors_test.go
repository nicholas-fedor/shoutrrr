package signalgrid_test

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
		tests := []struct {
			name       string
			statusCode int
			response   string
		}{
			{"bad request", http.StatusBadRequest, "invalid"},
			{"unauthorized", http.StatusUnauthorized, "denied"},
			{"forbidden", http.StatusForbidden, "forbidden"},
			{"not found", http.StatusNotFound, "missing"},
			{"internal server error", http.StatusInternalServerError, "boom"},
		}

		for _, tt := range tests {
			mockClient := &MockHTTPClient{}
			service := createTestService(t, validSignalgridURL, mockClient)

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
	t.Parallel()
	synctest.Test(t, func(t *testing.T) {
		mockClient := &MockHTTPClient{}
		service := createTestService(t, validSignalgridURL, mockClient)

		mockClient.On("Do", mock.Anything).
			Return(nil, errors.New("connection refused")).
			Once()

		err := service.Send("Test message", nil)
		require.Error(t, err)

		mockClient.AssertExpectations(t)
	})
}
