package e2e_test

import (
	"testing"

	"github.com/stretchr/testify/require"
)

// TestMQTTRetained verifies that messages are published to a real MQTT broker
// without the retain flag by default, with it when the URL sets retained, and
// twice in a row to one retained topic.
//
//nolint:paralleltest // Publishes to a live broker one message at a time.
func TestMQTTRetained(t *testing.T) {
	t.Run("non-retained by default", func(t *testing.T) {
		envURL := addCredentialsToURL(brokerURL(t))

		sendAndClose(t, envURL, "E2E Test: Non-retained message", nil)
	})

	t.Run("retained when configured", func(t *testing.T) {
		envURL := addCredentialsToURL(brokerURL(t))

		sendAndClose(t, appendQuery(envURL, "retained=true"), "E2E Test: Retained message content", nil)
	})

	t.Run("multiple retained messages to the same topic", func(t *testing.T) {
		envURL := addCredentialsToURL(brokerURL(t))

		service := newService(t, appendQuery(envURL, "retained=true"))

		require.NoError(t, service.Send("E2E Test: First retained message", nil))
		// The second retained message replaces the first on the broker.
		require.NoError(t, service.Send("E2E Test: Second retained message", nil))
		require.NoError(t, service.Close())
	})
}
