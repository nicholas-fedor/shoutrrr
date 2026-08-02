package ntfy_test

import (
	"bytes"
	"io"
	"net/http"
	"net/url"
	"strings"
	"testing"

	"github.com/stretchr/testify/mock"

	"github.com/nicholas-fedor/shoutrrr/pkg/services/push/ntfy"
	"github.com/nicholas-fedor/shoutrrr/pkg/types"
)

// MockHTTPClient is a testify mock that implements the types.HTTPClient interface.
type MockHTTPClient struct {
	mock.Mock
}

// mockLogger is a simple logger implementation for testing.
type mockLogger struct{}

func (m *mockLogger) Print(_ ...any)            {}
func (m *mockLogger) Printf(_ string, _ ...any) {}
func (m *mockLogger) Println(_ ...any)          {}

// createTestService creates an ntfy service instance configured for testing.
func createTestService(
	t *testing.T,
	serviceURL string,
	httpClients ...types.HTTPClient,
) *ntfy.Service {
	t.Helper()

	service := &ntfy.Service{}

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
		service.SetHTTPClient(httpClients[0])
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

// createTestServiceWithMock creates an ntfy service instance configured for testing,
// along with a MockHTTPClient that is already configured on the service and seeded
// with a default successful 200 OK Do expectation.
func createTestServiceWithMock(
	t *testing.T,
	serviceURL string,
) (*ntfy.Service, *MockHTTPClient) {
	t.Helper()

	mockClient := &MockHTTPClient{}
	service := createTestService(t, serviceURL, mockClient)

	mockClient.On("Do", mock.Anything).
		Return(createMockResponse(http.StatusOK, `{"code":200,"error":"OK"}`), nil).
		Once()

	return service, mockClient
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

// createMockResponse creates a mock HTTP response with the given status code and body.
func createMockResponse(statusCode int, body string) *http.Response {
	resp := &http.Response{
		StatusCode: statusCode,
		Body:       io.NopCloser(strings.NewReader(body)),
		Header:     make(http.Header),
	}

	return resp
}

// findMatchingRequest iterates over mock Do calls and returns the first request
// that matches the given predicate, or nil if no match is found.
func findMatchingRequest(
	mockClient *MockHTTPClient,
	predicate func(*http.Request) bool,
) *http.Request {
	for i := range mockClient.Calls {
		call := &mockClient.Calls[i]
		if call.Method == "Do" {
			req := call.Arguments[0].(*http.Request)
			if predicate(req) {
				return req
			}
		}
	}

	return nil
}

// assertRequestContains asserts that the HTTP request body contains the expected content.
func assertRequestContains(t *testing.T, mockClient *MockHTTPClient, expectedContent string) {
	t.Helper()

	req := findMatchingRequest(mockClient, func(req *http.Request) bool {
		body, err := io.ReadAll(req.Body)
		if err != nil {
			return false
		}

		req.Body = io.NopCloser(bytes.NewReader(body))

		return strings.Contains(string(body), expectedContent)
	})

	if req == nil {
		t.Errorf("Expected request body to contain %q, but no matching call found", expectedContent)
	}
}

// assertRequestHeaderContains asserts that at least one HTTP request contains the expected header value.
func assertRequestHeaderContains(
	t *testing.T,
	mockClient *MockHTTPClient,
	headerName,
	expectedValue string,
) {
	t.Helper()

	req := findMatchingRequest(mockClient, func(req *http.Request) bool {
		if values := req.Header.Values(headerName); len(values) > 0 {
			for _, v := range values {
				if strings.Contains(v, expectedValue) {
					return true
				}
			}
		}

		return false
	})

	if req == nil {
		t.Errorf("Expected request header %q to contain %q, but no matching call found", headerName, expectedValue)
	}
}

// assertRequestMade asserts that an HTTP POST request was made to the expected URL.
func assertRequestMade(
	t *testing.T,
	mockClient *MockHTTPClient,
	expectedURL string,
) {
	t.Helper()

	req := findMatchingRequest(mockClient, func(req *http.Request) bool {
		return req.Method == http.MethodPost && req.URL.String() == expectedURL
	})

	if req == nil {
		t.Errorf("Expected POST request to %s, but no matching call found", expectedURL)
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

	req := findMatchingRequest(mockClient, predicate)

	if req == nil {
		t.Errorf("Expected request to match %s, but no matching call found", description)
	}
}
