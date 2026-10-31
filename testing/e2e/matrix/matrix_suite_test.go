package e2e_test

import (
	"bufio"
	"io"
	"log"
	"net/url"
	"os"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/nicholas-fedor/shoutrrr/pkg/services/chat/matrix"
)

// logSwitch is the writer behind the shared service's logger. The shared
// service outlives each test, so each test that uses it points the writer at
// its own output for the test's duration.
type logSwitch struct {
	writer io.Writer
	mu     sync.Mutex
}

// Settings for the Matrix e2e tests.
const (
	// envValueTrue is the string value for boolean true in environment variables.
	envValueTrue = "true"
	// testDelay spaces the tests out to respect the server's rate limits.
	testDelay = 500 * time.Millisecond
)

var (
	// sharedLog routes the shared service's log output to the current test.
	sharedLog = &logSwitch{writer: io.Discard, mu: sync.Mutex{}}
	// sharedServiceOnce initializes the shared service on first use.
	sharedServiceOnce sync.Once
	// sharedService is the Matrix service the tests share, so its login and room
	// joins happen once rather than in every test.
	sharedService *matrix.Service
	// errSharedService is the error from initializing the shared service.
	errSharedService error
)

// TestMain loads the .env file in this directory, if any, before running the
// tests. These tests connect to a real Matrix server and verify actual
// behavior.
//
// Environment variables:
//   - SHOUTRRR_MATRIX_URL: the service URL, such as matrix://user:password@host:port (optional)
//   - SHOUTRRR_MATRIX_HOST, SHOUTRRR_MATRIX_USER, SHOUTRRR_MATRIX_PASSWORD: the
//     parts of the service URL, used when SHOUTRRR_MATRIX_URL is unset
//   - SHOUTRRR_MATRIX_ROOM: the room alias to send to (default: #test:localhost)
//   - SHOUTRRR_MATRIX_DISABLE_TLS: set to "true" to use HTTP instead of HTTPS (optional)
func TestMain(m *testing.M) {
	loadEnvFile(".env")

	os.Exit(m.Run())
}

// TestMatrixServiceID verifies the service ID, which needs no server.
func TestMatrixServiceID(t *testing.T) {
	t.Parallel()

	assert.Equal(t, "matrix", (&matrix.Service{}).GetID())
}

// Write writes p to the current test's output, or discards it between tests.
//
// Parameters:
//   - p: the log output.
//
// Returns:
//   - int: the number of bytes written.
//   - error: the writer's error.
func (s *logSwitch) Write(p []byte) (int, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	return s.writer.Write(p)
}

// use points the writer at a test's output until the test ends.
//
// Parameters:
//   - t: the test.
func (s *logSwitch) use(t *testing.T) {
	t.Helper()

	s.mu.Lock()
	s.writer = t.Output()
	s.mu.Unlock()

	t.Cleanup(func() {
		s.mu.Lock()
		s.writer = io.Discard
		s.mu.Unlock()
	})
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
		t.Skip("Matrix server not configured, skipping test. " +
			"Set SHOUTRRR_MATRIX_USER and SHOUTRRR_MATRIX_PASSWORD environment variables")
	}

	time.Sleep(testDelay)

	return serviceURL
}

// getSharedService returns the shared Matrix service, initializing it on first
// use, with its log output routed to t. It skips the test when no server is
// configured.
//
// Parameters:
//   - t: the test.
//
// Returns:
//   - *matrix.Service: the shared service.
func getSharedService(t *testing.T) *matrix.Service {
	t.Helper()

	serviceURL := requireServiceURL(t)

	sharedServiceOnce.Do(func() {
		parsedURL, err := url.Parse(serviceURL)
		if err != nil {
			errSharedService = err

			return
		}

		service := &matrix.Service{}

		errSharedService = service.Initialize(parsedURL, log.New(sharedLog, "[Test] ", 0))
		if errSharedService == nil {
			sharedService = service
		}
	})

	require.NoError(t, errSharedService, "initializing the shared service")
	sharedLog.use(t)

	return sharedService
}

// newService parses a service URL and initializes a matrix.Service from it
// with a logger that writes to the test output.
//
// Parameters:
//   - t: the test.
//   - rawURL: the service URL.
//
// Returns:
//   - *matrix.Service: the initialized service.
func newService(t *testing.T, rawURL string) *matrix.Service {
	t.Helper()

	parsedURL, err := url.Parse(rawURL)
	require.NoError(t, err)

	service := &matrix.Service{}
	require.NoError(t, service.Initialize(parsedURL, log.New(t.Output(), "[Test] ", 0)))

	return service
}

// initErr initializes a matrix.Service from a URL with a logger that writes to
// the test output, for tests that expect initialization to fail.
//
// Parameters:
//   - t: the test.
//   - serviceURL: the service URL.
//
// Returns:
//   - error: the initialization error.
func initErr(t *testing.T, serviceURL *url.URL) error {
	t.Helper()

	service := &matrix.Service{}

	return service.Initialize(serviceURL, log.New(t.Output(), "[Test] ", 0))
}

// matrixServerURL returns the Matrix server URL from the environment.
//
// Returns:
//   - string: SHOUTRRR_MATRIX_URL, or a URL built from the host, user, and
//     password variables, or "" when no credentials are set.
func matrixServerURL() string {
	if envURL := os.Getenv("SHOUTRRR_MATRIX_URL"); envURL != "" {
		return envURL
	}

	host := os.Getenv("SHOUTRRR_MATRIX_HOST")
	if host == "" {
		host = "localhost:8008"
	}

	user := os.Getenv("SHOUTRRR_MATRIX_USER")
	password := os.Getenv("SHOUTRRR_MATRIX_PASSWORD")

	if user != "" && password != "" {
		return "matrix://" + url.UserPassword(user, password).String() + "@" + host
	}

	return ""
}

// matrixRoom returns the Matrix room from the environment.
//
// Returns:
//   - string: SHOUTRRR_MATRIX_ROOM, or #test:localhost when it is unset.
func matrixRoom() string {
	if room := os.Getenv("SHOUTRRR_MATRIX_ROOM"); room != "" {
		return room
	}

	return "#test:localhost"
}

// buildServiceURL builds the Matrix service URL with its room and TLS setting.
//
// Returns:
//   - string: the service URL, or "" when no server is configured.
func buildServiceURL() string {
	baseURL := matrixServerURL()
	if baseURL == "" {
		return ""
	}

	parsedURL, err := url.Parse(baseURL)
	if err != nil {
		return ""
	}

	query := parsedURL.Query()
	query.Add("room", matrixRoom())

	if os.Getenv("SHOUTRRR_MATRIX_DISABLE_TLS") == envValueTrue {
		query.Add("disableTLS", "true")
	}

	parsedURL.RawQuery = query.Encode()

	return parsedURL.String()
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
			// Only set if not already present in the environment
			if _, exists := os.LookupEnv(key); !exists {
				_ = os.Setenv(key, value) // Ignore error as it's test setup
			}
		}
	}
}
