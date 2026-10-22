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

	"github.com/nicholas-fedor/shoutrrr/pkg/services/chat/discord"
)

// testDelay spaces the tests out to avoid hitting Discord's rate limits.
const testDelay = 100 * time.Millisecond

// TestMain loads the .env file in this directory, if any, before running the
// tests. Values in the file replace those already in the environment.
//
// Environment variables:
//   - SHOUTRRR_DISCORD_URL: the Discord webhook service URL, such as discord://TOKEN@WEBHOOK_ID
//   - SHOUTRRR_DISCORD_THREAD_ID: an existing thread in the webhook's channel (optional)
func TestMain(m *testing.M) {
	loadEnvFile(".env")

	os.Exit(m.Run())
}

// newService initializes a discord.Service from SHOUTRRR_DISCORD_URL with the
// given query values set, skipping the test when the URL is unset, then waits
// [testDelay] to avoid hitting Discord's rate limits.
//
// Parameters:
//   - t: the test.
//   - query: the query values to set, or nil.
//
// Returns:
//   - *discord.Service: the initialized service.
func newService(t *testing.T, query map[string]string) *discord.Service {
	t.Helper()

	envURL := os.Getenv("SHOUTRRR_DISCORD_URL")
	if envURL == "" {
		t.Skip("SHOUTRRR_DISCORD_URL not set, skipping Discord e2e test")
	}

	time.Sleep(testDelay)

	serviceURL, err := url.Parse(envURL)
	require.NoError(t, err)

	values := serviceURL.Query()
	for key, value := range query {
		values.Set(key, value)
	}

	serviceURL.RawQuery = values.Encode()

	service := &discord.Service{}
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
