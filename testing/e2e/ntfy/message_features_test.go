package e2e_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/nicholas-fedor/shoutrrr/pkg/types"
)

// TestNtfyMessageFeatures verifies that messages sent to a real ntfy server
// with a click action, attachment URL, actions, delay, icon, several tags, or
// combined params are accepted and, where the server delivers them at once,
// received.
//
//nolint:paralleltest // Sends to a live server one message at a time.
func TestNtfyMessageFeatures(t *testing.T) {
	t.Run("click action", func(t *testing.T) {
		requireServer(t)

		service := initializeService(t, buildServiceURL())

		message := uniqueMessage("E2E Test: Click action message")

		require.NoError(t, service.Send(message, &types.Params{"click": "https://example.com"}))

		verifyMessageReceived(t, service.Config.Topic, message)
	})

	t.Run("attachment URL", func(t *testing.T) {
		requireServer(t)

		service := initializeService(t, buildServiceURL())

		message := uniqueMessage("E2E Test: Attachment message")

		require.NoError(t, service.Send(message, &types.Params{"attach": "https://example.com/image.png"}))

		verifyMessageReceived(t, service.Config.Topic, message)
	})

	t.Run("actions", func(t *testing.T) {
		requireServer(t)

		service := initializeService(t, buildServiceURL())

		message := uniqueMessage("E2E Test: Actions message")

		require.NoError(t, service.Send(message, &types.Params{
			"actions": "action=view,label=Open,url=https://example.com",
		}))

		verifyMessageReceived(t, service.Config.Topic, message)
	})

	t.Run("delay", func(t *testing.T) {
		requireServer(t)

		service := initializeService(t, buildServiceURL())

		// Delayed messages do not appear at once, so only the send is checked.
		require.NoError(t, service.Send("E2E Test: Delayed message", &types.Params{"delay": "10s"}))
	})

	t.Run("email notification", func(t *testing.T) {
		t.Skip("email notifications require server-side configuration")
	})

	t.Run("icon", func(t *testing.T) {
		requireServer(t)

		service := initializeService(t, buildServiceURL())

		message := uniqueMessage("E2E Test: Icon message")

		require.NoError(t, service.Send(message, &types.Params{"icon": "https://example.com/icon.png"}))

		verifyMessageReceived(t, service.Config.Topic, message)
	})

	t.Run("filename", func(t *testing.T) {
		t.Skip("attachments require server-side configuration")
	})

	t.Run("multiple tags", func(t *testing.T) {
		requireServer(t)

		service := initializeService(t, buildServiceURL())

		message := uniqueMessage("E2E Test: Multiple tags message")

		require.NoError(t, service.Send(message, &types.Params{"tags": "warning,skull,fire"}))

		msg := verifyMessageReceived(t, service.Config.Topic, message)
		assert.ElementsMatch(t, []string{"warning", "skull", "fire"}, msg.Tags)
	})

	t.Run("combined params", func(t *testing.T) {
		requireServer(t)

		service := initializeService(t, buildServiceURL())

		message := uniqueMessage("E2E Test: Combined params message")

		require.NoError(t, service.Send(message, &types.Params{
			"title":    "Combined",
			"priority": "5",
			"tags":     "warning,skull",
		}))

		msg := verifyMessageReceived(t, service.Config.Topic, message)
		assert.Equal(t, "Combined", msg.Title)
	})
}
