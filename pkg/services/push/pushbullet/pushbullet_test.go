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
	"testing/synctest"
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
	// deadline is the deadline of the request's context.
	deadline time.Time
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

// noteResponse returns the JSON body of a created note push, which is
// active.
//
// Parameters:
//   - t: the test.
//   - body: the push body.
//   - title: the push title.
//
// Returns:
//   - string: the JSON body.
func noteResponse(t *testing.T, body, title string) string {
	t.Helper()

	encoded, err := json.Marshal(PushResponse{Type: "note", Body: body, Title: title, Active: true})
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
		deadline, _ := req.Context().Deadline()

		*captured = append(*captured, capturedRequest{
			deadline:    deadline,
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

		client, ok := service.client.Client().(*http.Client)
		require.True(t, ok)
		assert.Zero(t, client.Timeout)
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
				responses[i] = cannedResponse{statusCode: http.StatusOK, body: noteResponse(t, "Message", tt.title)}
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

// TestSendChecksConfig verifies that a send checks the access token of a
// config built in code before any request. A config without targets pushes to
// all devices, as a URL without a path does.
func TestSendChecksConfig(t *testing.T) {
	t.Parallel()

	t.Run("invalid token", func(t *testing.T) {
		t.Parallel()

		service := newServiceWithConfig(&Config{Token: "short", Title: "", Targets: []string{"device"}})
		service.SetHTTPClient(mocks.NewMockHTTPClient(t))

		require.ErrorIs(t, service.Send("Message", nil), ErrTokenIncorrectSize)
	})

	t.Run("no targets", func(t *testing.T) {
		t.Parallel()

		service := newServiceWithConfig(&Config{Token: testToken, Title: "Title", Targets: nil})
		client, captured := newRespondingClient(t, cannedResponse{statusCode: http.StatusOK, body: noteResponse(t, "Message", "Title")})
		service.SetHTTPClient(client)

		require.NoError(t, service.Send("Message", nil))
		require.Len(t, *captured, 1)
		assert.JSONEq(t,
			`{"type":"note","title":"Title","body":"Message","email":"","channel_tag":"","device_iden":""}`,
			(*captured)[0].body)
	})
}

// TestSendParamsLeaveConfigUnchanged verifies that params apply to a copy of
// the config for one send, so the stored config and the next send keep the
// configured title.
func TestSendParamsLeaveConfigUnchanged(t *testing.T) {
	t.Parallel()

	service := newTestService(t, "pushbullet://"+testToken+"/device?title=Configured")
	want := *service.Config

	client, _ := newRespondingClient(t, cannedResponse{statusCode: http.StatusOK, body: noteResponse(t, "Message", "Override")})
	service.SetHTTPClient(client)

	require.NoError(t, service.Send("Message", &types.Params{"title": "Override"}))
	assert.Equal(t, want, *service.Config)

	client, captured := newRespondingClient(t, cannedResponse{statusCode: http.StatusOK, body: noteResponse(t, "Message", "Configured")})
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

// TestSendContextErrors verifies that a send reports the API's error message
// naming the target, wraps other request errors, keeps sending to the targets
// after a failing one, rejects an unknown param before any request, sends
// nothing when the caller's context is already canceled, and skips the
// remaining targets once it ends.
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
		assert.EqualError(t, err, `sending to target "device": API error: Access token is missing or invalid.`)
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

	t.Run("keeps sending after a failing target", func(t *testing.T) {
		t.Parallel()

		service := newTestService(t, "pushbullet://"+testToken+"/first/second")
		client, captured := newRespondingClient(t,
			cannedResponse{statusCode: http.StatusUnauthorized, body: invalidTokenBody},
			cannedResponse{statusCode: http.StatusOK, body: noteResponse(t, "Second", "Shoutrrr notification")},
		)
		service.SetHTTPClient(client)

		err := service.SendContext(t.Context(), "Second", nil)

		require.Error(t, err)
		assert.Contains(t, err.Error(), `target "first"`)
		assert.NotContains(t, err.Error(), `target "second"`)
		require.Len(t, *captured, 2)
		assert.JSONEq(t, `{"type":"note","title":"Shoutrrr notification","body":"Second","email":"","channel_tag":"","device_iden":"second"}`,
			(*captured)[1].body)
	})

	t.Run("names every failing target", func(t *testing.T) {
		t.Parallel()

		rejected := cannedResponse{statusCode: http.StatusUnauthorized, body: invalidTokenBody}

		service := newTestService(t, "pushbullet://"+testToken+"/first/second")
		client, captured := newRespondingClient(t, rejected, rejected)
		service.SetHTTPClient(client)

		err := service.SendContext(t.Context(), "Message", nil)

		require.Error(t, err)
		assert.Contains(t, err.Error(), `target "first"`)
		assert.Contains(t, err.Error(), `target "second"`)
		assert.Len(t, *captured, 2)
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

		service := newTestService(t, "pushbullet://"+testToken)
		// The mock has no expectations, so a request fails the test.
		service.SetHTTPClient(mocks.NewMockHTTPClient(t))

		require.ErrorIs(t, service.SendContext(ctx, "message", nil), context.Canceled)
	})

	t.Run("canceled between targets", func(t *testing.T) {
		t.Parallel()

		ctx, cancel := context.WithCancel(t.Context())
		t.Cleanup(cancel)

		// The client expects one request, so a request to the second target
		// fails the test.
		client := mocks.NewMockHTTPClient(t)
		client.EXPECT().Do(mock.Anything).RunAndReturn(func(*http.Request) (*http.Response, error) {
			cancel()

			return &http.Response{
				StatusCode: http.StatusOK,
				Body:       io.NopCloser(strings.NewReader(noteResponse(t, "Message", "Shoutrrr notification"))),
			}, nil
		}).Once()

		service := newTestService(t, "pushbullet://"+testToken+"/first/second")
		service.SetHTTPClient(client)

		require.ErrorIs(t, service.SendContext(ctx, "Message", nil), context.Canceled)
	})
}

// TestSendShares verifies that each target's request may use an equal share of
// the time left, so a target that does not answer cannot use up the others'
// time. The first of two gets half of the budget, and the last gets the rest.
func TestSendShares(t *testing.T) {
	t.Parallel()

	synctest.Test(t, func(t *testing.T) {
		note := cannedResponse{
			statusCode: http.StatusOK,
			body:       noteResponse(t, "Message", "Shoutrrr notification"),
		}
		client, captured := newRespondingClient(t, note, note)

		service := newTestService(t, "pushbullet://"+testToken+"/first/second")
		service.SetHTTPClient(client)

		start := time.Now()
		budget := service.ServiceTimeout(nil)

		require.NoError(t, service.Send("Message", nil))
		assert.Equal(t, []time.Time{start.Add(budget / 2), start.Add(budget)},
			requestDeadlines(*captured))
	})
}

// TestServiceTimeout verifies that the send budget is [defaultHTTPTimeout] for
// each target, and at least [defaultHTTPTimeout].
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

// TestSetHTTPClient verifies that sends use the client SetHTTPClient installs,
// or the default client that nil restores, and that the API client built on
// either carries the access token.
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
				assert.Same(t, tt.client, service.client.Client())
			} else {
				assert.IsType(t, &http.Client{}, service.client.Client())
			}

			assert.Equal(t, testToken, service.newJSONClient().Headers().Get("Access-Token"))
		})
	}
}

// requestDeadlines returns the deadline of each captured request.
//
// Parameters:
//   - captured: the captured requests.
//
// Returns:
//   - []time.Time: the deadlines, in order.
func requestDeadlines(captured []capturedRequest) []time.Time {
	deadlines := make([]time.Time, 0, len(captured))
	for _, request := range captured {
		deadlines = append(deadlines, request.deadline)
	}

	return deadlines
}
