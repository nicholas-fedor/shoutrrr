package zulip_test

import (
	"net/url"
	"testing"
	"testing/synctest"

	"github.com/stretchr/testify/require"

	"github.com/nicholas-fedor/shoutrrr/pkg/services/chat/zulip"
	"github.com/nicholas-fedor/shoutrrr/pkg/services/chat/zulip/mocks"
)

func TestServiceInitializeWithValidURL(t *testing.T) {
	t.Parallel()
	synctest.Test(t, func(t *testing.T) {
		mockClient := mocks.NewMockHTTPClient(t)
		service := createTestService(
			t,
			"zulip://bot@example.com:secret-key@zulip.example.com?stream=general&topic=announcements",
			mockClient,
		)

		require.NotNil(t, service)
		require.Equal(t, "zulip", service.GetID())

		mockClient.AssertExpectations(t)
	})
}

func TestServiceInitializeWithMinimalURL(t *testing.T) {
	t.Parallel()
	synctest.Test(t, func(t *testing.T) {
		mockClient := mocks.NewMockHTTPClient(t)
		service := createTestService(
			t,
			"zulip://bot@example.com:secret-key@zulip.example.com",
			mockClient,
		)

		require.NotNil(t, service)
		require.Equal(t, "zulip", service.GetID())

		mockClient.AssertExpectations(t)
	})
}

func TestServiceInitializeWithMissingBotMail(t *testing.T) {
	t.Parallel()
	synctest.Test(t, func(t *testing.T) {
		parsedURL, err := url.Parse("zulip://:secret-key@zulip.example.com")
		require.NoError(t, err)

		service := &zulip.Service{}
		initErr := service.Initialize(parsedURL, &mockLogger{})

		require.Error(t, initErr)
		require.ErrorIs(t, initErr, zulip.ErrMissingBotMail)
	})
}

func TestServiceInitializeWithMissingAPIKey(t *testing.T) {
	t.Parallel()
	synctest.Test(t, func(t *testing.T) {
		parsedURL, err := url.Parse("zulip://bot@example.com@zulip.example.com")
		require.NoError(t, err)

		service := &zulip.Service{}
		initErr := service.Initialize(parsedURL, &mockLogger{})

		require.Error(t, initErr)
		require.ErrorIs(t, initErr, zulip.ErrMissingAPIKey)
	})
}

func TestServiceInitializeWithMissingHost(t *testing.T) {
	t.Parallel()
	synctest.Test(t, func(t *testing.T) {
		parsedURL, err := url.Parse("zulip://bot@example.com:secret-key@")
		require.NoError(t, err)

		service := &zulip.Service{}
		initErr := service.Initialize(parsedURL, &mockLogger{})

		require.Error(t, initErr)
		require.ErrorIs(t, initErr, zulip.ErrMissingHost)
	})
}

func TestServiceInitializeStreamOnly(t *testing.T) {
	t.Parallel()
	synctest.Test(t, func(t *testing.T) {
		mockClient := mocks.NewMockHTTPClient(t)
		service := createTestService(
			t,
			"zulip://bot@example.com:secret-key@zulip.example.com?stream=general",
			mockClient,
		)

		require.NotNil(t, service)

		mockClient.AssertExpectations(t)
	})
}

func TestServiceInitializeTopicOnly(t *testing.T) {
	t.Parallel()
	synctest.Test(t, func(t *testing.T) {
		mockClient := mocks.NewMockHTTPClient(t)
		service := createTestService(
			t,
			"zulip://bot@example.com:secret-key@zulip.example.com?topic=announcements",
			mockClient,
		)

		require.NotNil(t, service)

		mockClient.AssertExpectations(t)
	})
}

func TestServiceInitializeWithSpecialCharsInTopic(t *testing.T) {
	t.Parallel()
	synctest.Test(t, func(t *testing.T) {
		mockClient := mocks.NewMockHTTPClient(t)
		service := createTestService(
			t,
			"zulip://bot@example.com:secret-key@zulip.example.com?topic=hello+world",
			mockClient,
		)

		require.NotNil(t, service)

		mockClient.AssertExpectations(t)
	})
}

func TestServiceInitializeWithPort(t *testing.T) {
	t.Parallel()
	synctest.Test(t, func(t *testing.T) {
		mockClient := mocks.NewMockHTTPClient(t)
		service := createTestService(
			t,
			"zulip://bot@example.com:secret-key@zulip.example.com:443",
			mockClient,
		)

		require.NotNil(t, service)

		mockClient.AssertExpectations(t)
	})
}

func TestServiceGetID(t *testing.T) {
	t.Parallel()
	synctest.Test(t, func(t *testing.T) {
		mockClient := mocks.NewMockHTTPClient(t)
		service := createTestService(
			t,
			"zulip://bot@example.com:secret-key@zulip.example.com",
			mockClient,
		)

		require.Equal(t, "zulip", service.GetID())

		mockClient.AssertExpectations(t)
	})
}
