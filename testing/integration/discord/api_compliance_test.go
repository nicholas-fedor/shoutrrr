package discord_test

import (
	"net/http"
	"net/url"
	"testing"
	"testing/synctest"

	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	"github.com/nicholas-fedor/shoutrrr/pkg/services/chat/discord"
	"github.com/nicholas-fedor/shoutrrr/pkg/types"
)

const contentTypeJSON = "application/json"

func TestWebhookURLFormatCompliance(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		mockClient := &MockHTTPClient{}

		tests := []struct {
			name        string
			url         string
			shouldError bool
		}{
			{"valid webhook URL", "discord://token@123", false},
			{"webhook with numbers", "discord://123@456", false},
			{"webhook with special chars", "discord://token_123@webhook-456", false},
			{"empty token", "discord://@webhook", true},
			{"empty webhook", "discord://token@", true},
			{"missing scheme", "token@webhook", true},
			{"wrong scheme", "http://token@webhook", false},
			{"invalid host", "discord://token@notdiscord.com/webhook", true},
			{"invalid path", "discord://token@123456789/invalid", true},
		}

		for _, tt := range tests {
			parsedURL, parseErr := url.Parse(tt.url)
			if parseErr != nil {
				// URL parsing failed
				if tt.shouldError {
					// Expected to fail
					continue
				}
				// Unexpected parse failure
				t.Fatalf("Unexpected URL parse error for %s: %v", tt.name, parseErr)
			}

			// URL parsed successfully, try to create service
			service := &discord.Service{}

			initErr := service.Initialize(parsedURL, &mockLogger{})
			if initErr != nil {
				// Initialize failed
				if tt.shouldError {
					// Expected to fail
					continue
				}
				// Unexpected init failure
				t.Fatalf("Unexpected Initialize error for %s: %v", tt.name, initErr)
			}

			// Initialize succeeded, set up mock and test Send
			mockClient.On("Do", mock.Anything).
				Return(createMockResponse(http.StatusNoContent, ""), nil).
				Once()

			// Override HTTPClient after Initialize
			service.HTTPClient = mockClient

			err := service.Send("test", nil)
			require.NoError(t, err, "Expected no error for valid test case: %s", tt.name)
		}

		mockClient.AssertExpectations(t)
	})
}

func TestPayloadStructureCompliance(t *testing.T) {
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

		err := service.Send("Test message", nil)

		require.NoError(t, err)
		assertRequestContains(t, mockClient, `"content":"Test message"`)

		mockClient.AssertExpectations(t)
	})
}

func TestEmbedStructureCompliance(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		mockClient := &MockHTTPClient{}
		mockClient.On("Do", mock.Anything).
			Return(createMockResponse(http.StatusNoContent, ""), nil).
			Once()

		service := createTestService(
			t,
			"discord://test-token@test-webhook",
			mockClient,
		)

		items := []types.MessageItem{
			{
				Text: "Test embed",
				Fields: []types.Field{
					{Key: "embed_author_name", Value: "Test Author"},
					{Key: "embed_image_url", Value: "https://example.com/image.png"},
					{Key: "embed_thumbnail_url", Value: "https://example.com/thumb.png"},
					{Key: "Field1", Value: "Value1"},
				},
				Level: types.Info,
			},
		}

		err := service.SendItems(items, nil)

		require.NoError(t, err)
		// Verify embed structure
		assertRequestContains(t, mockClient, `"embeds":[{`)
		assertRequestContains(t, mockClient, `"description":"Test embed"`)
		assertRequestContains(t, mockClient, `"author":{"name":"Test Author"}`)
		assertRequestContains(
			t,
			mockClient,
			`"fields":[{"name":"Field1","value":"Value1"}]`,
		)
		assertRequestContains(
			t,
			mockClient,
			`"image":{"url":"https://example.com/image.png"}`,
		)
		assertRequestContains(
			t,
			mockClient,
			`"thumbnail":{"url":"https://example.com/thumb.png"}`,
		)
		assertRequestContains(t, mockClient, `"footer":{"text":"Info"}`)

		mockClient.AssertExpectations(t)
	})
}

func TestFileUploadCompliance(t *testing.T) {
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

		items := []types.MessageItem{
			createTestMessageItemWithFile("File message", "test.txt", []byte("file content")),
		}

		err := service.SendItems(items, nil)

		require.NoError(t, err)
		// Verify multipart form data structure
		assertRequestContains(t, mockClient, "Content-Disposition: form-data")
		assertRequestContains(t, mockClient, `name="payload_json"`)
		assertRequestContains(t, mockClient, `name="files[0]"`)
		assertRequestContains(t, mockClient, `filename="test.txt"`)

		mockClient.AssertExpectations(t)
	})
}

func TestThreadParameterCompliance(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		mockClient := &MockHTTPClient{}
		mockClient.On("Do", mock.Anything).
			Return(createMockResponse(http.StatusNoContent, ""), nil).
			Once()

		service := createTestService(
			t,
			"discord://test-token@test-webhook?thread_id=123456",
			mockClient,
		)

		err := service.Send("Thread message", nil)

		require.NoError(t, err)
		assertRequestMade(
			t,
			mockClient,
			"https://discord.com/api/webhooks/test-webhook/test-token?thread_id=123456",
		)

		mockClient.AssertExpectations(t)
	})
}

func TestUsernameAvatarCompliance(t *testing.T) {
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

		params := createTestParams(
			"username",
			"TestBot",
			"avatar",
			"https://example.com/avatar.png",
		)
		err := service.Send("Message with username/avatar", params)

		require.NoError(t, err)
		assertRequestContains(t, mockClient, `"username":"TestBot"`)
		assertRequestContains(
			t,
			mockClient,
			`"avatar_url":"https://example.com/avatar.png"`,
		)

		mockClient.AssertExpectations(t)
	})
}

func TestJSONModeCompliance(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		mockClient := &MockHTTPClient{}
		mockClient.On("Do", mock.Anything).
			Return(createMockResponse(http.StatusNoContent, ""), nil).
			Once()

		service := createTestService(
			t,
			"discord://test-token@test-webhook?json=true",
			mockClient,
		)

		jsonPayload := `{"content": "Raw JSON message", "tts": true}`

		err := service.Send(jsonPayload, nil)

		require.NoError(t, err)
		assertRequestBody(t, mockClient, jsonPayload)

		mockClient.AssertExpectations(t)
	})
}

func TestRateLimitHeaderCompliance(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		mockClient := &MockHTTPClient{}
		service := createTestService(
			t,
			"discord://test-token@test-webhook",
			mockClient,
		)

		// Test that service respects rate limit responses
		mockClient.On("Do", mock.Anything).
			Return(createMockResponse(http.StatusTooManyRequests, `{"retry_after": 1}`), nil).
			Once()
		mockClient.On("Do", mock.Anything).
			Return(createMockResponse(http.StatusNoContent, ""), nil).
			Once()

		err := service.Send("Rate limited message", nil)

		require.NoError(t, err)
		mockClient.AssertNumberOfCalls(t, "Do", 2)

		mockClient.AssertExpectations(t)
	})
}

func TestContentTypeHeaderCompliance(t *testing.T) {
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

		err := service.Send("Test message", nil)

		require.NoError(t, err)
		// Verify Content-Type header is set correctly
		assertRequestMatches(t, mockClient, func(req *http.Request) bool {
			return req.Header.Get("Content-Type") == contentTypeJSON
		}, "Content-Type header")

		mockClient.AssertExpectations(t)
	})
}

func TestUserAgentHeaderCompliance(t *testing.T) {
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

		err := service.Send("Test message", nil)

		require.NoError(t, err)
		// Verify User-Agent header is set
		assertRequestMatches(t, mockClient, func(req *http.Request) bool {
			return req.Header.Get("User-Agent") != ""
		}, "User-Agent header")

		mockClient.AssertExpectations(t)
	})
}

func TestHTTPSRequirementCompliance(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		mockClient := &MockHTTPClient{}
		service := createTestService(
			t,
			"discord://test-token@test-webhook",
			mockClient,
		)

		// This should be tested by the service itself - all requests must use HTTPS
		mockClient.On("Do", mock.Anything).
			Return(createMockResponse(http.StatusNoContent, ""), nil).
			Once()

		err := service.Send("Test message", nil)

		require.NoError(t, err)
		assertRequestMatches(t, mockClient, func(req *http.Request) bool {
			return req.URL.Scheme == "https"
		}, "HTTPS scheme")

		mockClient.AssertExpectations(t)
	})
}
