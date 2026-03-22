package matrix_test

import (
	"fmt"
	"net/url"
	"testing"

	"github.com/nicholas-fedor/shoutrrr/pkg/services/chat/matrix"
)

// testLogger implements types.StdLogger for testing.
type testLogger struct {
	messages []string
}

// Print logs an info message.
func (l *testLogger) Print(v ...any) {
	l.messages = append(l.messages, fmt.Sprint(v...))
}

// Printf logs a formatted info message.
func (l *testLogger) Printf(format string, v ...any) {
	l.messages = append(l.messages, fmt.Sprintf(format, v...))
}

// Println logs an info message with newline.
func (l *testLogger) Println(v ...any) {
	l.messages = append(l.messages, fmt.Sprintln(v...))
}

// createTestService creates a matrix service with the given URL string.
// Uses the "dummy" URL special case to avoid actual client initialization.
func createTestService(t *testing.T, serviceURL string) (*matrix.Service, error) {
	t.Helper()

	parsedURL, err := url.Parse(serviceURL)
	if err != nil {
		t.Fatalf("failed to parse URL: %v", err)
	}

	service := &matrix.Service{}

	err = service.Initialize(parsedURL, &testLogger{})
	if err != nil {
		return nil, err
	}

	return service, nil
}
