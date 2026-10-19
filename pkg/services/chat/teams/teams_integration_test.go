package teams_test

import (
	"context"
	"errors"
	"io"
	"log"
	"net"
	"net/http"
	"net/http/httptest"
	"net/url"
	"sync"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/nicholas-fedor/shoutrrr/pkg/services/chat/teams"
	"github.com/nicholas-fedor/shoutrrr/pkg/types"
)

// webhookRequest is a request that the fake workflow webhook received.
type webhookRequest struct {
	// method is the HTTP method.
	method string
	// host is the Host header, which names the workflow host the service dialed.
	host string
	// requestURI is the path and query of the request line.
	requestURI string
	// contentType is the Content-Type header.
	contentType string
	// body is the request body.
	body string
}

// fakeWebhook is a loopback TLS server that stands in for a Power Automate
// workflow webhook. It records each request and answers with a fixed status.
type fakeWebhook struct {
	// server is the loopback TLS server.
	server *httptest.Server
	// requests are the received requests, in order.
	requests []webhookRequest
	// statusCode is the status of every response.
	statusCode int
	// mu guards requests.
	mu sync.Mutex
}

// Values shared by the integration tests.
const (
	// testWorkflowHost is the host and port of the test workflow URL.
	testWorkflowHost = "prod-00.westus.logic.azure.com:443"
	// testWorkflowRequestURI is the path and query of the test workflow URL.
	testWorkflowRequestURI = "/workflows/abc123/triggers/manual/paths/invoke?api-version=2016-06-00&sp=/triggers/manual/run&sv=1.0&sig=SECRETsig"
	// testWorkflowURL is a Logic Apps workflow webhook URL with a signature.
	testWorkflowURL = "https://" + testWorkflowHost + testWorkflowRequestURI
	// testCertName is the DNS name in the certificate of [httptest.NewTLSServer].
	testCertName = "example.com"
)

// testServiceURL is the Teams service URL for the test workflow.
var testServiceURL = "teams://?host=" + url.QueryEscape(testWorkflowURL)

// errConnectionRefused is the error the refused dials report.
var errConnectionRefused = errors.New("connection refused")

// Requests returns the requests received so far.
//
// Returns:
//   - []webhookRequest: the requests, in order.
func (f *fakeWebhook) Requests() []webhookRequest {
	f.mu.Lock()
	defer f.mu.Unlock()

	return append([]webhookRequest(nil), f.requests...)
}

// ServeHTTP records the request and answers with the fixed status.
//
// Parameters:
//   - w: the response writer.
//   - r: the request.
func (f *fakeWebhook) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	body, err := io.ReadAll(r.Body)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)

		return
	}

	f.mu.Lock()
	f.requests = append(f.requests, webhookRequest{
		method:      r.Method,
		host:        r.Host,
		requestURI:  r.RequestURI,
		contentType: r.Header.Get("Content-Type"),
		body:        string(body),
	})
	f.mu.Unlock()

	w.WriteHeader(f.statusCode)
}

// newFakeWebhook starts a fake workflow webhook that answers every request
// with statusCode and stops when the test ends.
//
// Parameters:
//   - t: the test.
//   - statusCode: the status of every response.
//
// Returns:
//   - *fakeWebhook: the fake webhook.
func newFakeWebhook(t *testing.T, statusCode int) *fakeWebhook {
	t.Helper()

	fake := &fakeWebhook{statusCode: statusCode}
	fake.server = httptest.NewTLSServer(fake)
	t.Cleanup(fake.server.Close)

	return fake
}

// newWorkflowClient returns an HTTP client that sends every request to
// server over TLS, whatever host the request names, and trusts the server's
// certificate.
//
// Parameters:
//   - t: the test.
//   - server: the loopback TLS server.
//
// Returns:
//   - *http.Client: the client.
func newWorkflowClient(t *testing.T, server *httptest.Server) *http.Client {
	t.Helper()

	transport, ok := server.Client().Transport.(*http.Transport)
	require.True(t, ok, "the server client's transport is %T", server.Client().Transport)

	transport = transport.Clone()
	transport.TLSClientConfig.ServerName = testCertName
	transport.DialContext = func(ctx context.Context, network, _ string) (net.Conn, error) {
		var dialer net.Dialer

		return dialer.DialContext(ctx, network, server.Listener.Addr().String())
	}

	return &http.Client{Transport: transport}
}

// refusingClient returns an HTTP client whose every dial is refused, as by a
// host where nothing listens, so no connection leaves the process.
//
// Returns:
//   - *http.Client: the client.
func refusingClient() *http.Client {
	return &http.Client{Transport: &http.Transport{
		DialContext: func(context.Context, string, string) (net.Conn, error) {
			return nil, &net.OpError{Op: "dial", Net: "tcp", Err: errConnectionRefused}
		},
	}}
}

// newTestService returns a service initialized from rawURL that sends
// through client and logs to the test output.
//
// Parameters:
//   - t: the test.
//   - rawURL: the Teams service URL.
//   - client: the HTTP client.
//
// Returns:
//   - *teams.Service: the service.
func newTestService(t *testing.T, rawURL string, client *http.Client) *teams.Service {
	t.Helper()

	serviceURL, err := url.Parse(rawURL)
	require.NoError(t, err)

	service := &teams.Service{}
	require.NoError(t, service.Initialize(serviceURL, log.New(t.Output(), "", 0)))
	service.SetHTTPClient(client)

	return service
}

// TestIntegrationSend verifies that a send initialized from a service URL
// posts one JSON Adaptive Card message to the workflow URL over HTTPS, with
// the title and color from the URL or params and one text block per
// non-blank message line.
func TestIntegrationSend(t *testing.T) {
	t.Parallel()

	// tests lists service URL suffixes, params, and messages with the JSON
	// body that each send should post.
	tests := []struct {
		params   *types.Params
		name     string
		query    string
		message  string
		wantBody string
	}{
		{
			name:    "plain message",
			message: "Test plain text message",
			wantBody: `{"type":"message","attachments":[{"contentType":"application/vnd.microsoft.card.adaptive","contentUrl":null,` +
				`"content":{"$schema":"http://adaptivecards.io/schemas/adaptive-card.json","type":"AdaptiveCard","version":"1.2",` +
				`"body":[{"type":"TextBlock","text":"Test plain text message","wrap":true}]}}]}`,
		},
		{
			name:    "multi-line message with title and unknown color from the URL",
			query:   "&title=MyTitle&color=00ff00",
			message: "Line 1\nLine 2\nLine 3",
			wantBody: `{"type":"message","attachments":[{"contentType":"application/vnd.microsoft.card.adaptive","contentUrl":null,` +
				`"content":{"$schema":"http://adaptivecards.io/schemas/adaptive-card.json","type":"AdaptiveCard","version":"1.2","body":[` +
				`{"type":"TextBlock","text":"MyTitle","color":"default","weight":"Bolder","size":"Medium"},` +
				`{"type":"TextBlock","text":"Line 1","wrap":true},` +
				`{"type":"TextBlock","text":"Line 2","wrap":true},` +
				`{"type":"TextBlock","text":"Line 3","wrap":true}]}}]}`,
		},
		{
			name:    "title and color from params",
			params:  &types.Params{"title": "Important", "color": "attention"},
			message: "Message body",
			wantBody: `{"type":"message","attachments":[{"contentType":"application/vnd.microsoft.card.adaptive","contentUrl":null,` +
				`"content":{"$schema":"http://adaptivecards.io/schemas/adaptive-card.json","type":"AdaptiveCard","version":"1.2","body":[` +
				`{"type":"TextBlock","text":"Important","color":"attention","weight":"Bolder","size":"Medium"},` +
				`{"type":"TextBlock","text":"Message body","wrap":true}]}}]}`,
		},
		{
			name:    "empty params",
			params:  &types.Params{},
			message: "Message with empty params",
			wantBody: `{"type":"message","attachments":[{"contentType":"application/vnd.microsoft.card.adaptive","contentUrl":null,` +
				`"content":{"$schema":"http://adaptivecards.io/schemas/adaptive-card.json","type":"AdaptiveCard","version":"1.2",` +
				`"body":[{"type":"TextBlock","text":"Message with empty params","wrap":true}]}}]}`,
		},
		{
			name: "empty message",
			wantBody: `{"type":"message","attachments":[{"contentType":"application/vnd.microsoft.card.adaptive","contentUrl":null,` +
				`"content":{"$schema":"http://adaptivecards.io/schemas/adaptive-card.json","type":"AdaptiveCard","version":"1.2","body":[]}}]}`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			fake := newFakeWebhook(t, http.StatusOK)
			service := newTestService(t, testServiceURL+tt.query, newWorkflowClient(t, fake.server))

			require.NoError(t, service.Send(tt.message, tt.params))

			requests := fake.Requests()
			require.Len(t, requests, 1)
			assert.Equal(t, http.MethodPost, requests[0].method)
			assert.Equal(t, testWorkflowHost, requests[0].host)
			assert.Equal(t, testWorkflowRequestURI, requests[0].requestURI)
			assert.Equal(t, "application/json", requests[0].contentType)
			assert.JSONEq(t, tt.wantBody, requests[0].body)
		})
	}
}

// TestIntegrationSendResponseStatus verifies that a send succeeds when the
// workflow answers 202 Accepted and reports ErrSendFailedStatus with the
// status for an error status, after posting the message once.
func TestIntegrationSendResponseStatus(t *testing.T) {
	t.Parallel()

	// tests lists workflow response statuses and whether the send fails.
	tests := []struct {
		name       string
		statusCode int
		wantErr    bool
	}{
		{name: "202 Accepted", statusCode: http.StatusAccepted},
		{name: "400 Bad Request", statusCode: http.StatusBadRequest, wantErr: true},
		{name: "500 Internal Server Error", statusCode: http.StatusInternalServerError, wantErr: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			fake := newFakeWebhook(t, tt.statusCode)
			err := newTestService(t, testServiceURL, newWorkflowClient(t, fake.server)).Send("Test message", nil)

			assert.Len(t, fake.Requests(), 1)

			if !tt.wantErr {
				require.NoError(t, err)

				return
			}

			require.ErrorIs(t, err, teams.ErrSendFailedStatus)
			assert.ErrorContains(t, err, http.StatusText(tt.statusCode))
		})
	}
}

// TestIntegrationSendConnectionFailure verifies that a send to a workflow
// that refuses the connection reports ErrSendFailed without the workflow path
// or signature in the error.
func TestIntegrationSendConnectionFailure(t *testing.T) {
	t.Parallel()

	err := newTestService(t, testServiceURL, refusingClient()).Send("Test message", nil)

	require.ErrorIs(t, err, teams.ErrSendFailed)
	assert.NotContains(t, err.Error(), "abc123")
	assert.NotContains(t, err.Error(), "SECRETsig")
}

// TestIntegrationSendContextCanceled verifies that canceling the context of
// a send while the workflow holds the request ends the send with
// context.Canceled wrapped in ErrSendFailed.
func TestIntegrationSendContextCanceled(t *testing.T) {
	t.Parallel()

	received := make(chan struct{})
	server := httptest.NewTLSServer(http.HandlerFunc(func(_ http.ResponseWriter, r *http.Request) {
		// The server watches the connection for the client going away only
		// after the handler reads the whole body.
		_, _ = io.Copy(io.Discard, r.Body)

		close(received)
		<-r.Context().Done()
	}))
	t.Cleanup(server.Close)

	service := newTestService(t, testServiceURL, newWorkflowClient(t, server))

	ctx, cancel := context.WithCancel(t.Context())
	t.Cleanup(cancel)

	done := make(chan error, 1)

	go func() { done <- service.SendContext(ctx, "Test message", nil) }()

	select {
	case <-received:
	case err := <-done:
		require.FailNow(t, "the send returned before the workflow received the request", "error: %v", err)
	}

	cancel()

	err := <-done
	require.ErrorIs(t, err, context.Canceled)
	assert.ErrorIs(t, err, teams.ErrSendFailed)
}
