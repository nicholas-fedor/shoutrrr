package homeassistant_test

import (
	"bytes"
	"io"
	"net/http"
	"net/url"
	"strings"
	"testing"

	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	jsonv2 "encoding/json/v2"

	"github.com/nicholas-fedor/shoutrrr/pkg/services/push/homeassistant"
	"github.com/nicholas-fedor/shoutrrr/pkg/types"
)

type MockHTTPClient struct {
	mock.Mock
}

type mockLogger struct{}

type requestPayload struct {
	Message        string   `json:"message"`
	Title          string   `json:"title"`
	NotificationID string   `json:"notification_id"`
	Targets        []string `json:"target"`
}

const (
	validHomeAssistantURL = "homeassistant://s3cret@ha.example.com"
	persistentAPIURL      = "https://ha.example.com:443/api/services/persistent_notification/create"
)

func (*mockLogger) Print(_ ...any)            {}
func (*mockLogger) Printf(_ string, _ ...any) {}
func (*mockLogger) Println(_ ...any)          {}

func createTestService(
	t *testing.T,
	serviceURL string,
	httpClients ...types.HTTPClient,
) *homeassistant.Service {
	t.Helper()

	service := &homeassistant.Service{}

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
) (*homeassistant.Service, *MockHTTPClient) {
	t.Helper()

	mockClient := &MockHTTPClient{}
	service := createTestService(t, serviceURL, mockClient)

	mockClient.On("Do", mock.Anything).
		Return(createMockResponse(http.StatusOK, `[]`), nil).
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

func requestJSON(t *testing.T, mockClient *MockHTTPClient) requestPayload {
	t.Helper()

	req := findMatchingRequest(mockClient, func(*http.Request) bool { return true })
	require.NotNil(t, req)

	body, err := io.ReadAll(req.Body)
	require.NoError(t, err)

	req.Body = io.NopCloser(bytes.NewReader(body))

	var payload requestPayload
	require.NoError(t, jsonv2.Unmarshal(body, &payload))

	return payload
}

func requestBody(t *testing.T, mockClient *MockHTTPClient) []byte {
	t.Helper()

	req := findMatchingRequest(mockClient, func(*http.Request) bool { return true })
	require.NotNil(t, req)

	body, err := io.ReadAll(req.Body)
	require.NoError(t, err)

	req.Body = io.NopCloser(bytes.NewReader(body))

	return body
}
