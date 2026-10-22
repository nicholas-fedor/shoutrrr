package e2e_test

import (
	"net/url"
	"testing"

	"github.com/stretchr/testify/require"
)

// TestMQTTTopics verifies that messages are published to a real MQTT broker on
// single-level, nested, and escaped topics.
//
//nolint:paralleltest // Publishes to a live broker one message at a time.
func TestMQTTTopics(t *testing.T) {
	// topics lists the topics to publish to, unescaped, with the message for
	// each. The URL escapes them once.
	topics := []struct {
		name, path, message string
	}{
		{name: "single-level topic", path: "/test/single", message: "E2E Test: Single-level topic"},
		{name: "nested topic", path: "/home/alerts/notifications", message: "E2E Test: Nested topic message"},
		{
			name:    "topic with characters the URL escapes",
			path:    "/sensor device/température",
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
