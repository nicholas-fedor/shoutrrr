package mqtt_test

import (
	"testing"
	"testing/synctest"

	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	"github.com/nicholas-fedor/shoutrrr/pkg/services/push/mqtt"
)

func TestPublishBasicMessage(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		mockManager := &MockConnectionManager{}
		mockManager.On("AwaitConnection", mock.Anything).Return(nil)
		mockManager.On("Publish", mock.Anything, mock.Anything).
			Return(createMockPublishResponse(0), nil).
			Once()

		service := createTestService(t, "mqtt://broker.example.com/test/topic", mockManager)

		err := service.Send("Hello, MQTT!", nil)
		require.NoError(t, err)

		// Verify the publish was called with correct parameters
		assertPublishCalled(t, mockManager, "test/topic")
		assertPublishPayload(t, mockManager, "Hello, MQTT!")
		assertPublishQoS(t, mockManager, 0) // Default QoS

		mockManager.AssertExpectations(t)
	})
}

func TestPublishWithDifferentQoSLevels(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		tests := []struct {
			name string
			qos  byte
		}{
			{"QoS 0 - fire and forget", 0},
			{"QoS 1 - at least once", 1},
			{"QoS 2 - exactly once", 2},
		}

		for _, tt := range tests {
			mockManager := &MockConnectionManager{}
			mockManager.On("AwaitConnection", mock.Anything).Return(nil)
			mockManager.On("Publish", mock.Anything, mock.Anything).
				Return(createMockPublishResponse(0), nil).
				Once()

			url := "mqtt://broker.example.com/test/topic?qos=" + string(rune('0'+tt.qos))
			service := createTestService(t, url, mockManager)

			err := service.Send("Test message", nil)
			require.NoError(t, err)

			assertPublishQoS(t, mockManager, tt.qos)

			mockManager.AssertExpectations(t)
		}
	})
}

func TestPublishWithRetainedFlag(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		tests := []struct {
			name            string
			url             string
			expectedRetained bool
		}{
			{"retained not set", "mqtt://broker.example.com/test/topic", false},
			{"retained yes", "mqtt://broker.example.com/test/topic?retained=yes", true},
			{"retained true", "mqtt://broker.example.com/test/topic?retained=true", true},
		}

		for _, tt := range tests {
			mockManager := &MockConnectionManager{}
			mockManager.On("AwaitConnection", mock.Anything).Return(nil)
			mockManager.On("Publish", mock.Anything, mock.Anything).
				Return(createMockPublishResponse(0), nil).
				Once()

			service := createTestService(t, tt.url, mockManager)

			err := service.Send("Test message", nil)
			require.NoError(t, err)

			assertPublishRetained(t, mockManager, tt.expectedRetained)

			mockManager.AssertExpectations(t)
		}
	})
}

func TestPublishWithParamsOverride(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		mockManager := &MockConnectionManager{}
		mockManager.On("AwaitConnection", mock.Anything).Return(nil)
		mockManager.On("Publish", mock.Anything, mock.Anything).
			Return(createMockPublishResponse(0), nil).
			Once()

		// Create service with default QoS 0
		service := createTestService(t, "mqtt://broker.example.com/test/topic?qos=0", mockManager)

		// Override QoS via params
		params := createTestParams("qos", "2", "retained", "yes")
		err := service.Send("Test message", params)
		require.NoError(t, err)

		// Verify the override was applied
		assertPublishQoS(t, mockManager, 2)
		assertPublishRetained(t, mockManager, true)

		mockManager.AssertExpectations(t)
	})
}

func TestPublishMultipleMessages(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		mockManager := &MockConnectionManager{}
		mockManager.On("AwaitConnection", mock.Anything).Return(nil)

		// Set up mock for multiple publish calls
		for i := 0; i < 3; i++ {
			mockManager.On("Publish", mock.Anything, mock.Anything).
				Return(createMockPublishResponse(0), nil).
				Once()
		}

		service := createTestService(t, "mqtt://broker.example.com/test/topic", mockManager)

		messages := []string{
			"First message",
			"Second message",
			"Third message",
		}

		for _, msg := range messages {
			err := service.Send(msg, nil)
			require.NoError(t, err)
		}

		mockManager.AssertExpectations(t)
	})
}

func TestPublishTopicVariations(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		tests := []struct {
			name          string
			url           string
			expectedTopic string
		}{
			{"simple topic", "mqtt://broker.example.com/alerts", "alerts"},
			{"nested topic", "mqtt://broker.example.com/home/sensors/temperature", "home/sensors/temperature"},
			{"deep topic", "mqtt://broker.example.com/a/b/c/d/e/f", "a/b/c/d/e/f"},
		}

		for _, tt := range tests {
			mockManager := &MockConnectionManager{}
			mockManager.On("AwaitConnection", mock.Anything).Return(nil)
			mockManager.On("Publish", mock.Anything, mock.Anything).
				Return(createMockPublishResponse(0), nil).
				Once()

			service := createTestService(t, tt.url, mockManager)

			err := service.Send("Test message", nil)
			require.NoError(t, err)

			assertPublishCalled(t, mockManager, tt.expectedTopic)

			mockManager.AssertExpectations(t)
		}
	})
}

func TestPublishVerifyPublishStructFields(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		mockManager := &MockConnectionManager{}
		mockManager.On("AwaitConnection", mock.Anything).Return(nil)
		mockManager.On("Publish", mock.Anything, mock.Anything).
			Return(createMockPublishResponse(0), nil).
			Once()

		service := createTestService(
			t,
			"mqtt://broker.example.com/test/topic?qos=1&retained=yes",
			mockManager,
		)

		err := service.Send("Test payload", nil)
		require.NoError(t, err)

		// Get the publish struct and verify all fields
		publish := getPublishCall(mockManager)
		require.NotNil(t, publish)
		require.Equal(t, "test/topic", publish.Topic)
		require.Equal(t, byte(1), publish.QoS)
		require.Equal(t, true, publish.Retain)
		require.Equal(t, []byte("Test payload"), publish.Payload)

		mockManager.AssertExpectations(t)
	})
}

func TestPublishWithReasonCodeZero(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		mockManager := &MockConnectionManager{}
		mockManager.On("AwaitConnection", mock.Anything).Return(nil)
		mockManager.On("Publish", mock.Anything, mock.Anything).
			Return(createMockPublishResponse(0), nil). // Success
			Once()

		service := createTestService(t, "mqtt://broker.example.com/test/topic", mockManager)

		err := service.Send("Test message", nil)
		require.NoError(t, err)

		mockManager.AssertExpectations(t)
	})
}

func TestPublishWithNonFailureReasonCode(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		mockManager := &MockConnectionManager{}
		mockManager.On("AwaitConnection", mock.Anything).Return(nil)
		// Reason code 1 is a non-failure code (success with continuation)
		mockManager.On("Publish", mock.Anything, mock.Anything).
			Return(createMockPublishResponse(1), nil).
			Once()

		service := createTestService(t, "mqtt://broker.example.com/test/topic", mockManager)

		// Non-failure reason codes should not return an error
		err := service.Send("Test message", nil)
		require.NoError(t, err)

		mockManager.AssertExpectations(t)
	})
}

func TestPublishServiceConfigAfterInit(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		mockManager := &MockConnectionManager{}
		mockManager.On("AwaitConnection", mock.Anything).Return(nil)
		mockManager.On("Publish", mock.Anything, mock.Anything).
			Return(createMockPublishResponse(0), nil).
			Once()

		service := createTestService(
			t,
			"mqtt://user:pass@broker.example.com:1883/test/topic?qos=1&retained=yes&clientid=testclient",
			mockManager,
		)

		// Verify config is properly set
		require.Equal(t, "broker.example.com", service.Config.Host)
		require.Equal(t, 1883, service.Config.Port)
		require.Equal(t, "test/topic", service.Config.Topic)
		require.Equal(t, "user", service.Config.Username)
		require.Equal(t, "pass", service.Config.Password)
		require.Equal(t, mqtt.QoS(1), service.Config.QoS)
		require.Equal(t, true, service.Config.Retained)
		require.Equal(t, "testclient", service.Config.ClientID)

		err := service.Send("Test message", nil)
		require.NoError(t, err)

		mockManager.AssertExpectations(t)
	})
}
