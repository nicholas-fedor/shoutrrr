package mqtt_test

import (
	"strings"
	"testing"
	"testing/synctest"

	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

func TestPublishEmptyMessage(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		mockManager := &MockConnectionManager{}
		mockManager.On("AwaitConnection", mock.Anything).Return(nil)
		mockManager.On("Publish", mock.Anything, mock.Anything).
			Return(createMockPublishResponse(0), nil).
			Once()

		service := createTestService(t, "mqtt://broker.example.com/test/topic", mockManager)

		err := service.Send("", nil)
		require.NoError(t, err)

		// Verify empty payload was sent
		assertPublishPayload(t, mockManager, "")

		mockManager.AssertExpectations(t)
	})
}

func TestPublishVeryLongMessage(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		mockManager := &MockConnectionManager{}
		mockManager.On("AwaitConnection", mock.Anything).Return(nil)
		mockManager.On("Publish", mock.Anything, mock.Anything).
			Return(createMockPublishResponse(0), nil).
			Once()

		service := createTestService(t, "mqtt://broker.example.com/test/topic", mockManager)

		// Create a very long message (1MB)
		longMessage := strings.Repeat("x", 1024*1024)

		err := service.Send(longMessage, nil)
		require.NoError(t, err)

		// Verify the full payload was sent
		assertPublishPayload(t, mockManager, longMessage)

		mockManager.AssertExpectations(t)
	})
}

func TestPublishSpecialCharactersInTopic(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		tests := []struct {
			name          string
			topic         string
			expectedTopic string
		}{
			{
				name:          "topic with underscores",
				topic:         "test_topic_name",
				expectedTopic: "test_topic_name",
			},
			{
				name:          "topic with hyphens",
				topic:         "test-topic-name",
				expectedTopic: "test-topic-name",
			},
			{
				name:          "topic with numbers",
				topic:         "test123/topic456",
				expectedTopic: "test123/topic456",
			},
			{
				name:          "deep nested topic",
				topic:         "a/b/c/d/e/f/g/h",
				expectedTopic: "a/b/c/d/e/f/g/h",
			},
		}

		for _, tt := range tests {
			mockManager := &MockConnectionManager{}
			mockManager.On("AwaitConnection", mock.Anything).Return(nil)
			mockManager.On("Publish", mock.Anything, mock.Anything).
				Return(createMockPublishResponse(0), nil).
				Once()

			url := "mqtt://broker.example.com/" + tt.topic
			service := createTestService(t, url, mockManager)

			err := service.Send("Test message", nil)
			require.NoError(t, err)

			assertPublishCalled(t, mockManager, tt.expectedTopic)

			mockManager.AssertExpectations(t)
		}
	})
}

func TestPublishUnicodeInMessage(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		tests := []struct {
			name    string
			message string
		}{
			{"emoji", "Hello 👋 World 🌍"},
			{"chinese", "你好世界"},
			{"japanese", "こんにちは"},
			{"arabic", "مرحبا بالعالم"},
			{"russian", "Привет мир"},
			{"mixed unicode", "Hello 世界 🌍 Привет"},
			{"unicode with special chars", "Test™ © 2024 • ★ ♠"},
		}

		for _, tt := range tests {
			mockManager := &MockConnectionManager{}
			mockManager.On("AwaitConnection", mock.Anything).Return(nil)
			mockManager.On("Publish", mock.Anything, mock.Anything).
				Return(createMockPublishResponse(0), nil).
				Once()

			service := createTestService(t, "mqtt://broker.example.com/test/topic", mockManager)

			err := service.Send(tt.message, nil)
			require.NoError(t, err)

			assertPublishPayload(t, mockManager, tt.message)

			mockManager.AssertExpectations(t)
		}
	})
}

func TestPublishNewlinesInMessage(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		tests := []struct {
			name    string
			message string
		}{
			{"unix newlines", "Line1\nLine2\nLine3"},
			{"windows newlines", "Line1\r\nLine2\r\nLine3"},
			{"mixed newlines", "Line1\nLine2\r\nLine3"},
			{"only newlines", "\n\n\n"},
		}

		for _, tt := range tests {
			mockManager := &MockConnectionManager{}
			mockManager.On("AwaitConnection", mock.Anything).Return(nil)
			mockManager.On("Publish", mock.Anything, mock.Anything).
				Return(createMockPublishResponse(0), nil).
				Once()

			service := createTestService(t, "mqtt://broker.example.com/test/topic", mockManager)

			err := service.Send(tt.message, nil)
			require.NoError(t, err)

			assertPublishPayload(t, mockManager, tt.message)

			mockManager.AssertExpectations(t)
		}
	})
}

func TestPublishTabsInMessage(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		mockManager := &MockConnectionManager{}
		mockManager.On("AwaitConnection", mock.Anything).Return(nil)
		mockManager.On("Publish", mock.Anything, mock.Anything).
			Return(createMockPublishResponse(0), nil).
			Once()

		service := createTestService(t, "mqtt://broker.example.com/test/topic", mockManager)

		message := "Column1\tColumn2\tColumn3"
		err := service.Send(message, nil)
		require.NoError(t, err)

		assertPublishPayload(t, mockManager, message)

		mockManager.AssertExpectations(t)
	})
}

func TestPublishJSONMessage(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		mockManager := &MockConnectionManager{}
		mockManager.On("AwaitConnection", mock.Anything).Return(nil)
		mockManager.On("Publish", mock.Anything, mock.Anything).
			Return(createMockPublishResponse(0), nil).
			Once()

		service := createTestService(t, "mqtt://broker.example.com/test/topic", mockManager)

		jsonMessage := `{"key": "value", "nested": {"array": [1, 2, 3]}}`
		err := service.Send(jsonMessage, nil)
		require.NoError(t, err)

		assertPublishPayload(t, mockManager, jsonMessage)

		mockManager.AssertExpectations(t)
	})
}

func TestPublishXMLMessage(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		mockManager := &MockConnectionManager{}
		mockManager.On("AwaitConnection", mock.Anything).Return(nil)
		mockManager.On("Publish", mock.Anything, mock.Anything).
			Return(createMockPublishResponse(0), nil).
			Once()

		service := createTestService(t, "mqtt://broker.example.com/test/topic", mockManager)

		xmlMessage := `<message><to>user</to><from>sender</from><body>Hello</body></message>`
		err := service.Send(xmlMessage, nil)
		require.NoError(t, err)

		assertPublishPayload(t, mockManager, xmlMessage)

		mockManager.AssertExpectations(t)
	})
}

func TestPublishHTMLEntities(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		mockManager := &MockConnectionManager{}
		mockManager.On("AwaitConnection", mock.Anything).Return(nil)
		mockManager.On("Publish", mock.Anything, mock.Anything).
			Return(createMockPublishResponse(0), nil).
			Once()

		service := createTestService(t, "mqtt://broker.example.com/test/topic", mockManager)

		htmlMessage := "<tag> & \"quoted\" 'apostrophe'"
		err := service.Send(htmlMessage, nil)
		require.NoError(t, err)

		assertPublishPayload(t, mockManager, htmlMessage)

		mockManager.AssertExpectations(t)
	})
}

func TestPublishWhitespaceOnly(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		tests := []struct {
			name    string
			message string
		}{
			{"single space", " "},
			{"multiple spaces", "     "},
			{"tabs", "\t\t\t"},
			{"mixed whitespace", " \t\n\r "},
		}

		for _, tt := range tests {
			mockManager := &MockConnectionManager{}
			mockManager.On("AwaitConnection", mock.Anything).Return(nil)
			mockManager.On("Publish", mock.Anything, mock.Anything).
				Return(createMockPublishResponse(0), nil).
				Once()

			service := createTestService(t, "mqtt://broker.example.com/test/topic", mockManager)

			err := service.Send(tt.message, nil)
			require.NoError(t, err)

			assertPublishPayload(t, mockManager, tt.message)

			mockManager.AssertExpectations(t)
		}
	})
}

func TestPublishBinaryLikeData(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		mockManager := &MockConnectionManager{}
		mockManager.On("AwaitConnection", mock.Anything).Return(nil)
		mockManager.On("Publish", mock.Anything, mock.Anything).
			Return(createMockPublishResponse(0), nil).
			Once()

		service := createTestService(t, "mqtt://broker.example.com/test/topic", mockManager)

		// Create a string with all byte values (simulating binary data)
		var binaryLike strings.Builder
		for i := range 256 {
			binaryLike.WriteByte(byte(i))
		}

		message := binaryLike.String()

		err := service.Send(message, nil)
		require.NoError(t, err)

		assertPublishPayload(t, mockManager, message)

		mockManager.AssertExpectations(t)
	})
}

func TestPublishWithNilParams(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		mockManager := &MockConnectionManager{}
		mockManager.On("AwaitConnection", mock.Anything).Return(nil)
		mockManager.On("Publish", mock.Anything, mock.Anything).
			Return(createMockPublishResponse(0), nil).
			Once()

		service := createTestService(t, "mqtt://broker.example.com/test/topic", mockManager)

		err := service.Send("Test message", nil)
		require.NoError(t, err)

		mockManager.AssertExpectations(t)
	})
}

func TestPublishWithEmptyParams(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		mockManager := &MockConnectionManager{}
		mockManager.On("AwaitConnection", mock.Anything).Return(nil)
		mockManager.On("Publish", mock.Anything, mock.Anything).
			Return(createMockPublishResponse(0), nil).
			Once()

		service := createTestService(t, "mqtt://broker.example.com/test/topic", mockManager)

		params := createTestParams()
		err := service.Send("Test message", params)
		require.NoError(t, err)

		mockManager.AssertExpectations(t)
	})
}

func TestPublishMultipleConcurrentMessages(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		mockManager := &MockConnectionManager{}
		mockManager.On("AwaitConnection", mock.Anything).Return(nil)

		// Set up mock for multiple concurrent publish calls
		for range 5 {
			mockManager.On("Publish", mock.Anything, mock.Anything).
				Return(createMockPublishResponse(0), nil).
				Once()
		}

		service := createTestService(t, "mqtt://broker.example.com/test/topic", mockManager)

		// Send multiple messages
		done := make(chan error, 5)

		for i := range 5 {
			go func(idx int) {
				done <- service.Send("Message", nil)
			}(i)
		}

		// Wait for all to complete
		for range 5 {
			err := <-done
			require.NoError(t, err)
		}

		mockManager.AssertExpectations(t)
	})
}

func TestPublishWithEscapedCharactersInURL(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		mockManager := &MockConnectionManager{}
		mockManager.On("AwaitConnection", mock.Anything).Return(nil)
		mockManager.On("Publish", mock.Anything, mock.Anything).
			Return(createMockPublishResponse(0), nil).
			Once()

		// URL with percent-encoded characters in password
		service := createTestService(
			t,
			"mqtt://user:pass%40word@broker.example.com/test/topic",
			mockManager,
		)

		err := service.Send("Test message", nil)
		require.NoError(t, err)

		// Verify password was decoded correctly
		require.Equal(t, "pass@word", service.Config.Password)

		mockManager.AssertExpectations(t)
	})
}

func TestPublishWithPortInURL(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		mockManager := &MockConnectionManager{}
		mockManager.On("AwaitConnection", mock.Anything).Return(nil)
		mockManager.On("Publish", mock.Anything, mock.Anything).
			Return(createMockPublishResponse(0), nil).
			Once()

		service := createTestService(t, "mqtt://broker.example.com:1884/test/topic", mockManager)

		err := service.Send("Test message", nil)
		require.NoError(t, err)

		require.Equal(t, 1884, service.Config.Port)

		mockManager.AssertExpectations(t)
	})
}

func TestPublishDeepTopicPath(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		mockManager := &MockConnectionManager{}
		mockManager.On("AwaitConnection", mock.Anything).Return(nil)
		mockManager.On("Publish", mock.Anything, mock.Anything).
			Return(createMockPublishResponse(0), nil).
			Once()

		// Very deep topic path
		deepTopic := "level1/level2/level3/level4/level5/level6/level7/level8/level9/level10"
		service := createTestService(t, "mqtt://broker.example.com/"+deepTopic, mockManager)

		err := service.Send("Test message", nil)
		require.NoError(t, err)

		assertPublishCalled(t, mockManager, deepTopic)

		mockManager.AssertExpectations(t)
	})
}
