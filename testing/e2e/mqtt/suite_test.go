package e2e_test

import (
	"bufio"
	"log"
	"net/url"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/nicholas-fedor/shoutrrr/pkg/services/push/mqtt"
	"github.com/nicholas-fedor/shoutrrr/pkg/types"
)

// Settings for the MQTT e2e tests.
const (
	// envValueTrue is the string value for boolean true in environment variables.
	envValueTrue = "true"
	// testDelay spaces the tests out to avoid overwhelming the broker.
	testDelay = 100 * time.Millisecond
)

// TestMain loads the .env file in this directory, if any, before running the
// tests. Values in the file replace those already in the environment.
//
// Environment variables:
//   - SHOUTRRR_MQTT_URL: the broker URL with a topic, such as mqtt://localhost:1883/shoutrrr/test
//   - SHOUTRRR_MQTT_USERNAME and SHOUTRRR_MQTT_PASSWORD: the broker credentials (optional)
//   - SHOUTRRR_MQTT_TLS_URL: the TLS broker URL (optional)
//   - SHOUTRRR_MQTT_TLS_SKIP_VERIFY: set to "true" for self-signed certificates (optional)
//   - SHOUTRRR_MQTT_AUTH_REQUIRED: set to "true" when the broker rejects anonymous clients (optional)
func TestMain(m *testing.M) {
	loadEnvFile(".env")

	os.Exit(m.Run())
}

// brokerURL returns SHOUTRRR_MQTT_URL, skipping the test when it is unset, then
// waits [testDelay] to avoid overwhelming the broker.
//
// Parameters:
//   - t: the test.
//
// Returns:
//   - string: the broker URL.
func brokerURL(t *testing.T) string {
	t.Helper()

	envURL := os.Getenv("SHOUTRRR_MQTT_URL")
	if envURL == "" {
		t.Skip("SHOUTRRR_MQTT_URL not set, skipping MQTT e2e test")
	}

	time.Sleep(testDelay)

	return envURL
}

// newService parses a service URL and initializes an mqtt.Service from it with
// a logger that writes to the test output.
//
// Parameters:
//   - t: the test.
//   - rawURL: the service URL.
//
// Returns:
//   - *mqtt.Service: the initialized service, which the caller closes.
func newService(t *testing.T, rawURL string) *mqtt.Service {
	t.Helper()

	serviceURL, err := url.Parse(rawURL)
	require.NoError(t, err)

	service := &mqtt.Service{}
	require.NoError(t, service.Initialize(serviceURL, log.New(t.Output(), "[Test] ", 0)))

	return service
}

// sendAndClose sends a message through the service a URL configures, then
// closes it, requiring both to succeed.
//
// Parameters:
//   - t: the test.
//   - rawURL: the service URL.
//   - message: the message to send.
//   - params: the send params, or nil.
func sendAndClose(t *testing.T, rawURL, message string, params *types.Params) {
	t.Helper()

	service := newService(t, rawURL)

	require.NoError(t, service.Send(message, params))
	require.NoError(t, service.Close())
}

// sendExpectingFailure sends a message through the service a URL configures,
// requiring the send to fail, then closes it.
//
// Parameters:
//   - t: the test.
//   - rawURL: the service URL.
func sendExpectingFailure(t *testing.T, rawURL string) {
	t.Helper()

	service := newService(t, rawURL)

	require.Error(t, service.Send("E2E Test: Should fail", nil))

	_ = service.Close()
}

// appendQuery appends a raw query pair to a URL string.
//
// Parameters:
//   - rawURL: the URL.
//   - pair: the key=value pair to append.
//
// Returns:
//   - string: the URL with the pair appended to its query.
func appendQuery(rawURL, pair string) string {
	if strings.Contains(rawURL, "?") {
		return rawURL + "&" + pair
	}

	return rawURL + "?" + pair
}

// addCredentialsToURL adds the username and password from the environment to
// a URL, when either is set.
//
// Parameters:
//   - envURL: the URL.
//
// Returns:
//   - string: the URL with the credentials, or envURL when none are set or it
//     does not parse.
func addCredentialsToURL(envURL string) string {
	username := os.Getenv("SHOUTRRR_MQTT_USERNAME")
	password := os.Getenv("SHOUTRRR_MQTT_PASSWORD")

	if username == "" && password == "" {
		return envURL
	}

	serviceURL, err := url.Parse(envURL)
	if err != nil {
		return envURL
	}

	host := serviceURL.Host

	if username != "" {
		if password != "" {
			host = username + ":" + password + "@" + host
		} else {
			host = username + "@" + host
		}
	}

	newURL := serviceURL.Scheme + "://" + host + serviceURL.Path
	if serviceURL.RawQuery != "" {
		newURL += "?" + serviceURL.RawQuery
	}

	return newURL
}

// addTLSParam adds the skiptlsverify parameter to a URL when
// SHOUTRRR_MQTT_TLS_SKIP_VERIFY is set.
//
// Parameters:
//   - rawURL: the URL.
//
// Returns:
//   - string: the URL, with the parameter when verification is skipped.
func addTLSParam(rawURL string) string {
	if os.Getenv("SHOUTRRR_MQTT_TLS_SKIP_VERIFY") == envValueTrue {
		return appendQuery(rawURL, "skiptlsverify=yes")
	}

	return rawURL
}

// loadEnvFile sets the variables in a .env file, replacing any values already
// in the environment.
//
// Parameters:
//   - filename: the .env file, which may be missing.
func loadEnvFile(filename string) {
	file, err := os.Open(filename)
	if err != nil {
		// .env file doesn't exist, skip loading
		return
	}

	defer func() { _ = file.Close() }()

	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}

		parts := strings.SplitN(line, "=", 2)
		if len(parts) == 2 {
			key := strings.TrimSpace(parts[0])
			value := strings.TrimSpace(parts[1])
			// Remove quotes if present
			value = strings.Trim(value, `"'`)
			_ = os.Setenv(key, value) // Ignore error as it's test setup
		}
	}
}
