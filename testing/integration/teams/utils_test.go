package teams_test

import (
	"bytes"
	"io"
	"net/http"
	"net/url"
	"strings"
	"testing"

	"github.com/stretchr/testify/mock"

	"github.com/nicholas-fedor/shoutrrr/pkg/services/chat/teams"
)

// MockHTTPClient is a testify mock that implements the teams.HTTPClient interface.
type MockHTTPClient struct {
	mock.Mock
}

// mockLogger is a simple logger implementation for testing.
type mockLogger struct{}

const workflowURL = "https://prod-00.westus.logic.azure.com:443/workflows/abc123/triggers/manual/paths/invoke?api-version=2016-06-00&sp=/triggers/manual/run&sv=1.0&sig=XXXXXXXX"

func (m *mockLogger) Print(_ ...any)            {}
func (m *mockLogger) Printf(_ string, _ ...any) {}
func (m *mockLogger) Println(_ ...any)          {}

// createTestService creates a Teams service instance configured for testing.
func createTestService(
	t *testing.T,
	webhookURL string,
	httpClients ...teams.HTTPClient,
) *teams.Service {
	t.Helper()

	service := &teams.Service{}

	parsedURL, err := url.Parse(webhookURL)
	if err != nil {
		t.Fatalf("failed to parse webhook URL: %v", err)
	}

	err = service.Initialize(parsedURL, &mockLogger{})
	if err != nil {
		t.Fatalf("failed to initialize service: %v", err)
	}

	if len(httpClients) > 0 && httpClients[0] != nil {
		service.SetHTTPClient(httpClients[0])
	}

	return service
}

func (m *MockHTTPClient) Do(req *http.Request) (*http.Response, error) {
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

// createMockResponse creates a mock HTTP response with the given status code, body, and optional headers.
//
//nolint:unparam // body and headers are intentionally exposed for callers to use
func createMockResponse(statusCode int, body string, headers ...map[string]string) *http.Response {
	resp := &http.Response{
		StatusCode: statusCode,
		Body:       io.NopCloser(strings.NewReader(body)),
		Header:     make(http.Header),
	}

	for _, hdrMap := range headers {
		for k, v := range hdrMap {
			resp.Header.Set(k, v)
		}
	}

	return resp
}

// assertRequestContains asserts that the HTTP request body contains the expected content.
func assertRequestContains(t *testing.T, mockClient *MockHTTPClient, expectedContent string) {
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
