package discord_test

import (
	"net/http"
	"testing"
	"testing/synctest"
	"time"

	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	"github.com/nicholas-fedor/shoutrrr/pkg/types"
)

func TestSendEmbedWithAuthor(t *testing.T) {
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
			{
				Text: "Test embed with author",
				Fields: []types.Field{
					{Key: "embed_author_name", Value: "Test Author"},
					{Key: "embed_author_url", Value: "https://example.com/author"},
					{Key: "embed_author_icon_url", Value: "https://example.com/icon.png"},
				},
			},
		}

		err := service.SendItems(items, nil)

		require.NoError(t, err)
		assertRequestContains(
			t,
			mockClient,
			`"author":{"name":"Test Author","url":"https://example.com/author","icon_url":"https://example.com/icon.png"}`,
		)

		mockClient.AssertExpectations(t)
	})
}

func TestSendEmbedWithImage(t *testing.T) {
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
			{
				Text: "Test embed with image",
				Fields: []types.Field{
					{Key: "embed_image_url", Value: "https://example.com/image.png"},
				},
			},
		}

		err := service.SendItems(items, nil)

		require.NoError(t, err)
		assertRequestContains(
			t,
			mockClient,
			`"image":{"url":"https://example.com/image.png"}`,
		)

		mockClient.AssertExpectations(t)
	})
}

func TestSendEmbedWithThumbnail(t *testing.T) {
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
			{
				Text: "Test embed with thumbnail",
				Fields: []types.Field{
					{Key: "embed_thumbnail_url", Value: "https://example.com/thumb.png"},
				},
			},
		}

		err := service.SendItems(items, nil)

		require.NoError(t, err)
		assertRequestContains(
			t,
			mockClient,
			`"thumbnail":{"url":"https://example.com/thumb.png"}`,
		)

		mockClient.AssertExpectations(t)
	})
}

func TestSendEmbedWithFields(t *testing.T) {
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
			{
				Text: "Test embed with fields",
				Fields: []types.Field{
					{Key: "Status", Value: "Active"},
					{Key: "Priority", Value: "High"},
					{Key: "Assignee", Value: "John Doe"},
				},
				Level: types.Info,
			},
		}

		err := service.SendItems(items, nil)

		require.NoError(t, err)
		assertRequestContains(
			t,
			mockClient,
			`"fields":[{"name":"Status","value":"Active"},{"name":"Priority","value":"High"},{"name":"Assignee","value":"John Doe"}]`,
		)

		mockClient.AssertExpectations(t)
	})
}

func TestSendEmbedWithTimestamp(t *testing.T) {
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

		testTime := time.Date(2023, 12, 25, 12, 0, 0, 0, time.UTC)
		items := []types.MessageItem{
			{
				Text:      "Test embed with timestamp",
				Timestamp: testTime,
			},
		}

		err := service.SendItems(items, nil)

		require.NoError(t, err)
		assertRequestContains(t, mockClient, `"timestamp":"2023-12-25T12:00:00Z"`)

		mockClient.AssertExpectations(t)
	})
}

func TestSendEmbedWithColors(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		mockClient := &MockHTTPClient{}
		service := createTestService(
			t,
			"discord://test-token@test-webhook",
			mockClient,
		)

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
				{Text: "Test embed with color", Level: tt.level},
			}

			err := service.SendItems(items, nil)

			require.NoError(t, err)
			// Color handling depends on configuration
		}

		mockClient.AssertExpectations(t)
	})
}

func TestSendMultipleEmbeds(t *testing.T) {
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
			{Text: "First embed"},
			{Text: "Second embed"},
			{Text: "Third embed"},
		}

		err := service.SendItems(items, nil)

		require.NoError(t, err)
		// Verify multiple embeds are created
		assertRequestContains(t, mockClient, `"description":"First embed"`)
		assertRequestContains(t, mockClient, `"description":"Second embed"`)
		assertRequestContains(t, mockClient, `"description":"Third embed"`)

		mockClient.AssertExpectations(t)
	})
}
