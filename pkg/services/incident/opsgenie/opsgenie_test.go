package opsgenie

import (
	"context"
	"encoding/json/v2"
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
	// authorization is the request's Authorization header.
	authorization string
	// contentType is the request's Content-Type header.
	contentType string
	// body is the request body.
	body string
}

// failingReader is a response body whose reads fail.
type failingReader struct{}

// Response bodies in the format the OpsGenie API returns.
const (
	// acceptedBody is the body of an accepted create alert request.
	acceptedBody = `{"result":"Request will be processed","took":0.302,"requestId":"43a29c5c-3dbf-4fa4-9c26-f4f71023e120"}`
	// unauthorizedBody is the body of a request with an invalid API key.
	unauthorizedBody = `{"message":"Could not authenticate","took":0.0,"requestId":"5042853c-402d-4a18-abcb-168734a801de"}`
)

// Errors returned by the failing test doubles.
var (
	// errTransport is the transport failure returned by the failing HTTP client.
	errTransport = errors.New("transport failure")
	// errRead is the read failure returned by failingReader.
	errRead = errors.New("read failure")
)

// Read fails with errRead.
//
// Parameters:
//   - p: the buffer, which stays unchanged.
//
// Returns:
//   - int: zero.
//   - error: errRead.
func (failingReader) Read([]byte) (int, error) {
	return 0, errRead
}

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
			method:        req.Method,
			url:           req.URL.String(),
			authorization: req.Header.Get("Authorization"),
			contentType:   req.Header.Get("Content-Type"),
			body:          string(requestBody),
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

	assert.Equal(t, "opsgenie", (&Service{}).GetID())
}

// TestInitialize verifies that Initialize parses the service URL and rejects
// a URL without an API key.
func TestInitialize(t *testing.T) {
	t.Parallel()

	t.Run("valid URL", func(t *testing.T) {
		t.Parallel()

		service := newTestService(t, "opsgenie://api.opsgenie.com/"+testAPIKey+"?priority=P2")

		assert.Equal(t, &Config{Host: "api.opsgenie.com", Port: 443, APIKey: testAPIKey, Priority: "P2"}, service.Config)
	})

	t.Run("missing API key", func(t *testing.T) {
		t.Parallel()

		err := (&Service{}).Initialize(parseTestURL(t, "opsgenie://api.opsgenie.com"), log.New(t.Output(), "", 0))

		require.ErrorIs(t, err, ErrAPIKeyMissing)
	})
}

// TestSend verifies the request a send makes: an authenticated JSON POST to
// the alerts endpoint, with the title as the alert message and the message
// as its description, or the message alone when it fits and no title is set.
func TestSend(t *testing.T) {
	t.Parallel()

	longMessage := strings.Repeat("a", MaxMessageLength) + "tail"

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
			name:     "message only",
			rawURL:   "opsgenie://api.opsgenie.com/" + testAPIKey,
			message:  "hello world",
			wantURL:  "https://api.opsgenie.com:443/v2/alerts",
			wantBody: `{"message":"hello world"}`,
		},
		{
			name:     "omitted host",
			rawURL:   "opsgenie:///" + testAPIKey,
			message:  "hello world",
			wantURL:  "https://api.opsgenie.com:443/v2/alerts",
			wantBody: `{"message":"hello world"}`,
		},
		{
			name:     "IPv6 host and port",
			rawURL:   "opsgenie://[2001:db8::1]:8443/" + testAPIKey,
			message:  "hello world",
			wantURL:  "https://[2001:db8::1]:8443/v2/alerts",
			wantBody: `{"message":"hello world"}`,
		},
		{
			name:     "IPv6 host with a zone",
			rawURL:   "opsgenie://[fe80::1%25en0]:8443/" + testAPIKey,
			message:  "hello world",
			wantURL:  "https://[fe80::1%25en0]:8443/v2/alerts",
			wantBody: `{"message":"hello world"}`,
		},
		{
			name:     "EU host and port",
			rawURL:   "opsgenie://api.eu.opsgenie.com:8443/" + testAPIKey,
			message:  "hello world",
			wantURL:  "https://api.eu.opsgenie.com:8443/v2/alerts",
			wantBody: `{"message":"hello world"}`,
		},
		{
			name:     "long message",
			rawURL:   "opsgenie://api.opsgenie.com/" + testAPIKey,
			message:  longMessage,
			wantURL:  "https://api.opsgenie.com:443/v2/alerts",
			wantBody: `{"message":"` + longMessage[:MaxMessageLength] + `","description":"` + longMessage + `"}`,
		},
		{
			name:     "multibyte message within the limit",
			rawURL:   "opsgenie://api.opsgenie.com/" + testAPIKey,
			message:  strings.Repeat("é", MaxMessageLength),
			wantURL:  "https://api.opsgenie.com:443/v2/alerts",
			wantBody: `{"message":"` + strings.Repeat("é", MaxMessageLength) + `"}`,
		},
		{
			name:     "long multibyte message",
			rawURL:   "opsgenie://api.opsgenie.com/" + testAPIKey,
			message:  strings.Repeat("é", MaxMessageLength+1),
			wantURL:  "https://api.opsgenie.com:443/v2/alerts",
			wantBody: `{"message":"` + strings.Repeat("é", MaxMessageLength) + `","description":"` + strings.Repeat("é", MaxMessageLength+1) + `"}`,
		},
		{
			name:     "long title",
			rawURL:   "opsgenie://api.opsgenie.com/" + testAPIKey,
			message:  "Disk usage at 95%",
			params:   &types.Params{"title": strings.Repeat("t", MaxMessageLength+10)},
			wantURL:  "https://api.opsgenie.com:443/v2/alerts",
			wantBody: `{"message":"` + strings.Repeat("t", MaxMessageLength) + `","description":"Disk usage at 95%"}`,
		},
		{
			name:     "escalation and schedule responders by name",
			rawURL:   "opsgenie://api.opsgenie.com/" + testAPIKey,
			message:  "hello world",
			params:   &types.Params{"responders": "escalation:Nightwatch Escalation,schedule:First Responders Schedule"},
			wantURL:  "https://api.opsgenie.com:443/v2/alerts",
			wantBody: `{"message":"hello world","responders":[{"type":"escalation","name":"Nightwatch Escalation"},{"type":"schedule","name":"First Responders Schedule"}]}`,
		},
		{
			name:     "title param",
			rawURL:   "opsgenie://api.opsgenie.com/" + testAPIKey,
			message:  "Disk usage at 95%",
			params:   &types.Params{"title": "Disk full"},
			wantURL:  "https://api.opsgenie.com:443/v2/alerts",
			wantBody: `{"message":"Disk full","description":"Disk usage at 95%"}`,
		},
		{
			name:     "title and configured description",
			rawURL:   "opsgenie://api.opsgenie.com/" + testAPIKey + "?title=Disk+full&description=Host+db1",
			message:  "Disk usage at 95%",
			wantURL:  "https://api.opsgenie.com:443/v2/alerts",
			wantBody: `{"message":"Disk full","description":"Disk usage at 95%\nHost db1"}`,
		},
		{
			name:     "short message and configured description",
			rawURL:   "opsgenie://api.opsgenie.com/" + testAPIKey + "?description=Host+db1",
			message:  "Disk full",
			wantURL:  "https://api.opsgenie.com:443/v2/alerts",
			wantBody: `{"message":"Disk full","description":"Host db1"}`,
		},
		{
			name:    "alert props from params",
			rawURL:  "opsgenie://api.opsgenie.com/" + testAPIKey,
			message: "An example alert message",
			params: &types.Params{
				"alias":       "Life is too short for no alias",
				"description": "Every alert needs a description",
				"responders":  "team:" + testTeamID + ",team:NOC,user:Donald,user:696f0759-3b0f-4a15-b8c8-19d3dfca33f2",
				"visibleTo":   "team:rocket",
				"actions":     "action1,action2",
				"tags":        "tag1,tag2",
				"details":     "key1:value1,key2:value2",
				"entity":      "An example entity",
				"source":      "The source",
				"priority":    "P1",
				"user":        "Dracula",
				"note":        "Here is a note",
			},
			wantURL: "https://api.opsgenie.com:443/v2/alerts",
			wantBody: `{"message":"An example alert message","alias":"Life is too short for no alias",` +
				`"description":"Every alert needs a description",` +
				`"responders":[{"type":"team","id":"` + testTeamID + `"},{"type":"team","name":"NOC"},` +
				`{"type":"user","username":"Donald"},{"type":"user","id":"696f0759-3b0f-4a15-b8c8-19d3dfca33f2"}],` +
				`"visibleTo":[{"type":"team","name":"rocket"}],"actions":["action1","action2"],"tags":["tag1","tag2"],` +
				`"details":{"key1":"value1","key2":"value2"},"entity":"An example entity","source":"The source",` +
				`"priority":"P1","user":"Dracula","note":"Here is a note"}`,
		},
		{
			name:     "details with colons in a value",
			rawURL:   "opsgenie://api.opsgenie.com/" + testAPIKey,
			message:  "Disk full",
			params:   &types.Params{"details": "runbook:https://wiki.example.com/disk,owner:ops"},
			wantURL:  "https://api.opsgenie.com:443/v2/alerts",
			wantBody: `{"message":"Disk full","details":{"runbook":"https://wiki.example.com/disk","owner":"ops"}}`,
		},
		{
			name: "alert props from the URL",
			rawURL: "opsgenie://api.opsgenie.com/" + testAPIKey +
				"?alias=query-alias&description=query-description&responders=team:query_team&visibleTo=user:query_user" +
				"&actions=queryAction1,queryAction2&tags=queryTag1,queryTag2&details=queryKey1:queryValue1" +
				"&entity=query-entity&source=query-source&priority=P2&user=query-user&note=query-note",
			message: "An example alert message",
			wantURL: "https://api.opsgenie.com:443/v2/alerts",
			wantBody: `{"message":"An example alert message","alias":"query-alias","description":"query-description",` +
				`"responders":[{"type":"team","name":"query_team"}],"visibleTo":[{"type":"user","username":"query_user"}],` +
				`"actions":["queryAction1","queryAction2"],"tags":["queryTag1","queryTag2"],` +
				`"details":{"queryKey1":"queryValue1"},"entity":"query-entity","source":"query-source",` +
				`"priority":"P2","user":"query-user","note":"query-note"}`,
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
			assert.Equal(t, "GenieKey "+testAPIKey, captured.authorization)
			assert.Equal(t, "application/json", captured.contentType)
			assert.JSONEq(t, tt.wantBody, captured.body)
		})
	}
}

// TestSendChecksConfig verifies that a send checks a config built in code as
// reading a URL does. A config without an API key fails before any request,
// and one without a host or port posts to the default API.
func TestSendChecksConfig(t *testing.T) {
	t.Parallel()

	t.Run("no API key", func(t *testing.T) {
		t.Parallel()

		service := newTestService(t, "opsgenie://api.opsgenie.com/"+testAPIKey)
		// The mock has no expectations, so a request fails the test.
		service.SetHTTPClient(mocks.NewMockHTTPClient(t))
		*service.Config = Config{}

		require.ErrorIs(t, service.Send("Message", nil), ErrAPIKeyMissing)
	})

	t.Run("no host or port", func(t *testing.T) {
		t.Parallel()

		service := newTestService(t, "opsgenie://api.opsgenie.com/"+testAPIKey)
		client, captured := newRespondingClient(t, http.StatusAccepted, "")
		service.SetHTTPClient(client)
		service.Config.Host = ""
		service.Config.Port = 0

		require.NoError(t, service.Send("Message", nil))
		assert.Equal(t, "https://api.opsgenie.com:443/v2/alerts", captured.url)
	})
}

// TestSendParamsLeaveConfigUnchanged verifies that params apply to a copy of
// the config for one send, so the next send uses the configured props.
func TestSendParamsLeaveConfigUnchanged(t *testing.T) {
	t.Parallel()

	service := newTestService(t, "opsgenie://api.opsgenie.com/"+testAPIKey+"?alias=query-alias&responders=team:query_team&priority=P2")
	want := *service.Config

	client, _ := newRespondingClient(t, http.StatusAccepted, acceptedBody)
	service.SetHTTPClient(client)

	require.NoError(t, service.Send("1", &types.Params{"alias": "1", "responders": "team:1", "priority": "P1"}))
	assert.Equal(t, want, *service.Config)

	client, captured := newRespondingClient(t, http.StatusAccepted, acceptedBody)
	service.SetHTTPClient(client)

	require.NoError(t, service.Send("2", nil))
	assert.JSONEq(t,
		`{"message":"2","alias":"query-alias","responders":[{"type":"team","name":"query_team"}],"priority":"P2"}`,
		captured.body)
}

// TestSendContextErrors verifies that a send fails with ErrUnexpectedStatus
// and the response body for an error status, wraps a transport error, rejects
// an invalid param before any request, and stops when the caller's context is
// canceled.
func TestSendContextErrors(t *testing.T) {
	t.Parallel()

	t.Run("error status", func(t *testing.T) {
		t.Parallel()

		service := newTestService(t, "opsgenie://api.opsgenie.com/"+testAPIKey)
		client, _ := newRespondingClient(t, http.StatusUnauthorized, unauthorizedBody)
		service.SetHTTPClient(client)

		err := service.SendContext(t.Context(), "message", nil)

		require.ErrorIs(t, err, ErrUnexpectedStatus)
		assert.Contains(t, err.Error(), "401")
		assert.Contains(t, err.Error(), "Could not authenticate")
	})

	t.Run("unreadable error body", func(t *testing.T) {
		t.Parallel()

		service := newTestService(t, "opsgenie://api.opsgenie.com/"+testAPIKey)
		client := mocks.NewMockHTTPClient(t)
		client.EXPECT().Do(mock.Anything).
			Return(&http.Response{StatusCode: http.StatusBadGateway, Body: io.NopCloser(failingReader{})}, nil).Once()
		service.SetHTTPClient(client)

		err := service.SendContext(t.Context(), "message", nil)

		require.ErrorIs(t, err, ErrUnexpectedStatus)
		require.ErrorIs(t, err, errRead)
	})

	t.Run("transport error", func(t *testing.T) {
		t.Parallel()

		service := newTestService(t, "opsgenie://api.opsgenie.com/"+testAPIKey)
		client := mocks.NewMockHTTPClient(t)
		client.EXPECT().Do(mock.Anything).Return(nil, errTransport).Once()
		service.SetHTTPClient(client)

		require.ErrorIs(t, service.SendContext(t.Context(), "message", nil), errTransport)
	})

	// rejectedParams lists params that fail the send before any request.
	rejectedParams := []struct {
		params  *types.Params
		wantErr error
		name    string
	}{
		{name: "unknown param", params: &types.Params{"foo": "bar"}, wantErr: format.ErrInvalidConfigKey},
		{name: "invalid responder", params: &types.Params{"responders": "NOC"}, wantErr: ErrInvalidEntityFormat},
		{name: "responder without an identifier", params: &types.Params{"responders": "team:"}, wantErr: ErrMissingEntityIdentity},
		{name: "visibleTo entry without an identifier", params: &types.Params{"visibleTo": "user:"}, wantErr: ErrMissingEntityIdentity},
		{name: "escalation name in visibleTo", params: &types.Params{"visibleTo": "escalation:Nightwatch Escalation"}, wantErr: ErrUnexpectedEntityType},
		{
			name:    "schedule ID in visibleTo",
			params:  &types.Params{"visibleTo": "team:NOC,schedule:80564037-1984-4f38-b98e-8a1f662df552"},
			wantErr: ErrUnexpectedEntityType,
		},
	}

	for _, tt := range rejectedParams {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			service := newTestService(t, "opsgenie://api.opsgenie.com/"+testAPIKey)
			// The mock has no expectations, so a request fails the test.
			service.SetHTTPClient(mocks.NewMockHTTPClient(t))

			require.ErrorIs(t, service.SendContext(t.Context(), "message", tt.params), tt.wantErr)
		})
	}

	t.Run("canceled context", func(t *testing.T) {
		t.Parallel()

		ctx, cancel := context.WithCancel(t.Context())
		cancel()

		client := mocks.NewMockHTTPClient(t)
		client.EXPECT().Do(mock.Anything).RunAndReturn(func(req *http.Request) (*http.Response, error) {
			return nil, req.Context().Err()
		}).Once()

		service := newTestService(t, "opsgenie://opsgenie.example.invalid/KEY?responders=user:dummy")
		service.SetHTTPClient(client)

		require.ErrorIs(t, service.SendContext(ctx, "message", nil), context.Canceled)
	})
}

// TestSendRedactsUnparsableURL verifies that a host that makes the alerts URL
// unparsable fails the send before any request, without the host in the
// error.
func TestSendRedactsUnparsableURL(t *testing.T) {
	t.Parallel()

	service := newTestService(t, "opsgenie://api.opsgenie.com/"+testAPIKey)
	service.Config.Host = "SECRET host"
	// The mock has no expectations, so a request fails the test.
	service.SetHTTPClient(mocks.NewMockHTTPClient(t))

	err := service.Send("message", nil)

	_, isURLErr := errors.AsType[*url.Error](err)
	require.True(t, isURLErr, "error %v is not a *url.Error", err)
	assert.NotContains(t, err.Error(), "SECRET")
}

// TestSendDefaultClient verifies that a send without a custom client, or with
// a nil *http.Client, uses a default client that honors the caller's context.
// The context is canceled before the send, so the transport returns before it
// dials OpsGenie.
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

			service := newTestService(t, "opsgenie://api.opsgenie.com/"+testAPIKey)

			if tt.configure != nil {
				tt.configure(service)
			}

			ctx, cancel := context.WithCancel(t.Context())
			cancel()

			require.ErrorIs(t, service.SendContext(ctx, "message", nil), context.Canceled)
		})
	}
}

// TestDefaultHTTPClient verifies that the default client has no request
// timeout, since the send budget bounds each send, and sends through
// http.DefaultTransport, so it honors the proxy environment variables.
func TestDefaultHTTPClient(t *testing.T) {
	t.Parallel()

	service := newTestService(t, "opsgenie://api.opsgenie.com/"+testAPIKey)

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

// TestSendLevelPriority verifies that the level param sets the priority, debug
// to P5, info to P4, warning to P3, and error to P2, and that a priority from
// the service URL or the send params, or an unknown level, leaves it as it is.
func TestSendLevelPriority(t *testing.T) {
	t.Parallel()

	rawURL := "opsgenie://api.opsgenie.com/" + testAPIKey

	tests := map[string]struct {
		rawURL       string
		params       *types.Params
		wantPriority any
	}{
		"debug":               {rawURL: rawURL, params: &types.Params{"level": "Debug"}, wantPriority: "P5"},
		"info":                {rawURL: rawURL, params: &types.Params{"level": "Info"}, wantPriority: "P4"},
		"warning":             {rawURL: rawURL, params: &types.Params{"level": "Warning"}, wantPriority: "P3"},
		"error":               {rawURL: rawURL, params: &types.Params{"level": "Error"}, wantPriority: "P2"},
		"priority in the URL": {rawURL: rawURL + "?priority=P1", params: &types.Params{"level": "Debug"}, wantPriority: "P1"},
		"priority param":      {rawURL: rawURL, params: &types.Params{"level": "Error", "priority": "P4"}, wantPriority: "P4"},
		"unknown level":       {rawURL: rawURL, params: &types.Params{"level": "critical"}, wantPriority: nil},
		"no params":           {rawURL: rawURL, params: nil, wantPriority: nil},
	}

	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			service := newTestService(t, tc.rawURL)
			client, captured := newRespondingClient(t, http.StatusAccepted, acceptedBody)
			service.SetHTTPClient(client)

			require.NoError(t, service.Send("Disk full", tc.params))

			var body map[string]any
			require.NoError(t, json.Unmarshal([]byte(captured.body), &body))
			assert.Equal(t, tc.wantPriority, body["priority"])
		})
	}
}
