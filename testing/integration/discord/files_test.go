package discord_test

import (
	"net/http"
	"testing"
	"testing/synctest"

	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	"github.com/nicholas-fedor/shoutrrr/pkg/types"
)

func TestSendSingleFileAttachment(t *testing.T) {
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

		fileData := []byte("test file content")
		items := []types.MessageItem{
			createTestMessageItemWithFile("Test message with file", "test.txt", fileData),
		}

		err := service.SendItems(items, nil)

		require.NoError(t, err)
		// Verify multipart request was made (contains boundary)
		assertRequestContains(t, mockClient, "Content-Disposition: form-data")
		assertRequestContains(t, mockClient, `name="files[0]"`)
		assertRequestContains(t, mockClient, `filename="test.txt"`)
		assertRequestContains(t, mockClient, "test file content")

		mockClient.AssertExpectations(t)
	})
}

func TestSendMultipleFileAttachments(t *testing.T) {
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
			createTestMessageItemWithFile("File 1", "file1.txt", []byte("content 1")),
			createTestMessageItemWithFile("File 2", "file2.txt", []byte("content 2")),
			createTestMessageItemWithFile("File 3", "file3.txt", []byte("content 3")),
		}

		err := service.SendItems(items, nil)

		require.NoError(t, err)
		assertRequestContains(t, mockClient, `filename="file1.txt"`)
		assertRequestContains(t, mockClient, `filename="file2.txt"`)
		assertRequestContains(t, mockClient, `filename="file3.txt"`)
		assertRequestContains(t, mockClient, "content 1")
		assertRequestContains(t, mockClient, "content 2")
		assertRequestContains(t, mockClient, "content 3")

		mockClient.AssertExpectations(t)
	})
}

func TestSendLargeFileAttachment(t *testing.T) {
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

		// Create a large file (5MB)
		largeData := make([]byte, 5*1024*1024)
		for i := range largeData {
			largeData[i] = byte(i % 256)
		}

		items := []types.MessageItem{
			createTestMessageItemWithFile("Large file test", "large.bin", largeData),
		}

		err := service.SendItems(items, nil)

		require.NoError(t, err)
		assertRequestContains(t, mockClient, `filename="large.bin"`)

		mockClient.AssertExpectations(t)
	})
}

func TestSendFileWithSpecialCharactersInName(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		mockClient := &MockHTTPClient{}
		service := createTestService(
			t,
			"discord://test-token@test-webhook",
			mockClient,
		)

		specialNames := []string{
			"file with spaces.txt",
			"file-with-dashes.txt",
			"file_with_underscores.txt",
			"file.with.dots.txt",
			"file(1).txt",
			"file[1].txt",
			"file+plus.txt",
			"file%percent.txt",
		}

		for _, filename := range specialNames {
			mockClient.On("Do", mock.Anything).
				Return(createMockResponse(http.StatusNoContent, ""), nil).
				Once()

			items := []types.MessageItem{
				createTestMessageItemWithFile("Test file", filename, []byte("content")),
			}

			err := service.SendItems(items, nil)

			require.NoError(t, err)
			assertRequestContains(t, mockClient, `filename="`+filename+`"`)
		}

		mockClient.AssertExpectations(t)
	})
}

func TestSendFileWithDifferentTypes(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		mockClient := &MockHTTPClient{}
		service := createTestService(
			t,
			"discord://test-token@test-webhook",
			mockClient,
		)

		testCases := []struct {
			name     string
			filename string
			content  []byte
			mimeType string
		}{
			{"text file", "test.txt", []byte("plain text"), "text/plain"},
			{"json file", "data.json", []byte(`{"key": "value"}`), "application/json"},
			{"xml file", "data.xml", []byte(`<root><item>value</item></root>`), "application/xml"},
			{"binary file", "data.bin", []byte{0x00, 0x01, 0x02, 0xFF}, "application/octet-stream"},
			{"image file", "image.png", []byte("fake png content"), "image/png"},
			{"pdf file", "document.pdf", []byte("fake pdf content"), "application/pdf"},
		}

		for _, tc := range testCases {
			mockClient.On("Do", mock.Anything).
				Return(createMockResponse(http.StatusNoContent, ""), nil).
				Once()

			items := []types.MessageItem{
				createTestMessageItemWithFile("Test file", tc.filename, tc.content),
			}

			err := service.SendItems(items, nil)

			require.NoError(t, err)
			assertRequestContains(t, mockClient, `filename="`+tc.filename+`"`)
			assertRequestContains(t, mockClient, string(tc.content))
		}

		mockClient.AssertExpectations(t)
	})
}

func TestSendFileWithEmptyContent(t *testing.T) {
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
			createTestMessageItemWithFile("Empty file", "empty.txt", []byte{}),
		}

		err := service.SendItems(items, nil)

		require.NoError(t, err)
		assertRequestContains(t, mockClient, `filename="empty.txt"`)

		mockClient.AssertExpectations(t)
	})
}

func TestSendFileWithUnicodeContent(t *testing.T) {
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

		//nolint:gosmopolitan // Intentional string literal containing rune in Han script (gosmopolitan)
		unicodeContent := []byte("Hello 世界 🌍 éñüñ")
		items := []types.MessageItem{
			createTestMessageItemWithFile("Unicode file", "unicode.txt", unicodeContent),
		}

		err := service.SendItems(items, nil)

		require.NoError(t, err)
		assertRequestContains(t, mockClient, string(unicodeContent))

		mockClient.AssertExpectations(t)
	})
}

func TestSendFileWithMessageText(t *testing.T) {
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
			createTestMessageItemWithFile(
				"This is a message with a file attachment",
				"attachment.txt",
				[]byte("file content"),
			),
		}

		err := service.SendItems(items, nil)

		require.NoError(t, err)
		assertRequestContains(t, mockClient, "This is a message with a file attachment")
		assertRequestContains(t, mockClient, `filename="attachment.txt"`)
		assertRequestContains(t, mockClient, "file content")

		mockClient.AssertExpectations(t)
	})
}
