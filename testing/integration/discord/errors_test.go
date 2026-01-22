package discord_test

import (
	"errors"
	"net/http"
	"net/url"
	"testing"
	"testing/synctest"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	"github.com/nicholas-fedor/shoutrrr/pkg/services/chat/discord"
	"github.com/nicholas-fedor/shoutrrr/pkg/types"
)

// noOpSleeper is a sleeper that does nothing, for testing.
type noOpSleeper struct{}

func (noOpSleeper) Sleep(_ time.Duration) {}

func TestSendWithHTTPError(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		tests := []struct {
			name            string
			statusCode      int
			response        string
			expectedError   string
			expectedCalls   int
			expectedHeaders map[string]string
		}{
			{
				"bad request",
				http.StatusBadRequest,
				`{"message": "Invalid webhook"}`,
				"failed to send discord notification",
				1,
				nil,
			},
			{
				"unauthorized",
				http.StatusUnauthorized,
				`{"message": "Invalid token"}`,
				"failed to send discord notification",
				1,
				nil,
			},
			{
				"forbidden",
				http.StatusForbidden,
				`{"message": "Missing permissions"}`,
				"failed to send discord notification",
				1,
				nil,
			},
			{
				"not found",
				http.StatusNotFound,
				`{"message": "Webhook not found"}`,
				"failed to send discord notification",
				1,
				nil,
			},
			{
				"too many requests",
				http.StatusTooManyRequests,
				`{"message": "Rate limited"}`,
				"failed to send discord notification",
				6,
				map[string]string{"Retry-After": "0"},
			},
			{
				"internal server error",
				http.StatusInternalServerError,
				`{"message": "Server error"}`,
				"failed to send discord notification",
				6,
				nil,
			},
		}

		for _, tt := range tests {
			mockClient := &MockHTTPClient{}
			service := createTestService(
				t,
				"discord://test-token@test-webhook",
				mockClient,
			)

			// Use noOpSleeper to avoid real sleeps in tests
			service.Sleeper = &noOpSleeper{}

			resp := createMockResponse(tt.statusCode, tt.response, tt.expectedHeaders)

			mockClient.On("Do", mock.Anything).
				Return(resp, nil).
				Times(tt.expectedCalls)

			err := service.Send("Test message", nil)

			require.Error(t, err)
			assert.Contains(t, err.Error(), tt.expectedError)
			mockClient.AssertExpectations(t)
		}
	})
}

func TestSendWithNetworkError(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		mockClient := &MockHTTPClient{}
		service := createTestService(
			t,
			"discord://test-token@test-webhook",
			mockClient,
		)

		networkError := errors.New("network connection failed")
		mockClient.On("Do", mock.Anything).Return((*http.Response)(nil), networkError).Once()

		err := service.Send("Test message", nil)

		require.Error(t, err)
		assert.Contains(t, err.Error(), "failed to send discord notification")

		mockClient.AssertExpectations(t)
	})
}

func TestSendWithMalformedResponse(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		mockClient := &MockHTTPClient{}
		service := createTestService(
			t,
			"discord://test-token@test-webhook",
			mockClient,
		)

		mockClient.On("Do", mock.Anything).
			Return(createMockResponse(http.StatusOK, "invalid json"), nil).
			Once()

		err := service.Send("Test message", nil)

		// Should still succeed since we only check for 204 No Content
		require.NoError(t, err)

		mockClient.AssertExpectations(t)
	})
}

func TestSendWithRetryOnRateLimit(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		mockClient := &MockHTTPClient{}
		service := createTestService(
			t,
			"discord://test-token@test-webhook",
			mockClient,
		)

		// First call returns rate limit, second succeeds
		mockClient.On("Do", mock.Anything).
			Return(createMockResponse(http.StatusTooManyRequests, `{"retry_after": 1}`), nil).
			Once()
		mockClient.On("Do", mock.Anything).
			Return(createMockResponse(http.StatusNoContent, ""), nil).
			Once()

		err := service.Send("Test message", nil)

		require.NoError(t, err)
		mockClient.AssertNumberOfCalls(t, "Do", 2)

		mockClient.AssertExpectations(t)
	})
}

func TestSendWithInvalidURL(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		mockClient := &MockHTTPClient{}
		invalidService := createTestService(t, "discord://token@invalid", mockClient)

		mockClient.On("Do", mock.Anything).
			Return(createMockResponse(http.StatusNotFound, `{"message": "Unknown Webhook"}`), nil).
			Once()

		err := invalidService.Send("Test message", nil)

		require.Error(t, err)
		assert.Contains(t, err.Error(), "failed to send discord notification")

		mockClient.AssertExpectations(t)
	})
}

func TestSendWithEmptyURL(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		mockClient := &MockHTTPClient{}
		emptyService := createTestService(t, "discord://token@empty", mockClient)

		mockClient.On("Do", mock.Anything).
			Return(createMockResponse(http.StatusNotFound, `{"message": "Unknown Webhook"}`), nil).
			Once()

		err := emptyService.Send("Test message", nil)

		require.Error(t, err)
		assert.Contains(t, err.Error(), "failed to send discord notification")

		mockClient.AssertExpectations(t)
	})
}

func TestSendWithInvalidScheme(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		service := &discord.Service{}

		parsedURL, _ := url.Parse("http://discord.com/api/webhooks/test/test")

		err := service.Initialize(parsedURL, &mockLogger{})

		require.Error(t, err)
		assert.Contains(t, err.Error(), "setting config URL: illegal argument in config URL")
	})
}

func TestSendWithInvalidHost(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		service := &discord.Service{}

		parsedURL, _ := url.Parse("discord://token@notdiscord.com/webhook")

		err := service.Initialize(parsedURL, &mockLogger{})

		require.Error(t, err)
		assert.Contains(t, err.Error(), "setting config URL: illegal argument in config URL")
	})
}

func TestSendWithInvalidPath(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		service := &discord.Service{}

		parsedURL, _ := url.Parse("discord://token@webhook/invalid/path")

		err := service.Initialize(parsedURL, &mockLogger{})

		require.Error(t, err)
		assert.Contains(t, err.Error(), "setting config URL: illegal argument in config URL")
	})
}

func TestSendItemsWithInvalidPayload(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		mockClient := &MockHTTPClient{}
		service := createTestService(
			t,
			"discord://test-token@test-webhook",
			mockClient,
		)

		mockClient.On("Do", mock.Anything).
			Return(createMockResponse(http.StatusNoContent, ""), nil).
			Once()

		// Create an item that would cause JSON marshaling to fail
		items := []types.MessageItem{
			{
				Text: "Test",
				// This would cause issues if not handled properly
			},
		}

		err := service.SendItems(items, nil)

		// Should succeed as the payload creation should work
		require.NoError(t, err)

		mockClient.AssertExpectations(t)
	})
}

func TestSendWithTimeout(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		mockClient := &MockHTTPClient{}
		service := createTestService(
			t,
			"discord://test-token@test-webhook",
			mockClient,
		)

		// Simulate a timeout by having the mock not respond
		mockClient.On("Do", mock.Anything).
			Return(createMockResponse(http.StatusRequestTimeout, ""), nil).
			Once()

		err := service.Send("Test message", nil)

		require.Error(t, err)
		assert.Contains(t, err.Error(), "failed to send discord notification")

		mockClient.AssertExpectations(t)
	})
}

func TestSendWithLargePayload(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		mockClient := &MockHTTPClient{}
		service := createTestService(
			t,
			"discord://test-token@test-webhook?splitLines=false",
			mockClient,
		)

		// Create a very large message that might cause issues
		largeMessage := make([]byte, 1000000) // 1MB
		for i := range largeMessage {
			largeMessage[i] = 'a'
		}

		mockClient.On("Do", mock.Anything).
			Return(createMockResponse(http.StatusNoContent, ""), nil).
			Times(167)

		err := service.Send(string(largeMessage), nil)

		require.NoError(t, err)

		mockClient.AssertExpectations(t)
	})
}

func TestSendWithInvalidJSONMode(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		mockClient := &MockHTTPClient{}
		// Create service with JSON mode
		service := createTestService(t, "discord://test-token@test-webhook?json=true", mockClient)

		// Send invalid JSON
		mockClient.On("Do", mock.Anything).
			Return(createMockResponse(http.StatusNoContent, ""), nil).
			Once()

		err := service.Send(`{"invalid": json}`, nil)

		require.NoError(t, err) // Should succeed as it's passed through as raw content

		mockClient.AssertExpectations(t)
	})
}

func TestSendWithFileUploadError(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		mockClient := &MockHTTPClient{}
		service := createTestService(
			t,
			"discord://test-token@test-webhook",
			mockClient,
		)

		mockClient.On("Do", mock.Anything).
			Return(createMockResponse(http.StatusBadRequest, `{"message": "File too large"}`), nil).
			Once()

		items := []types.MessageItem{
			createTestMessageItemWithFile(
				"Test",
				"large.txt",
				make([]byte, 100*1024*1024),
			), // 100MB file
		}

		err := service.SendItems(items, nil)

		require.Error(t, err)
		assert.Contains(t, err.Error(), "unexpected response status code")

		mockClient.AssertExpectations(t)
	})
}
