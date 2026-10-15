package pagerduty

import (
	"context"
	"errors"
	"io"
	"log"
	"net/http"
	"net/url"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	"github.com/nicholas-fedor/shoutrrr/pkg/format"
	"github.com/nicholas-fedor/shoutrrr/pkg/types"
	"github.com/nicholas-fedor/shoutrrr/pkg/types/mocks"
)

// capturedRequest records the parts of a request that an HTTP client received.
type capturedRequest struct {
	// method is the request method.
	method string
	// url is the full request URL.
	url string
	// contentType is the request's Content-Type header.
	contentType string
	// body is the request body.
	body string
}

// Values shared by the service tests.
const (
	// testServiceURL is a service URL with the PagerDuty events host and port.
	testServiceURL = "pagerduty://events.pagerduty.com:443/" + testIntegrationKey
	// testEndpoint is the events endpoint testServiceURL posts to.
	testEndpoint = "https://events.pagerduty.com:443/v2/enqueue"
	// acceptedBody is the body of an accepted event.
	acceptedBody = `{"status":"success","message":"Event processed","dedup_key":"srv01/HTTP"}`
	// invalidEventBody is the body of a rejected event.
	invalidEventBody = `{"status":"invalid event","message":"Event object is invalid","errors":["Length of 'routing_key' is incorrect (should be 32 characters)"]}`
)

// errTransport is the transport failure returned by the failing HTTP client.
var errTransport = errors.New("transport failure")

// newRespondingClient returns a mock HTTP client that expects one request,
// records it, and answers with the given status code and body.
//
// Parameters:
//   - t: the test that owns the mock.
//   - statusCode: the response status code.
//   - body: the response body.
//
// Returns:
//   - *mocks.MockHTTPClient: the client.
//   - *capturedRequest: the request, filled in once the client receives it.
func newRespondingClient(t *testing.T, statusCode int, body string) (*mocks.MockHTTPClient, *capturedRequest) {
	t.Helper()

	captured := &capturedRequest{}
	client := mocks.NewMockHTTPClient(t)
	client.EXPECT().Do(mock.Anything).RunAndReturn(func(req *http.Request) (*http.Response, error) {
		requestBody, err := io.ReadAll(req.Body)
		require.NoError(t, err)

		*captured = capturedRequest{
			method:      req.Method,
			url:         req.URL.String(),
			contentType: req.Header.Get("Content-Type"),
			body:        string(requestBody),
		}

		return &http.Response{StatusCode: statusCode, Body: io.NopCloser(strings.NewReader(body))}, nil
	}).Once()

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

	assert.Equal(t, "pagerduty", (&Service{}).GetID())
}

// TestServiceTimeout verifies that the router budget for a send is the HTTP
// timeout.
func TestServiceTimeout(t *testing.T) {
	t.Parallel()

	assert.Equal(t, defaultHTTPTimeout, (&Service{}).ServiceTimeout(nil))
}

// TestInitialize verifies that Initialize applies the event defaults before
// the URL, builds a client with the HTTP timeout unless one is set, and
// rejects an invalid integration key.
func TestInitialize(t *testing.T) {
	t.Parallel()

	t.Run("defaults and default client", func(t *testing.T) {
		t.Parallel()

		service := newTestService(t, testServiceURL+"?source=db1")

		assert.Equal(t, &Config{
			IntegrationKey: testIntegrationKey,
			Host:           "events.pagerduty.com",
			Port:           443,
			Severity:       "error",
			Source:         "db1",
			Action:         "trigger",
		}, service.Config)

		client, ok := service.httpClient.(*http.Client)
		require.True(t, ok, "client is a %T", service.httpClient)
		assert.Equal(t, defaultHTTPTimeout, client.Timeout)
	})

	t.Run("default host and port", func(t *testing.T) {
		t.Parallel()

		service := newTestService(t, "pagerduty:///"+testIntegrationKey)

		assert.Equal(t, "events.pagerduty.com", service.Config.Host)
		assert.Equal(t, uint16(443), service.Config.Port)
	})

	t.Run("keeps an injected client", func(t *testing.T) {
		t.Parallel()

		client := mocks.NewMockHTTPClient(t)
		service := &Service{}
		service.SetHTTPClient(client)
		require.NoError(t, service.Initialize(parseTestURL(t, testServiceURL), log.New(t.Output(), "", 0)))

		assert.Same(t, client, service.httpClient)
	})

	t.Run("invalid integration key", func(t *testing.T) {
		t.Parallel()

		err := (&Service{}).Initialize(parseTestURL(t, "pagerduty://events.pagerduty.com:443/invalid-key"), log.New(t.Output(), "", 0))

		require.ErrorIs(t, err, errInvalidIntegrationKey)
	})
}

// TestSend verifies the request a send makes: a JSON POST to the events
// endpoint with the message as the summary, cut to 1024 characters, and the
// severity, source, action, deduplication key, and client from the config and
// params.
func TestSend(t *testing.T) {
	t.Parallel()

	// tests lists service URLs, messages, and params with the request URL and
	// JSON body the send should produce.
	tests := []struct {
		params   *types.Params
		name     string
		rawURL   string
		message  string
		wantURL  string
		wantBody string
	}{
		{
			name:    "defaults",
			rawURL:  testServiceURL,
			message: "hello world",
			wantURL: testEndpoint,
			wantBody: `{"payload":{"summary":"hello world","severity":"error","source":"default"},` +
				`"routing_key":"` + testIntegrationKey + `","event_action":"trigger"}`,
		},
		{
			name:    "documented URL without a host or port",
			rawURL:  "pagerduty:///" + testIntegrationKey,
			message: "hello world",
			wantURL: testEndpoint,
			wantBody: `{"payload":{"summary":"hello world","severity":"error","source":"default"},` +
				`"routing_key":"` + testIntegrationKey + `","event_action":"trigger"}`,
		},
		{
			name:    "documented URL without a port",
			rawURL:  "pagerduty://events.pagerduty.com/" + testIntegrationKey,
			message: "hello world",
			wantURL: testEndpoint,
			wantBody: `{"payload":{"summary":"hello world","severity":"error","source":"default"},` +
				`"routing_key":"` + testIntegrationKey + `","event_action":"trigger"}`,
		},
		{
			name:    "custom host and port",
			rawURL:  "pagerduty://events.eu.pagerduty.com:8443/" + testIntegrationKey,
			message: "hello world",
			wantURL: "https://events.eu.pagerduty.com:8443/v2/enqueue",
			wantBody: `{"payload":{"summary":"hello world","severity":"error","source":"default"},` +
				`"routing_key":"` + testIntegrationKey + `","event_action":"trigger"}`,
		},
		{
			name:    "URL props",
			rawURL:  testServiceURL + "?severity=critical&source=beszel&action=resolve&dedup_key=srv01/HTTP&client=Beszel&client_url=https://beszel.example.com",
			message: "An example alert message",
			wantURL: testEndpoint,
			wantBody: `{"payload":{"summary":"An example alert message","severity":"critical","source":"beszel"},` +
				`"routing_key":"` + testIntegrationKey + `","event_action":"resolve","dedup_key":"srv01/HTTP",` +
				`"client":"Beszel","client_url":"https://beszel.example.com"}`,
		},
		{
			name:    "params",
			rawURL:  testServiceURL + "?severity=critical",
			message: "Disk recovered",
			params:  &types.Params{"severity": "info", "source": "db1", "action": "acknowledge", "dedup_key": "disk-db1"},
			wantURL: testEndpoint,
			wantBody: `{"payload":{"summary":"Disk recovered","severity":"info","source":"db1"},` +
				`"routing_key":"` + testIntegrationKey + `","event_action":"acknowledge","dedup_key":"disk-db1"}`,
		},
		{
			name:    "summary cut to 1024 characters",
			rawURL:  testServiceURL,
			message: strings.Repeat("a", 1100),
			wantURL: testEndpoint,
			wantBody: `{"payload":{"summary":"` + strings.Repeat("a", 1024) + `","severity":"error","source":"default"},` +
				`"routing_key":"` + testIntegrationKey + `","event_action":"trigger"}`,
		},
		{
			name:    "summary cut on a character boundary",
			rawURL:  testServiceURL,
			message: strings.Repeat("é", 1100),
			wantURL: testEndpoint,
			wantBody: `{"payload":{"summary":"` + strings.Repeat("é", 1024) + `","severity":"error","source":"default"},` +
				`"routing_key":"` + testIntegrationKey + `","event_action":"trigger"}`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			service := newTestService(t, tt.rawURL)
			client, captured := newRespondingClient(t, http.StatusAccepted, acceptedBody)
			service.SetHTTPClient(client)

			require.NoError(t, service.Send(tt.message, tt.params))

			assert.Equal(t, http.MethodPost, captured.method)
			assert.Equal(t, tt.wantURL, captured.url)
			assert.Equal(t, "application/json", captured.contentType)
			assert.JSONEq(t, tt.wantBody, captured.body)
		})
	}
}

// TestSendWithContext verifies that the deprecated SendWithContext sends the
// event like SendContext.
func TestSendWithContext(t *testing.T) {
	t.Parallel()

	service := newTestService(t, testServiceURL)
	client, captured := newRespondingClient(t, http.StatusAccepted, acceptedBody)
	service.SetHTTPClient(client)

	require.NoError(t, service.SendWithContext(t.Context(), "hello world", nil))

	assert.Equal(t, testEndpoint, captured.url)
}

// TestSendParamsLeaveConfigUnchanged verifies that params apply to a copy of
// the config for one send, so the stored config is unchanged.
func TestSendParamsLeaveConfigUnchanged(t *testing.T) {
	t.Parallel()

	service := newTestService(t, testServiceURL+"?severity=critical")
	want := *service.Config

	client, _ := newRespondingClient(t, http.StatusAccepted, acceptedBody)
	service.SetHTTPClient(client)

	require.NoError(t, service.Send("message", &types.Params{"severity": "info", "action": "resolve", "dedup_key": "k"}))

	assert.Equal(t, want, *service.Config)
}

// TestNewEventPayloadOptionalFields verifies that the details prop is parsed
// as JSON and the contexts prop into link and image contexts.
func TestNewEventPayloadOptionalFields(t *testing.T) {
	t.Parallel()

	service := newTestService(t, testServiceURL)

	payload, err := service.newEventPayload("message", &types.Params{
		"details":  `{"key":"value","count":2}`,
		"contexts": "link:http://example.com,text:Ignored,image:http://example.com/img.png",
	})

	require.NoError(t, err)
	assert.Equal(t, map[string]any{"key": "value", "count": float64(2)}, payload.Details)
	assert.Equal(t, []PagerDutyContext{
		{Type: "link", Href: "http://example.com"},
		{Type: "image", Src: "http://example.com/img.png"},
	}, payload.Contexts)
}

// TestSendRejectsInvalidEvents verifies that a send rejects params that make
// an invalid event before any request.
func TestSendRejectsInvalidEvents(t *testing.T) {
	t.Parallel()

	// tests lists params with the error each should produce, or only an error
	// when wantErr is nil.
	tests := []struct {
		params  *types.Params
		wantErr error
		name    string
	}{
		{name: "unknown param", params: &types.Params{"foo": "bar"}, wantErr: format.ErrInvalidConfigKey},
		{name: "invalid severity", params: &types.Params{"severity": "fatal"}, wantErr: errInvalidSeverity},
		{name: "invalid action", params: &types.Params{"action": "escalate"}, wantErr: errInvalidEventAction},
		{name: "details not JSON", params: &types.Params{"details": "key=value"}},
		{name: "invalid contexts", params: &types.Params{"contexts": "link"}, wantErr: errInvalidContextFormat},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			service := newTestService(t, testServiceURL)
			// The mock has no expectations, so a request fails the test.
			service.SetHTTPClient(mocks.NewMockHTTPClient(t))

			err := service.Send("message", tt.params)

			require.Error(t, err)

			if tt.wantErr != nil {
				require.ErrorIs(t, err, tt.wantErr)
			}
		})
	}
}

// TestSendErrorResponses verifies that a send fails with
// errPagerDutyNotificationFailed for an error status, reporting the message,
// error, or errors from a JSON body, the raw body otherwise, and the status
// when there is no body.
func TestSendErrorResponses(t *testing.T) {
	t.Parallel()

	// tests lists error responses with the text the error should report.
	tests := []struct {
		name       string
		body       string
		want       string
		statusCode int
	}{
		{name: "message", statusCode: http.StatusBadRequest, body: invalidEventBody, want: "Event object is invalid"},
		{name: "error field", statusCode: http.StatusTooManyRequests, body: `{"error":"rate limited"}`, want: "rate limited"},
		{name: "errors list", statusCode: http.StatusBadRequest, body: `{"errors":["first","second"]}`, want: "first; second"},
		{name: "JSON without a message", statusCode: http.StatusBadRequest, body: `{"status":"invalid event"}`, want: "HTTP 400"},
		{name: "not JSON", statusCode: http.StatusBadGateway, body: "<html>Bad Gateway</html>", want: "<html>Bad Gateway</html>"},
		{name: "empty body", statusCode: http.StatusInternalServerError, body: "", want: "HTTP 500"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			service := newTestService(t, testServiceURL)
			client, _ := newRespondingClient(t, tt.statusCode, tt.body)
			service.SetHTTPClient(client)

			err := service.SendContext(t.Context(), "message", nil)

			require.ErrorIs(t, err, errPagerDutyNotificationFailed)
			assert.Contains(t, err.Error(), tt.want)
		})
	}
}

// TestSendContextErrors verifies that a send wraps a transport error without
// the endpoint path, fails without a client, and stops when the caller's
// context is canceled.
func TestSendContextErrors(t *testing.T) {
	t.Parallel()

	t.Run("transport error", func(t *testing.T) {
		t.Parallel()

		service := newTestService(t, testServiceURL)
		client := mocks.NewMockHTTPClient(t)
		client.EXPECT().Do(mock.Anything).RunAndReturn(func(req *http.Request) (*http.Response, error) {
			return nil, &url.Error{Op: "Post", URL: req.URL.String(), Err: errTransport}
		}).Once()
		service.SetHTTPClient(client)

		err := service.SendContext(t.Context(), "message", nil)

		require.ErrorIs(t, err, errTransport)
		assert.NotContains(t, err.Error(), "/v2/enqueue")
	})

	t.Run("no client", func(t *testing.T) {
		t.Parallel()

		service := newTestService(t, testServiceURL)
		service.httpClient = nil

		require.ErrorIs(t, service.SendContext(t.Context(), "message", nil), errServiceNotInitialized)
	})

	t.Run("canceled context", func(t *testing.T) {
		t.Parallel()

		ctx, cancel := context.WithCancel(t.Context())
		cancel()

		client := mocks.NewMockHTTPClient(t)
		client.EXPECT().Do(mock.Anything).RunAndReturn(func(req *http.Request) (*http.Response, error) {
			return nil, req.Context().Err()
		}).Once()

		service := newTestService(t, "pagerduty://events.example.invalid:443/"+testIntegrationKey)
		service.SetHTTPClient(client)

		require.ErrorIs(t, service.SendContext(ctx, "message", nil), context.Canceled)
	})
}

// TestSendRedactsUnparsableURL verifies that a host that makes the events URL
// unparsable fails the send before any request, without the host in the
// error.
func TestSendRedactsUnparsableURL(t *testing.T) {
	t.Parallel()

	service := newTestService(t, testServiceURL)
	service.Config.Host = "SECRET host"
	// The mock has no expectations, so a request fails the test.
	service.SetHTTPClient(mocks.NewMockHTTPClient(t))

	err := service.Send("message", nil)

	_, isURLErr := errors.AsType[*url.Error](err)
	require.True(t, isURLErr, "error %v is not a *url.Error", err)
	assert.NotContains(t, err.Error(), "SECRET")
}

// TestSetHTTPClient verifies that SetHTTPClient stores a custom client and
// selects a client with the HTTP timeout for nil.
func TestSetHTTPClient(t *testing.T) {
	t.Parallel()

	// tests lists constructors for the clients to set, which build any mock
	// with the subtest's t, and whether the default client should replace them.
	tests := []struct {
		client      func(t *testing.T) types.HTTPClient
		name        string
		wantDefault bool
	}{
		{name: "custom client", client: func(t *testing.T) types.HTTPClient {
			t.Helper()

			return mocks.NewMockHTTPClient(t)
		}},
		{name: "nil", client: func(*testing.T) types.HTTPClient { return nil }, wantDefault: true},
		{name: "nil *http.Client", client: func(*testing.T) types.HTTPClient { return (*http.Client)(nil) }, wantDefault: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			client := tt.client(t)
			service := &Service{}
			service.SetHTTPClient(client)

			if !tt.wantDefault {
				assert.Same(t, client, service.httpClient)

				return
			}

			defaultClient, ok := service.httpClient.(*http.Client)
			require.True(t, ok, "client is a %T", service.httpClient)
			assert.Equal(t, defaultHTTPTimeout, defaultClient.Timeout)
		})
	}
}

// TestParseContexts verifies that parseContexts reads a JSON array of link
// and image contexts, or the comma-separated type:value format, where text
// contexts and empty entries are skipped.
func TestParseContexts(t *testing.T) {
	t.Parallel()

	// tests lists context props with the contexts each should produce.
	tests := []struct {
		name  string
		input string
		want  []PagerDutyContext
	}{
		{name: "empty", input: "", want: nil},
		{name: "link", input: "link:http://example.com", want: []PagerDutyContext{{Type: "link", Href: "http://example.com"}}},
		{name: "image", input: "image:http://example.com/img.png", want: []PagerDutyContext{{Type: "image", Src: "http://example.com/img.png"}}},
		{name: "text only", input: "text:Some text", want: []PagerDutyContext{}},
		{
			name:  "several with spaces and empty entries",
			input: " link:http://example.com , ,image: http://example.com/img.png,",
			want: []PagerDutyContext{
				{Type: "link", Href: "http://example.com"},
				{Type: "image", Src: "http://example.com/img.png"},
			},
		},
		{
			name:  "JSON array",
			input: `[{"type":"link","href":"http://example.com","text":"Dashboard"},{"type":"image","src":"http://example.com/img.png"}]`,
			want: []PagerDutyContext{
				{Type: "link", Href: "http://example.com", Text: "Dashboard"},
				{Type: "image", Src: "http://example.com/img.png"},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			contexts, err := parseContexts(tt.input)

			require.NoError(t, err)
			assert.Equal(t, tt.want, contexts)
		})
	}
}

// TestParseContextsErrors verifies that parseContexts rejects malformed and
// unsupported contexts in both formats.
func TestParseContextsErrors(t *testing.T) {
	t.Parallel()

	// tests lists invalid context props with the error each should produce.
	tests := []struct {
		wantErr error
		name    string
		input   string
	}{
		{name: "missing colon", input: "link", wantErr: errInvalidContextFormat},
		{name: "empty type", input: ":http://example.com", wantErr: errEmptyContextTypeOrValue},
		{name: "empty value", input: "link: ", wantErr: errEmptyContextTypeOrValue},
		{name: "unsupported type", input: "video:http://example.com", wantErr: errInvalidContextFormat},
		{name: "JSON with an unsupported type", input: `[{"type":"text","text":"x"}]`, wantErr: errInvalidContextType},
		{name: "JSON link without href", input: `[{"type":"link"}]`, wantErr: errMissingHrefForLinkContext},
		{name: "JSON image without src", input: `[{"type":"image"}]`, wantErr: errMissingSrcForImageContext},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			contexts, err := parseContexts(tt.input)

			require.ErrorIs(t, err, tt.wantErr)
			assert.Nil(t, contexts)
		})
	}
}

// TestValidateSeverityAndAction verifies that only the severities and event
// actions the Events API v2 defines are accepted.
func TestValidateSeverityAndAction(t *testing.T) {
	t.Parallel()

	for _, severity := range []string{"critical", "error", "warning", "info"} {
		require.NoError(t, validateSeverity(severity), severity)
	}

	for _, severity := range []string{"", "fatal", "Critical"} {
		require.ErrorIs(t, validateSeverity(severity), errInvalidSeverity, severity)
	}

	for _, action := range []string{"trigger", "acknowledge", "resolve"} {
		require.NoError(t, validateEventAction(action), action)
	}

	for _, action := range []string{"", "escalate", "Trigger"} {
		require.ErrorIs(t, validateEventAction(action), errInvalidEventAction, action)
	}
}
