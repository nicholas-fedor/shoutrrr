package zulip_test

import (
	"bytes"
	"io"
	"net/http"
	"net/url"
	"strings"
	"testing"

	"github.com/nicholas-fedor/shoutrrr/pkg/services/chat/zulip"
	"github.com/nicholas-fedor/shoutrrr/pkg/services/chat/zulip/mocks"
	"github.com/nicholas-fedor/shoutrrr/pkg/types"
)

// mockLogger is a simple logger implementation for testing.
type mockLogger struct{}

func (m *mockLogger) Print(_ ...any)            {}
func (m *mockLogger) Printf(_ string, _ ...any) {}
func (m *mockLogger) Println(_ ...any)          {}

// createTestService creates a Zulip service instance configured for testing.
func createTestService(
	t *testing.T,
	serviceURL string,
	httpClients ...zulip.HTTPClient,
) *zulip.Service {
	t.Helper()

	service := &zulip.Service{}

	parsedURL, err := url.Parse(serviceURL)
	if err != nil {
		t.Fatalf("failed to parse service URL: %v", err)
	}

	err = service.Initialize(parsedURL, &mockLogger{})
	if err != nil {
		t.Fatalf("failed to initialize service: %v", err)
	}

	// Override the HTTPClient if provided (after Initialize sets the default)
	if len(httpClients) > 0 && httpClients[0] != nil {
		service.HTTPClient = httpClients[0]
	}

	return service
}

// createTestParams creates test parameters with the given key-value pairs.
func createTestParams(pairs ...string) *types.Params {
	params := make(types.Params)

	for i := 0; i < len(pairs); i += 2 {
		if i+1 < len(pairs) {
			params[pairs[i]] = pairs[i+1]
		}
	}

	return &params
}

// createMockResponse creates a mock HTTP response with the given status code and body.
func createMockResponse(statusCode int, body string) *http.Response {
	resp := &http.Response{
		StatusCode: statusCode,
		Body:       io.NopCloser(strings.NewReader(body)),
		Header:     make(http.Header),
	}

	return resp
}

// assertRequestContains asserts that the HTTP request body contains the expected content.
func assertRequestContains(t *testing.T, mockClient *mocks.MockHTTPClient, expectedContent string) {
	t.Helper()

	found := false

	for i := range mockClient.Calls {
		call := &mockClient.Calls[i]
		if call.Method == "Do" {
			req := call.Arguments[0].(*http.Request)

			body, err := io.ReadAll(req.Body)
			if err != nil {
				continue
			}
			// Reset the body for potential future reads
			req.Body = io.NopCloser(bytes.NewReader(body))
			if strings.Contains(string(body), expectedContent) {
				found = true

				break
			}
		}
	}

	if !found {
		t.Errorf("Expected request body to contain %q, but no matching call found", expectedContent)
	}
}

// assertRequestMatches asserts that at least one HTTP request matches the given predicate.
func assertRequestMatches(
	t *testing.T,
	mockClient *mocks.MockHTTPClient,
	predicate func(*http.Request) bool,
	description string,
) {
	t.Helper()

	found := false

	for i := range mockClient.Calls {
		call := &mockClient.Calls[i]
		if call.Method == "Do" {
			req := call.Arguments[0].(*http.Request)
			if predicate(req) {
				found = true

				break
			}
		}
	}

	if !found {
		t.Errorf("Expected request to match %s, but no matching call found", description)
	}
}
