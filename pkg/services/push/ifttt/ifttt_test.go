package ifttt

import (
	"context"
	"errors"
	"io"
	"log"
	"net/http"
	"net/url"
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
	// body is the request body.
	body string
}

// cannedResponse is a response the fake webhook returns.
type cannedResponse struct {
	// status is the response status line.
	status string
	// statusCode is the response status code.
	statusCode int
}

// okResponse is the response the webhook returns for a fired event.
var okResponse = cannedResponse{status: "200 OK", statusCode: http.StatusOK}

// errTransport is the transport failure returned by the failing HTTP client.
var errTransport = errors.New("transport failure")

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
			body:        string(body),
		})

		return &http.Response{
			Status:     response.status,
			StatusCode: response.statusCode,
			Body:       io.NopCloser(strings.NewReader("")),
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

// TestGetID verifies that the service identifies itself by its scheme.
func TestGetID(t *testing.T) {
	t.Parallel()

	assert.Equal(t, "ifttt", (&Service{}).GetID())
}

// TestInitialize verifies that Initialize parses the service URL with the
// message in the second value by default, and rejects an invalid URL.
func TestInitialize(t *testing.T) {
	t.Parallel()

	t.Run("valid URL", func(t *testing.T) {
		t.Parallel()

		service := newTestService(t, "ifttt://dummyID/?events=event1")

		assert.Equal(t, &Config{WebHookID: "dummyID", Events: []string{"event1"}, UseMessageAsValue: DefaultMessageValue}, service.Config)
	})

	t.Run("missing events", func(t *testing.T) {
		t.Parallel()

		err := (&Service{}).Initialize(parseTestURL(t, "ifttt://dummyID"), log.New(t.Output(), "", 0))

		require.ErrorIs(t, err, ErrMissingEvents)
	})
}

// TestSend verifies the requests a send makes: one JSON POST to the event's
// webhook URL for each event, in order, carrying the same payload.
func TestSend(t *testing.T) {
	t.Parallel()

	// tests lists service URLs and params with the webhook URLs and payload the
	// send should produce.
	tests := []struct {
		params   *types.Params
		name     string
		rawURL   string
		wantURLs []string
		wantBody string
	}{
		{
			name:     "one event",
			rawURL:   "ifttt://dummy/?events=foo",
			wantURLs: []string{"https://maker.ifttt.com/trigger/foo/with/key/dummy"},
			wantBody: `{"value1":"","value2":"hello","value3":""}`,
		},
		{
			name:   "two events",
			rawURL: "ifttt://dummy/?events=event1,event2&messagevalue=1&value3=c",
			wantURLs: []string{
				"https://maker.ifttt.com/trigger/event1/with/key/dummy",
				"https://maker.ifttt.com/trigger/event2/with/key/dummy",
			},
			wantBody: `{"value1":"hello","value2":"","value3":"c"}`,
		},
		{
			name:     "configured title",
			rawURL:   "ifttt://dummy/?events=event1&titlevalue=1&title=Configured",
			wantURLs: []string{"https://maker.ifttt.com/trigger/event1/with/key/dummy"},
			wantBody: `{"value1":"Configured","value2":"hello","value3":""}`,
		},
		{
			name:     "title param",
			rawURL:   "ifttt://dummy/?events=event1&titlevalue=3&title=Configured",
			params:   &types.Params{"title": "Override"},
			wantURLs: []string{"https://maker.ifttt.com/trigger/event1/with/key/dummy"},
			wantBody: `{"value1":"","value2":"hello","value3":"Override"}`,
		},
		{
			name:     "params",
			rawURL:   "ifttt://dummy/?events=event1",
			params:   &types.Params{"messagevalue": "3", "value1": "x", "events": "other"},
			wantURLs: []string{"https://maker.ifttt.com/trigger/other/with/key/dummy"},
			wantBody: `{"value1":"x","value2":"","value3":"hello"}`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			responses := make([]cannedResponse, len(tt.wantURLs))
			for i := range responses {
				responses[i] = okResponse
			}

			service := newTestService(t, tt.rawURL)
			client, captured := newRespondingClient(t, responses...)
			service.SetHTTPClient(client)

			require.NoError(t, service.Send("hello", tt.params))

			require.Len(t, *captured, len(tt.wantURLs))

			for i, request := range *captured {
				assert.Equal(t, http.MethodPost, request.method)
				assert.Equal(t, tt.wantURLs[i], request.url)
				assert.Equal(t, "application/json", request.contentType)
				assert.JSONEq(t, tt.wantBody, request.body)
			}
		})
	}
}

// TestSendParamsLeaveConfigUnchanged verifies that params apply to a copy of
// the config for one send, so the stored config and the next send keep the
// configured values.
func TestSendParamsLeaveConfigUnchanged(t *testing.T) {
	t.Parallel()

	service := newTestService(t, "ifttt://dummy/?events=event1")
	want := *service.Config

	client, _ := newRespondingClient(t, okResponse)
	service.SetHTTPClient(client)

	require.NoError(t, service.Send("hello", &types.Params{"messagevalue": "1", "events": "other"}))
	assert.Equal(t, want, *service.Config)

	client, captured := newRespondingClient(t, okResponse)
	service.SetHTTPClient(client)

	require.NoError(t, service.Send("hello", nil))
	require.Len(t, *captured, 1)
	assert.Equal(t, "https://maker.ifttt.com/trigger/event1/with/key/dummy", (*captured)[0].url)
	assert.JSONEq(t, `{"value1":"","value2":"hello","value3":""}`, (*captured)[0].body)
}

// TestSendContextErrors verifies that a send fails with ErrSendFailed naming
// the event for an error status or a transport error, keeps sending to the
// events after a failing one, rejects an invalid param before any request,
// sends nothing when the caller's context is already canceled, and skips the
// remaining events once it ends.
func TestSendContextErrors(t *testing.T) {
	t.Parallel()

	t.Run("error status", func(t *testing.T) {
		t.Parallel()

		service := newTestService(t, "ifttt://dummy/?events=foo")
		client, _ := newRespondingClient(t, cannedResponse{status: "401 Unauthorized", statusCode: http.StatusUnauthorized})
		service.SetHTTPClient(client)

		err := service.SendContext(t.Context(), "hello", nil)

		require.ErrorIs(t, err, ErrSendFailed)
		require.ErrorIs(t, err, ErrUnexpectedStatus)
		assert.EqualError(t, err, `failed to send IFTTT event: event "foo": got unexpected response status code: 401 Unauthorized`)
	})

	t.Run("second event fails", func(t *testing.T) {
		t.Parallel()

		service := newTestService(t, "ifttt://dummy/?events=event1,event2,event3")
		client, captured := newRespondingClient(t, okResponse, cannedResponse{status: "404 Not Found", statusCode: http.StatusNotFound}, okResponse)
		service.SetHTTPClient(client)

		err := service.SendContext(t.Context(), "hello", nil)

		require.ErrorIs(t, err, ErrUnexpectedStatus)
		assert.Contains(t, err.Error(), `event "event2"`)
		assert.NotContains(t, err.Error(), `event "event1"`)
		assert.NotContains(t, err.Error(), `event "event3"`)
		assert.Len(t, *captured, 3)
	})

	t.Run("two events fail", func(t *testing.T) {
		t.Parallel()

		notFound := cannedResponse{status: "404 Not Found", statusCode: http.StatusNotFound}

		service := newTestService(t, "ifttt://dummy/?events=event1,event2,event3")
		client, captured := newRespondingClient(t, notFound, okResponse, notFound)
		service.SetHTTPClient(client)

		err := service.SendContext(t.Context(), "hello", nil)

		require.ErrorIs(t, err, ErrUnexpectedStatus)
		assert.Contains(t, err.Error(), `event "event1"`)
		assert.Contains(t, err.Error(), `event "event3"`)
		assert.NotContains(t, err.Error(), `event "event2"`)
		assert.Len(t, *captured, 3)
	})

	t.Run("transport error", func(t *testing.T) {
		t.Parallel()

		service := newTestService(t, "ifttt://SECRETwebhookID/?events=event1")
		client := mocks.NewMockHTTPClient(t)
		client.EXPECT().Do(mock.Anything).RunAndReturn(func(req *http.Request) (*http.Response, error) {
			return nil, &url.Error{Op: "Post", URL: req.URL.String(), Err: errTransport}
		}).Once()
		service.SetHTTPClient(client)

		err := service.SendContext(t.Context(), "hello", nil)

		require.ErrorIs(t, err, ErrSendFailed)
		require.ErrorIs(t, err, errTransport)
		assert.NotContains(t, err.Error(), "SECRETwebhookID")
	})

	t.Run("invalid param", func(t *testing.T) {
		t.Parallel()

		service := newTestService(t, "ifttt://dummy/?events=event1")
		// The mock has no expectations, so a request fails the test.
		service.SetHTTPClient(mocks.NewMockHTTPClient(t))

		err := service.SendContext(t.Context(), "hello", &types.Params{"foo": "bar"})

		require.ErrorIs(t, err, format.ErrInvalidConfigKey)
	})

	// valueFieldParams lists value field params that the URL would reject, with
	// the error the send should return before any request.
	valueFieldParams := []struct {
		params  *types.Params
		wantErr error
		name    string
	}{
		{name: "messagevalue above 3", params: &types.Params{"messagevalue": "5"}, wantErr: ErrInvalidMessageValue},
		{name: "messagevalue zero", params: &types.Params{"messagevalue": "0"}, wantErr: ErrInvalidMessageValue},
		{name: "titlevalue above 3", params: &types.Params{"titlevalue": "4"}, wantErr: ErrInvalidTitleValue},
		{name: "titlevalue equals messagevalue", params: &types.Params{"titlevalue": "2"}, wantErr: ErrTitleMessageConflict},
	}

	for _, tt := range valueFieldParams {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			service := newTestService(t, "ifttt://dummy/?events=event1")
			// The mock has no expectations, so a request fails the test.
			service.SetHTTPClient(mocks.NewMockHTTPClient(t))

			require.ErrorIs(t, service.SendContext(t.Context(), "hello", tt.params), tt.wantErr)
		})
	}

	t.Run("canceled context", func(t *testing.T) {
		t.Parallel()

		ctx, cancel := context.WithCancel(t.Context())
		cancel()

		service := newTestService(t, "ifttt://webhookid/?events=event")
		// The mock has no expectations, so a request fails the test.
		service.SetHTTPClient(mocks.NewMockHTTPClient(t))

		require.ErrorIs(t, service.SendContext(ctx, "message", nil), context.Canceled)
	})

	t.Run("canceled between events", func(t *testing.T) {
		t.Parallel()

		ctx, cancel := context.WithCancel(t.Context())
		t.Cleanup(cancel)

		// The client expects one request, so a request for the second event
		// fails the test.
		client := mocks.NewMockHTTPClient(t)
		client.EXPECT().Do(mock.Anything).RunAndReturn(func(*http.Request) (*http.Response, error) {
			cancel()

			return &http.Response{Status: "200 OK", StatusCode: http.StatusOK, Body: io.NopCloser(strings.NewReader(""))}, nil
		}).Once()

		service := newTestService(t, "ifttt://webhookid/?events=event1,event2")
		service.SetHTTPClient(client)

		require.ErrorIs(t, service.SendContext(ctx, "message", nil), context.Canceled)
	})
}

// TestSendRedactsUnparsableURL verifies that an event that makes the webhook
// URL unparsable fails the send before any request, without the webhook ID in
// the error.
func TestSendRedactsUnparsableURL(t *testing.T) {
	t.Parallel()

	service := newTestService(t, "ifttt://SECRETwebhookID/?events=%25zz")
	// The mock has no expectations, so a request fails the test.
	service.SetHTTPClient(mocks.NewMockHTTPClient(t))

	err := service.Send("message", nil)

	_, isURLErr := errors.AsType[*url.Error](err)
	require.True(t, isURLErr, "error %v is not a *url.Error", err)
	assert.NotContains(t, err.Error(), "SECRETwebhookID")
}

// TestSendDefaultClient verifies that a send without a custom client, or with
// a nil *http.Client, uses a default client that honors the caller's context.
// The context is canceled before the send, so the transport returns before it
// dials IFTTT.
func TestSendDefaultClient(t *testing.T) {
	t.Parallel()

	// tests lists how each case sets up the service's HTTP client.
	tests := []struct {
		configure func(*Service)
		name      string
	}{
		{name: "no client set"},
		{name: "nil *http.Client", configure: func(s *Service) { s.SetHTTPClient((*http.Client)(nil)) }},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			service := newTestService(t, "ifttt://SECRETwebhookID/?events=event1")

			if tt.configure != nil {
				tt.configure(service)
			}

			ctx, cancel := context.WithCancel(t.Context())
			cancel()

			err := service.SendContext(ctx, "hello", nil)

			require.ErrorIs(t, err, context.Canceled)
			assert.NotContains(t, err.Error(), "SECRETwebhookID")
		})
	}
}

// TestSendShares verifies that each event's request may use an equal share of
// the time left, so an event that does not answer cannot use up the others'
// time. The first of two gets half of the budget, and the last gets the rest.
func TestSendShares(t *testing.T) {
	t.Parallel()

	synctest.Test(t, func(t *testing.T) {
		ok := cannedResponse{status: "200 OK", statusCode: http.StatusOK}
		client, captured := newRespondingClient(t, ok, ok)

		service := newTestService(t, "ifttt://webhookid/?events=a,b")
		service.SetHTTPClient(client)

		start := time.Now()
		budget := service.ServiceTimeout(nil)

		require.NoError(t, service.Send("Message", nil))
		assert.Equal(t, []time.Time{start.Add(budget / 2), start.Add(budget)},
			requestDeadlines(*captured))
	})
}

// TestSendSharesFromParams verifies that events set by params set the send's
// budget. Two events from params share the budget for two events, though the
// service URL names one.
func TestSendSharesFromParams(t *testing.T) {
	t.Parallel()

	synctest.Test(t, func(t *testing.T) {
		ok := cannedResponse{status: "200 OK", statusCode: http.StatusOK}
		client, captured := newRespondingClient(t, ok, ok)

		service := newTestService(t, "ifttt://webhookid/?events=a")
		service.SetHTTPClient(client)

		params := types.Params{"events": "a,b"}
		start := time.Now()
		budget := service.ServiceTimeout(&params)

		require.NoError(t, service.Send("Message", &params))
		assert.Equal(t, []time.Time{start.Add(budget / 2), start.Add(budget)},
			requestDeadlines(*captured))
	})
}

// TestServiceTimeout verifies that the send budget is [defaultTimeout] for
// each event, counting events set by params, and at least [defaultTimeout].
func TestServiceTimeout(t *testing.T) {
	t.Parallel()

	// tests lists params with the budget each should get for a service with
	// three configured events.
	tests := []struct {
		params *types.Params
		name   string
		want   time.Duration
	}{
		{name: "configured events", want: 3 * defaultTimeout},
		{name: "events set by params", params: &types.Params{"events": "a,b"}, want: 2 * defaultTimeout},
		{name: "invalid param keeps the configured events", params: &types.Params{"messagevalue": "abc"}, want: 3 * defaultTimeout},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			service := newTestService(t, "ifttt://webhookid/?events=a,b,c")

			assert.Equal(t, tt.want, service.ServiceTimeout(tt.params))
		})
	}

	t.Run("uninitialized service", func(t *testing.T) {
		t.Parallel()

		assert.Equal(t, defaultTimeout, (&Service{}).ServiceTimeout(nil))
	})
}

// TestDefaultHTTPClient verifies that the default client has no request
// timeout, since the send budget bounds each send, and sends through
// http.DefaultTransport, so it honors the proxy environment variables.
func TestDefaultHTTPClient(t *testing.T) {
	t.Parallel()

	service := newTestService(t, "ifttt://dummyID/?events=event1")

	httpClient, ok := service.client.Client().(*http.Client)
	require.True(t, ok)
	assert.Zero(t, httpClient.Timeout)
	assert.Nil(t, httpClient.Transport)
}

// TestSetHTTPClient verifies that SetHTTPClient stores a custom client and
// clears it for a nil *http.Client.
func TestSetHTTPClient(t *testing.T) {
	t.Parallel()

	t.Run("custom client", func(t *testing.T) {
		t.Parallel()

		client := mocks.NewMockHTTPClient(t)
		service := &Service{}
		service.SetHTTPClient(client)

		assert.Same(t, client, service.client.Client())
	})

	t.Run("nil *http.Client", func(t *testing.T) {
		t.Parallel()

		service := &Service{}
		service.SetHTTPClient(mocks.NewMockHTTPClient(t))
		service.SetHTTPClient((*http.Client)(nil))

		assert.IsType(t, &http.Client{}, service.client.Client())
	})
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
