package e2e_test

import (
	"testing"
)

// TestMQTTBasic verifies that a plain text message is published to a real
// MQTT broker.
//
//nolint:paralleltest // Publishes to a live broker one message at a time.
func TestMQTTBasic(t *testing.T) {
	t.Run("basic text content", func(t *testing.T) {
		envURL := addCredentialsToURL(brokerURL(t))

		sendAndClose(t, envURL, "E2E Test: Basic text content notification", nil)
	})
}
