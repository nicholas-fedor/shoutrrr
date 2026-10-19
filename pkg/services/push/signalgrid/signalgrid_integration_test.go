package signalgrid_test

import (
	"io"
	"log"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/nicholas-fedor/shoutrrr/internal/meta"
	"github.com/nicholas-fedor/shoutrrr/pkg/services/push/signalgrid"
	"github.com/nicholas-fedor/shoutrrr/pkg/types"
)

// receivedRequest records what the fake Push API received.
type receivedRequest struct {
	// form is the form the API parsed from the request body.
	form url.Values
	// method is the request method.
	method string
	// url is the full request URL.
	url string
	// contentType is the request's Content-Type header.
	contentType string
	// userAgent is the request's User-Agent header.
	userAgent string
}

// roundTripFunc adapts a function to an [http.RoundTripper].
type roundTripFunc func(req *http.Request) (*http.Response, error)

// Values shared by the integration tests.
const (
	// acceptedBody is the body the Push API returns for an accepted notification.
	acceptedBody = `{"code":"200","text":"OK"}`
	// pushURL is the Signalgrid Push API endpoint.
	pushURL = "https://api.signalgrid.co/v1/push"
	// testServiceURL is a service URL with a client key and channel token.
	testServiceURL = "signalgrid://clientkey@channeltoken"
	// formContentType is the form encoding the Push API requires.
	formContentType = "application/x-www-form-urlencoded"
)

// RoundTrip calls f with the request.
//
// Parameters:
//   - req: the request to send.
//
// Returns:
//   - *http.Response: the response from f.
//   - error: the error from f.
func (f roundTripFunc) RoundTrip(req *http.Request) (*http.Response, error) {
	return f(req)
}

// newFakeAPI returns an HTTP client whose requests reach an in-process fake
// Push API. The fake API parses the request as a form, records it, and
// answers with the given status and body.
//
// Parameters:
//   - t: the test.
//   - statusCode: the response status code.
//   - body: the response body.
//
// Returns:
//   - *http.Client: the client.
//   - *receivedRequest: the request, filled in once the API receives it.
func newFakeAPI(t *testing.T, statusCode int, body string) (*http.Client, *receivedRequest) {
	t.Helper()

	received := &receivedRequest{}
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.NoError(t, r.ParseForm())

		*received = receivedRequest{
			form:        r.PostForm,
			method:      r.Method,
			url:         r.URL.String(),
			contentType: r.Header.Get("Content-Type"),
			userAgent:   r.UserAgent(),
		}

		w.WriteHeader(statusCode)
		_, _ = io.WriteString(w, body)
	})

	client := &http.Client{Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
		recorder := httptest.NewRecorder()
		handler.ServeHTTP(recorder, req)

		return recorder.Result(), nil
	})}

	return client, received
}

// newTestService returns a service initialized from rawURL that sends through
// client.
//
// Parameters:
//   - t: the test.
//   - rawURL: the service URL.
//   - client: the HTTP client for the service.
//
// Returns:
//   - *signalgrid.Service: the service.
func newTestService(t *testing.T, rawURL string, client types.HTTPClient) *signalgrid.Service {
	t.Helper()

	serviceURL, err := url.Parse(rawURL)
	require.NoError(t, err)

	service := &signalgrid.Service{}
	require.NoError(t, service.Initialize(serviceURL, log.New(t.Output(), "", 0)))
	service.SetHTTPClient(client)

	return service
}

// TestIntegrationSendForm verifies the form the Push API receives for a send
// configured from a service URL and params: the credentials keep their case,
// an empty message still sends the body field, and a title param replaces
// the title from the URL.
func TestIntegrationSendForm(t *testing.T) {
	t.Parallel()

	// tests lists sends with the form the Push API should receive.
	tests := []struct {
		params   *types.Params
		wantForm url.Values
		name     string
		rawURL   string
		message  string
	}{
		{
			name:     "empty message",
			rawURL:   testServiceURL,
			wantForm: url.Values{"client_key": {"clientkey"}, "channel": {"channeltoken"}, "body": {""}, "type": {"INFO"}},
		},
		{
			name:     "mixed-case credentials",
			rawURL:   "signalgrid://AbC123@ChannelToken",
			message:  "Backup done",
			wantForm: url.Values{"client_key": {"AbC123"}, "channel": {"ChannelToken"}, "body": {"Backup done"}, "type": {"INFO"}},
		},
		{
			name:    "title, type, and critical from the URL",
			rawURL:  testServiceURL + "?title=Alert&type=WARN&critical=true",
			message: "Disk almost full",
			wantForm: url.Values{
				"client_key": {"clientkey"}, "channel": {"channeltoken"}, "body": {"Disk almost full"}, "type": {"WARN"},
				"title": {"Alert"}, "critical": {"true"},
			},
		},
		{
			name:    "title param over the URL title",
			rawURL:  testServiceURL + "?title=Alert",
			params:  &types.Params{"title": "Override"},
			message: "Disk almost full",
			wantForm: url.Values{
				"client_key": {"clientkey"}, "channel": {"channeltoken"}, "body": {"Disk almost full"}, "type": {"INFO"},
				"title": {"Override"},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			client, received := newFakeAPI(t, http.StatusOK, acceptedBody)
			service := newTestService(t, tt.rawURL, client)

			require.NoError(t, service.Send(tt.message, tt.params))

			assert.Equal(t, http.MethodPost, received.method)
			assert.Equal(t, pushURL, received.url)
			assert.Equal(t, formContentType, received.contentType)
			assert.Equal(t, meta.UserAgent(), received.userAgent)
			assert.Equal(t, tt.wantForm, received.form)
		})
	}
}

// TestIntegrationSendErrorStatus verifies that an error status from the Push
// API reaches the caller as ErrSendFailed and ErrUnexpectedStatus, with the
// status and the response body in the message.
func TestIntegrationSendErrorStatus(t *testing.T) {
	t.Parallel()

	// tests lists error responses with the status line each should report.
	tests := []struct {
		name       string
		body       string
		wantStatus string
		statusCode int
	}{
		{name: "bad request", statusCode: http.StatusBadRequest, body: "invalid", wantStatus: "400 Bad Request"},
		{name: "forbidden", statusCode: http.StatusForbidden, body: "forbidden", wantStatus: "403 Forbidden"},
		{name: "not found", statusCode: http.StatusNotFound, body: "missing", wantStatus: "404 Not Found"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			client, _ := newFakeAPI(t, tt.statusCode, tt.body)
			service := newTestService(t, testServiceURL, client)

			err := service.Send("Test message", nil)

			require.ErrorIs(t, err, signalgrid.ErrSendFailed)
			require.ErrorIs(t, err, signalgrid.ErrUnexpectedStatus)
			assert.ErrorContains(t, err, tt.wantStatus+": "+tt.body)
		})
	}
}

// TestIntegrationInitializeMissingChannel verifies that Initialize rejects a
// URL with a client key and no channel token.
func TestIntegrationInitializeMissingChannel(t *testing.T) {
	t.Parallel()

	serviceURL, err := url.Parse("signalgrid://key@")
	require.NoError(t, err)

	err = (&signalgrid.Service{}).Initialize(serviceURL, log.New(t.Output(), "", 0))

	require.ErrorIs(t, err, signalgrid.ErrChannelMissing)
}

// TestIntegrationConfigURLRoundTrip verifies that a service initialized from
// the URL that another service's config produces sends the same form.
func TestIntegrationConfigURLRoundTrip(t *testing.T) {
	t.Parallel()

	client, original := newFakeAPI(t, http.StatusOK, acceptedBody)
	service := newTestService(t, "signalgrid://AbC123@ChannelToken?title=Alert&type=CRIT&critical=true", client)
	require.NoError(t, service.Send("Outage", nil))

	roundTripClient, roundTrip := newFakeAPI(t, http.StatusOK, acceptedBody)
	roundTripService := newTestService(t, service.Config.GetURL().String(), roundTripClient)
	require.NoError(t, roundTripService.Send("Outage", nil))

	assert.Equal(t, url.Values{
		"client_key": {"AbC123"}, "channel": {"ChannelToken"}, "body": {"Outage"}, "type": {"CRIT"},
		"title": {"Alert"}, "critical": {"true"},
	}, original.form)
	assert.Equal(t, original.form, roundTrip.form)
}
