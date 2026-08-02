package ntfy_test

import (
	"strings"
	"testing"
	"testing/synctest"

	"github.com/stretchr/testify/require"
)

func TestSendBasicMessage(t *testing.T) {
	t.Parallel()
	synctest.Test(t, func(t *testing.T) {
		service, mockClient := createTestServiceWithMock(
			t,
			validNtfyURL,
		)

		err := service.Send("Hello from ntfy integration test", nil)
		require.NoError(t, err)

		assertRequestContains(t, mockClient, "Hello from ntfy integration test")
		assertRequestMade(t, mockClient, service.Config.GetAPIURL())

		mockClient.AssertExpectations(t)
	})
}

func TestSendEmptyMessage(t *testing.T) {
	t.Parallel()
	synctest.Test(t, func(t *testing.T) {
		service, mockClient := createTestServiceWithMock(
			t,
			validNtfyURL,
		)

		err := service.Send("", nil)
		require.NoError(t, err)

		mockClient.AssertExpectations(t)
	})
}

func TestSendWithTitle(t *testing.T) {
	t.Parallel()
	synctest.Test(t, func(t *testing.T) {
		service, mockClient := createTestServiceWithMock(
			t,
			validNtfyURL,
		)

		params := createTestParams("title", "Alert")
		err := service.Send("Something happened", params)

		require.NoError(t, err)
		assertRequestHeaderContains(t, mockClient, "Title", "Alert")

		mockClient.AssertExpectations(t)
	})
}

func TestSendWithPriority(t *testing.T) {
	t.Parallel()
	synctest.Test(t, func(t *testing.T) {
		service, mockClient := createTestServiceWithMock(
			t,
			validNtfyURL,
		)

		params := createTestParams("priority", "High")
		err := service.Send("Important message", params)

		require.NoError(t, err)
		assertRequestHeaderContains(t, mockClient, "Priority", "High")

		mockClient.AssertExpectations(t)
	})
}

func TestSendWithTags(t *testing.T) {
	t.Parallel()
	synctest.Test(t, func(t *testing.T) {
		service, mockClient := createTestServiceWithMock(
			t,
			validNtfyURL,
		)

		params := createTestParams("tags", "warning,skull")
		err := service.Send("Tagged message", params)

		require.NoError(t, err)
		assertRequestHeaderContains(t, mockClient, "Tags", "warning,skull")

		mockClient.AssertExpectations(t)
	})
}

func TestSendWithMarkdown(t *testing.T) {
	t.Parallel()
	synctest.Test(t, func(t *testing.T) {
		service, mockClient := createTestServiceWithMock(
			t,
			validNtfyURL+"?markdown=yes",
		)

		err := service.Send("**bold** text", nil)
		require.NoError(t, err)

		mockClient.AssertExpectations(t)
	})
}

func TestSendWithClick(t *testing.T) {
	t.Parallel()
	synctest.Test(t, func(t *testing.T) {
		service, mockClient := createTestServiceWithMock(
			t,
			validNtfyURL,
		)

		params := createTestParams("click", "https://example.com")
		err := service.Send("Click me", params)

		require.NoError(t, err)
		assertRequestHeaderContains(t, mockClient, "Click", "https://example.com")

		mockClient.AssertExpectations(t)
	})
}

func TestSendWithAttach(t *testing.T) {
	t.Parallel()
	synctest.Test(t, func(t *testing.T) {
		service, mockClient := createTestServiceWithMock(
			t,
			validNtfyURL,
		)

		params := createTestParams("attach", "https://example.com/image.png")
		err := service.Send("Check this out", params)

		require.NoError(t, err)
		assertRequestHeaderContains(t, mockClient, "Attach", "https://example.com/image.png")

		mockClient.AssertExpectations(t)
	})
}

func TestSendWithActions(t *testing.T) {
	t.Parallel()
	synctest.Test(t, func(t *testing.T) {
		service, mockClient := createTestServiceWithMock(
			t,
			validNtfyURL,
		)

		params := createTestParams("actions", "view;open")
		err := service.Send("Action message", params)

		require.NoError(t, err)
		assertRequestHeaderContains(t, mockClient, "Actions", "view;open")

		mockClient.AssertExpectations(t)
	})
}

func TestSendWithDelay(t *testing.T) {
	t.Parallel()
	synctest.Test(t, func(t *testing.T) {
		service, mockClient := createTestServiceWithMock(
			t,
			validNtfyURL,
		)

		params := createTestParams("delay", "2h")
		err := service.Send("Delayed message", params)

		require.NoError(t, err)
		assertRequestHeaderContains(t, mockClient, "Delay", "2h")

		mockClient.AssertExpectations(t)
	})
}

func TestSendWithEmail(t *testing.T) {
	t.Parallel()
	synctest.Test(t, func(t *testing.T) {
		service, mockClient := createTestServiceWithMock(
			t,
			validNtfyURL,
		)

		params := createTestParams("email", "user@example.com")
		err := service.Send("Email notification", params)

		require.NoError(t, err)
		assertRequestHeaderContains(t, mockClient, "Email", "user@example.com")

		mockClient.AssertExpectations(t)
	})
}

func TestSendWithCacheDisabled(t *testing.T) {
	t.Parallel()
	synctest.Test(t, func(t *testing.T) {
		service, mockClient := createTestServiceWithMock(
			t,
			validNtfyURL+"?cache=no",
		)

		err := service.Send("No cache", nil)
		require.NoError(t, err)
		assertRequestHeaderContains(t, mockClient, "Cache", "no")

		mockClient.AssertExpectations(t)
	})
}

func TestSendWithFirebaseDisabled(t *testing.T) {
	t.Parallel()
	synctest.Test(t, func(t *testing.T) {
		service, mockClient := createTestServiceWithMock(
			t,
			validNtfyURL+"?firebase=no",
		)

		err := service.Send("No firebase", nil)
		require.NoError(t, err)
		assertRequestHeaderContains(t, mockClient, "Firebase", "no")

		mockClient.AssertExpectations(t)
	})
}

func TestSendWithBasicAuth(t *testing.T) {
	t.Parallel()
	synctest.Test(t, func(t *testing.T) {
		service, mockClient := createTestServiceWithMock(
			t,
			"ntfy://user:pass@ntfy.example.com/mytopic",
		)

		err := service.Send("Authenticated message", nil)
		require.NoError(t, err)
		assertRequestHeaderContains(t, mockClient, "Authorization", "Basic ")

		mockClient.AssertExpectations(t)
	})
}

func TestSendWithUnicodeMessage(t *testing.T) {
	t.Parallel()
	synctest.Test(t, func(t *testing.T) {
		service, mockClient := createTestServiceWithMock(
			t,
			validNtfyURL,
		)

		message := "Hello 世界 🌍"
		err := service.Send(message, nil)
		require.NoError(t, err)

		assertRequestContains(t, mockClient, message)

		mockClient.AssertExpectations(t)
	})
}

func TestSendWithIcon(t *testing.T) {
	t.Parallel()
	synctest.Test(t, func(t *testing.T) {
		service, mockClient := createTestServiceWithMock(
			t,
			validNtfyURL,
		)

		params := createTestParams("icon", "https://example.com/icon.png")
		err := service.Send("With icon", params)

		require.NoError(t, err)
		assertRequestHeaderContains(t, mockClient, "X-Icon", "https://example.com/icon.png")

		mockClient.AssertExpectations(t)
	})
}

func TestSendWithFilename(t *testing.T) {
	t.Parallel()
	synctest.Test(t, func(t *testing.T) {
		service, mockClient := createTestServiceWithMock(
			t,
			validNtfyURL,
		)

		params := createTestParams("filename", "report.pdf")
		err := service.Send("Here is the report", params)

		require.NoError(t, err)
		assertRequestHeaderContains(t, mockClient, "Filename", "report.pdf")

		mockClient.AssertExpectations(t)
	})
}

func TestSendWithMultipleTags(t *testing.T) {
	t.Parallel()
	synctest.Test(t, func(t *testing.T) {
		service, mockClient := createTestServiceWithMock(
			t,
			validNtfyURL,
		)

		params := createTestParams("tags", "warning,skull,fire")
		err := service.Send("Tagged message", params)

		require.NoError(t, err)
		assertRequestHeaderContains(t, mockClient, "Tags", "warning,skull,fire")

		mockClient.AssertExpectations(t)
	})
}

func TestSendWithMultipleActions(t *testing.T) {
	t.Parallel()
	synctest.Test(t, func(t *testing.T) {
		service, mockClient := createTestServiceWithMock(
			t,
			validNtfyURL,
		)

		params := createTestParams("actions", "view;open;reply")
		err := service.Send("Action message", params)

		require.NoError(t, err)
		assertRequestHeaderContains(t, mockClient, "Actions", "view;open;reply")

		mockClient.AssertExpectations(t)
	})
}

func TestSendWithCombinedParams(t *testing.T) {
	t.Parallel()
	synctest.Test(t, func(t *testing.T) {
		service, mockClient := createTestServiceWithMock(
			t,
			validNtfyURL,
		)

		params := createTestParams(
			"title", "Combined",
			"priority", "High",
			"tags", "warning,skull",
		)
		err := service.Send("Combined message", params)

		require.NoError(t, err)
		assertRequestHeaderContains(t, mockClient, "Title", "Combined")
		assertRequestHeaderContains(t, mockClient, "Priority", "High")
		assertRequestHeaderContains(t, mockClient, "Tags", "warning,skull")

		mockClient.AssertExpectations(t)
	})
}

func TestSendWithTitleViaURL(t *testing.T) {
	t.Parallel()
	synctest.Test(t, func(t *testing.T) {
		service, mockClient := createTestServiceWithMock(
			t,
			validNtfyURL+"?title=URLTitle",
		)

		err := service.Send("Message body", nil)
		require.NoError(t, err)
		assertRequestHeaderContains(t, mockClient, "Title", "URLTitle")

		mockClient.AssertExpectations(t)
	})
}

func TestSendWithPriorityViaURL(t *testing.T) {
	t.Parallel()
	synctest.Test(t, func(t *testing.T) {
		service, mockClient := createTestServiceWithMock(
			t,
			validNtfyURL+"?priority=Max",
		)

		err := service.Send("Important", nil)
		require.NoError(t, err)
		assertRequestHeaderContains(t, mockClient, "Priority", "Max")

		mockClient.AssertExpectations(t)
	})
}

func TestSendWithSpecialCharacters(t *testing.T) {
	t.Parallel()
	synctest.Test(t, func(t *testing.T) {
		service, mockClient := createTestServiceWithMock(
			t,
			validNtfyURL,
		)

		message := "Special: <>@#$%^&*()"
		err := service.Send(message, nil)
		require.NoError(t, err)

		assertRequestContains(t, mockClient, message)

		mockClient.AssertExpectations(t)
	})
}

func TestSendWithNewlines(t *testing.T) {
	t.Parallel()
	synctest.Test(t, func(t *testing.T) {
		service, mockClient := createTestServiceWithMock(
			t,
			validNtfyURL,
		)

		err := service.Send("Line 1\nLine 2\nLine 3", nil)
		require.NoError(t, err)
		assertRequestContains(t, mockClient, "Line 1\nLine 2\nLine 3")

		mockClient.AssertExpectations(t)
	})
}

func TestSendWithLongMessage(t *testing.T) {
	t.Parallel()
	synctest.Test(t, func(t *testing.T) {
		service, mockClient := createTestServiceWithMock(
			t,
			validNtfyURL,
		)

		longMessage := strings.Repeat("a", 10000)
		err := service.Send(longMessage, nil)
		require.NoError(t, err)
		assertRequestContains(t, mockClient, longMessage)

		mockClient.AssertExpectations(t)
	})
}

func TestSendWithDisableTLSVerification(t *testing.T) {
	t.Parallel()
	synctest.Test(t, func(t *testing.T) {
		service, mockClient := createTestServiceWithMock(
			t,
			"ntfy://ntfy.example.com/mytopic?disabletlsverification=yes",
		)

		err := service.Send("test", nil)
		require.NoError(t, err)

		mockClient.AssertExpectations(t)
	})
}
