package e2e_test

import (
	"testing"

	"github.com/nicholas-fedor/shoutrrr/pkg/types"
)

// TestMQTTQoS verifies that messages are published to a real MQTT broker at
// each QoS level set in the URL, and with the QoS set by a param.
//
//nolint:paralleltest // Publishes to a live broker one message at a time.
func TestMQTTQoS(t *testing.T) {
	// levels lists the QoS levels set in the URL, with the message for each.
	levels := []struct {
		name, qos, message string
	}{
		{name: "QoS 0 (at most once)", qos: "0", message: "E2E Test: QoS 0 message"},
		{name: "QoS 1 (at least once)", qos: "1", message: "E2E Test: QoS 1 message"},
		{name: "QoS 2 (exactly once)", qos: "2", message: "E2E Test: QoS 2 message"},
	}

	for _, level := range levels {
		t.Run(level.name, func(t *testing.T) {
			envURL := addCredentialsToURL(brokerURL(t))

			sendAndClose(t, appendQuery(envURL, "qos="+level.qos), level.message, nil)
		})
	}

	t.Run("QoS via params", func(t *testing.T) {
		envURL := addCredentialsToURL(brokerURL(t))

		sendAndClose(t, envURL, "E2E Test: QoS via params", &types.Params{"qos": "1"})
	})
}
