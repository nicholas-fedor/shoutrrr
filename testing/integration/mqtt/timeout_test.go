package mqtt_test

import (
	"context"
	"errors"
	"testing"
	"testing/synctest"

	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

func TestPublishTimeout(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		mockManager := &MockConnectionManager{}
		mockManager.On("AwaitConnection", mock.Anything).Return(nil)

		// Simulate a timeout error from publish
		mockManager.On("Publish", mock.Anything, mock.Anything).
			Return(nil, context.DeadlineExceeded).
			Once()

		service := createTestService(t, "mqtt://broker.example.com/test/topic", mockManager)

		err := service.Send("Test message", nil)
		require.Error(t, err)
		require.Contains(t, err.Error(), "publishing to MQTT topic")

		mockManager.AssertExpectations(t)
	})
}

func TestAwaitConnectionTimeout(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		mockManager := &MockConnectionManager{}

		// Simulate a connection timeout
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

func TestDisconnectTimeout(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		mockManager := &MockConnectionManager{}
		mockManager.On("AwaitConnection", mock.Anything).Return(nil)
		mockManager.On("Publish", mock.Anything, mock.Anything).
			Return(createMockPublishResponse(0), nil).
			Once()

		// Simulate a disconnect timeout
		mockManager.On("Disconnect", mock.Anything).
			Return(context.DeadlineExceeded).
			Once()

		service := createTestService(t, "mqtt://broker.example.com/test/topic", mockManager)

		// Send a message first
		err := service.Send("Test message", nil)
		require.NoError(t, err)

		err = service.Close()
		require.Error(t, err)
		require.Contains(t, err.Error(), "disconnecting from MQTT broker")

		mockManager.AssertExpectations(t)
	})
}

func TestContextCancellationDuringPublish(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		mockManager := &MockConnectionManager{}
		mockManager.On("AwaitConnection", mock.Anything).Return(nil)

		// Simulate context cancellation
		mockManager.On("Publish", mock.Anything, mock.Anything).
			Return(nil, context.Canceled).
			Once()

		service := createTestService(t, "mqtt://broker.example.com/test/topic", mockManager)

		err := service.Send("Test message", nil)
		require.Error(t, err)
		require.Contains(t, err.Error(), "publishing to MQTT topic")

		mockManager.AssertExpectations(t)
	})
}

func TestContextCancellationDuringDisconnect(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		mockManager := &MockConnectionManager{}
		mockManager.On("AwaitConnection", mock.Anything).Return(nil)
		mockManager.On("Publish", mock.Anything, mock.Anything).
			Return(createMockPublishResponse(0), nil).
			Once()
		mockManager.On("Disconnect", mock.Anything).
			Return(context.Canceled).
			Once()

		service := createTestService(t, "mqtt://broker.example.com/test/topic", mockManager)

		// Send a message first
		err := service.Send("Test message", nil)
		require.NoError(t, err)

		err = service.Close()
		require.Error(t, err)
		require.Contains(t, err.Error(), "disconnecting from MQTT broker")

		mockManager.AssertExpectations(t)
	})
}

func TestFastOperationCompletesBeforeTimeout(t *testing.T) {
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

func TestTimeoutErrorWrapping(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		mockManager := &MockConnectionManager{}
		mockManager.On("AwaitConnection", mock.Anything).
			Return(context.DeadlineExceeded).
			Once()

		service := createTestService(t, "mqtt://broker.example.com/test/topic", mockManager)

		err := service.Send("Test message", nil)
		require.Error(t, err)

		// Verify the error is properly wrapped
		require.ErrorIs(t, err, context.DeadlineExceeded)
		require.Contains(t, err.Error(), "connecting to MQTT broker")

		mockManager.AssertExpectations(t)
	})
}

func TestCancelledContextErrorWrapping(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		mockManager := &MockConnectionManager{}
		mockManager.On("AwaitConnection", mock.Anything).
			Return(context.Canceled).
			Once()

		service := createTestService(t, "mqtt://broker.example.com/test/topic", mockManager)

		err := service.Send("Test message", nil)
		require.Error(t, err)

		// Verify the error is properly wrapped
		require.ErrorIs(t, err, context.Canceled)
		require.Contains(t, err.Error(), "connecting to MQTT broker")

		mockManager.AssertExpectations(t)
	})
}

func TestMultipleOperationsWithTimeouts(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		mockManager := &MockConnectionManager{}

		// First call succeeds
		mockManager.On("AwaitConnection", mock.Anything).Return(nil).Once()
		mockManager.On("Publish", mock.Anything, mock.Anything).
			Return(createMockPublishResponse(0), nil).
			Once()

		// Second call times out
		mockManager.On("AwaitConnection", mock.Anything).
			Return(context.DeadlineExceeded).
			Once()

		service := createTestService(t, "mqtt://broker.example.com/test/topic", mockManager)

		// First send should succeed
		err := service.Send("First message", nil)
		require.NoError(t, err)

		// Second send should fail with timeout
		err = service.Send("Second message", nil)
		require.Error(t, err)
		require.ErrorIs(t, err, context.DeadlineExceeded)

		mockManager.AssertExpectations(t)
	})
}

func TestTimeoutWithCustomNetworkError(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		customErr := errors.New("custom network timeout error")

		mockManager := &MockConnectionManager{}
		mockManager.On("AwaitConnection", mock.Anything).Return(nil)
		mockManager.On("Publish", mock.Anything, mock.Anything).
			Return(nil, customErr).
			Once()

		service := createTestService(t, "mqtt://broker.example.com/test/topic", mockManager)

		err := service.Send("Test message", nil)
		require.Error(t, err)
		require.Contains(t, err.Error(), "publishing to MQTT topic")
		require.Contains(t, err.Error(), customErr.Error())

		mockManager.AssertExpectations(t)
	})
}
