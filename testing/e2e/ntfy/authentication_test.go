package e2e_test

import (
	"os"
	"testing"

	"github.com/stretchr/testify/require"
)

// TestNtfyAuthentication verifies that messages sent with basic or
// username-only authentication reach a real ntfy server, when the matching
// credentials are configured.
//
//nolint:paralleltest // Sends to a live server one message at a time.
func TestNtfyAuthentication(t *testing.T) {
	t.Run("basic authentication", func(t *testing.T) {
		requireServer(t)

		if os.Getenv("SHOUTRRR_NTFY_USERNAME") == "" || os.Getenv("SHOUTRRR_NTFY_PASSWORD") == "" {
			t.Skip("ntfy credentials not configured, skipping auth test")
		}

		service := initializeService(t, buildServiceURL())

		message := uniqueMessage("E2E Test: Authenticated message")

		require.NoError(t, service.Send(message, nil))

		verifyMessageReceived(t, service.Config.Topic, message)
	})

	t.Run("username-only authentication", func(t *testing.T) {
		requireServer(t)

		if os.Getenv("SHOUTRRR_NTFY_USERNAME") == "" || os.Getenv("SHOUTRRR_NTFY_PASSWORD") != "" {
			t.Skip("username-only auth not configured, skipping test")
		}

		service := initializeService(t, buildServiceURL())

		message := uniqueMessage("E2E Test: Username-only auth message")

		require.NoError(t, service.Send(message, nil))

		verifyMessageReceived(t, service.Config.Topic, message)
	})
}
