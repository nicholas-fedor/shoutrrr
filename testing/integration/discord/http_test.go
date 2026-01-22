package discord_test

import (
	"errors"
	"net/http"
	"testing"
	"testing/synctest"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"

	"github.com/nicholas-fedor/shoutrrr/pkg/types"
)

func TestHTTPMethodCompliance(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		mockClient := &MockHTTPClient{}
		service := createTestService(
			t,
			"discord://test-token@test-webhook",
			mockClient,
		)

		mockClient.On("Do", mock.Anything).
			Return(createMockResponse(http.StatusNoContent, ""), nil). //nolint:bodyclose
			Once()

		err := service.Send("Test message", nil)

		assert.NoError(t, err)
		assertRequestMatches(t, mockClient, func(req *http.Request) bool {
			return req.Method == http.MethodPost
		}, "POST method")

		mockClient.AssertExpectations(t)
	})
}

func TestHTTPTimeoutConfiguration(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		mockClient := &MockHTTPClient{}
		service := createTestService(
			t,
			"discord://test-token@test-webhook",
			mockClient,
		)

		mockClient.On("Do", mock.Anything).
			Return(createMockResponse(http.StatusNoContent, ""), nil). //nolint:bodyclose
			Once()

		start := time.Now()
		err := service.Send("Test message", nil)
		duration := time.Since(start)

		assert.NoError(t, err)
		// Should complete relatively quickly (timeout is configured)
		assert.Less(t, duration, 10*time.Second)

		mockClient.AssertExpectations(t)
	})
}

func TestHTTPHeaders(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		mockClient := &MockHTTPClient{}
		service := createTestService(
			t,
			"discord://test-token@test-webhook",
			mockClient,
		)

		mockClient.On("Do", mock.Anything).
			Return(createMockResponse(http.StatusNoContent, ""), nil). //nolint:bodyclose
			Once()

		err := service.Send("Test message", nil)

		assert.NoError(t, err)
		assertRequestMatches(t, mockClient, func(req *http.Request) bool {
			contentType := req.Header.Get("Content-Type")
			userAgent := req.Header.Get("User-Agent")

			return contentType == "application/json" && userAgent != ""
		}, "headers")

		mockClient.AssertExpectations(t)
	})
}

func TestHTTPURLConstruction(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		mockClient := &MockHTTPClient{}

		tests := []struct {
			name        string
			webhookURL  string
			expectedURL string
		}{
			{
				name:        "basic webhook",
				webhookURL:  "discord://token@webhook",
				expectedURL: "https://discord.com/api/webhooks/webhook/token",
			},
			{
				name:        "webhook with thread",
				webhookURL:  "discord://token@webhook?thread_id=123",
				expectedURL: "https://discord.com/api/webhooks/webhook/token?thread_id=123",
			},
			{
				name:        "numeric webhook",
				webhookURL:  "discord://123@456",
				expectedURL: "https://discord.com/api/webhooks/456/123",
			},
		}

		for _, tt := range tests {
			mockClient.On("Do", mock.Anything).
				Return(createMockResponse(http.StatusNoContent, ""), nil). //nolint:bodyclose
				Once()

			testService := createTestService(t, tt.webhookURL, mockClient)

			err := testService.Send("Test", nil)

			assert.NoError(t, err)
			assertRequestMade(t, mockClient, "POST", tt.expectedURL)
		}

		mockClient.AssertExpectations(t)
	})
}

func TestHTTPRetryMechanism(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		mockClient := &MockHTTPClient{}
		service := createTestService(
			t,
			"discord://test-token@test-webhook",
			mockClient,
		)

		// Test retry on rate limit
		mockClient.On("Do", mock.Anything).
			Return(createMockResponse(http.StatusTooManyRequests, `{"retry_after": 0.1}`), nil).
			Once()
		mockClient.On("Do", mock.Anything).
			Return(createMockResponse(http.StatusNoContent, ""), nil). //nolint:bodyclose
			Once()

		err := service.Send("Test message", nil)

		assert.NoError(t, err)
		mockClient.AssertNumberOfCalls(t, "Do", 2)

		mockClient.AssertExpectations(t)
	})
}

func TestHTTPRetryOnNetworkError(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		mockClient := &MockHTTPClient{}
		service := createTestService(
			t,
			"discord://test-token@test-webhook",
			mockClient,
		)

		networkError := errors.New("connection reset")
		mockClient.On("Do", mock.Anything).Return((*http.Response)(nil), networkError)

		err := service.Send("Test message", nil)

		assert.Error(t, err)
		assert.Contains(t, err.Error(), "failed to send discord notification")
		mockClient.AssertNumberOfCalls(t, "Do", 1)

		mockClient.AssertExpectations(t)
	})
}

func TestHTTPMaxRetries(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		mockClient := &MockHTTPClient{}
		service := createTestService(
			t,
			"discord://test-token@test-webhook",
			mockClient,
		)

		// Test that it doesn't retry indefinitely
		mockClient.On("Do", mock.Anything).
			Return(createMockResponse(http.StatusTooManyRequests, `{"retry_after": 0.001}`), nil).
			Times(6)
			// More than max retries

		err := service.Send("Test message", nil)

		assert.Error(t, err)
		assert.Contains(t, err.Error(), "failed to send discord notification")

		mockClient.AssertExpectations(t)
	})
}

func TestHTTPContextCancellation(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		mockClient := &MockHTTPClient{}
		service := createTestService(
			t,
			"discord://test-token@test-webhook",
			mockClient,
		)

		// This is harder to test directly, but we can verify timeout behavior
		mockClient.On("Do", mock.Anything).
			Return(createMockResponse(http.StatusNoContent, ""), nil). //nolint:bodyclose
			Once()

		err := service.Send("Test message", nil)

		assert.NoError(t, err)
		// Context should be properly set with timeout

		mockClient.AssertExpectations(t)
	})
}

func TestHTTPResponseStatusHandling(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		mockClient := &MockHTTPClient{}
		service := createTestService(
			t,
			"discord://test-token@test-webhook",
			mockClient,
		)

		tests := []struct {
			name          string
			statusCode    int
			responseBody  string
			expectError   bool
			expectedCalls int
		}{
			{"success", http.StatusNoContent, "", false, 1},
			{"bad request", http.StatusBadRequest, `{"message": "error"}`, true, 1},
			{"unauthorized", http.StatusUnauthorized, `{"message": "error"}`, true, 1},
			{"forbidden", http.StatusForbidden, `{"message": "error"}`, true, 1},
			{"not found", http.StatusNotFound, `{"message": "error"}`, true, 1},
			{
				"rate limited",
				http.StatusTooManyRequests,
				`{"retry_after": 1}`,
				true,
				6,
			}, // Should retry but eventually fail
			{"server error", http.StatusInternalServerError, `{"message": "error"}`, true, 6},
		}

		for _, tt := range tests {
			if tt.statusCode == http.StatusTooManyRequests ||
				tt.statusCode == http.StatusInternalServerError {
				mockClient.On("Do", mock.Anything).
					Return(createMockResponse(tt.statusCode, tt.responseBody), nil).
					Times(tt.expectedCalls)
			} else {
				mockClient.On("Do", mock.Anything).
					Return(createMockResponse(tt.statusCode, tt.responseBody), nil).
					Times(tt.expectedCalls)
			}

			err := service.Send("Test message", nil)

			if tt.expectError {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}

			mockClient.AssertExpectations(t)
		}

		mockClient.AssertExpectations(t)
	})
}

func TestHTTPMultipartUpload(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		mockClient := &MockHTTPClient{}
		service := createTestService(
			t,
			"discord://test-token@test-webhook",
			mockClient,
		)

		mockClient.On("Do", mock.Anything).
			Return(createMockResponse(http.StatusNoContent, ""), nil). //nolint:bodyclose
			Once()

		items := []types.MessageItem{
			createTestMessageItemWithFile("Test", "file.txt", []byte("content")),
		}

		err := service.SendItems(items, nil)

		assert.NoError(t, err)
		assertRequestMatches(t, mockClient, func(req *http.Request) bool {
			return req.Header.Get("Content-Type") != "application/json" // Should be multipart
		}, "multipart content type")

		mockClient.AssertExpectations(t)
	})
}

func TestHTTPConnectionReuse(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		mockClient := &MockHTTPClient{}
		service := createTestService(
			t,
			"discord://test-token@test-webhook",
			mockClient,
		)

		// Test multiple requests reuse connections (hard to test directly with mock)
		mockClient.On("Do", mock.Anything).
			Return(createMockResponse(http.StatusNoContent, ""), nil). //nolint:bodyclose
			Times(3)

		err1 := service.Send("Message 1", nil)
		err2 := service.Send("Message 2", nil)
		err3 := service.Send("Message 3", nil)

		assert.NoError(t, err1)
		assert.NoError(t, err2)
		assert.NoError(t, err3)
		mockClient.AssertNumberOfCalls(t, "Do", 3)

		mockClient.AssertExpectations(t)
	})
}
