package zulip_test

import (
	"io"
	"net/http"
	"strings"
	"testing"
	"testing/synctest"

	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	"github.com/nicholas-fedor/shoutrrr/pkg/services/chat/zulip/mocks"
)

const contentTypeForm = "application/x-www-form-urlencoded"

func TestAPIURLFormatCompliance(t *testing.T) {
	t.Parallel()
	synctest.Test(t, func(t *testing.T) {
		mockClient := mocks.NewMockHTTPClient(t)
		mockClient.On("Do", mock.AnythingOfType("*http.Request")).
			Return(createMockResponse(http.StatusOK, `{"result": "success"}`), nil).
			Once()

		service := createTestService(
			t,
			"zulip://bot@example.com:secret-key@zulip.example.com?stream=general&topic=announcements",
			mockClient,
		)

		err := service.Send("Test message", nil)

		require.NoError(t, err)
		assertRequestMatches(t, mockClient, func(req *http.Request) bool {
			return req.URL.Scheme == "https" &&
				req.URL.Host == "zulip.example.com" &&
				req.URL.Path == "/api/v1/messages"
		}, "Zulip API URL format")

		mockClient.AssertExpectations(t)
	})
}

func TestAPIPayloadStructureCompliance(t *testing.T) {
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
		assertRequestContains(t, mockClient, "type=stream")
		assertRequestContains(t, mockClient, "to=general")
		assertRequestContains(t, mockClient, "topic=announcements")
		assertRequestContains(t, mockClient, "content=Test+message")

		mockClient.AssertExpectations(t)
	})
}

func TestAPIContentTypeHeaderCompliance(t *testing.T) {
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

		err := service.Send("Test message", nil)

		require.NoError(t, err)
		assertRequestMatches(t, mockClient, func(req *http.Request) bool {
			return req.Header.Get("Content-Type") == contentTypeForm
		}, "Content-Type header")

		mockClient.AssertExpectations(t)
	})
}

func TestAPIMethodCompliance(t *testing.T) {
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

		err := service.Send("Test message", nil)

		require.NoError(t, err)
		assertRequestMatches(t, mockClient, func(req *http.Request) bool {
			return req.Method == http.MethodPost
		}, "POST method")

		mockClient.AssertExpectations(t)
	})
}

func TestAPICredentialInURLCompliance(t *testing.T) {
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

		err := service.Send("Test message", nil)

		require.NoError(t, err)
		assertRequestMatches(t, mockClient, func(req *http.Request) bool {
			return req.URL.User != nil &&
				req.URL.User.Username() == "bot@example.com"
		}, "credentials in URL")

		mockClient.AssertExpectations(t)
	})
}

func TestAPIHTTPSCompliance(t *testing.T) {
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

		err := service.Send("Test message", nil)

		require.NoError(t, err)
		assertRequestMatches(t, mockClient, func(req *http.Request) bool {
			return req.URL.Scheme == "https"
		}, "HTTPS scheme")

		mockClient.AssertExpectations(t)
	})
}

func TestAPIWithoutTopicCompliance(t *testing.T) {
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

		err := service.Send("Test message", nil)

		require.NoError(t, err)
		assertRequestMatches(t, mockClient, func(req *http.Request) bool {
			body, readErr := io.ReadAll(req.Body)
			if readErr != nil {
				return false
			}

			req.Body = io.NopCloser(strings.NewReader(string(body)))

			return !strings.Contains(string(body), "topic=")
		}, "no topic when not configured")

		mockClient.AssertExpectations(t)
	})
}

func TestAPIWithOnlyTopicNoStream(t *testing.T) {
	t.Parallel()
	synctest.Test(t, func(t *testing.T) {
		mockClient := mocks.NewMockHTTPClient(t)
		service := createTestService(
			t,
			"zulip://bot@example.com:secret-key@zulip.example.com?topic=announcements",
			mockClient,
		)

		mockClient.On("Do", mock.AnythingOfType("*http.Request")).
			Return(createMockResponse(http.StatusOK, `{"result": "success"}`), nil).
			Once()

		err := service.Send("Test message", nil)

		require.NoError(t, err)
		assertRequestContains(t, mockClient, "topic=announcements")

		mockClient.AssertExpectations(t)
	})
}
