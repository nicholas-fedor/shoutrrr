package e2e_test

import (
	"net/url"
	"testing"

	"github.com/stretchr/testify/require"
)

// TestMQTTTopics verifies that messages are published to a real MQTT broker on
// single-level, nested, wildcard, and escaped topics.
//
//nolint:paralleltest // Publishes to a live broker one message at a time.
func TestMQTTTopics(t *testing.T) {
	// topics lists the URL paths to publish to, with the message for each.
	topics := []struct {
		name, path, message string
	}{
		{name: "single-level topic", path: "/test/single", message: "E2E Test: Single-level topic"},
		{name: "nested topic", path: "/home/alerts/notifications", message: "E2E Test: Nested topic message"},
		{name: "wildcard topic base", path: "/sensors/+/temperature", message: "E2E Test: Wildcard topic base"},
		{name: "multi-level wildcard topic", path: "/home/#", message: "E2E Test: Multi-level wildcard topic"},
		{
			name:    "topic with special characters",
			path:    "/sensor%2Fdevice%231/temperature",
			message: "E2E Test: Topic with special characters",
		},
	}

	for _, topic := range topics {
		t.Run(topic.name, func(t *testing.T) {
			serviceURL, err := url.Parse(addCredentialsToURL(brokerURL(t)))
			require.NoError(t, err)

			serviceURL.Path = topic.path

			sendAndClose(t, serviceURL.String(), topic.message, nil)
		})
	}
}
