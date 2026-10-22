package e2e_test

import (
	"net/url"
	"os"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

// TestMQTTTLS verifies that messages are published to a real MQTT broker over
// TLS with the mqtts scheme, on its TLS port, and that TLS to the plain MQTT
// port fails.
//
//nolint:paralleltest // Publishes to a live broker one message at a time.
func TestMQTTTLS(t *testing.T) {
	t.Run("mqtts scheme", func(t *testing.T) {
		envURL := os.Getenv("SHOUTRRR_MQTT_TLS_URL")
		if envURL == "" {
			// Without a TLS URL, the base URL is used with the mqtts scheme.
			envURL = strings.Replace(brokerURL(t), "mqtt://", "mqtts://", 1)
		}

		sendAndClose(t, addTLSParam(addCredentialsToURL(envURL)), "E2E Test: TLS message", nil)
	})

	t.Run("mqtts with custom port", func(t *testing.T) {
		sendAndClose(t, tlsURLWithPort(t, "8883"), "E2E Test: TLS on port 8883", nil)
	})

	t.Run("TLS to the non-TLS port fails", func(t *testing.T) {
		// Port 1883 serves plain MQTT, so a TLS handshake there fails.
		sendExpectingFailure(t, tlsURLWithPort(t, "1883"))
	})
}

// tlsURLWithPort returns the broker URL with the mqtts scheme and the given
// port, keeping its host, topic, and query, with the credentials and TLS
// parameter from the environment.
//
// Parameters:
//   - t: the test.
//   - port: the broker port.
//
// Returns:
//   - string: the TLS service URL.
func tlsURLWithPort(t *testing.T, port string) string {
	t.Helper()

	serviceURL, err := url.Parse(brokerURL(t))
	require.NoError(t, err)

	newURL := "mqtts://" + serviceURL.Hostname() + ":" + port + serviceURL.Path
	if serviceURL.RawQuery != "" {
		newURL += "?" + serviceURL.RawQuery
	}

	return addTLSParam(addCredentialsToURL(newURL))
}
