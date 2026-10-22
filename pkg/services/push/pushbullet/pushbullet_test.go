package pushbullet

import (
	"context"
	"encoding/json/v2"
	"errors"
	"io"
	"log"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	"github.com/nicholas-fedor/shoutrrr/pkg/format"
	"github.com/nicholas-fedor/shoutrrr/pkg/types"
	"github.com/nicholas-fedor/shoutrrr/pkg/types/mocks"
	"github.com/nicholas-fedor/shoutrrr/pkg/util/jsonclient"
)

// capturedRequest records the parts of a request that an HTTP client received.
type capturedRequest struct {
	// method is the request method.
	method string
	// url is the full request URL.
	url string
	// contentType is the request's Content-Type header.
	contentType string
	// accessToken is the request's Access-Token header.
	accessToken string
	// body is the request body.
	body string
}

// cannedResponse is a response the fake Pushbullet API returns.
type cannedResponse struct {
	// body is the response body.
	body string
	// statusCode is the response status code.
	statusCode int
}

// invalidTokenBody is the body the Pushbullet API returns for a missing or
// invalid access token.
const invalidTokenBody = `{"error":{"cat":"~(=^‥^)","message":"Access token is missing or invalid.","type":"invalid_request"}}`

// errTransport is the transport failure returned by the failing HTTP client.
var errTransport = errors.New("transport failure")

// noteResponse returns the JSON body of a created note push.
//
// Parameters:
//   - t: the test.
//   - body: the push body.
//   - title: the push title.
//   - active: whether the push is active.
//
// Returns:
//   - string: the JSON body.
func noteResponse(t *testing.T, body, title string, active bool) string {
	t.Helper()

	encoded, err := json.Marshal(PushResponse{Type: "note", Body: body, Title: title, Active: active})
	require.NoError(t, err)

	return string(encoded)
}

// newRespondingClient returns a mock HTTP client that expects one request for
// each response, records the requests, and answers them in order.
//
// Parameters:
//   - t: the test that owns the mock.
//   - responses: the responses, one for each expected request.
//
// Returns:
//   - *mocks.MockHTTPClient: the client.
//   - *[]capturedRequest: the requests, appended as the client receives them.
func newRespondingClient(t *testing.T, responses ...cannedResponse) (*mocks.MockHTTPClient, *[]capturedRequest) {
	t.Helper()

	captured := &[]capturedRequest{}
	client := mocks.NewMockHTTPClient(t)
	client.EXPECT().Do(mock.Anything).RunAndReturn(func(req *http.Request) (*http.Response, error) {
		body, err := io.ReadAll(req.Body)
		require.NoError(t, err)

		response := responses[len(*captured)]
		*captured = append(*captured, capturedRequest{
			method:      req.Method,
			url:         req.URL.String(),
			contentType: req.Header.Get("Content-Type"),
			accessToken: req.Header.Get("Access-Token"),
			body:        string(body),
		})

		return &http.Response{
			Status:     http.StatusText(response.statusCode),
			StatusCode: response.statusCode,
			Body:       io.NopCloser(strings.NewReader(response.body)),
		}, nil
	}).Times(len(responses))

	return client, captured
}

// newTestService returns a service initialized from rawURL that logs to the
// test output.
//
// Parameters:
//   - t: the test.
//   - rawURL: the service URL.
//
// Returns:
//   - *Service: the service.
func newTestService(t *testing.T, rawURL string) *Service {
	t.Helper()

	service := &Service{}
	require.NoError(t, service.Initialize(parseTestURL(t, rawURL), log.New(t.Output(), "", 0)))

	return service
}

// newServiceWithConfig returns a service that uses config without parsing a
// URL, for configs a URL cannot produce.
//
// Parameters:
//   - config: the service config.
//
// Returns:
//   - *Service: the service.
func newServiceWithConfig(config *Config) *Service {
	return &Service{Config: config, pkr: format.NewPropKeyResolver(config)}
}

// TestGetID verifies that the service identifies itself by its scheme.
func TestGetID(t *testing.T) {
	t.Parallel()

	assert.Equal(t, "pushbullet", (&Service{}).GetID())
}

// TestInitialize verifies that Initialize applies the default title, parses
// the service URL, and rejects a token of the wrong length.
func TestInitialize(t *testing.T) {
	t.Parallel()

	t.Run("valid URL", func(t *testing.T) {
		t.Parallel()

		service := newTestService(t, "pushbullet://"+testToken+"/device")

		assert.Equal(t, &Config{Token: testToken, Targets: []string{"device"}, Title: "Shoutrrr notification"}, service.Config)
		assert.Equal(t, testToken, service.client.Headers().Get("Access-Token"))
	})

	t.Run("short token", func(t *testing.T) {
		t.Parallel()

		err := (&Service{}).Initialize(parseTestURL(t, "pushbullet://shorttoken/device"), log.New(t.Output(), "", 0))

		require.ErrorIs(t, err, ErrTokenIncorrectSize)
	})
}

// TestSend verifies the requests a send makes: one authenticated JSON POST to
// the pushes endpoint for each target, in order, with the target in the field
// that matches its kind.
func TestSend(t *testing.T) {
	t.Parallel()

	// tests lists service URLs and params with the request bodies the send
	// should produce.
	tests := []struct {
		params     *types.Params
		name       string
		rawURL     string
		title      string
		wantBodies []string
	}{
		{
			name:   "no target",
			rawURL: "pushbullet://" + testToken,
			title:  "Shoutrrr notification",
			wantBodies: []string{
				`{"type":"note","title":"Shoutrrr notification","body":"Message","email":"","channel_tag":"","device_iden":""}`,
			},
		},
		{
			name:   "device, email, and channel targets",
			rawURL: "pushbullet://" + testToken + "/device/iam@email.com#channel",
			title:  "Shoutrrr notification",
			wantBodies: []string{
				`{"type":"note","title":"Shoutrrr notification","body":"Message","email":"","channel_tag":"","device_iden":"device"}`,
				`{"type":"note","title":"Shoutrrr notification","body":"Message","email":"iam@email.com","channel_tag":"","device_iden":""}`,
				`{"type":"note","title":"Shoutrrr notification","body":"Message","email":"","channel_tag":"channel","device_iden":""}`,
			},
		},
		{
			name:   "title param",
			rawURL: "pushbullet://" + testToken + "/device?title=Configured",
			params: &types.Params{"title": "Override"},
			title:  "Override",
			wantBodies: []string{
				`{"type":"note","title":"Override","body":"Message","email":"","channel_tag":"","device_iden":"device"}`,
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			responses := make([]cannedResponse, len(tt.wantBodies))
			for i := range responses {
				responses[i] = cannedResponse{statusCode: http.StatusOK, body: noteResponse(t, "Message", tt.title, true)}
			}

			service := newTestService(t, tt.rawURL)
			client, captured := newRespondingClient(t, responses...)
			service.SetHTTPClient(client)

			require.NoError(t, service.Send("Message", tt.params))

			require.Len(t, *captured, len(tt.wantBodies))

			for i, request := range *captured {
				assert.Equal(t, http.MethodPost, request.method)
				assert.Equal(t, pushesEndpoint, request.url)
				assert.Equal(t, "application/json", request.contentType)
				assert.Equal(t, testToken, request.accessToken)
				assert.JSONEq(t, tt.wantBodies[i], request.body)
			}
		})
	}
}

// TestSendParamsLeaveConfigUnchanged verifies that params apply to a copy of
// the config for one send, so the stored config and the next send keep the
// configured title.
func TestSendParamsLeaveConfigUnchanged(t *testing.T) {
	t.Parallel()

	service := newTestService(t, "pushbullet://"+testToken+"/device?title=Configured")
	want := *service.Config

	client, _ := newRespondingClient(t, cannedResponse{statusCode: http.StatusOK, body: noteResponse(t, "Message", "Override", true)})
	service.SetHTTPClient(client)

	require.NoError(t, service.Send("Message", &types.Params{"title": "Override"}))
	assert.Equal(t, want, *service.Config)

	client, captured := newRespondingClient(t, cannedResponse{statusCode: http.StatusOK, body: noteResponse(t, "Message", "Configured", true)})
	service.SetHTTPClient(client)

	require.NoError(t, service.Send("Message", nil))
	require.Len(t, *captured, 1)
	assert.JSONEq(t, `{"type":"note","title":"Configured","body":"Message","email":"","channel_tag":"","device_iden":"device"}`,
		(*captured)[0].body)
}

// TestSendResponseValidation verifies that a send rejects a created push that
// does not match the request: another type, body, or title, or an inactive
// push.
func TestSendResponseValidation(t *testing.T) {
	t.Parallel()

	// tests lists created pushes with the error each should produce.
	tests := []struct {
		response PushResponse
		wantErr  error
		name     string
	}{
		{
			name:     "another type",
			response: PushResponse{Type: "link", Body: "Message", Title: "Shoutrrr notification", Active: true},
			wantErr:  ErrUnexpectedResponseType,
		},
		{
			name:     "another body",
			response: PushResponse{Type: "note", Body: "Wrong message", Title: "Shoutrrr notification", Active: true},
			wantErr:  ErrResponseBodyMismatch,
		},
		{
			name:     "another title",
			response: PushResponse{Type: "note", Body: "Message", Title: "Wrong Title", Active: true},
			wantErr:  ErrResponseTitleMismatch,
		},
		{
			name:     "inactive",
			response: PushResponse{Type: "note", Body: "Message", Title: "Shoutrrr notification", Active: false},
			wantErr:  ErrPushNotActive,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			body, err := json.Marshal(tt.response)
			require.NoError(t, err)

			service := newTestService(t, "pushbullet://"+testToken+"/device")
			client, _ := newRespondingClient(t, cannedResponse{statusCode: http.StatusOK, body: string(body)})
			service.SetHTTPClient(client)

			require.ErrorIs(t, service.Send("Message", nil), tt.wantErr)
		})
	}
}

// TestSendContextErrors verifies that a send reports the API's error message,
// wraps other request errors, stops at the first failing target, rejects an
// unknown param before any request, and stops when the caller's context is
// canceled.
func TestSendContextErrors(t *testing.T) {
	t.Parallel()

	t.Run("API error", func(t *testing.T) {
		t.Parallel()

		service := newTestService(t, "pushbullet://"+testToken+"/device")
		client, _ := newRespondingClient(t, cannedResponse{statusCode: http.StatusUnauthorized, body: invalidTokenBody})
		service.SetHTTPClient(client)

		err := service.SendContext(t.Context(), "Message", nil)

		responseErr, ok := errors.AsType[*ResponseError](err)
		require.True(t, ok, "error %v is not a *ResponseError", err)
		assert.Equal(t, "invalid_request", responseErr.ErrorData.Type)
		assert.EqualError(t, err, "API error: Access token is missing or invalid.")
	})

	t.Run("error status without a JSON body", func(t *testing.T) {
		t.Parallel()

		service := newTestService(t, "pushbullet://"+testToken+"/device")
		client, _ := newRespondingClient(t, cannedResponse{statusCode: http.StatusBadGateway, body: "<html>Bad Gateway</html>"})
		service.SetHTTPClient(client)

		err := service.SendContext(t.Context(), "Message", nil)

		require.ErrorIs(t, err, jsonclient.ErrUnexpectedStatus)
		assert.Contains(t, err.Error(), "failed to push")
	})

	t.Run("transport error", func(t *testing.T) {
		t.Parallel()

		service := newTestService(t, "pushbullet://"+testToken+"/device")
		client := mocks.NewMockHTTPClient(t)
		client.EXPECT().Do(mock.Anything).Return(nil, errTransport).Once()
		service.SetHTTPClient(client)

		require.ErrorIs(t, service.SendContext(t.Context(), "Message", nil), errTransport)
	})

	t.Run("stops at the first failing target", func(t *testing.T) {
		t.Parallel()

		service := newTestService(t, "pushbullet://"+testToken+"/first/second")
		client, captured := newRespondingClient(t, cannedResponse{statusCode: http.StatusUnauthorized, body: invalidTokenBody})
		service.SetHTTPClient(client)

		require.Error(t, service.SendContext(t.Context(), "Message", nil))
		require.Len(t, *captured, 1)
		assert.JSONEq(t, `{"type":"note","title":"Shoutrrr notification","body":"Message","email":"","channel_tag":"","device_iden":"first"}`,
			(*captured)[0].body)
	})

	t.Run("unknown param", func(t *testing.T) {
		t.Parallel()

		service := newTestService(t, "pushbullet://"+testToken+"/device")
		// The mock has no expectations, so a request fails the test.
		service.SetHTTPClient(mocks.NewMockHTTPClient(t))

		err := service.SendContext(t.Context(), "Message", &types.Params{"foo": "bar"})

		require.ErrorIs(t, err, format.ErrInvalidConfigKey)
	})

	t.Run("canceled context", func(t *testing.T) {
		t.Parallel()

		ctx, cancel := context.WithCancel(t.Context())
		cancel()

		client := mocks.NewMockHTTPClient(t)
		client.EXPECT().Do(mock.Anything).RunAndReturn(func(req *http.Request) (*http.Response, error) {
			return nil, req.Context().Err()
		}).Once()

		service := newTestService(t, "pushbullet://"+testToken)
		service.SetHTTPClient(client)

		require.ErrorIs(t, service.SendContext(ctx, "message", nil), context.Canceled)
	})
}

// TestServiceTimeout verifies that the router budget is one client timeout for
// each target, and at least one client timeout.
func TestServiceTimeout(t *testing.T) {
	t.Parallel()

	// tests lists services and params with the budget each should get.
	tests := []struct {
		service *Service
		params  *types.Params
		name    string
		want    time.Duration
	}{
		{name: "uninitialized service", service: &Service{}, want: defaultHTTPTimeout},
		{name: "no targets", service: newServiceWithConfig(&Config{}), want: defaultHTTPTimeout},
		{name: "one target", service: newTestService(t, "pushbullet://"+testToken+"/dev1"), want: defaultHTTPTimeout},
		{
			name:    "three targets",
			service: newTestService(t, "pushbullet://"+testToken+"/dev1/dev2/dev3"),
			want:    3 * defaultHTTPTimeout,
		},
		{
			name:    "invalid param keeps the stored targets",
			service: newTestService(t, "pushbullet://"+testToken+"/dev1/dev2"),
			params:  &types.Params{"foo": "bar"},
			want:    2 * defaultHTTPTimeout,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			assert.Equal(t, tt.want, tt.service.ServiceTimeout(tt.params))
		})
	}
}

// TestSetHTTPClient verifies that every client SetHTTPClient installs carries
// the access token, including the default client that nil restores.
func TestSetHTTPClient(t *testing.T) {
	t.Parallel()

	// tests lists the clients to set and the HTTP client each should leave.
	tests := []struct {
		client     types.HTTPClient
		name       string
		wantCustom bool
	}{
		{name: "custom client", client: mocks.NewMockHTTPClient(t), wantCustom: true},
		{name: "nil", client: nil},
		{name: "nil *http.Client", client: (*http.Client)(nil)},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			service := newTestService(t, "pushbullet://"+testToken+"/device")
			service.SetHTTPClient(mocks.NewMockHTTPClient(t))
			service.SetHTTPClient(tt.client)

			if tt.wantCustom {
				assert.Same(t, tt.client, service.httpClient)
			} else {
				assert.Nil(t, service.httpClient)
			}

			assert.Equal(t, testToken, service.client.Headers().Get("Access-Token"))
		})
	}
}
