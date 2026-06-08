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

		msgResp := createMockResponse(http.StatusOK, `{"result": "success"}`)
		setupRegisterThenMessage(t, mockClient, msgResp)

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

		msgResp := createMockResponse(http.StatusOK, `{"result": "success"}`)
		setupRegisterThenMessage(t, mockClient, msgResp)

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

		msgResp := createMockResponse(http.StatusOK, `{"result": "success"}`)
		setupRegisterThenMessage(t, mockClient, msgResp)

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

		msgResp := createMockResponse(http.StatusOK, `{"result": "success"}`)
		setupRegisterThenMessage(t, mockClient, msgResp)

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

		msgResp := createMockResponse(http.StatusOK, `{"result": "success"}`)
		setupRegisterThenMessage(t, mockClient, msgResp)

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

		msgResp := createMockResponse(http.StatusOK, `{"result": "success"}`)
		setupRegisterThenMessage(t, mockClient, msgResp)

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

		// register succeeds (once), messages call gets the cancel (the ctx is for the send)
		mockClient.On("Do", mock.AnythingOfType("*http.Request")).
			Return(createMockResponse(http.StatusOK, `{"result": "success"}`), nil).
			Once()

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

		msgResp := createMockResponse(http.StatusOK, `{"result": "success"}`)
		setupRegisterThenMessage(t, mockClient, msgResp)

		params := createTestParams("topic", "Alert")
		err := service.Send("Something happened", params)

		require.NoError(t, err)
		assertRequestContains(t, mockClient, "topic=Alert")

		mockClient.AssertExpectations(t)
	})
}

func TestServiceSendTitleAsTopicFallback(t *testing.T) {
	t.Parallel()
	synctest.Test(t, func(t *testing.T) {
		mockClient := mocks.NewMockHTTPClient(t)
		service := createTestService(
			t,
			"zulip://bot@example.com:secret-key@zulip.example.com?stream=general",
			mockClient,
		)

		msgResp := createMockResponse(http.StatusOK, `{"result": "success"}`)
		setupRegisterThenMessage(t, mockClient, msgResp)

		params := createTestParams("title", "Fallback Topic")
		err := service.Send("body content", params)

		require.NoError(t, err)
		assertRequestContains(t, mockClient, "topic=Fallback+Topic")

		mockClient.AssertExpectations(t)
	})
}

func TestServiceSendTitleDoesNotOverrideExplicitTopic(t *testing.T) {
	t.Parallel()
	synctest.Test(t, func(t *testing.T) {
		mockClient := mocks.NewMockHTTPClient(t)
		service := createTestService(
			t,
			"zulip://bot@example.com:secret-key@zulip.example.com?stream=general&topic=explicit",
			mockClient,
		)

		msgResp := createMockResponse(http.StatusOK, `{"result": "success"}`)
		setupRegisterThenMessage(t, mockClient, msgResp)

		params := createTestParams("title", "Notification Title")
		err := service.Send("body content", params)

		require.NoError(t, err)
		assertRequestContains(t, mockClient, "topic=explicit")
		assertRequestContains(t, mockClient, "content=Notification+Title")

		mockClient.AssertExpectations(t)
	})
}

func TestServiceSendTitleExceedingTopicLimit(t *testing.T) {
	t.Parallel()
	synctest.Test(t, func(t *testing.T) {
		mockClient := mocks.NewMockHTTPClient(t)
		service := createTestService(
			t,
			"zulip://bot@example.com:secret-key@zulip.example.com?stream=general",
			mockClient,
		)

		longTitle := strings.Repeat("a", 61)
		params := createTestParams("title", longTitle)

		// register happens; title-as-topic length check fails before messages Do
		mockClient.On("Do", mock.AnythingOfType("*http.Request")).
			Return(createMockResponse(http.StatusOK, `{"result": "success"}`), nil).
			Once()

		err := service.Send("body content", params)

		require.Error(t, err)
		require.ErrorIs(t, err, zulip.ErrTopicTooLong)

		assertNoMessagesAPICall(t, mockClient)
	})
}

func TestServiceSendWithBothTopicAndTitle(t *testing.T) {
	t.Parallel()
	synctest.Test(t, func(t *testing.T) {
		mockClient := mocks.NewMockHTTPClient(t)
		service := createTestService(
			t,
			"zulip://bot@example.com:secret-key@zulip.example.com?stream=general",
			mockClient,
		)

		msgResp := createMockResponse(http.StatusOK, `{"result": "success"}`)
		setupRegisterThenMessage(t, mockClient, msgResp)

		params := createTestParams("topic", "my-topic", "title", "my-title")
		err := service.Send("body content", params)

		require.NoError(t, err)
		assertRequestContains(t, mockClient, "topic=my-topic")
		assertRequestContains(t, mockClient, "content=my-title%0A%0Abody+content")

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

		msgResp := createMockResponse(http.StatusOK, `{"result": "success"}`)
		setupRegisterThenMessage(t, mockClient, msgResp)

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

		msgResp := createMockResponse(http.StatusOK, `{"result": "success"}`)
		setupRegisterThenMessage(t, mockClient, msgResp)

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

		msgResp := createMockResponse(http.StatusOK, `{"result": "success"}`)
		setupRegisterThenMessage(t, mockClient, msgResp)

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

		// only register (fetch) happens; size error before messages Do
		mockClient.On("Do", mock.AnythingOfType("*http.Request")).
			Return(createMockResponse(http.StatusOK, `{"result": "success"}`), nil).
			Once()

		err := service.Send(longMessage, nil)

		require.Error(t, err)
		require.ErrorIs(t, err, zulip.ErrMessageTooLong)

		assertNoMessagesAPICall(t, mockClient)
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

		msgResp := createMockResponse(http.StatusOK, `{"result": "success"}`)
		setupRegisterThenMessage(t, mockClient, msgResp)

		params := createTestParams("topic", "modified")
		err := service.Send("Test message", params)

		require.NoError(t, err)
		require.Equal(t, "original", service.Config.Topic)
		assertRequestContains(t, mockClient, "topic=modified")

		mockClient.AssertExpectations(t)
	})
}

// --- New tests for direct message support and server-side limit fetching ---

func TestServiceSendDirectMessage(t *testing.T) {
	t.Parallel()
	synctest.Test(t, func(t *testing.T) {
		mockClient := mocks.NewMockHTTPClient(t)
		service := createTestService(
			t,
			"zulip://bot@example.com:secret-key@zulip.example.com",
			mockClient,
		)

		msgResp := createMockResponse(http.StatusOK, `{"result": "success"}`)
		setupRegisterThenMessage(t, mockClient, msgResp)

		params := createTestParams("type", "direct", "to", "user1@example.com,user2@example.com")
		err := service.Send("Direct hello", params)

		require.NoError(t, err)
		assertRequestContains(t, mockClient, "type=direct")
		assertRequestContains(t, mockClient, `to=%5B%22user1%40example.com%22%2C%22user2%40example.com%22%5D`)

		mockClient.AssertExpectations(t)
	})
}

func TestServiceSendDirectWithReadBySender(t *testing.T) {
	t.Parallel()
	synctest.Test(t, func(t *testing.T) {
		mockClient := mocks.NewMockHTTPClient(t)
		service := createTestService(
			t,
			"zulip://bot@example.com:secret-key@zulip.example.com",
			mockClient,
		)

		msgResp := createMockResponse(http.StatusOK, `{"result": "success"}`)
		setupRegisterThenMessage(t, mockClient, msgResp)

		params := createTestParams("type", "direct", "to", "dm@example.com", "read_by_sender", "true")
		err := service.Send("DM with read", params)

		require.NoError(t, err)
		assertRequestContains(t, mockClient, "read_by_sender=true")

		mockClient.AssertExpectations(t)
	})
}

func TestServiceSendInvalidMessageType(t *testing.T) {
	t.Parallel()
	synctest.Test(t, func(t *testing.T) {
		mockClient := mocks.NewMockHTTPClient(t)
		service := createTestService(
			t,
			"zulip://bot@example.com:secret-key@zulip.example.com?stream=general",
			mockClient,
		)

		// type validation happens before fetch, so no Do calls at all
		err := service.Send("bad type", createTestParams("type", "foo"))

		require.Error(t, err)
		require.ErrorIs(t, err, zulip.ErrInvalidMessageType)

		mockClient.AssertNotCalled(t, "Do", mock.AnythingOfType("*http.Request"))
	})
}

func TestServiceSendMissingRecipientChannel(t *testing.T) {
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
			Once() // register only

		err := service.Send("no recipient", nil)

		require.Error(t, err)
		require.ErrorIs(t, err, zulip.ErrMissingRecipient)

		assertNoMessagesAPICall(t, mockClient)
	})
}

func TestServiceSendMissingRecipientDirect(t *testing.T) {
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
			Once() // register only

		err := service.Send("dm no to", createTestParams("type", "direct"))

		require.Error(t, err)
		require.ErrorIs(t, err, zulip.ErrMissingRecipient)

		assertNoMessagesAPICall(t, mockClient)
	})
}

func TestServiceSendUsesCustomServerLimits(t *testing.T) {
	t.Parallel()
	synctest.Test(t, func(t *testing.T) {
		mockClient := mocks.NewMockHTTPClient(t)
		service := createTestService(
			t,
			"zulip://bot@example.com:secret-key@zulip.example.com?stream=general",
			mockClient,
		)

		// first call: register returns custom limits (larger)
		registerResp := createMockResponse(http.StatusOK, `{"max_message_length": 20000, "max_topic_length": 120}`)
		mockClient.On("Do", mock.AnythingOfType("*http.Request")).
			Return(registerResp, nil).
			Once()

		// second: messages
		msgResp := createMockResponse(http.StatusOK, `{"result": "success"}`)
		mockClient.On("Do", mock.AnythingOfType("*http.Request")).
			Return(msgResp, nil).
			Once()

		// 15000 byte message should be allowed with custom 20000 limit
		longMsg := strings.Repeat("a", 15000)
		err := service.Send(longMsg, nil)

		require.NoError(t, err)

		mockClient.AssertExpectations(t)
	})
}

func TestServiceSendFallsBackToDefaultLimitsOnRegisterFailure(t *testing.T) {
	t.Parallel()
	synctest.Test(t, func(t *testing.T) {
		mockClient := mocks.NewMockHTTPClient(t)
		service := createTestService(
			t,
			"zulip://bot@example.com:secret-key@zulip.example.com?stream=general",
			mockClient,
		)

		// register fails -> fallback to defaults (10000)
		mockClient.On("Do", mock.AnythingOfType("*http.Request")).
			Return(createMockResponse(http.StatusServiceUnavailable, ""), nil).
			Once()

		// the send should still happen (using default limit)
		msgResp := createMockResponse(http.StatusOK, `{"result": "success"}`)
		mockClient.On("Do", mock.AnythingOfType("*http.Request")).
			Return(msgResp, nil).
			Once()

		err := service.Send("short after fallback", nil)

		require.NoError(t, err)

		mockClient.AssertExpectations(t)
	})
}
