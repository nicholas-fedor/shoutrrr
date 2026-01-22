package discord_test

import (
	"net/http"
	"testing"
	"testing/synctest"

	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	"github.com/nicholas-fedor/shoutrrr/pkg/types"
)

func TestSendMessageToThread(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		mockClient := &MockHTTPClient{}
		mockClient.On("Do", mock.Anything).
			Return(createMockResponse(http.StatusNoContent, ""), nil).
			Once()

		// Create service with thread ID in URL
		service := createTestService(
			t,
			"discord://test-token@test-webhook?thread_id=123456789",
			mockClient,
		)

		err := service.Send("Message in thread", nil)

		require.NoError(t, err)
		assertRequestMade(
			t,
			mockClient,
			"https://discord.com/api/webhooks/test-webhook/test-token?thread_id=123456789",
		)

		mockClient.AssertExpectations(t)
	})
}

func TestSendItemsToThread(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		mockClient := &MockHTTPClient{}
		mockClient.On("Do", mock.Anything).
			Return(createMockResponse(http.StatusNoContent, ""), nil).
			Once()

		// Create service with thread ID in URL
		service := createTestService(
			t,
			"discord://test-token@test-webhook?thread_id=987654321",
			mockClient,
		)

		items := []types.MessageItem{
			{Text: "Thread message with embed"},
		}

		err := service.SendItems(items, nil)

		require.NoError(t, err)
		assertRequestMade(
			t,
			mockClient,
			"https://discord.com/api/webhooks/test-webhook/test-token?thread_id=987654321",
		)

		mockClient.AssertExpectations(t)
	})
}

func TestSendMessageToThreadWithParams(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		mockClient := &MockHTTPClient{}
		mockClient.On("Do", mock.Anything).
			Return(createMockResponse(http.StatusNoContent, ""), nil).
			Once()

		// Create service with thread ID in URL
		service := createTestService(
			t,
			"discord://test-token@test-webhook?thread_id=111222333",
			mockClient,
		)

		params := createTestParams("thread_id", "444555666") // Override thread_id via params

		err := service.Send("Message with param thread override", params)

		require.NoError(t, err)
		assertRequestMade(
			t,
			mockClient,
			"https://discord.com/api/webhooks/test-webhook/test-token?thread_id=444555666",
		)

		mockClient.AssertExpectations(t)
	})
}

func TestSendFileToThread(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		mockClient := &MockHTTPClient{}
		mockClient.On("Do", mock.Anything).
			Return(createMockResponse(http.StatusNoContent, ""), nil).
			Once()

		// Create service with thread ID in URL
		service := createTestService(
			t,
			"discord://test-token@test-webhook?thread_id=thread123",
			mockClient,
		)

		items := []types.MessageItem{
			createTestMessageItemWithFile(
				"File in thread",
				"thread-file.txt",
				[]byte("thread content"),
			),
		}

		err := service.SendItems(items, nil)

		require.NoError(t, err)
		assertRequestMade(
			t,
			mockClient,
			"https://discord.com/api/webhooks/test-webhook/test-token?thread_id=thread123",
		)

		mockClient.AssertExpectations(t)
	})
}

func TestThreadIDWithSpecialCharacters(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		mockClient := &MockHTTPClient{}
		mockClient.On("Do", mock.Anything).
			Return(createMockResponse(http.StatusNoContent, ""), nil).
			Once()

		// Test thread ID with various characters
		service := createTestService(
			t,
			"discord://test-token@test-webhook?thread_id=thread-123_456.789",
			mockClient,
		)

		err := service.Send("Message with special thread ID", nil)

		require.NoError(t, err)
		assertRequestMade(
			t,
			mockClient,
			"https://discord.com/api/webhooks/test-webhook/test-token?thread_id=thread-123_456.789",
		)

		mockClient.AssertExpectations(t)
	})
}

func TestThreadIDValidation(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		mockClient := &MockHTTPClient{}
		// Test that invalid thread IDs are handled properly
		service := createTestService(
			t,
			"discord://test-token@test-webhook?thread_id=invalid-thread-id",
			mockClient,
		)

		mockClient.On("Do", mock.Anything).
			Return(createMockResponse(http.StatusNoContent, ""), nil).
			Once()

		err := service.Send("Message with invalid thread ID", nil)

		require.NoError(t, err)
		// Should still work as Discord accepts various thread ID formats

		mockClient.AssertExpectations(t)
	})
}

func TestThreadMessageWithEmbed(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		mockClient := &MockHTTPClient{}
		mockClient.On("Do", mock.Anything).
			Return(createMockResponse(http.StatusNoContent, ""), nil).
			Once()

		service := createTestService(
			t,
			"discord://test-token@test-webhook?thread_id=embed-thread",
			mockClient,
		)

		items := []types.MessageItem{
			{
				Text: "Thread message with rich embed",
				Fields: []types.Field{
					{Key: "Thread", Value: "Yes"},
					{Key: "Type", Value: "Test"},
				},
				Level: types.Info,
			},
		}

		err := service.SendItems(items, nil)

		require.NoError(t, err)
		assertRequestMade(
			t,
			mockClient,
			"https://discord.com/api/webhooks/test-webhook/test-token?thread_id=embed-thread",
		)
		assertRequestContains(
			t,
			mockClient,
			`"description":"Thread message with rich embed"`,
		)
		assertRequestContains(
			t,
			mockClient,
			`"name":"Thread","value":"Yes"`,
		)

		mockClient.AssertExpectations(t)
	})
}

func TestThreadMessageWithMultipleFiles(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		mockClient := &MockHTTPClient{}
		mockClient.On("Do", mock.Anything).
			Return(createMockResponse(http.StatusNoContent, ""), nil).
			Once()

		service := createTestService(
			t,
			"discord://test-token@test-webhook?thread_id=file-thread",
			mockClient,
		)

		items := []types.MessageItem{
			createTestMessageItemWithFile("First file in thread", "file1.txt", []byte("content 1")),
			createTestMessageItemWithFile(
				"Second file in thread",
				"file2.txt",
				[]byte("content 2"),
			),
		}

		err := service.SendItems(items, nil)

		require.NoError(t, err)
		assertRequestMade(
			t,
			mockClient,
			"https://discord.com/api/webhooks/test-webhook/test-token?thread_id=file-thread",
		)
		assertRequestContains(t, mockClient, `filename="file1.txt"`)
		assertRequestContains(t, mockClient, `filename="file2.txt"`)

		mockClient.AssertExpectations(t)
	})
}

func TestThreadMessageWithUsernameAvatar(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		mockClient := &MockHTTPClient{}
		mockClient.On("Do", mock.Anything).
			Return(createMockResponse(http.StatusNoContent, ""), nil).
			Once()

		service := createTestService(
			t,
			"discord://test-token@test-webhook?thread_id=custom-thread",
			mockClient,
		)

		params := createTestParams(
			"username",
			"ThreadBot",
			"avatar",
			"https://example.com/thread-avatar.png",
		)

		err := service.Send("Thread message with custom appearance", params)

		require.NoError(t, err)
		assertRequestMade(
			t,
			mockClient,
			"https://discord.com/api/webhooks/test-webhook/test-token?thread_id=custom-thread",
		)
		assertRequestContains(t, mockClient, `"username":"ThreadBot"`)
		assertRequestContains(
			t,
			mockClient,
			`"avatar_url":"https://example.com/thread-avatar.png"`,
		)

		mockClient.AssertExpectations(t)
	})
}
