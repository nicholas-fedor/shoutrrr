package e2e_test

import (
	"bufio"
	"log"
	"net/url"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/nicholas-fedor/shoutrrr/pkg/services/chat/zulip"
)

// testDelay spaces the tests out to respect the server's rate limits.
const testDelay = 250 * time.Millisecond

// TestMain loads the .env file in this directory, if any, before running the
// tests. These tests connect to a real Zulip server and verify actual behavior.
//
// Environment variables:
//   - SHOUTRRR_ZULIP_HOST: Zulip server hostname (default: localhost)
//   - SHOUTRRR_ZULIP_BOT_MAIL: Bot email address
//   - SHOUTRRR_ZULIP_BOT_KEY: Bot API key
//   - SHOUTRRR_ZULIP_STREAM: Target stream name (optional)
//   - SHOUTRRR_ZULIP_TOPIC: Message topic (optional)
//   - SHOUTRRR_ZULIP_DM_TO: Comma-separated recipients (emails or user IDs) for direct message tests (optional; falls back to bot mail for self-DM to exercise the feature)
//   - SHOUTRRR_ZULIP_TYPE: Message type (channel/direct) for tests (optional)
//   - SHOUTRRR_ZULIP_READ_BY_SENDER: "true" to set read_by_sender (optional)
func TestMain(m *testing.M) {
	loadEnvFile(".env")

	os.Exit(m.Run())
}

// TestZulipServiceID verifies the service ID, which needs no server.
func TestZulipServiceID(t *testing.T) {
	t.Parallel()

	service := &zulip.Service{}
	assert.Equal(t, "zulip", service.GetID())
}

// requireServiceURL returns the configured service URL, skipping the test when
// no server is configured, then waits [testDelay] to respect the server's rate
// limits.
//
// Parameters:
//   - t: the test.
//
// Returns:
//   - string: the service URL.
func requireServiceURL(t *testing.T) string {
	t.Helper()

	serviceURL := buildServiceURL()
	if serviceURL == "" {
		t.Skip("Zulip server not configured, skipping test")
	}

	time.Sleep(testDelay)

	return serviceURL
}

// newService parses a service URL and initializes a zulip.Service from it with
// a logger that writes to the test output.
//
// Parameters:
//   - t: the test.
//   - rawURL: the service URL.
//
// Returns:
//   - *zulip.Service: the initialized service.
func newService(t *testing.T, rawURL string) *zulip.Service {
	t.Helper()

	serviceURL, err := url.Parse(rawURL)
	require.NoError(t, err)

	service := &zulip.Service{}
	require.NoError(t, service.Initialize(serviceURL, log.New(t.Output(), "[Test] ", 0)))

	return service
}

// buildServiceURL constructs the Zulip service URL from environment variables.
//
// Returns:
//   - string: the service URL, or "" when the bot mail or key is unset.
func buildServiceURL() string {
	host := os.Getenv("SHOUTRRR_ZULIP_HOST")
	if host == "" {
		host = "localhost"
	}

	botMail := os.Getenv("SHOUTRRR_ZULIP_BOT_MAIL")
	botKey := os.Getenv("SHOUTRRR_ZULIP_BOT_KEY")

	if botMail == "" || botKey == "" {
		return ""
	}

	serviceURL := &url.URL{
		Scheme: "zulip",
		User:   url.UserPassword(botMail, botKey),
		Host:   host,
	}

	q := url.Values{}
	if stream := os.Getenv("SHOUTRRR_ZULIP_STREAM"); stream != "" {
		q.Set("stream", stream)
	}

	if topic := os.Getenv("SHOUTRRR_ZULIP_TOPIC"); topic != "" {
		q.Set("topic", topic)
	}

	if typ := os.Getenv("SHOUTRRR_ZULIP_TYPE"); typ != "" {
		q.Set("type", typ)
	}

	if to := os.Getenv("SHOUTRRR_ZULIP_DM_TO"); to != "" {
		q.Set("to", to)
	}

	if read := os.Getenv("SHOUTRRR_ZULIP_READ_BY_SENDER"); read == "true" {
		q.Set("read_by_sender", "true")
	}

	if len(q) > 0 {
		serviceURL.RawQuery = q.Encode()
	}

	return serviceURL.String()
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
