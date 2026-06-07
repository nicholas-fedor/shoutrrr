package zulip_test

import (
	"context"
	"net/http"
	"strings"
	"testing"
	"testing/synctest"

	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	"github.com/nicholas-fedor/shoutrrr/pkg/services/chat/zulip"
	"github.com/nicholas-fedor/shoutrrr/pkg/services/chat/zulip/mocks"
)

func TestServiceSendPlainText(t *testing.T) {
	t.Parallel()
	synctest.Test(t, func(t *testing.T) {
		mockClient := mocks.NewMockHTTPClient(t)
		service := createTestService(
			t,
			"zulip://bot@example.com:secret-key@zulip.example.com?stream=general&topic=announcements",
			mockClient,
		)

		mockClient.On("Do", mock.AnythingOfType("*http.Request")).
			Return(createMockResponse(http.StatusOK, `{"result": "success"}`), nil).
			Once()

		err := service.Send("Hello, World!", nil)

		require.NoError(t, err)
		assertRequestContains(t, mockClient, "content=Hello%2C+World%21")

		mockClient.AssertExpectations(t)
	})
}

func TestServiceSendEmptyMessage(t *testing.T) {
	t.Parallel()
	synctest.Test(t, func(t *testing.T) {
		mockClient := mocks.NewMockHTTPClient(t)
		service := createTestService(
			t,
			"zulip://bot@example.com:secret-key@zulip.example.com?stream=general",
			mockClient,
		)

		mockClient.On("Do", mock.AnythingOfType("*http.Request")).
			Return(createMockResponse(http.StatusOK, `{"result": "success"}`), nil).
			Once()

		err := service.Send("", nil)

		require.NoError(t, err)
		assertRequestContains(t, mockClient, "content=")

		mockClient.AssertExpectations(t)
	})
}

func TestServiceSendWithStreamParam(t *testing.T) {
	t.Parallel()
	synctest.Test(t, func(t *testing.T) {
		mockClient := mocks.NewMockHTTPClient(t)
		service := createTestService(
			t,
			"zulip://bot@example.com:secret-key@zulip.example.com",
			mockClient,
		)

		mockClient.On("Do", mock.AnythingOfType("*http.Request")).
			Return(createMockResponse(http.StatusOK, `{"result": "success"}`), nil).
			Once()

		params := createTestParams("stream", "general")
		err := service.Send("Test message", params)

		require.NoError(t, err)
		assertRequestContains(t, mockClient, "to=general")

		mockClient.AssertExpectations(t)
	})
}

func TestServiceSendWithTopicParam(t *testing.T) {
	t.Parallel()
	synctest.Test(t, func(t *testing.T) {
		mockClient := mocks.NewMockHTTPClient(t)
		service := createTestService(
			t,
			"zulip://bot@example.com:secret-key@zulip.example.com?stream=general",
			mockClient,
		)

		mockClient.On("Do", mock.AnythingOfType("*http.Request")).
			Return(createMockResponse(http.StatusOK, `{"result": "success"}`), nil).
			Once()

		params := createTestParams("topic", "announcements")
		err := service.Send("Test message", params)

		require.NoError(t, err)
		assertRequestContains(t, mockClient, "topic=announcements")

		mockClient.AssertExpectations(t)
	})
}

func TestServiceSendWithNilParams(t *testing.T) {
	t.Parallel()
	synctest.Test(t, func(t *testing.T) {
		mockClient := mocks.NewMockHTTPClient(t)
		service := createTestService(
			t,
			"zulip://bot@example.com:secret-key@zulip.example.com?stream=general&topic=announcements",
			mockClient,
		)

		mockClient.On("Do", mock.AnythingOfType("*http.Request")).
			Return(createMockResponse(http.StatusOK, `{"result": "success"}`), nil).
			Once()

		err := service.Send("Test message", nil)

		require.NoError(t, err)
		assertRequestContains(t, mockClient, "to=general")
		assertRequestContains(t, mockClient, "topic=announcements")

		mockClient.AssertExpectations(t)
	})
}

func TestServiceSendWithContext(t *testing.T) {
	t.Parallel()
	synctest.Test(t, func(t *testing.T) {
		mockClient := mocks.NewMockHTTPClient(t)
		service := createTestService(
			t,
			"zulip://bot@example.com:secret-key@zulip.example.com?stream=general",
			mockClient,
		)

		mockClient.On("Do", mock.AnythingOfType("*http.Request")).
			Return(createMockResponse(http.StatusOK, `{"result": "success"}`), nil).
			Once()

		ctx := context.Background()
		err := service.SendWithContext(ctx, "Context message", nil)

		require.NoError(t, err)
		assertRequestContains(t, mockClient, "content=Context+message")

		mockClient.AssertExpectations(t)
	})
}

func TestServiceSendWithCancelledContext(t *testing.T) {
	t.Parallel()
	synctest.Test(t, func(t *testing.T) {
		mockClient := mocks.NewMockHTTPClient(t)
		service := createTestService(
			t,
			"zulip://bot@example.com:secret-key@zulip.example.com?stream=general",
			mockClient,
		)

		mockClient.On("Do", mock.AnythingOfType("*http.Request")).
			Return(nil, context.Canceled).
			Maybe()

		ctx, cancel := context.WithCancel(context.Background())
		cancel()

		err := service.SendWithContext(ctx, "Canceled message", nil)

		require.Error(t, err)

		mockClient.AssertExpectations(t)
	})
}

func TestServiceSendWithTitleParam(t *testing.T) {
	t.Parallel()
	synctest.Test(t, func(t *testing.T) {
		mockClient := mocks.NewMockHTTPClient(t)
		service := createTestService(
			t,
			"zulip://bot@example.com:secret-key@zulip.example.com?stream=general",
			mockClient,
		)

		mockClient.On("Do", mock.AnythingOfType("*http.Request")).
			Return(createMockResponse(http.StatusOK, `{"result": "success"}`), nil).
			Once()

		params := createTestParams("title", "Alert")
		err := service.Send("Something happened", params)

		require.NoError(t, err)
		assertRequestContains(t, mockClient, "topic=Alert")

		mockClient.AssertExpectations(t)
	})
}

func TestServiceSendWithUnicodeMessage(t *testing.T) {
	t.Parallel()
	synctest.Test(t, func(t *testing.T) {
		mockClient := mocks.NewMockHTTPClient(t)
		service := createTestService(
			t,
			"zulip://bot@example.com:secret-key@zulip.example.com?stream=general",
			mockClient,
		)

		mockClient.On("Do", mock.AnythingOfType("*http.Request")).
			Return(createMockResponse(http.StatusOK, `{"result": "success"}`), nil).
			Once()

		err := service.Send("Hello 世界 🌍", nil) //nolint:gosmopolitan // unicode test message

		require.NoError(t, err)
		assertRequestContains(t, mockClient, "content=")

		mockClient.AssertExpectations(t)
	})
}

func TestServiceSendWithSpecialCharacters(t *testing.T) {
	t.Parallel()
	synctest.Test(t, func(t *testing.T) {
		mockClient := mocks.NewMockHTTPClient(t)
		service := createTestService(
			t,
			"zulip://bot@example.com:secret-key@zulip.example.com?stream=general",
			mockClient,
		)

		mockClient.On("Do", mock.AnythingOfType("*http.Request")).
			Return(createMockResponse(http.StatusOK, `{"result": "success"}`), nil).
			Once()

		err := service.Send("Test <script>alert('xss')</script>", nil)

		require.NoError(t, err)
		assertRequestContains(t, mockClient, "content=")

		mockClient.AssertExpectations(t)
	})
}

func TestServiceSendWithLongMessage(t *testing.T) {
	t.Parallel()
	synctest.Test(t, func(t *testing.T) {
		mockClient := mocks.NewMockHTTPClient(t)
		service := createTestService(
			t,
			"zulip://bot@example.com:secret-key@zulip.example.com?stream=general",
			mockClient,
		)

		longMessage := strings.Repeat("a", 10000)

		mockClient.On("Do", mock.AnythingOfType("*http.Request")).
			Return(createMockResponse(http.StatusOK, `{"result": "success"}`), nil).
			Once()

		err := service.Send(longMessage, nil)

		require.NoError(t, err)

		mockClient.AssertExpectations(t)
	})
}

func TestServiceSendWithMessageExceedingLimit(t *testing.T) {
	t.Parallel()
	synctest.Test(t, func(t *testing.T) {
		mockClient := mocks.NewMockHTTPClient(t)
		service := createTestService(
			t,
			"zulip://bot@example.com:secret-key@zulip.example.com?stream=general",
			mockClient,
		)

		longMessage := strings.Repeat("a", 10001)

		err := service.Send(longMessage, nil)

		require.Error(t, err)
		require.ErrorIs(t, err, zulip.ErrMessageTooLong)

		mockClient.AssertNotCalled(t, "Do", mock.AnythingOfType("*http.Request"))
	})
}

func TestServiceSendWithTopicExceedingLimit(t *testing.T) {
	t.Parallel()
	synctest.Test(t, func(t *testing.T) {
		mockClient := mocks.NewMockHTTPClient(t)
		service := createTestService(
			t,
			"zulip://bot@example.com:secret-key@zulip.example.com?stream=general",
			mockClient,
		)

		longTopic := strings.Repeat("a", 61)

		params := createTestParams("topic", longTopic)
		err := service.Send("Test message", params)

		require.Error(t, err)
		require.ErrorIs(t, err, zulip.ErrTopicTooLong)

		mockClient.AssertNotCalled(t, "Do", mock.AnythingOfType("*http.Request"))
	})
}

func TestServiceSendWithTopicAtExactLimit(t *testing.T) {
	t.Parallel()
	synctest.Test(t, func(t *testing.T) {
		mockClient := mocks.NewMockHTTPClient(t)
		service := createTestService(
			t,
			"zulip://bot@example.com:secret-key@zulip.example.com?stream=general",
			mockClient,
		)

		exactTopic := strings.Repeat("a", 60)

		mockClient.On("Do", mock.AnythingOfType("*http.Request")).
			Return(createMockResponse(http.StatusOK, `{"result": "success"}`), nil).
			Once()

		params := createTestParams("topic", exactTopic)
		err := service.Send("Test message", params)

		require.NoError(t, err)

		mockClient.AssertExpectations(t)
	})
}

func TestServiceSendParamsDoNotMutateConfig(t *testing.T) {
	t.Parallel()
	synctest.Test(t, func(t *testing.T) {
		mockClient := mocks.NewMockHTTPClient(t)
		service := createTestService(
			t,
			"zulip://bot@example.com:secret-key@zulip.example.com?stream=general&topic=original",
			mockClient,
		)

		mockClient.On("Do", mock.AnythingOfType("*http.Request")).
			Return(createMockResponse(http.StatusOK, `{"result": "success"}`), nil).
			Once()

		params := createTestParams("topic", "modified")
		err := service.Send("Test message", params)

		require.NoError(t, err)
		assertRequestContains(t, mockClient, "topic=modified")

		mockClient.AssertExpectations(t)
	})
}
