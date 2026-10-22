package e2e_test

import (
	"net/url"
	"os"
	"testing"

	"github.com/stretchr/testify/require"
)

// TestMQTTAuthentication verifies that a real MQTT broker accepts an anonymous
// client when it allows one, accepts the configured credentials, and rejects
// a wrong password when it requires authentication.
//
//nolint:paralleltest // Publishes to a live broker one message at a time.
func TestMQTTAuthentication(t *testing.T) {
	t.Run("without authentication", func(t *testing.T) {
		envURL := brokerURL(t)

		if os.Getenv("SHOUTRRR_MQTT_AUTH_REQUIRED") == envValueTrue {
			t.Skip("Skipping unauthenticated test when authentication is required")
		}

		sendAndClose(t, envURL, "E2E Test: No auth message", nil)
	})

	t.Run("with username and password", func(t *testing.T) {
		envURL := brokerURL(t)

		if os.Getenv("SHOUTRRR_MQTT_USERNAME") == "" || os.Getenv("SHOUTRRR_MQTT_PASSWORD") == "" {
			t.Skip("SHOUTRRR_MQTT_USERNAME and/or SHOUTRRR_MQTT_PASSWORD not set, skipping authenticated test")
		}

		sendAndClose(t, addCredentialsToURL(envURL), "E2E Test: Authenticated message", nil)
	})

	t.Run("wrong credentials", func(t *testing.T) {
		envURL := brokerURL(t)

		// A broker that allows anonymous clients accepts wrong credentials too.
		if os.Getenv("SHOUTRRR_MQTT_AUTH_REQUIRED") != envValueTrue {
			t.Skip("Skipping auth failure test when anonymous access is allowed")
		}

		username := os.Getenv("SHOUTRRR_MQTT_USERNAME")
		password := os.Getenv("SHOUTRRR_MQTT_PASSWORD")

		if username == "" || password == "" {
			t.Skip("SHOUTRRR_MQTT_USERNAME and/or SHOUTRRR_MQTT_PASSWORD not set, skipping auth failure test")
		}

		serviceURL, err := url.Parse(envURL)
		require.NoError(t, err)

		serviceURL.User = url.UserPassword(username, "wrong_password_"+password)

		sendExpectingFailure(t, serviceURL.String())
	})
}
