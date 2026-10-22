package e2e_test

import (
	"os"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
)

// TestNtfyConfig verifies that a service initialized from its URL against a
// real ntfy server reads the topic, host, credentials, TLS setting, and query
// settings from it. It runs serially because a subtest sets the environment.
func TestNtfyConfig(t *testing.T) {
	t.Run("default configuration", func(t *testing.T) {
		requireServer(t)

		service := initializeService(t, buildServiceURL())

		assert.NotEmpty(t, service.Config.Topic)
		assert.NotEmpty(t, service.Config.Host)
	})

	t.Run("credentials from URL", func(t *testing.T) {
		requireServer(t)

		service := initializeService(t, buildServiceURL())

		if username := os.Getenv("SHOUTRRR_NTFY_USERNAME"); username != "" {
			assert.Equal(t, username, service.Config.Username)
		}

		if password := os.Getenv("SHOUTRRR_NTFY_PASSWORD"); password != "" {
			assert.Equal(t, password, service.Config.Password)
		}
	})

	t.Run("DisableTLS when configured", func(t *testing.T) {
		requireServer(t)
		t.Setenv("SHOUTRRR_NTFY_DISABLE_TLS", envValueTrue)

		service := initializeService(t, buildServiceURL())

		assert.True(t, service.Config.DisableTLS)
		assert.True(t, strings.HasPrefix(service.Config.GetAPIURL(), "http://"), service.Config.GetAPIURL())
	})

	t.Run("priority from URL", func(t *testing.T) {
		requireServer(t)

		service := initializeService(t, addQueryParam(buildServiceURL(), "priority", "Max"))

		assert.Equal(t, "Max", service.Config.Priority.String())
	})

	t.Run("markdown enabled from URL", func(t *testing.T) {
		requireServer(t)

		service := initializeService(t, addQueryParam(buildServiceURL(), "markdown", "yes"))

		assert.True(t, service.Config.Markdown)
	})

	t.Run("tags from URL", func(t *testing.T) {
		requireServer(t)

		service := initializeService(t, addQueryParam(buildServiceURL(), "tags", "warning,skull"))

		assert.ElementsMatch(t, []string{"warning", "skull"}, service.Config.Tags)
	})

	t.Run("title from URL", func(t *testing.T) {
		requireServer(t)

		service := initializeService(t, addQueryParam(buildServiceURL(), "title", "MyTitle"))

		assert.Equal(t, "MyTitle", service.Config.Title)
	})
}
