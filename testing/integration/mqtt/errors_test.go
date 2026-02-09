package mqtt_test

import (
	"context"
	"errors"
	"testing"
	"testing/synctest"

	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	"github.com/nicholas-fedor/shoutrrr/pkg/services/push/mqtt"
)

func TestSendWithConnectionError(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		mockManager := &MockConnectionManager{}
		mockManager.On("AwaitConnection", mock.Anything).
			Return(errors.New("connection refused")).
			Once()

		service := createTestService(t, "mqtt://broker.example.com/test/topic", mockManager)

		err := service.Send("Test message", nil)
		require.Error(t, err)
		require.Contains(t, err.Error(), "connecting to MQTT broker")

		mockManager.AssertExpectations(t)
	})
}

func TestSendWithPublishError(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		mockManager := &MockConnectionManager{}
		mockManager.On("AwaitConnection", mock.Anything).Return(nil)
		mockManager.On("Publish", mock.Anything, mock.Anything).
			Return(nil, errors.New("network error")).
			Once()

		service := createTestService(t, "mqtt://broker.example.com/test/topic", mockManager)

		err := service.Send("Test message", nil)
		require.Error(t, err)
		require.Contains(t, err.Error(), "publishing to MQTT topic")

		mockManager.AssertExpectations(t)
	})
}

func TestSendWithFailureReasonCode(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		tests := []struct {
			name         string
			reasonCode   byte
			reasonString string
		}{
			{
				name:         "unspecified error",
				reasonCode:   0x80,
				reasonString: "Unspecified error",
			},
			{
				name:         "malformed packet",
				reasonCode:   0x81,
				reasonString: "Malformed Packet",
			},
			{
				name:         "protocol error",
				reasonCode:   0x82,
				reasonString: "Protocol Error",
			},
			{
				name:         "implementation specific error",
				reasonCode:   0x83,
				reasonString: "Implementation specific error",
			},
			{
				name:         "unsupported protocol version",
				reasonCode:   0x84,
				reasonString: "Unsupported Protocol Version",
			},
			{
				name:         "client identifier not valid",
				reasonCode:   0x85,
				reasonString: "Client Identifier not valid",
			},
			{
				name:         "bad user name or password",
				reasonCode:   0x86,
				reasonString: "Bad User Name or Password",
			},
			{
				name:         "not authorized",
				reasonCode:   0x87,
				reasonString: "Not authorized",
			},
		}

		for _, tt := range tests {
			mockManager := &MockConnectionManager{}
			mockManager.On("AwaitConnection", mock.Anything).Return(nil)
			mockManager.On("Publish", mock.Anything, mock.Anything).
				Return(createMockPublishResponseWithReason(tt.reasonCode, tt.reasonString), nil).
				Once()

			service := createTestService(t, "mqtt://broker.example.com/test/topic", mockManager)

			err := service.Send("Test message", nil)
			require.Error(t, err)

			// Verify it's a PublishError
			var publishErr mqtt.PublishError
			require.True(t, errors.As(err, &publishErr))
			require.Equal(t, tt.reasonCode, publishErr.ReasonCode)
			require.Equal(t, tt.reasonString, publishErr.ReasonString)

			mockManager.AssertExpectations(t)
		}
	})
}

func TestSendWithReasonCodeNoReasonString(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		mockManager := &MockConnectionManager{}
		mockManager.On("AwaitConnection", mock.Anything).Return(nil)
		mockManager.On("Publish", mock.Anything, mock.Anything).
			Return(createMockPublishResponse(0x80), nil). // Failure code without reason string
			Once()

		service := createTestService(t, "mqtt://broker.example.com/test/topic", mockManager)

		err := service.Send("Test message", nil)
		require.Error(t, err)

		var publishErr mqtt.PublishError
		require.True(t, errors.As(err, &publishErr))
		require.Equal(t, byte(0x80), publishErr.ReasonCode)
		require.Empty(t, publishErr.ReasonString)

		mockManager.AssertExpectations(t)
	})
}

func TestSendWithInvalidQoS(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		// Create service without mock manager - it should fail before connection
		service := createTestService(t, "mqtt://broker.example.com/test/topic")

		// Manually set invalid QoS
		service.Config.QoS = 3

		err := service.Send("Test message", nil)
		require.Error(t, err)
		require.Contains(t, err.Error(), "validating QoS")
	})
}

func TestSendWithNegativeQoS(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		// Create service without mock manager - it should fail before connection
		service := createTestService(t, "mqtt://broker.example.com/test/topic")

		// Manually set invalid QoS
		service.Config.QoS = -1

		err := service.Send("Test message", nil)
		require.Error(t, err)
		require.Contains(t, err.Error(), "validating QoS")
	})
}

func TestDisconnectError(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		mockManager := &MockConnectionManager{}
		mockManager.On("AwaitConnection", mock.Anything).Return(nil)
		mockManager.On("Publish", mock.Anything, mock.Anything).
			Return(createMockPublishResponse(0), nil).
			Once()
		mockManager.On("Disconnect", mock.Anything).
			Return(errors.New("disconnect error")).
			Once()

		service := createTestService(t, "mqtt://broker.example.com/test/topic", mockManager)

		// Send a message first
		err := service.Send("Test message", nil)
		require.NoError(t, err)

		// Close should return the disconnect error
		err = service.Close()
		require.Error(t, err)
		require.Contains(t, err.Error(), "disconnecting from MQTT broker")

		mockManager.AssertExpectations(t)
	})
}

func TestCloseWithoutConnection(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		// Create service without initializing connection
		service := createTestService(t, "mqtt://broker.example.com/test/topic")

		// Close should not error when connection was never established
		err := service.Close()
		require.NoError(t, err)
	})
}

func TestCloseIdempotent(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		mockManager := &MockConnectionManager{}
		mockManager.On("AwaitConnection", mock.Anything).Return(nil)
		mockManager.On("Publish", mock.Anything, mock.Anything).
			Return(createMockPublishResponse(0), nil).
			Once()
		mockManager.On("Disconnect", mock.Anything).Return(nil).Once()

		service := createTestService(t, "mqtt://broker.example.com/test/topic", mockManager)

		// Send a message first
		err := service.Send("Test message", nil)
		require.NoError(t, err)

		// First close
		err = service.Close()
		require.NoError(t, err)

		// Second close should return same result (no error, no panic)
		err = service.Close()
		require.NoError(t, err)

		// Disconnect should only be called once
		mockManager.AssertExpectations(t)
	})
}

func TestContextCancellationDuringAwaitConnection(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		mockManager := &MockConnectionManager{}
		mockManager.On("AwaitConnection", mock.Anything).
			Return(context.Canceled).
			Once()

		service := createTestService(t, "mqtt://broker.example.com/test/topic", mockManager)

		err := service.Send("Test message", nil)
		require.Error(t, err)
		require.Contains(t, err.Error(), "connecting to MQTT broker")

		mockManager.AssertExpectations(t)
	})
}

func TestContextDeadlineExceededDuringAwaitConnection(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		mockManager := &MockConnectionManager{}
		mockManager.On("AwaitConnection", mock.Anything).
			Return(context.DeadlineExceeded).
			Once()

		service := createTestService(t, "mqtt://broker.example.com/test/topic", mockManager)

		err := service.Send("Test message", nil)
		require.Error(t, err)
		require.Contains(t, err.Error(), "connecting to MQTT broker")

		mockManager.AssertExpectations(t)
	})
}

func TestPublishErrorErrorMethod(t *testing.T) {
	tests := []struct {
		name         string
		reasonCode   byte
		reasonString string
		expectedMsg  string
	}{
		{
			name:         "with reason string",
			reasonCode:   0x87,
			reasonString: "Not authorized",
			expectedMsg:  "MQTT publish failed: reason code 0x87 - Not authorized",
		},
		{
			name:         "without reason string",
			reasonCode:   0x80,
			reasonString: "",
			expectedMsg:  "MQTT publish failed: reason code 0x80",
		},
	}

	for _, tt := range tests {
		err := mqtt.PublishError{
			ReasonCode:   tt.reasonCode,
			ReasonString: tt.reasonString,
		}

		require.Equal(t, tt.expectedMsg, err.Error())
	}
}

func TestIsFailureCode(t *testing.T) {
	tests := []struct {
		code     byte
		expected bool
	}{
		{0x00, false}, // Success
		{0x01, false}, // Granted QoS 1
		{0x02, false}, // Granted QoS 2
		{0x10, false}, // No matching subscribers (not a failure)
		{0x7F, false}, // Just below threshold
		{0x80, true},  // Unspecified error
		{0x87, true},  // Not authorized
		{0xFF, true},  // Maximum value
	}

	for _, tt := range tests {
		result := mqtt.IsFailureCode(tt.code)
		require.Equal(t, tt.expected, result, "IsFailureCode(0x%02X)", tt.code)
	}
}
