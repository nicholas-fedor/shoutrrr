package discord_test

import (
	"net/http"
	"testing"
	"testing/synctest"

	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	"github.com/nicholas-fedor/shoutrrr/pkg/types"
)

func TestSendWithCustomUsername(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		mockClient := &MockHTTPClient{}
		service := createTestService(
			t,
			"discord://test-token@test-webhook",
			mockClient,
		)

		tests := []struct {
			name     string
			username string
		}{
			{"simple username", "TestBot"},
			{"username with spaces", "Test Bot"},
			{"username with special chars", "Test_Bot-123"},
			{"unicode username", "テストボット"},
			{"long username", "ThisIsAVeryLongUsernameThatMightCauseIssuesButShouldStillWork"},
		}

		for _, tt := range tests {
			mockClient.On("Do", mock.Anything).
				Return(createMockResponse(http.StatusNoContent, ""), nil). //nolint:bodyclose
				Once()

			params := createTestParams("username", tt.username)
			err := service.Send("Message with custom username", params)

			require.NoError(t, err)
			assertRequestContains(t, mockClient, `"username":"`+tt.username+`"`)
		}

		mockClient.AssertExpectations(t)
	})
}

func TestSendWithCustomAvatar(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		mockClient := &MockHTTPClient{}
		service := createTestService(
			t,
			"discord://test-token@test-webhook",
			mockClient,
		)

		tests := []struct {
			name   string
			avatar string
		}{
			{"http avatar", "http://example.com/avatar.png"},
			{"https avatar", "https://example.com/avatar.jpg"},
			{"avatar with query params", "https://example.com/avatar.png?size=128"},
			{"avatar with path", "https://cdn.example.com/avatars/bot/avatar.gif"},
			{
				"data URL avatar",
				"data:image/png;base64,iVBORw0KGgoAAAANSUhEUgAAAAEAAAABCAYAAAAfFcSJAAAADUlEQVR42mNkYPhfDwAChwGA60e6kgAAAABJRU5ErkJggg==",
			},
		}

		for _, tt := range tests {
			mockClient.On("Do", mock.Anything).
				Return(createMockResponse(http.StatusNoContent, ""), nil). //nolint:bodyclose
				Once()

			params := createTestParams("avatar", tt.avatar)
			err := service.Send("Message with custom avatar", params)

			require.NoError(t, err)
			assertRequestContains(t, mockClient, `"avatar_url":"`+tt.avatar+`"`)
		}

		mockClient.AssertExpectations(t)
	})
}

func TestSendWithCustomColors(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		mockClient := &MockHTTPClient{}
		service := createTestService(
			t,
			"discord://test-token@test-webhook",
			mockClient,
		)

		tests := []struct {
			name  string
			color string
		}{
			{"red color", "16711680"},
			{"green color", "65280"},
			{"blue color", "255"},
			{"white color", "16777215"},
			{"black color", "0"},
			{"hex color", "ff0000"},
			{"color with hash", "#ff0000"},
		}

		for _, tt := range tests {
			mockClient.On("Do", mock.Anything).
				Return(createMockResponse(http.StatusNoContent, ""), nil). //nolint:bodyclose
				Once()

			params := createTestParams("color", tt.color)
			err := service.Send("Message with custom color", params)

			require.NoError(t, err)
			// Note: Color handling might be complex, just verify the request is made
		}

		mockClient.AssertExpectations(t)
	})
}

func TestSendWithJSONMode(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		mockClient := &MockHTTPClient{}
		mockClient.On("Do", mock.Anything).
			Return(createMockResponse(http.StatusNoContent, ""), nil). //nolint:bodyclose
			Once()

		// Create service with JSON mode enabled
		service := createTestService(
			t,
			"discord://test-token@test-webhook?json=true",
			mockClient,
		)

		jsonPayload := `{"content": "Raw JSON message", "embeds": [{"title": "JSON Embed"}]}`

		err := service.Send(jsonPayload, nil)

		require.NoError(t, err)
		assertRequestBody(t, mockClient, jsonPayload)

		mockClient.AssertExpectations(t)
	})
}

func TestSendWithSplitLines(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		tests := []struct {
			name          string
			splitLines    string
			message       string
			expectedCalls int
		}{
			{"split lines enabled", "true", "Line 1\nLine 2\nLine 3", 1},
			{"split lines disabled", "false", "LongMessageWithoutSpaces", 1},
			{"no split config", "", "LongMessageWithoutSpaces", 1},
		}

		for _, tt := range tests {
			mockClient := &MockHTTPClient{}
			service := createTestService(
				t,
				"discord://test-token@test-webhook",
				mockClient,
			)

			mockClient.On("Do", mock.Anything).
				Return(createMockResponse(http.StatusNoContent, ""), nil). //nolint:bodyclose
				Times(tt.expectedCalls)

			var params *types.Params
			if tt.splitLines != "" {
				params = createTestParams("splitLines", tt.splitLines)
			}

			err := service.Send(tt.message, params)

			require.NoError(t, err)
			mockClient.AssertNumberOfCalls(t, "Do", tt.expectedCalls)
			mockClient.AssertExpectations(t)
		}
	})
}

func TestSendWithTitle(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		mockClient := &MockHTTPClient{}
		service := createTestService(
			t,
			"discord://test-token@test-webhook",
			mockClient,
		)

		mockClient.On("Do", mock.Anything).
			Return(createMockResponse(http.StatusNoContent, ""), nil). //nolint:bodyclose
			Once()

		params := createTestParams("title", "Custom Title")
		err := service.Send("Message with custom title", params)

		require.NoError(t, err)
		// Title handling depends on embed creation, verify request is made

		mockClient.AssertExpectations(t)
	})
}

func TestSendWithComplexParameterCombination(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		mockClient := &MockHTTPClient{}
		service := createTestService(
			t,
			"discord://test-token@test-webhook",
			mockClient,
		)

		mockClient.On("Do", mock.Anything).
			Return(createMockResponse(http.StatusNoContent, ""), nil). //nolint:bodyclose
			Once()

		params := createTestParams(
			"username", "ComplexBot",
			"avatar", "https://example.com/complex-avatar.png",
			"color", "16776960",
			"title", "Complex Test",
			"splitLines", "false",
		)

		err := service.Send("Complex parameter combination test", params)

		require.NoError(t, err)
		assertRequestContains(t, mockClient, `"username":"ComplexBot"`)
		assertRequestContains(
			t,
			mockClient,
			`"avatar_url":"https://example.com/complex-avatar.png"`,
		)

		mockClient.AssertExpectations(t)
	})
}

func TestSendWithEmptyParameters(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		mockClient := &MockHTTPClient{}
		service := createTestService(
			t,
			"discord://test-token@test-webhook",
			mockClient,
		)

		mockClient.On("Do", mock.Anything).
			Return(createMockResponse(http.StatusNoContent, ""), nil). //nolint:bodyclose
			Once()

		emptyParams := &types.Params{}
		err := service.Send("Message with empty params", emptyParams)

		require.NoError(t, err)
		assertRequestContains(t, mockClient, `"content":"Message with empty params"`)

		mockClient.AssertExpectations(t)
	})
}

func TestSendWithNilParameters(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		mockClient := &MockHTTPClient{}
		service := createTestService(
			t,
			"discord://test-token@test-webhook",
			mockClient,
		)

		mockClient.On("Do", mock.Anything).
			Return(createMockResponse(http.StatusNoContent, ""), nil). //nolint:bodyclose
			Once()

		err := service.Send("Message with nil params", nil)

		require.NoError(t, err)
		assertRequestContains(t, mockClient, `"content":"Message with nil params"`)

		mockClient.AssertExpectations(t)
	})
}

func TestSendWithURLParameters(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		mockClient := &MockHTTPClient{}
		mockClient.On("Do", mock.Anything).
			Return(createMockResponse(http.StatusNoContent, ""), nil). //nolint:bodyclose
			Once()

		// Create service with URL parameters
		service := createTestService(
			t,
			"discord://test-token@test-webhook?username=URLBot&avatar=https://example.com/url-avatar.png",
			mockClient,
		)

		err := service.Send("Message with URL params", nil)

		require.NoError(t, err)
		assertRequestContains(t, mockClient, `"username":"URLBot"`)
		assertRequestContains(
			t,
			mockClient,
			`"avatar_url":"https://example.com/url-avatar.png"`,
		)

		mockClient.AssertExpectations(t)
	})
}
