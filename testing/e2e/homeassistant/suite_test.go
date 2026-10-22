package e2e_test

import (
	"bufio"
	"context"
	"log"
	"net/http"
	"net/url"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/gorilla/websocket"
	"github.com/stretchr/testify/require"

	"github.com/nicholas-fedor/shoutrrr/pkg/services/push/homeassistant"
)

// haNotification is a persistent notification as the Home Assistant
// WebSocket API returns it.
type haNotification struct {
	Message        string `json:"message"`
	NotificationID string `json:"notification_id"`
	Title          string `json:"title"`
}

// Timeouts for the requests the tests make to Home Assistant directly.
const (
	// defaultHTTPTimeout bounds the WebSocket handshake, reads, and writes.
	defaultHTTPTimeout = 10 * time.Second
	// availabilityTimeout bounds the API check that decides whether to skip.
	availabilityTimeout = 2 * time.Second
)

// TestMain loads the .env file in this directory, which setup.sh writes after
// onboarding, before running the tests.
//
// Environment variables:
//   - SHOUTRRR_HOMEASSISTANT_URL: the service URL with an access token, such as
//     homeassistant://TOKEN@localhost:8123/?disabletls=yes
func TestMain(m *testing.M) {
	loadEnvFile(".env")

	os.Exit(m.Run())
}

// serviceURL returns the configured Home Assistant service URL.
//
// Returns:
//   - string: the service URL, or "" when it is not set.
func serviceURL() string {
	return os.Getenv("SHOUTRRR_HOMEASSISTANT_URL")
}

// requireHomeAssistant skips the test when Home Assistant is not configured or
// not reachable.
//
// Parameters:
//   - t: the test.
func requireHomeAssistant(t *testing.T) {
	t.Helper()

	if !isHomeAssistantAvailable(t.Context()) {
		t.Skip("Home Assistant is not available, skipping e2e tests")
	}
}

// initializeService parses a service URL string, creates a
// homeassistant.Service, and initializes it with a logger that writes to the
// test output.
//
// Parameters:
//   - t: the test.
//   - rawURL: the service URL.
//
// Returns:
//   - *homeassistant.Service: the initialized service.
func initializeService(t *testing.T, rawURL string) *homeassistant.Service {
	t.Helper()

	parsed, err := url.Parse(rawURL)
	require.NoError(t, err)

	service := &homeassistant.Service{}
	require.NoError(t, service.Initialize(parsed, log.New(t.Output(), "[Test] ", 0)))

	return service
}

// withNid returns rawURL with its nid query parameter set.
//
// Parameters:
//   - t: the test.
//   - rawURL: the service URL.
//   - nid: the persistent notification ID.
//
// Returns:
//   - string: the service URL with the notification ID.
func withNid(t *testing.T, rawURL, nid string) string {
	t.Helper()

	parsed, err := url.Parse(rawURL)
	require.NoError(t, err)

	query := parsed.Query()
	query.Set("nid", nid)
	parsed.RawQuery = query.Encode()

	return parsed.String()
}

// isHomeAssistantAvailable reports whether the configured token can reach the
// Home Assistant API.
//
// Parameters:
//   - ctx: cancellation for the API check.
//
// Returns:
//   - bool: true when the API accepts the token.
func isHomeAssistantAvailable(ctx context.Context) bool {
	baseURL := serviceURL()
	if baseURL == "" {
		return false
	}

	parsed, err := url.Parse(baseURL)
	if err != nil || parsed.User == nil {
		return false
	}

	ctx, cancel := context.WithTimeout(ctx, availabilityTimeout)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, "http://localhost:8123/api/", http.NoBody)
	if err != nil {
		return false
	}

	req.Header.Set("Authorization", "Bearer "+parsed.User.Username())

	client := &http.Client{Timeout: availabilityTimeout}

	resp, err := client.Do(req)
	if err != nil {
		return false
	}

	defer func() { _ = resp.Body.Close() }()

	return resp.StatusCode == http.StatusOK
}

// fetchPersistentNotification reads a persistent notification through the Home
// Assistant WebSocket API.
//
// Parameters:
//   - t: the test.
//   - token: the access token.
//   - nid: the persistent notification ID.
//
// Returns:
//   - haNotification: the notification, which the test requires to exist.
func fetchPersistentNotification(t *testing.T, token, nid string) haNotification {
	t.Helper()

	dialer := websocket.Dialer{HandshakeTimeout: defaultHTTPTimeout}
	conn, resp, err := dialer.DialContext(t.Context(), "ws://localhost:8123/api/websocket", nil)
	require.NoError(t, err)

	if resp != nil && resp.Body != nil {
		_ = resp.Body.Close()
	}

	defer func() { _ = conn.Close() }()

	require.NoError(t, conn.SetReadDeadline(time.Now().Add(defaultHTTPTimeout)))
	require.NoError(t, conn.SetWriteDeadline(time.Now().Add(defaultHTTPTimeout)))

	var hello map[string]any
	require.NoError(t, conn.ReadJSON(&hello))
	require.Equal(t, "auth_required", hello["type"])

	require.NoError(t, conn.WriteJSON(map[string]string{
		"type":         "auth",
		"access_token": token,
	}))

	var auth map[string]any
	require.NoError(t, conn.ReadJSON(&auth))
	require.Equal(t, "auth_ok", auth["type"])

	require.NoError(t, conn.WriteJSON(map[string]any{
		"id":   1,
		"type": "persistent_notification/get",
	}))

	var result struct {
		Success bool             `json:"success"`
		Result  []haNotification `json:"result"`
	}
	require.NoError(t, conn.ReadJSON(&result))
	require.True(t, result.Success)

	for _, notification := range result.Result {
		if notification.NotificationID == nid {
			return notification
		}
	}

	require.Failf(t, "notification not found", "no persistent notification with ID %q in %v", nid, result.Result)

	return haNotification{}
}

// accessToken returns the access token in a service URL.
//
// Parameters:
//   - t: the test.
//   - rawURL: the service URL.
//
// Returns:
//   - string: the token, which is the URL's username.
func accessToken(t *testing.T, rawURL string) string {
	t.Helper()

	parsed, err := url.Parse(rawURL)
	require.NoError(t, err)
	require.NotNil(t, parsed.User)

	return parsed.User.Username()
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
		if len(parts) != 2 {
			continue
		}

		key := strings.TrimSpace(parts[0])

		value := strings.Trim(strings.TrimSpace(parts[1]), `"'`)
		if _, exists := os.LookupEnv(key); !exists {
			_ = os.Setenv(key, value)
		}
	}
}
