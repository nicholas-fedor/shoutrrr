package signalgrid_test

import (
	"bytes"
	"io"
	"net/http"
	"net/url"
	"strings"
	"testing"

	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	"github.com/nicholas-fedor/shoutrrr/pkg/services/push/signalgrid"
	"github.com/nicholas-fedor/shoutrrr/pkg/types"
)

type MockHTTPClient struct {
	mock.Mock
}

type mockLogger struct{}

const (
	validSignalgridURL = "signalgrid://clientkey@channeltoken"
	signalgridAPIURL   = "https://api.signalgrid.co/v1/push"
)

func (*mockLogger) Print(_ ...any)            {}
func (*mockLogger) Printf(_ string, _ ...any) {}
func (*mockLogger) Println(_ ...any)          {}

func createTestService(
	t *testing.T,
	serviceURL string,
	httpClients ...types.HTTPClient,
) *signalgrid.Service {
	t.Helper()

	service := &signalgrid.Service{}

	parsedURL, err := url.Parse(serviceURL)
	require.NoError(t, err)

	err = service.Initialize(parsedURL, &mockLogger{})
	require.NoError(t, err)

	if len(httpClients) > 0 && httpClients[0] != nil {
		service.SetHTTPClient(httpClients[0])
	}

	return service
}

func createTestServiceWithMock(
	t *testing.T,
	serviceURL string,
) (*signalgrid.Service, *MockHTTPClient) {
	t.Helper()

	mockClient := &MockHTTPClient{}
	service := createTestService(t, serviceURL, mockClient)

	mockClient.On("Do", mock.Anything).
		Return(createMockResponse(http.StatusOK, `{"code":"200","text":"OK"}`), nil).
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

func createMockResponse(statusCode int, body string) *http.Response {
	return &http.Response{
		StatusCode: statusCode,
		Body:       io.NopCloser(strings.NewReader(body)),
		Header:     make(http.Header),
	}
}

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

func assertRequestMade(t *testing.T, mockClient *MockHTTPClient, expectedURL string) {
	t.Helper()

	req := findMatchingRequest(mockClient, func(req *http.Request) bool {
		return req.Method == http.MethodPost && req.URL.String() == expectedURL
	})

	if req == nil {
		t.Errorf("Expected POST request to %s, but no matching call found", expectedURL)
	}
}

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

func requestForm(t *testing.T, mockClient *MockHTTPClient) url.Values {
	t.Helper()

	req := findMatchingRequest(mockClient, func(*http.Request) bool { return true })
	require.NotNil(t, req)

	body, err := io.ReadAll(req.Body)
	require.NoError(t, err)

	req.Body = io.NopCloser(bytes.NewReader(body))

	form, err := url.ParseQuery(string(body))
	require.NoError(t, err)

	return form
}
