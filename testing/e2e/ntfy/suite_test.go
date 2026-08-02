package e2e_test

import (
	"bufio"
	"context"
	"net/http"
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

// Test_Ntfy_E2E runs the ntfy E2E test suite using Ginkgo.
// These tests connect to a real ntfy server and verify actual behavior.
//
// Environment variables:
//   - SHOUTRRR_NTFY_URL: ntfy server URL (default: ntfy://localhost:8080/shoutrrr-e2e-test)
//   - SHOUTRRR_NTFY_USERNAME: Basic auth username (optional)
//   - SHOUTRRR_NTFY_PASSWORD: Basic auth password (optional)
//   - SHOUTRRR_NTFY_DISABLE_TLS: Set to "true" to use HTTP instead of HTTPS (optional)
func Test_Ntfy_E2E(t *testing.T) { //nolint:paralleltest // mutates process-wide env via loadEnvFile
	loadEnvFile(".env")

	gomega.RegisterFailHandler(ginkgo.Fail)

	// Add delay between tests to respect ntfy rate limits
	ginkgo.BeforeEach(func() {
		time.Sleep(500 * time.Millisecond)
	})

	ginkgo.RunSpecs(t, "ntfy E2E Tests")
}

// buildServiceURL constructs the ntfy service URL from environment variables.
func buildServiceURL() string {
	baseURL := os.Getenv("SHOUTRRR_NTFY_URL")
	if baseURL == "" {
		baseURL = "ntfy://localhost:8080/shoutrrr-e2e-test"
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
			if _, exists := os.LookupEnv(key); !exists {
				_ = os.Setenv(key, value)
			}
		}
	}

	if err := scanner.Err(); err != nil {
		return
	}
}

// getNtfyBaseURL returns the base URL for the ntfy server using shared host and scheme logic.
func getNtfyBaseURL() string {
	host := os.Getenv("SHOUTRRR_NTFY_HOST")
	if host == "" {
		host = "localhost:8080"
	}

	scheme := "http"
	if os.Getenv("SHOUTRRR_NTFY_DISABLE_TLS") != envValueTrue {
		scheme = "https"
	}

	return scheme + "://" + host
}

// isNtfyServerAvailable checks if the ntfy server is reachable.
func isNtfyServerAvailable() bool {
	baseURL := getNtfyBaseURL()

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, baseURL+"/v1/health", http.NoBody)
	if err != nil {
		return false
	}

	client := &http.Client{Timeout: 2 * time.Second}

	resp, err := client.Do(req)
	if err != nil {
		return false
	}

	defer func() { _ = resp.Body.Close() }()

	return resp.StatusCode == http.StatusOK
}
