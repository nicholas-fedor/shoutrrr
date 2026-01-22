package discord_test

import (
	"net/http"
	"strings"
	"testing"
	"testing/synctest"
	"unicode/utf8"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	"github.com/nicholas-fedor/shoutrrr/pkg/types"
)

func TestEmptyMessage(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		mockClient := &MockHTTPClient{}
		service := createTestService(
			t,
			"discord://test-token@test-webhook",
			mockClient,
		)

		err := service.Send("", nil)

		require.Error(t, err)
		require.Contains(t, err.Error(), "message is empty")

		mockClient.AssertExpectations(t)
	})
}

func TestNilMessageItems(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		mockClient := &MockHTTPClient{}
		service := createTestService(
			t,
			"discord://test-token@test-webhook",
			mockClient,
		)

		err := service.SendItems(nil, nil)

		require.Error(t, err)
		require.Contains(t, err.Error(), "creating payload")

		mockClient.AssertExpectations(t)
	})
}

func TestEmptyMessageItems(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		mockClient := &MockHTTPClient{}
		service := createTestService(
			t,
			"discord://test-token@test-webhook",
			mockClient,
		)

		err := service.SendItems([]types.MessageItem{}, nil)

		require.Error(t, err)
		require.Contains(t, err.Error(), "creating payload")
	})
}

func TestUnicodeMessages(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		mockClient := &MockHTTPClient{}
		service := createTestService(
			t,
			"discord://test-token@test-webhook",
			mockClient,
		)

		tests := []struct {
			name    string
			message string
		}{
			{"emoji", "Hello 🌍 🚀 ❤️"},
			{"chinese", "你好世界"},
			{"japanese", "こんにちは世界"},
			{"arabic", "مرحبا بالعالم"},
			{"mixed", "Hello 世界 🌍 こんにちは 🚀"},
			{"combining", "café naïve résumé"},
			{"zero width", "test\u200bhidden\u200btext"}, // zero-width spaces
		}

		for _, tt := range tests {
			mockClient.On("Do", mock.Anything).
				Return(createMockResponse(http.StatusNoContent, ""), nil). //nolint:bodyclose
				Once()

			err := service.Send(tt.message, nil)

			require.NoError(t, err)
			require.True(t, utf8.ValidString(tt.message), "Message should be valid UTF-8")
		}

		mockClient.AssertExpectations(t)
	})
}

func TestControlCharacters(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		tests := []struct {
			name    string
			message string
		}{
			{"newlines", "Line 1\nLine 2\nLine 3"},
			{"tabs", "Col1\tCol2\tCol3"},
			{"carriage return", "Line 1\r\nLine 2"},
			{"form feed", "Page 1\fPage 2"},
			{"vertical tab", "Line 1\vLine 2"},
			{"backspace", "Text\b"},
			{"null byte", "Text\x00More"},
			{"escape", "Text\x1b[31mRed\x1b[0m"},
		}

		for _, tt := range tests {
			mockClient := &MockHTTPClient{}
			service := createTestService(
				t,
				"discord://test-token@test-webhook",
				mockClient,
			)

			// With splitLines enabled by default, all lines are sent as embeds in one request
			expectedCalls := 1
			mockClient.On("Do", mock.Anything).
				Return(createMockResponse(http.StatusNoContent, ""), nil). //nolint:bodyclose
				Times(expectedCalls)

			err := service.Send(tt.message, nil)

			require.NoError(t, err)
			mockClient.AssertExpectations(t)
		}
	})
}

func TestVeryLongMessage(t *testing.T) {
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
			// Long message sent as one

		longMessage := strings.Repeat("This is a very long message that should be chunked. ", 1000)
		err := service.Send(longMessage, nil)

		require.NoError(t, err)
		mockClient.AssertNumberOfCalls(t, "Do", 1)

		mockClient.AssertExpectations(t)
	})
}

func TestMessageWithMaximumLength(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		mockClient := &MockHTTPClient{}
		service := createTestService(
			t,
			"discord://test-token@test-webhook",
			mockClient,
		)

		// Discord's max message length is 2000 chars
		maxLengthMessage := strings.Repeat("a", 2000)

		mockClient.On("Do", mock.Anything).
			Return(createMockResponse(http.StatusNoContent, ""), nil).
			Once()

		err := service.Send(maxLengthMessage, nil)

		require.NoError(t, err)

		mockClient.AssertExpectations(t)
	})
}

func TestMessageWithNullBytes(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		mockClient := &MockHTTPClient{}
		service := createTestService(
			t,
			"discord://test-token@test-webhook",
			mockClient,
		)

		messageWithNulls := "Hello\x00World\x00Test"

		mockClient.On("Do", mock.Anything).
			Return(createMockResponse(http.StatusNoContent, ""), nil).
			Once()

		err := service.Send(messageWithNulls, nil)

		require.NoError(t, err)

		mockClient.AssertExpectations(t)
	})
}

func TestInvalidUTF8(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		mockClient := &MockHTTPClient{}
		service := createTestService(
			t,
			"discord://test-token@test-webhook",
			mockClient,
		)

		// Create invalid UTF-8 sequence
		invalidUTF8 := []byte{0x80, 0x81, 0x82} // Invalid UTF-8 start bytes
		message := string(invalidUTF8)

		require.False(t, utf8.ValidString(message), "Should be invalid UTF-8")

		mockClient.On("Do", mock.Anything).
			Return(createMockResponse(http.StatusNoContent, ""), nil).
			Once()

		err := service.Send(message, nil)

		require.NoError(t, err) // Should handle invalid UTF-8 gracefully

		mockClient.AssertExpectations(t)
	})
}

func TestExtremelyLargeFile(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		mockClient := &MockHTTPClient{}
		service := createTestService(
			t,
			"discord://test-token@test-webhook",
			mockClient,
		)

		// Test with a very large file (simulated)
		largeData := make([]byte, 50*1024*1024) // 50MB
		for i := range largeData {
			largeData[i] = byte(i % 256)
		}

		mockClient.On("Do", mock.Anything).
			Return(createMockResponse(http.StatusNoContent, ""), nil).
			Once()

		items := []types.MessageItem{
			createTestMessageItemWithFile("Large file", "large.dat", largeData),
		}

		err := service.SendItems(items, nil)

		assert.NoError(t, err)

		mockClient.AssertExpectations(t)
	})
}

func TestFileWithEmptyName(t *testing.T) {
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
			createTestMessageItemWithFile("Test", "", []byte("content")),
		}

		err := service.SendItems(items, nil)

		require.NoError(t, err)

		mockClient.AssertExpectations(t)
	})
}

func TestFileWithSpecialCharactersInName(t *testing.T) {
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
			"file#hash.txt",
			"file@at.txt",
			"file$dollar.txt",
			"file&ampersand.txt",
			"file*star.txt",
			"file?question.txt",
			"file^caret.txt",
			"file`backtick.txt",
			"file|pipe.txt",
			"file\\backslash.txt",
			"file/slash.txt",
			"file:colon.txt",
			"file;semicolon.txt",
			"file<less.txt",
			"file>greater.txt",
			"file\"quote.txt",
			"file'apostrophe.txt",
		}

		for _, filename := range specialNames {
			mockClient.On("Do", mock.Anything).
				Return(createMockResponse(http.StatusNoContent, ""), nil).
				Once()

			items := []types.MessageItem{
				createTestMessageItemWithFile("Test file", filename, []byte("content")),
			}

			err := service.SendItems(items, nil)

			assert.NoError(t, err)
		}

		mockClient.AssertExpectations(t)
	})
}

func TestConcurrentRequests(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		// Test multiple concurrent requests
		done := make(chan bool, 3)

		for i := range 3 {
			go func(id int) {
				mockClient := &MockHTTPClient{}
				mockClient.On("Do", mock.Anything).
					Return(createMockResponse(http.StatusNoContent, ""), nil).
					Once()

				service := createTestService(t, "discord://test-token@test-webhook", mockClient)

				err := service.Send("Concurrent message", nil)
				require.NoError(t, err)

				mockClient.AssertExpectations(t)

				done <- true
			}(i)
		}

		// Wait for all goroutines to complete
		for range 3 {
			<-done
		}
	})
}

func TestMemoryExhaustionSimulation(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		mockClient := &MockHTTPClient{}
		service := createTestService(
			t,
			"discord://test-token@test-webhook",
			mockClient,
		)

		// Test with many large embeds
		mockClient.On("Do", mock.Anything).
			Return(createMockResponse(http.StatusNoContent, ""), nil).
			Once()

		var items []types.MessageItem
		for i := range 10 {
			items = append(items, types.MessageItem{
				Text: "Embed " + string(rune(i+'0')),
				Fields: []types.Field{
					{Key: "Field1", Value: strings.Repeat("Value", 100)},
					{Key: "Field2", Value: strings.Repeat("Value", 100)},
				},
			})
		}

		err := service.SendItems(items, nil)

		assert.NoError(t, err)

		mockClient.AssertExpectations(t)
	})
}

func TestRapidSuccessionRequests(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		mockClient := &MockHTTPClient{}
		service := createTestService(
			t,
			"discord://test-token@test-webhook",
			mockClient,
		)

		// Test many requests in quick succession
		for range 10 {
			mockClient.On("Do", mock.Anything).
				Return(createMockResponse(http.StatusNoContent, ""), nil).
				Once()
		}

		for range 10 {
			err := service.Send("Message", nil)
			assert.NoError(t, err)
		}

		mockClient.AssertExpectations(t)
	})
}
