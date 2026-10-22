package e2e_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/nicholas-fedor/shoutrrr/pkg/types"
)

// TestHomeAssistant verifies that sends to a real Home Assistant instance
// create, title, and replace persistent notifications, work over HTTP, and
// fail with an unauthorized token.
//
//nolint:paralleltest // Sends to a live server one message at a time.
func TestHomeAssistant(t *testing.T) {
	t.Run("creates a persistent notification", func(t *testing.T) {
		requireHomeAssistant(t)

		rawURL := withNid(t, serviceURL(), "shoutrrr_e2e")
		service := initializeService(t, rawURL)

		message := "E2E Test: persistent notification"
		require.NoError(t, service.Send(message, nil))

		state := fetchPersistentNotification(t, accessToken(t, rawURL), "shoutrrr_e2e")
		assert.Equal(t, message, state.Message)
	})

	t.Run("includes a title", func(t *testing.T) {
		requireHomeAssistant(t)

		rawURL := withNid(t, serviceURL(), "shoutrrr_e2e_title")
		service := initializeService(t, rawURL)

		params := types.Params{"title": "E2E Title"}
		require.NoError(t, service.Send("E2E Test: titled notification", &params))

		state := fetchPersistentNotification(t, accessToken(t, rawURL), "shoutrrr_e2e_title")
		assert.Equal(t, "E2E Title", state.Title)
		assert.Equal(t, "E2E Test: titled notification", state.Message)
	})

	t.Run("replaces a persistent notification when nid is reused", func(t *testing.T) {
		requireHomeAssistant(t)

		rawURL := withNid(t, serviceURL(), "shoutrrr_e2e_replace")
		service := initializeService(t, rawURL)

		require.NoError(t, service.Send("first message", nil))
		require.NoError(t, service.Send("second message", nil))

		state := fetchPersistentNotification(t, accessToken(t, rawURL), "shoutrrr_e2e_replace")
		assert.Equal(t, "second message", state.Message)
	})

	t.Run("sends over HTTP when disabletls is set", func(t *testing.T) {
		requireHomeAssistant(t)
		require.Contains(t, serviceURL(), "disabletls")

		rawURL := withNid(t, serviceURL(), "shoutrrr_e2e_http")
		service := initializeService(t, rawURL)

		require.NoError(t, service.Send("E2E Test: HTTP", nil))

		state := fetchPersistentNotification(t, accessToken(t, rawURL), "shoutrrr_e2e_http")
		assert.Equal(t, "E2E Test: HTTP", state.Message)
	})

	t.Run("rejects an unauthorized token", func(t *testing.T) {
		requireHomeAssistant(t)

		service := initializeService(t, "homeassistant://invalid-token@localhost:8123/?disabletls=yes")

		require.Error(t, service.Send("should fail", nil))
	})
}
