package ntfy_test

import (
	"net/url"
	"testing"
	"testing/synctest"

	"github.com/stretchr/testify/require"

	"github.com/nicholas-fedor/shoutrrr/pkg/services/push/ntfy"
)

const validNtfyURL = "ntfy://ntfy.example.com/mytopic"

func TestServiceInitialization(t *testing.T) {
	t.Parallel()
	synctest.Test(t, func(t *testing.T) {
		mockClient := &MockHTTPClient{}
		service := createTestService(
			t,
			validNtfyURL,
			mockClient,
		)

		require.NotNil(t, service)
		require.Equal(t, "ntfy", service.GetID())

		mockClient.AssertExpectations(t)
	})
}

func TestServiceInitializationWithCredentials(t *testing.T) {
	t.Parallel()
	synctest.Test(t, func(t *testing.T) {
		mockClient := &MockHTTPClient{}
		serviceURL := "ntfy://user:pass@ntfy.example.com/mytopic"
		service := createTestService(
			t,
			serviceURL,
			mockClient,
		)

		require.NotNil(t, service)
		require.Equal(t, "ntfy", service.GetID())
		require.Equal(t, "user", service.Config.Username)
		require.Equal(t, "pass", service.Config.Password)

		mockClient.AssertExpectations(t)
	})
}

func TestServiceInitializationWithTopicPath(t *testing.T) {
	t.Parallel()
	synctest.Test(t, func(t *testing.T) {
		mockClient := &MockHTTPClient{}
		serviceURL := "ntfy://ntfy.example.com/alerts/critical"
		service := createTestService(
			t,
			serviceURL,
			mockClient,
		)

		require.NotNil(t, service)
		require.Equal(t, "alerts/critical", service.Config.Topic)

		mockClient.AssertExpectations(t)
	})
}

func TestServiceInitializationWithDisableTLS(t *testing.T) {
	t.Parallel()
	synctest.Test(t, func(t *testing.T) {
		mockClient := &MockHTTPClient{}
		serviceURL := "ntfy://ntfy.example.com/mytopic?disabletls=yes"
		service := createTestService(
			t,
			serviceURL,
			mockClient,
		)

		require.NotNil(t, service)
		require.Equal(t, "http", service.Config.Scheme)

		mockClient.AssertExpectations(t)
	})
}

func TestServiceInitializationWithDisableTLSVerification(t *testing.T) {
	t.Parallel()
	synctest.Test(t, func(t *testing.T) {
		mockClient := &MockHTTPClient{}
		serviceURL := "ntfy://ntfy.example.com/mytopic?disabletlsverification=yes"
		service := createTestService(
			t,
			serviceURL,
			mockClient,
		)

		require.NotNil(t, service)
		require.True(t, service.Config.DisableTLSVerification)

		mockClient.AssertExpectations(t)
	})
}

func TestServiceInitializationWithPriority(t *testing.T) {
	t.Parallel()
	synctest.Test(t, func(t *testing.T) {
		mockClient := &MockHTTPClient{}
		serviceURL := "ntfy://ntfy.example.com/mytopic?priority=Max"
		service := createTestService(
			t,
			serviceURL,
			mockClient,
		)

		require.NotNil(t, service)
		require.Equal(t, ntfy.PriorityMax, service.Config.Priority)

		mockClient.AssertExpectations(t)
	})
}

func TestServiceInitializationWithMarkdown(t *testing.T) {
	t.Parallel()
	synctest.Test(t, func(t *testing.T) {
		mockClient := &MockHTTPClient{}
		serviceURL := "ntfy://ntfy.example.com/mytopic?markdown=yes"
		service := createTestService(
			t,
			serviceURL,
			mockClient,
		)

		require.NotNil(t, service)
		require.True(t, service.Config.Markdown)

		mockClient.AssertExpectations(t)
	})
}

func TestServiceInitializationWithTags(t *testing.T) {
	t.Parallel()
	synctest.Test(t, func(t *testing.T) {
		mockClient := &MockHTTPClient{}
		serviceURL := "ntfy://ntfy.example.com/mytopic?tags=warning,skull"
		service := createTestService(
			t,
			serviceURL,
			mockClient,
		)

		require.NotNil(t, service)
		require.Equal(t, []string{"warning", "skull"}, service.Config.Tags)

		mockClient.AssertExpectations(t)
	})
}

func TestServiceInitializationWithTitle(t *testing.T) {
	t.Parallel()
	synctest.Test(t, func(t *testing.T) {
		mockClient := &MockHTTPClient{}
		serviceURL := "ntfy://ntfy.example.com/mytopic?title=Alert"
		service := createTestService(
			t,
			serviceURL,
			mockClient,
		)

		require.NotNil(t, service)
		require.Equal(t, "Alert", service.Config.Title)

		mockClient.AssertExpectations(t)
	})
}

func TestServiceInitializationMissingTopic(t *testing.T) {
	t.Parallel()
	synctest.Test(t, func(t *testing.T) {
		service := &ntfy.Service{}
		serviceURL := mustParseURL(t, "ntfy://ntfy.example.com/")

		err := service.Initialize(serviceURL, &mockLogger{})
		require.Error(t, err)
		require.ErrorIs(t, err, ntfy.ErrTopicRequired)
	})
}

func TestServiceInitializationWithUsernameOnly(t *testing.T) {
	t.Parallel()
	synctest.Test(t, func(t *testing.T) {
		mockClient := &MockHTTPClient{}
		serviceURL := "ntfy://myuser@ntfy.example.com/mytopic"
		service := createTestService(
			t,
			serviceURL,
			mockClient,
		)

		require.NotNil(t, service)
		require.Equal(t, "myuser", service.Config.Username)
		require.Empty(t, service.Config.Password)

		mockClient.AssertExpectations(t)
	})
}

func TestConfigDefaults(t *testing.T) {
	t.Parallel()
	synctest.Test(t, func(t *testing.T) {
		service := &ntfy.Service{}

		parsedURL, err := url.Parse("ntfy://ntfy.example.com/mytopic")
		require.NoError(t, err)

		err = service.Initialize(parsedURL, &mockLogger{})
		require.NoError(t, err)

		require.Equal(t, "https", service.Config.Scheme)
		require.Equal(t, "ntfy.example.com", service.Config.Host)
		require.Equal(t, "mytopic", service.Config.Topic)
		require.Empty(t, service.Config.Username)
		require.Empty(t, service.Config.Password)
		require.Empty(t, service.Config.Title)
		require.Equal(t, "Default", service.Config.Priority.String())
		require.False(t, service.Config.Markdown)
		require.True(t, service.Config.Cache)
		require.True(t, service.Config.Firebase)
		require.False(t, service.Config.DisableTLSVerification)
		require.False(t, service.Config.DisableTLS)
	})
}

func TestConfigURLRoundTrip(t *testing.T) {
	t.Parallel()
	synctest.Test(t, func(t *testing.T) {
		mockClient := &MockHTTPClient{}
		service := createTestService(
			t,
			"ntfy://user:pass@ntfy.example.com/mytopic?priority=High&tags=warning,skull&markdown=yes",
			mockClient,
		)

		require.NotNil(t, service)

		reconstructed := service.Config.GetURL()
		require.Equal(t, "user", reconstructed.User.Username())
		password, _ := reconstructed.User.Password()
		require.Equal(t, "pass", password)
		require.Equal(t, "ntfy.example.com", reconstructed.Host)
		require.Equal(t, "/mytopic", reconstructed.Path)
		require.Equal(t, "High", reconstructed.Query().Get("priority"))
		require.Equal(t, "warning,skull", reconstructed.Query().Get("tags"))
		require.Equal(t, "Yes", reconstructed.Query().Get("markdown"))

		mockClient.AssertExpectations(t)
	})
}

func mustParseURL(t *testing.T, raw string) *url.URL {
	t.Helper()

	parsed, err := url.Parse(raw)
	require.NoError(t, err)

	return parsed
}
