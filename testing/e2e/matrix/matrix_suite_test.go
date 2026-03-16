package e2e_test

import (
	"bufio"
	"fmt"
	"net/url"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/onsi/ginkgo/v2"
	"github.com/onsi/gomega"

	"github.com/nicholas-fedor/shoutrrr/internal/testutils"
	"github.com/nicholas-fedor/shoutrrr/pkg/services/chat/matrix"
)

const envValueTrue = "true"

// sharedService is the authenticated Matrix service shared across tests.
// Initialized once in BeforeSuite to avoid repeated login operations.
var sharedService *matrix.Service

// Test_Matrix_E2E runs the Matrix E2E test suite using Ginkgo.
// These tests connect to a real Matrix server and verify actual behavior.
func Test_Matrix_E2E(t *testing.T) {
	t.Parallel()

	// Load .env file if it exists
	loadEnvFile(".env")

	gomega.RegisterFailHandler(ginkgo.Fail)

	// Initialize shared service once before all tests
	ginkgo.BeforeSuite(func() {
		serviceURL := buildServiceURL()
		if serviceURL == "" {
			return // tests will skip anyway
		}

		parsedURL, err := url.Parse(serviceURL)
		if err != nil {
			return
		}

		sharedService = &matrix.Service{}
		// Initialize once (this does the login)
		err = sharedService.Initialize(parsedURL, testutils.TestLogger())
		if err != nil {
			// Log but continue; individual tests will handle failures
			fmt.Printf("Warning: shared service init failed: %v\n", err)
		}
	})

	ginkgo.BeforeEach(func() {
		time.Sleep(500 * time.Millisecond) // slightly longer safety delay
	})

	ginkgo.RunSpecs(t, "Matrix E2E Tests")
}

// matrixServerURL returns the Matrix server URL from environment or default.
// Expected format: matrix://user:password@host:port
func matrixServerURL() string {
	// Check for URL in environment variable
	envURL := os.Getenv("SHOUTRRR_MATRIX_URL")
	if envURL != "" {
		return envURL
	}

	// Build URL from individual components
	host := os.Getenv("SHOUTRRR_MATRIX_HOST")
	if host == "" {
		host = "localhost:8008"
	}

	user := os.Getenv("SHOUTRRR_MATRIX_USER")
	password := os.Getenv("SHOUTRRR_MATRIX_PASSWORD")

	if user != "" && password != "" {
		return "matrix://" + url.UserPassword(user, password).String() + "@" + host
	}

	// Return empty string if no credentials available
	return ""
}

// matrixRoom returns the Matrix room from environment or default test room.
func matrixRoom() string {
	room := os.Getenv("SHOUTRRR_MATRIX_ROOM")
	if room != "" {
		return room
	}

	// Default test room alias
	return "#test:localhost"
}

// matrixDisableTLS returns whether TLS should be disabled.
func matrixDisableTLS() bool {
	return os.Getenv("SHOUTRRR_MATRIX_DISABLE_TLS") == envValueTrue
}

// buildServiceURL builds a complete Matrix service URL with all parameters.
func buildServiceURL() string {
	baseURL := matrixServerURL()

	// If no URL from environment, return empty
	if baseURL == "" {
		return ""
	}

	// Parse to add query params
	parsedURL, err := url.Parse(baseURL)
	if err != nil {
		return ""
	}

	// Add room parameter
	room := matrixRoom()
	if room != "" {
		q := parsedURL.Query()
		q.Add("room", room)
		parsedURL.RawQuery = q.Encode()
	}

	// Add TLS disable if needed
	if matrixDisableTLS() {
		q := parsedURL.Query()
		q.Add("disableTLS", "true")
		parsedURL.RawQuery = q.Encode()
	}

	return parsedURL.String()
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
