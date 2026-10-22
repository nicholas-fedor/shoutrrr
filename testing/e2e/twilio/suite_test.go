package e2e_test

import (
	"bufio"
	"log"
	"net/url"
	"os"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/nicholas-fedor/shoutrrr/pkg/services/sms/twilio"
)

// TestMain loads the .env file in this directory, if any, before running the
// tests. Values in the file replace those already in the environment.
//
// Environment variables:
//   - SHOUTRRR_TWILIO_URL: the Twilio service URL, such as
//     twilio://ACCOUNT_SID:AUTH_TOKEN@FROM_NUMBER/TO_NUMBER
func TestMain(m *testing.M) {
	loadEnvFile(".env")

	os.Exit(m.Run())
}

// newService initializes a twilio.Service from SHOUTRRR_TWILIO_URL with the
// given query values set, skipping the test when the URL is unset.
//
// Parameters:
//   - t: the test.
//   - query: the query values to set, or nil.
//
// Returns:
//   - *twilio.Service: the initialized service.
func newService(t *testing.T, query map[string]string) *twilio.Service {
	t.Helper()

	envURL := os.Getenv("SHOUTRRR_TWILIO_URL")
	if envURL == "" {
		t.Skip("SHOUTRRR_TWILIO_URL not set, skipping Twilio e2e test")
	}

	serviceURL, err := url.Parse(envURL)
	require.NoError(t, err)

	values := serviceURL.Query()
	for key, value := range query {
		values.Set(key, value)
	}

	serviceURL.RawQuery = values.Encode()

	service := &twilio.Service{}
	require.NoError(t, service.Initialize(serviceURL, log.New(t.Output(), "[Test] ", 0)))

	return service
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
