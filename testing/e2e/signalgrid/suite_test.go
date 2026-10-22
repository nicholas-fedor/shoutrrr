package e2e_test

import (
	"bufio"
	"log"
	"net/url"
	"os"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/nicholas-fedor/shoutrrr/pkg/services/push/signalgrid"
)

// TestMain loads the .env file in this directory, if any, before running the
// tests.
//
// Environment variables:
//   - SHOUTRRR_SIGNALGRID_URL: the Signalgrid service URL, such as
//     signalgrid://CLIENT_KEY@CHANNEL_TOKEN
func TestMain(m *testing.M) {
	loadEnvFile(".env")

	os.Exit(m.Run())
}

// sendWithQuery sends a message through the service SHOUTRRR_SIGNALGRID_URL
// configures, with the given query values set on the URL, skipping the test
// when the URL is unset.
//
// Parameters:
//   - t: the test.
//   - query: the query values to set, or nil.
//   - message: the message to send.
func sendWithQuery(t *testing.T, query map[string]string, message string) {
	t.Helper()

	envURL := os.Getenv("SHOUTRRR_SIGNALGRID_URL")
	if envURL == "" {
		t.Skip("SHOUTRRR_SIGNALGRID_URL not set, skipping Signalgrid e2e test")
	}

	serviceURL, err := url.Parse(envURL)
	require.NoError(t, err)

	values := serviceURL.Query()
	for key, value := range query {
		values.Set(key, value)
	}

	serviceURL.RawQuery = values.Encode()

	service := &signalgrid.Service{}
	require.NoError(t, service.Initialize(serviceURL, log.New(t.Output(), "[Test] ", 0)))
	require.NoError(t, service.Send(message, nil))
}

// loadEnvFile sets the variables in a .env file that the environment does not
// already set.
//
// Parameters:
//   - filename: the .env file, which may be missing.
func loadEnvFile(filename string) {
	file, err := os.Open(filename)
	if err != nil {
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

			value = strings.Trim(value, `"'`)
			if _, exists := os.LookupEnv(key); !exists {
				_ = os.Setenv(key, value)
			}
		}
	}
}
