package discord_test

import (
	"errors"
	"net"
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

type timeoutError struct{}

var _ net.Error = timeoutError{}

func (noOpSleeper) Sleep(_ time.Duration) {}

func (timeoutError) Error() string { return "timeout" }

func (timeoutError) Timeout() bool { return true }

func (timeoutError) Temporary() bool { return false }

// computeExpectedBatches computes the number of batches for a message based on Discord limits.
func computeExpectedBatches(message string, splitLines bool) int {
	const (
		chunkSize      = 2000
		totalChunkSize = 6000
	)

	if splitLines {
		// For split lines, each line is a separate item, but simplified for test
		return 1
	}

	messageLen := len(message)
	if messageLen == 0 {
		return 0
	}

	chunks := (messageLen + chunkSize - 1) / chunkSize                                    // Ceiling division
	batches := (chunks + (totalChunkSize / chunkSize) - 1) / (totalChunkSize / chunkSize) // Ceiling division

	return batches
}

//nolint:funlen
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
			t.Logf("Running test case: %s", tt.name)

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

		// Generic errors are not retried; only transient errors (net.Error with Timeout()/Temporary()) trigger retry logic.
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

		// Should still succeed since non-error status codes (2xx) are accepted
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

		// Use noOpSleeper to avoid real sleeps in tests
		service.Sleeper = &noOpSleeper{}

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

func TestSendWithNonExistentWebhook(t *testing.T) {
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

func TestSendWithUnknownWebhook(t *testing.T) {
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

		// Create an item with invalid UTF-8 that gets sent as invalid JSON
		items := []types.MessageItem{
			{
				Text: string([]byte{0xff, 0xfe, 0xfd}), // Invalid UTF-8 bytes
			},
		}

		mockClient.On("Do", mock.Anything).
			Return(createMockResponse(http.StatusBadRequest, `{"message": "Invalid JSON"}`), nil).
			Once()

		err := service.SendItems(items, nil)

		// Should fail due to invalid JSON payload
		require.Error(t, err)
		assert.Contains(t, err.Error(), "unexpected response status code")

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

		// Use noOpSleeper to avoid real sleeps in tests
		service.Sleeper = &noOpSleeper{}

		// Simulate a network timeout by returning a timeout error
		// Timeout errors are transient and will be retried up to maxTransportRetries (3) times
		mockClient.On("Do", mock.Anything).
			Return(nil, timeoutError{}).
			Times(4) // Initial attempt + 3 retries

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

		// Compute expected number of calls based on message splitting logic
		expectedCalls := computeExpectedBatches(string(largeMessage), false)

		mockClient.On("Do", mock.Anything).
			Return(createMockResponse(http.StatusNoContent, ""), nil).
			Times(expectedCalls)

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
				make([]byte, 1024),
			), // 1KB file
		}

		err := service.SendItems(items, nil)

		require.Error(t, err)
		assert.Contains(t, err.Error(), "unexpected response status code")

		mockClient.AssertExpectations(t)
	})
}
