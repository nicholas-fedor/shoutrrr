package twilio_test

import (
	"bytes"
	"io"
	"net/http"
	"net/url"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"

	"github.com/nicholas-fedor/shoutrrr/pkg/services/sms/twilio"
)

// MockHTTPClient is a testify mock that implements the HTTPClient interface.
type MockHTTPClient struct {
	mock.Mock
}

// mockLogger is a simple logger implementation for testing.
type mockLogger struct{}

func (m *mockLogger) Print(_ ...any)            {}
func (m *mockLogger) Printf(_ string, _ ...any) {}
func (m *mockLogger) Println(_ ...any)          {}

// createTestService creates a Twilio service instance configured for testing.
func createTestService(
	t *testing.T,
	twilioURL string,
	httpClients ...twilio.HTTPClient,
) *twilio.Service {
	t.Helper()

	service := &twilio.Service{}

	parsedURL, err := url.Parse(twilioURL)
	assert.NoError(t, err) //nolint:testifylint

	err = service.Initialize(parsedURL, &mockLogger{})
	assert.NoError(t, err) //nolint:testifylint

	// Override the HTTPClient if provided (after Initialize sets the default)
	if len(httpClients) > 0 && httpClients[0] != nil {
		service.HTTPClient = httpClients[0]
	}

	return service
}

func (m *MockHTTPClient) Do(req *http.Request) (*http.Response, error) {
	// Capture the request body before it's consumed
	if req.Body != nil {
		body, err := io.ReadAll(req.Body)
		if err == nil {
			req.Body = io.NopCloser(bytes.NewReader(body))
		}
	}

	args := m.Called(req)

	if args.Get(0) == nil {
		return nil, args.Error(1)
	}

	return args.Get(0).(*http.Response), args.Error(1)
}

// createMockResponse creates a mock HTTP response with the given status code and body.
func createMockResponse(statusCode int, body string) *http.Response {
	return &http.Response{
		StatusCode: statusCode,
		Body:       io.NopCloser(bytes.NewReader([]byte(body))),
		Header:     make(http.Header),
	}
}

// assertRequestContains asserts that the HTTP request body contains the expected content.
func assertRequestContains(t *testing.T, mockClient *MockHTTPClient, expectedContent string) {
	t.Helper()

	found := false

	for _, call := range mockClient.Calls {
		if call.Method == "Do" {
			req := call.Arguments[0].(*http.Request)

			body, err := io.ReadAll(req.Body)
			if err != nil {
				continue
			}
			// Reset the body for potential future reads
			req.Body = io.NopCloser(bytes.NewReader(body))

			if bytes.Contains(body, []byte(expectedContent)) {
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
	mockClient *MockHTTPClient,
	predicate func(*http.Request) bool,
	description string,
) {
	t.Helper()

	found := false

	for _, call := range mockClient.Calls {
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

// assertRequestMade asserts that an HTTP POST request was made to the expected URL.
func assertRequestMade(
	t *testing.T,
	mockClient *MockHTTPClient,
	expectedURL string,
) {
	t.Helper()

	found := false

	for _, call := range mockClient.Calls {
		if call.Method == "Do" {
			req := call.Arguments[0].(*http.Request)
			if req.Method == http.MethodPost && req.URL.String() == expectedURL {
				found = true

				break
			}
		}
	}

	if !found {
		t.Errorf("Expected POST request to %s, but no matching call found", expectedURL)
	}
}
