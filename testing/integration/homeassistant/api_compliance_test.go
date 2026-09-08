package homeassistant_test

import (
	"net/http"
	"testing"
	"testing/synctest"

	"github.com/stretchr/testify/require"

	"github.com/nicholas-fedor/shoutrrr/internal/meta"
)

func TestAPIURLFormatCompliance(t *testing.T) {
	t.Parallel()
	synctest.Test(t, func(t *testing.T) {
		service, mockClient := createTestServiceWithMock(t, validHomeAssistantURL)

		err := service.Send("Test message", nil)
		require.NoError(t, err)

		req := findMatchingRequest(mockClient, func(req *http.Request) bool {
			return req.Method == http.MethodPost && req.URL.String() == persistentAPIURL
		})
		require.NotNil(t, req)

		mockClient.AssertExpectations(t)
	})
}

func TestContentTypeAuthorizationAndUserAgent(t *testing.T) {
	t.Parallel()
	synctest.Test(t, func(t *testing.T) {
		service, mockClient := createTestServiceWithMock(t, validHomeAssistantURL)

		err := service.Send("Test message", nil)
		require.NoError(t, err)

		req := findMatchingRequest(mockClient, func(req *http.Request) bool {
			return req.Method == http.MethodPost &&
				req.Header.Get("Content-Type") == "application/json" &&
				req.Header.Get("Authorization") == "Bearer s3cret" &&
				req.Header.Get("User-Agent") == "shoutrrr/"+meta.Version
		})
		require.NotNil(t, req)

		mockClient.AssertExpectations(t)
	})
}

func TestNotifyServicePath(t *testing.T) {
	t.Parallel()
	synctest.Test(t, func(t *testing.T) {
		service, mockClient := createTestServiceWithMock(
			t,
			validHomeAssistantURL+"?service=mobile_app_phone",
		)

		err := service.Send("ping", nil)
		require.NoError(t, err)

		req := findMatchingRequest(mockClient, func(req *http.Request) bool {
			return req.URL.String() == "https://ha.example.com:443/api/services/notify/mobile_app_phone"
		})
		require.NotNil(t, req)

		mockClient.AssertExpectations(t)
	})
}

func TestExplicitHTTPSPort(t *testing.T) {
	t.Parallel()
	synctest.Test(t, func(t *testing.T) {
		service, mockClient := createTestServiceWithMock(
			t,
			"homeassistant://s3cret@homeassistant.local:8123",
		)

		err := service.Send("lan", nil)
		require.NoError(t, err)

		req := findMatchingRequest(mockClient, func(req *http.Request) bool {
			return req.URL.String() ==
				"https://homeassistant.local:8123/api/services/persistent_notification/create"
		})
		require.NotNil(t, req)

		mockClient.AssertExpectations(t)
	})
}

func TestReverseProxyPrefix(t *testing.T) {
	t.Parallel()
	synctest.Test(t, func(t *testing.T) {
		service, mockClient := createTestServiceWithMock(
			t,
			"homeassistant://s3cret@ha.example.com/hass/",
		)

		err := service.Send("proxied", nil)
		require.NoError(t, err)

		req := findMatchingRequest(mockClient, func(req *http.Request) bool {
			return req.URL.String() ==
				"https://ha.example.com:443/hass/api/services/persistent_notification/create"
		})
		require.NotNil(t, req)

		mockClient.AssertExpectations(t)
	})
}
