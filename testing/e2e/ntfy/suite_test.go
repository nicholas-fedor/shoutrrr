package e2e_test

import (
	"bufio"
	"context"
	"fmt"
	"log"
	"net/http"
	"net/url"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/nicholas-fedor/shoutrrr/pkg/services/push/ntfy"
)

// Settings for the ntfy e2e tests.
const (
	// envValueTrue is the string value for boolean true in environment variables.
	envValueTrue = "true"
	// defaultPort is the host port docker-compose.yaml publishes the server on
	// when NTFY_PORT is unset.
	defaultPort = "8080"
	// healthTimeout bounds the server health check.
	healthTimeout = 2 * time.Second
	// sendDelay spaces the tests out to respect ntfy's rate limits.
	sendDelay = 500 * time.Millisecond
)

// TestMain loads the .env file in this directory, if any, before running the
// tests. These tests connect to a real ntfy server and verify actual behavior.
//
// Environment variables:
//   - SHOUTRRR_NTFY_URL: ntfy server URL (default: ntfy://localhost:8080/shoutrrr-e2e-test,
//     with the port NTFY_PORT names when it is set)
//   - NTFY_PORT: the host port of the local server (optional, default 8080)
//   - SHOUTRRR_NTFY_USERNAME: Basic auth username (optional)
//   - SHOUTRRR_NTFY_PASSWORD: Basic auth password (optional)
//   - SHOUTRRR_NTFY_DISABLE_TLS: Set to "true" to use HTTP instead of HTTPS (optional)
func TestMain(m *testing.M) {
	loadEnvFile(".env")

	os.Exit(m.Run())
}

// TestNtfyServiceID verifies the service ID, which needs no server.
func TestNtfyServiceID(t *testing.T) {
	t.Parallel()

	service := &ntfy.Service{}
	assert.Equal(t, "ntfy", service.GetID())
}

// requireServer skips the test when the ntfy server is not reachable, then
// waits [sendDelay] to respect ntfy's rate limits.
//
// Parameters:
//   - t: the test.
func requireServer(t *testing.T) {
	t.Helper()

	if !isNtfyServerAvailable(t.Context()) {
		t.Skip("ntfy server not available, skipping e2e tests")
	}

	time.Sleep(sendDelay)
}

// uniqueMessage appends a suffix that is unique to this send, so a message
// the server cached from an earlier run cannot satisfy a poll.
//
// Parameters:
//   - text: the message text.
//
// Returns:
//   - string: the text with a unique suffix.
func uniqueMessage(text string) string {
	return fmt.Sprintf("%s %d", text, time.Now().UnixNano())
}

// serverPort returns the host port the local server is published on.
//
// Returns:
//   - string: NTFY_PORT, or 8080 when it is unset.
func serverPort() string {
	if port := os.Getenv("NTFY_PORT"); port != "" {
		return port
	}

	return defaultPort
}

// defaultServiceURL returns the service URL used when SHOUTRRR_NTFY_URL is
// unset, matching the server that docker-compose.yaml starts.
//
// Returns:
//   - string: the service URL for the local server's port.
func defaultServiceURL() string {
	return "ntfy://localhost:" + serverPort() + "/shoutrrr-e2e-test"
}

// buildServiceURL constructs the ntfy service URL from environment variables.
//
// Returns:
//   - string: the service URL, with credentials and disabletls applied.
func buildServiceURL() string {
	baseURL := os.Getenv("SHOUTRRR_NTFY_URL")
	if baseURL == "" {
		baseURL = defaultServiceURL()
	}

	serviceURL, err := url.Parse(baseURL)
	if err != nil {
		return baseURL
	}

	query := serviceURL.Query()
	if os.Getenv("SHOUTRRR_NTFY_DISABLE_TLS") == envValueTrue {
		query.Set("disabletls", "yes")
	}

	username := os.Getenv("SHOUTRRR_NTFY_USERNAME")
	password := os.Getenv("SHOUTRRR_NTFY_PASSWORD")

	if username != "" || password != "" {
		if password != "" {
			serviceURL.User = url.UserPassword(username, password)
		} else {
			serviceURL.User = url.User(username)
		}
	}

	serviceURL.RawQuery = query.Encode()

	result := serviceURL.String()
	if !strings.Contains(result, "?") {
		result += "?"
	}

	return result
}

// addQueryParam adds a query parameter to a URL string.
//
// Parameters:
//   - rawURL: the URL.
//   - key: the parameter name.
//   - value: the parameter value.
//
// Returns:
//   - string: the URL with the parameter set, or rawURL when it does not parse.
func addQueryParam(rawURL, key, value string) string {
	serviceURL, err := url.Parse(rawURL)
	if err != nil {
		return rawURL
	}

	query := serviceURL.Query()
	query.Set(key, value)
	serviceURL.RawQuery = query.Encode()

	return serviceURL.String()
}

// initializeService parses a service URL string, creates an ntfy.Service, and
// initializes it with a logger that writes to the test output.
//
// Parameters:
//   - t: the test.
//   - rawURL: the service URL.
//
// Returns:
//   - *ntfy.Service: the initialized service.
func initializeService(t *testing.T, rawURL string) *ntfy.Service {
	t.Helper()

	serviceURL, err := url.Parse(rawURL)
	require.NoError(t, err)

	service := &ntfy.Service{}
	require.NoError(t, service.Initialize(serviceURL, log.New(t.Output(), "[Test] ", 0)))

	return service
}

// loadEnvFile sets the variables in a .env file that the environment does not
// already set.
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
			if _, exists := os.LookupEnv(key); !exists {
				_ = os.Setenv(key, value)
			}
		}
	}
}

// getNtfyBaseURL returns the base URL for the ntfy server using the
// SHOUTRRR_NTFY_URL environment variable.
//
// Returns:
//   - string: the server's HTTP or HTTPS base URL.
func getNtfyBaseURL() string {
	baseURL := os.Getenv("SHOUTRRR_NTFY_URL")
	if baseURL == "" {
		baseURL = defaultServiceURL()
	}

	parsed, err := url.Parse(baseURL)
	if err != nil {
		return "http://localhost:" + serverPort()
	}

	scheme := "https"
	if os.Getenv("SHOUTRRR_NTFY_DISABLE_TLS") == envValueTrue {
		scheme = "http"
	}

	return scheme + "://" + parsed.Host
}

// isNtfyServerAvailable checks if the ntfy server is reachable.
//
// Parameters:
//   - ctx: cancellation for the health check.
//
// Returns:
//   - bool: true when the server reports itself healthy.
func isNtfyServerAvailable(ctx context.Context) bool {
	ctx, cancel := context.WithTimeout(ctx, healthTimeout)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, getNtfyBaseURL()+"/v1/health", http.NoBody)
	if err != nil {
		return false
	}

	client := &http.Client{Timeout: healthTimeout}

	resp, err := client.Do(req)
	if err != nil {
		return false
	}

	defer func() { _ = resp.Body.Close() }()

	return resp.StatusCode == http.StatusOK
}
