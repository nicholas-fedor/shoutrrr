package discord_test

import (
	"bytes"
	"io"
	"net/http"
	"net/url"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"

	"github.com/nicholas-fedor/shoutrrr/pkg/services/chat/discord"
	"github.com/nicholas-fedor/shoutrrr/pkg/types"
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

// createTestService creates a Discord service instance configured for testing.
func createTestService(
	t *testing.T,
	webhookURL string,
	httpClients ...discord.HTTPClient,
) *discord.Service {
	t.Helper()

	service := &discord.Service{}

	parsedURL, err := url.Parse(webhookURL)
	assert.NoError(t, err) //nolint:testifylint

	err = service.Initialize(parsedURL, &mockLogger{})
	assert.NoError(t, err) //nolint:testifylint

	// Override the HTTPClient if provided (after Initialize sets the default)
	if len(httpClients) > 0 && httpClients[0] != nil {
		service.HTTPClient = httpClients[0]
	}

	return service
}

// createTestMessageItem creates a test MessageItem with the given text.
func createTestMessageItem(text string) types.MessageItem {
	return types.MessageItem{Text: text}
}

// createTestMessageItemWithFile creates a test MessageItem with a file attachment.
func createTestMessageItemWithFile(text, filename string, data []byte) types.MessageItem {
	return types.MessageItem{
		Text: text,
		File: &types.File{
			Name: filename,
			Data: data,
		},
	}
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
func createMockResponse(statusCode int, body string, headers ...map[string]string) *http.Response {
	resp := &http.Response{
		StatusCode: statusCode,
		Body:       io.NopCloser(strings.NewReader(body)),
		Header:     make(http.Header),
	}

	if len(headers) > 0 && headers[0] != nil {
		for k, v := range headers[0] {
			resp.Header.Set(k, v)
		}
	}

	return resp
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

// assertRequestBody asserts that the HTTP request body exactly matches the expected content.
func assertRequestBody(t *testing.T, mockClient *MockHTTPClient, expectedBody string) {
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
			if string(body) == expectedBody {
				found = true

				break
			}
		}
	}

	if !found {
		t.Errorf("Expected request body to be %q, but no matching call found", expectedBody)
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
