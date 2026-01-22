package discord_test

import (
	"net/http"
	"testing"
	"testing/synctest"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"

	"github.com/nicholas-fedor/shoutrrr/pkg/types"
)

func TestServiceInitialization(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		mockClient := &MockHTTPClient{}
		service := createTestService(
			t,
			"discord://test-token@test-webhook",
			mockClient,
		)

		// Test that service initializes correctly
		assert.NotNil(t, service)
		assert.Equal(t, "discord", service.GetID())

		mockClient.AssertExpectations(t)
	})
}

func TestSendItemsWithPlainText(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		mockClient := &MockHTTPClient{}
		service := createTestService(
			t,
			"discord://test-token@test-webhook",
			mockClient,
		)

		// Test SendItems with plain text message
		mockClient.On("Do", mock.Anything).
			Return(createMockResponse(http.StatusNoContent, ""), nil).
			Once()

		items := []types.MessageItem{
			createTestMessageItem("Test plain text message"),
		}

		err := service.SendItems(items, nil)
		assert.NoError(t, err)

		mockClient.AssertExpectations(t)
	})
}

func TestSendItemsWithEmptyMessage(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		mockClient := &MockHTTPClient{}
		service := createTestService(
			t,
			"discord://test-token@test-webhook",
			mockClient,
		)

		// Test SendItems with empty message
		mockClient.On("Do", mock.Anything).
			Return(createMockResponse(http.StatusNoContent, ""), nil).
			Once()

		items := []types.MessageItem{
			createTestMessageItem(""),
		}

		err := service.SendItems(items, nil)
		assert.NoError(t, err)

		mockClient.AssertExpectations(t)
	})
}

func TestSendItemsWithMultipleItems(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		mockClient := &MockHTTPClient{}
		service := createTestService(
			t,
			"discord://test-token@test-webhook",
			mockClient,
		)

		// Test SendItems with multiple plain text items
		mockClient.On("Do", mock.Anything).
			Return(createMockResponse(http.StatusNoContent, ""), nil).
			Once()

		items := []types.MessageItem{
			createTestMessageItem("First message"),
			createTestMessageItem("Second message"),
			createTestMessageItem("Third message"),
		}

		err := service.SendItems(items, nil)
		assert.NoError(t, err)

		mockClient.AssertExpectations(t)
	})
}

func TestSendItemsWithParams(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		mockClient := &MockHTTPClient{}
		service := createTestService(
			t,
			"discord://test-token@test-webhook",
			mockClient,
		)

		// Test SendItems with parameters
		mockClient.On("Do", mock.Anything).
			Return(createMockResponse(http.StatusNoContent, ""), nil).
			Once()

		items := []types.MessageItem{
			createTestMessageItem("Message with params"),
		}

		params := createTestParams(
			"username",
			"TestBot",
			"avatar",
			"https://example.com/avatar.png",
		)
		err := service.SendItems(items, params)
		assert.NoError(t, err)

		mockClient.AssertExpectations(t)
	})
}

func TestSendItemsWithTimestamp(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		mockClient := &MockHTTPClient{}
		service := createTestService(
			t,
			"discord://test-token@test-webhook",
			mockClient,
		)

		// Test SendItems with timestamp
		mockClient.On("Do", mock.Anything).
			Return(createMockResponse(http.StatusNoContent, ""), nil).
			Once()

		items := []types.MessageItem{
			{
				Text:      "Message with timestamp",
				Timestamp: time.Now(),
			},
		}

		err := service.SendItems(items, nil)
		assert.NoError(t, err)

		mockClient.AssertExpectations(t)
	})
}

func TestSendItemsWithLevel(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		mockClient := &MockHTTPClient{}
		service := createTestService(
			t,
			"discord://test-token@test-webhook",
			mockClient,
		)

		// Test SendItems with different message levels
		tests := []struct {
			name  string
			level types.MessageLevel
		}{
			{"info level", types.Info},
			{"warning level", types.Warning},
			{"error level", types.Error},
		}

		for _, tt := range tests {
			mockClient.On("Do", mock.Anything).
				Return(createMockResponse(http.StatusNoContent, ""), nil).
				Once()

			items := []types.MessageItem{
				{
					Text:  "Message with level",
					Level: tt.level,
				},
			}

			err := service.SendItems(items, nil)
			assert.NoError(t, err)
		}

		mockClient.AssertExpectations(t)
	})
}
