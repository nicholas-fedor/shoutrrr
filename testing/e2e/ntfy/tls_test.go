package e2e_test

import (
	"os"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestNtfyTLS verifies that a message reaches a real ntfy server over HTTP
// when SHOUTRRR_NTFY_DISABLE_TLS is set, and over HTTPS otherwise.
//
//nolint:paralleltest // Sends to a live server one message at a time.
func TestNtfyTLS(t *testing.T) {
	t.Run("HTTP when DisableTLS is set", func(t *testing.T) {
		requireServer(t)

		if os.Getenv("SHOUTRRR_NTFY_DISABLE_TLS") != envValueTrue {
			t.Skip("HTTP test skipped because DisableTLS is not set")
		}

		service := initializeService(t, buildServiceURL())

		assert.True(t, service.Config.DisableTLS)
		assert.True(t, strings.HasPrefix(service.Config.GetAPIURL(), "http://"), service.Config.GetAPIURL())

		message := uniqueMessage("E2E Test: HTTP message")

		require.NoError(t, service.Send(message, nil))

		verifyMessageReceived(t, service.Config.Topic, message)
	})

	t.Run("HTTPS by default", func(t *testing.T) {
		requireServer(t)

		if os.Getenv("SHOUTRRR_NTFY_DISABLE_TLS") == envValueTrue {
			t.Skip("HTTPS test skipped because DisableTLS is set")
		}

		service := initializeService(t, buildServiceURL())

		assert.False(t, service.Config.DisableTLS)
		assert.True(t, strings.HasPrefix(service.Config.GetAPIURL(), "https://"), service.Config.GetAPIURL())

		message := uniqueMessage("E2E Test: HTTPS message")

		require.NoError(t, service.Send(message, nil))

		verifyMessageReceived(t, service.Config.Topic, message)
	})
}
