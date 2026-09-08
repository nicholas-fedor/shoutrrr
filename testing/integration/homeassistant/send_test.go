package homeassistant_test

import (
	"bytes"
	"testing"
	"testing/synctest"

	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	"github.com/nicholas-fedor/shoutrrr/pkg/types"
)

func TestSendBasicMessage(t *testing.T) {
	t.Parallel()
	synctest.Test(t, func(t *testing.T) {
		service, mockClient := createTestServiceWithMock(t, validHomeAssistantURL)

		err := service.Send("Hello from Home Assistant", nil)
		require.NoError(t, err)

		payload := requestJSON(t, mockClient)
		require.Equal(t, "Hello from Home Assistant", payload.Message)
		require.Empty(t, payload.Title)
		require.Empty(t, payload.NotificationID)

		body := requestBody(t, mockClient)
		require.False(t, bytes.Contains(body, []byte(`"title"`)))
		require.False(t, bytes.Contains(body, []byte(`"notification_id"`)))
		require.False(t, bytes.Contains(body, []byte(`"target"`)))

		mockClient.AssertExpectations(t)
	})
}

func TestSendWithTitleAndNid(t *testing.T) {
	t.Parallel()
	synctest.Test(t, func(t *testing.T) {
		service, mockClient := createTestServiceWithMock(
			t,
			validHomeAssistantURL+"?title=Alert&nid=watchtower",
		)

		err := service.Send("Something happened", nil)
		require.NoError(t, err)

		payload := requestJSON(t, mockClient)
		require.Equal(t, "Alert", payload.Title)
		require.Equal(t, "watchtower", payload.NotificationID)
		require.Equal(t, "Something happened", payload.Message)

		mockClient.AssertExpectations(t)
	})
}

func TestSendWithTitleParam(t *testing.T) {
	t.Parallel()
	synctest.Test(t, func(t *testing.T) {
		service, mockClient := createTestServiceWithMock(t, validHomeAssistantURL)

		params := types.Params{"title": "Override"}
		err := service.Send("updated", &params)
		require.NoError(t, err)

		require.Equal(t, "Override", requestJSON(t, mockClient).Title)

		mockClient.AssertExpectations(t)
	})
}

func TestSendNotifyServiceOmitsNotificationID(t *testing.T) {
	t.Parallel()
	synctest.Test(t, func(t *testing.T) {
		service, mockClient := createTestServiceWithMock(
			t,
			validHomeAssistantURL+"?service=notify.mobile_app_phone&targets=device1&nid=ignored",
		)

		err := service.Send("ping", nil)
		require.NoError(t, err)

		payload := requestJSON(t, mockClient)
		require.Equal(t, "ping", payload.Message)
		require.Empty(t, payload.NotificationID)
		require.Equal(t, []string{"device1"}, payload.Targets)
		require.False(t, bytes.Contains(requestBody(t, mockClient), []byte(`"notification_id"`)))
		require.False(t, bytes.Contains(requestBody(t, mockClient), []byte(`"targets"`)))
		require.Contains(t, string(requestBody(t, mockClient)), `"target"`)

		mockClient.AssertExpectations(t)
	})
}

func TestSendEmptyMessage(t *testing.T) {
	t.Parallel()
	synctest.Test(t, func(t *testing.T) {
		mockClient := &MockHTTPClient{}
		service := createTestService(t, validHomeAssistantURL, mockClient)

		err := service.Send("", nil)
		require.Error(t, err)

		mockClient.AssertNotCalled(t, "Do", mock.Anything)
	})
}
