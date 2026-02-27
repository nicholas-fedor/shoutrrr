package e2e_test

import (
	"bufio"
	"net/url"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/onsi/ginkgo/v2"
	"github.com/onsi/gomega"
)

// envValueTrue is the string value for boolean true in environment variables.
const envValueTrue = "true"

// addCredentialsToURL adds username and password from environment variables to the URL
// if they are set. Returns the modified URL string.
func addCredentialsToURL(envURL string) string {
	username := os.Getenv("SHOUTRRR_MQTT_USERNAME")
	password := os.Getenv("SHOUTRRR_MQTT_PASSWORD")

	// If either username or password is set, add them to the URL
	if username != "" || password != "" {
		serviceURL, err := url.Parse(envURL)
		if err != nil {
			return envURL
		}

		// Build host with credentials
		host := serviceURL.Host

		if username != "" {
			if password != "" {
				host = username + ":" + password + "@" + host
			} else {
				host = username + "@" + host
			}
		}

		// Reconstruct URL with credentials
		newURL := serviceURL.Scheme + "://" + host + serviceURL.Path
		if serviceURL.RawQuery != "" {
			newURL += "?" + serviceURL.RawQuery
		}

		return newURL
	}

	return envURL
}

// addTLSParam adds TLS skip verify parameter to URL if enabled via environment variable.
func addTLSParam(rawURL string) string {
	tlsSkipVerify := os.Getenv("SHOUTRRR_MQTT_TLS_SKIP_VERIFY")
	if tlsSkipVerify == envValueTrue {
		if strings.Contains(rawURL, "?") {
			return rawURL + "&disabletlsverification=yes"
		}

		return rawURL + "?disabletlsverification=yes"
	}

	return rawURL
}

func TestMQTT_E2E(t *testing.T) {
	// Load .env file if it exists
	loadEnvFile(".env")

	gomega.RegisterFailHandler(ginkgo.Fail)

	// Add delay between tests to respect broker rate limits
	ginkgo.BeforeEach(func() {
		// Add a small delay between tests to avoid overwhelming the broker
		time.Sleep(100 * time.Millisecond)
	})

	ginkgo.RunSpecs(t, "MQTT E2E Tests")
}

// loadEnvFile loads environment variables from a .env file.
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
