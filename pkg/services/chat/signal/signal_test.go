package signal

import (
	"bytes"
	"context"
	"crypto/tls"
	"errors"
	"io"
	"log"
	"net"
	"net/http"
	"net/http/httptest"
	"net/url"
	"slices"
	"strings"
	"sync/atomic"
	"testing"
	"testing/synctest"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	"github.com/nicholas-fedor/shoutrrr/internal/meta"
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
	// userAgent is the request's User-Agent header.
	userAgent string
	// authorization is the request's Authorization header.
	authorization string
	// body is the request body.
	body string
}

// cannedResponse is what the recording client returns for one request.
type cannedResponse struct {
	// err is a transport error returned instead of a response.
	err error
	// body is the response body.
	body string
	// statusCode is the response status code.
	statusCode int
}

// Values shared by the service tests.
const (
	// testSource is the sender's phone number in the test URLs.
	testSource = "+15550000000"
	// testRecipient is a phone number recipient in the test URLs.
	testRecipient = "+15551234567"
	// testServiceURL is a service URL with one phone number recipient.
	testServiceURL = "signal://localhost:8080/" + testSource + "/" + testRecipient
	// testEndpoint is the send endpoint for testServiceURL.
	testEndpoint = "https://localhost:8080/v2/send"
	// sentBody is the response body of a successful send, a JSON array with
	// one result per signal-cli call.
	sentBody = `[{"timestamp":"1760500000000"}]`
)

// errTransport is the transport failure returned by a failing HTTP client.
var errTransport = errors.New("transport failure")

// accepted is the response to a successful send.
var accepted = cannedResponse{statusCode: http.StatusCreated, body: sentBody}

// newRecordingClient returns a mock HTTP client that expects one request per
// response, records each request, and answers them in order.
//
// Parameters:
//   - t: the test that owns the mock.
//   - responses: the responses to return, in request order.
//
// Returns:
//   - *mocks.MockHTTPClient: the client.
//   - *[]capturedRequest: the requests, appended as the client receives them.
func newRecordingClient(t *testing.T, responses ...cannedResponse) (*mocks.MockHTTPClient, *[]capturedRequest) {
	t.Helper()

	var captured []capturedRequest

	client := mocks.NewMockHTTPClient(t)
	client.EXPECT().Do(mock.Anything).RunAndReturn(func(req *http.Request) (*http.Response, error) {
		body, err := io.ReadAll(req.Body)
		require.NoError(t, err)

		deadline, _ := req.Context().Deadline()

		captured = append(captured, capturedRequest{
			deadline:      deadline,
			method:        req.Method,
			url:           req.URL.String(),
			contentType:   req.Header.Get("Content-Type"),
			userAgent:     req.Header.Get("User-Agent"),
			authorization: req.Header.Get("Authorization"),
			body:          string(body),
		})

		response := responses[len(captured)-1]
		if response.err != nil {
			return nil, &url.Error{Op: "Post", URL: req.URL.String(), Err: response.err}
		}

		return &http.Response{
			StatusCode: response.statusCode,
			Body:       io.NopCloser(strings.NewReader(response.body)),
		}, nil
	}).Times(len(responses))

	return client, &captured
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

	assert.Equal(t, "signal", (&Service{}).GetID())
}

// TestInitialize verifies that Initialize parses the service URL over the
// defaults and creates the default client, which requires TLS 1.2 and skips
// certificate verification only when skiptlsverify is set.
func TestInitialize(t *testing.T) {
	t.Parallel()

	service := newTestService(t, testServiceURL)

	assert.Equal(t, &Config{
		Host:       "localhost",
		Port:       8080,
		Source:     testSource,
		Recipients: []string{testRecipient},
		NotifySelf: true,
	}, service.Config)

	// Initialize logs nothing for skiptlsverify. The first send that skips
	// verification warns about it.
	var logs bytes.Buffer

	skipping := &Service{}
	require.NoError(t, skipping.Initialize(parseTestURL(t, testServiceURL+"?skiptlsverify=yes"), log.New(&logs, "", 0)))
	assert.Empty(t, logs.String())
	assertDefaultClient(t, skipping.client.Client(), false)
}

// TestInitializeErrors verifies that Initialize reports an invalid service
// URL.
func TestInitializeErrors(t *testing.T) {
	t.Parallel()

	service := &Service{}
	err := service.Initialize(parseTestURL(t, "signal://localhost:8080/"+testSource), log.New(t.Output(), "", 0))

	require.ErrorIs(t, err, ErrNoRecipients)
}

// TestInitializeKeepsCustomClient verifies that a client set before
// Initialize is kept and used for sends.
func TestInitializeKeepsCustomClient(t *testing.T) {
	t.Parallel()

	client, captured := newRecordingClient(t, accepted)

	service := &Service{}
	service.SetHTTPClient(client)
	require.NoError(t, service.Initialize(parseTestURL(t, testServiceURL), log.New(t.Output(), "", 0)))

	assert.Same(t, client, service.client.Client())
	require.NoError(t, service.Send("Hello", nil))
	assert.Len(t, *captured, 1)
}

// TestSend verifies the request a send makes: a JSON POST to /v2/send with
// the message, the source number, and the recipients, plus the title, text
// mode, attachments, and notify_self settings from the URL and params.
func TestSend(t *testing.T) {
	t.Parallel()

	// tests lists sends with the request body each should produce.
	tests := []struct {
		params   *types.Params
		name     string
		rawURL   string
		message  string
		wantBody string
	}{
		{
			name:     "message only",
			rawURL:   testServiceURL,
			message:  "Hello",
			wantBody: `{"message":"Hello","number":"+15550000000","recipients":["+15551234567"]}`,
		},
		{
			name:     "styled text mode",
			rawURL:   testServiceURL + "?textmode=styled",
			message:  "*Hello*",
			wantBody: `{"message":"*Hello*","number":"+15550000000","recipients":["+15551234567"],"text_mode":"styled"}`,
		},
		{
			name:     "normal text mode",
			rawURL:   testServiceURL + "?text_mode=normal",
			message:  "Hello",
			wantBody: `{"message":"Hello","number":"+15550000000","recipients":["+15551234567"],"text_mode":"normal"}`,
		},
		{
			name:     "text mode param",
			rawURL:   testServiceURL,
			params:   &types.Params{"textmode": "styled"},
			message:  "Hello",
			wantBody: `{"message":"Hello","number":"+15550000000","recipients":["+15551234567"],"text_mode":"styled"}`,
		},
		{
			name:     "title",
			rawURL:   testServiceURL + "?title=Alert",
			message:  "Disk full",
			wantBody: `{"message":"Alert\nDisk full","number":"+15550000000","recipients":["+15551234567"]}`,
		},
		{
			name:     "styled title",
			rawURL:   testServiceURL + "?title=Alert&textmode=styled",
			message:  "Disk full",
			wantBody: `{"message":"**Alert**\nDisk full","number":"+15550000000","recipients":["+15551234567"],"text_mode":"styled"}`,
		},
		{
			name:     "title param without a message",
			rawURL:   testServiceURL,
			params:   &types.Params{"title": "Alert"},
			message:  "",
			wantBody: `{"message":"Alert","number":"+15550000000","recipients":["+15551234567"]}`,
		},
		{
			name:     "notify self off",
			rawURL:   testServiceURL + "?notifyself=no",
			message:  "Hello",
			wantBody: `{"message":"Hello","number":"+15550000000","recipients":["+15551234567"],"notify_self":false}`,
		},
		{
			name:     "raw base64 attachments param",
			rawURL:   testServiceURL,
			params:   &types.Params{"attachments": "aGVsbG8=, d29ybGQ="},
			message:  "Hello",
			wantBody: `{"message":"Hello","number":"+15550000000","recipients":["+15551234567"],"base64_attachments":["aGVsbG8=","d29ybGQ="]}`,
		},
		{
			name:     "data URI attachment",
			rawURL:   testServiceURL,
			params:   &types.Params{"attachments": "data:image/png;filename=dot.png;base64,iVBORw0KGgo="},
			message:  "Hello",
			wantBody: `{"message":"Hello","number":"+15550000000","recipients":["+15551234567"],"base64_attachments":["data:image/png;filename=dot.png;base64,iVBORw0KGgo="]}`,
		},
		{
			name:     "phone numbers share a request",
			rawURL:   testServiceURL + "/+15559876543",
			message:  "Hello",
			wantBody: `{"message":"Hello","number":"+15550000000","recipients":["+15551234567","+15559876543"]}`,
		},
		{
			name:     "username without the u: prefix",
			rawURL:   "signal://localhost:8080/" + testSource + "/u:alice.01",
			message:  "Hello",
			wantBody: `{"message":"Hello","number":"+15550000000","recipients":["alice.01"]}`,
		},
		{
			name:     "group ID with a slash",
			rawURL:   "signal://localhost:8080/" + testSource + "/group.ABCD/EFGH=",
			message:  "Hello",
			wantBody: `{"message":"Hello","number":"+15550000000","recipients":["group.ABCD/EFGH="]}`,
		},
		{
			name:     "skiptlsverify param keeps the custom client",
			rawURL:   testServiceURL,
			params:   &types.Params{"skiptlsverify": "yes"},
			message:  "Hello",
			wantBody: `{"message":"Hello","number":"+15550000000","recipients":["+15551234567"]}`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			service := newTestService(t, tt.rawURL)
			client, captured := newRecordingClient(t, accepted)
			service.SetHTTPClient(client)

			require.NoError(t, service.Send(tt.message, tt.params))

			require.Len(t, *captured, 1)
			request := (*captured)[0]
			assert.Equal(t, http.MethodPost, request.method)
			assert.Equal(t, testEndpoint, request.url)
			assert.Equal(t, "application/json", request.contentType)
			assert.Equal(t, meta.UserAgent(), request.userAgent)
			assert.JSONEq(t, tt.wantBody, request.body)
		})
	}
}

// TestSendEndpointAndAuthentication verifies that a send posts to the
// configured host and port over HTTPS, or HTTP with disabletls, and
// authenticates with the token as a Bearer token, or else with the URL's
// user and password as Basic auth.
func TestSendEndpointAndAuthentication(t *testing.T) {
	t.Parallel()

	// tests lists URLs with the endpoint and Authorization header each should
	// produce.
	tests := []struct {
		name     string
		rawURL   string
		wantURL  string
		wantAuth string
	}{
		{name: "no credentials", rawURL: testServiceURL, wantURL: testEndpoint},
		{
			name:     "token",
			rawURL:   testServiceURL + "?token=SECRETtoken",
			wantURL:  testEndpoint,
			wantAuth: "Bearer SECRETtoken",
		},
		{
			name:     "apikey alias",
			rawURL:   testServiceURL + "?apikey=SECRETtoken",
			wantURL:  testEndpoint,
			wantAuth: "Bearer SECRETtoken",
		},
		{
			name:     "user and password",
			rawURL:   "signal://user:pass@localhost:8080/" + testSource + "/" + testRecipient,
			wantURL:  testEndpoint,
			wantAuth: "Basic dXNlcjpwYXNz",
		},
		{
			name:     "user without a password",
			rawURL:   "signal://user@localhost:8080/" + testSource + "/" + testRecipient,
			wantURL:  testEndpoint,
			wantAuth: "Basic dXNlcjo=",
		},
		{
			name:     "token over user and password",
			rawURL:   "signal://user:pass@localhost:8080/" + testSource + "/" + testRecipient + "?token=SECRETtoken",
			wantURL:  testEndpoint,
			wantAuth: "Bearer SECRETtoken",
		},
		{
			name:    "disabletls",
			rawURL:  testServiceURL + "?disabletls=yes",
			wantURL: "http://localhost:8080/v2/send",
		},
		{
			name:    "custom host and port",
			rawURL:  "signal://signal.example.com:9922/" + testSource + "/" + testRecipient,
			wantURL: "https://signal.example.com:9922/v2/send",
		},
		{
			name:    "default port",
			rawURL:  "signal://signal.example.com/" + testSource + "/" + testRecipient,
			wantURL: "https://signal.example.com:8080/v2/send",
		},
		{
			name:    "IPv6 host",
			rawURL:  "signal://[::1]:9922/" + testSource + "/" + testRecipient,
			wantURL: "https://[::1]:9922/v2/send",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			service := newTestService(t, tt.rawURL)
			client, captured := newRecordingClient(t, accepted)
			service.SetHTTPClient(client)

			require.NoError(t, service.Send("Hello", nil))

			require.Len(t, *captured, 1)
			assert.Equal(t, tt.wantURL, (*captured)[0].url)
			assert.Equal(t, tt.wantAuth, (*captured)[0].authorization)
		})
	}
}

// TestSendSplitsRecipientTypes verifies that a send makes one request for the
// phone numbers, one for the usernames, and one per group, because the REST
// API rejects mixed recipient types and several groups in one request.
func TestSendSplitsRecipientTypes(t *testing.T) {
	t.Parallel()

	service := newTestService(t, testServiceURL+"/group.Z3JvdXAx/u:alice.01/+15559876543/group.Z3JvdXAy/u:bob.02")
	client, captured := newRecordingClient(t, accepted, accepted, accepted, accepted)
	service.SetHTTPClient(client)

	require.NoError(t, service.Send("Hello", nil))

	require.Len(t, *captured, 4)

	for i, recipients := range []string{
		`["+15551234567","+15559876543"]`,
		`["alice.01","bob.02"]`,
		`["group.Z3JvdXAx"]`,
		`["group.Z3JvdXAy"]`,
	} {
		assert.JSONEq(t, `{"message":"Hello","number":"+15550000000","recipients":`+recipients+`}`, (*captured)[i].body)
	}
}

// TestSendLeavesConfigUnchanged verifies that param overrides apply to one
// send only and leave the caller's params unchanged.
func TestSendLeavesConfigUnchanged(t *testing.T) {
	t.Parallel()

	service := newTestService(t, testServiceURL)
	client, captured := newRecordingClient(t, accepted, accepted)
	service.SetHTTPClient(client)

	want := *service.Config
	want.Recipients = slices.Clone(service.Config.Recipients)
	params := types.Params{"title": "Alert", "textmode": "styled", "recipients": "+15559876543"}

	require.NoError(t, service.Send("Disk full", &params))
	require.NoError(t, service.Send("Hello", nil))

	assert.Equal(t, &want, service.Config)
	assert.Equal(t, types.Params{"title": "Alert", "textmode": "styled", "recipients": "+15559876543"}, params)
	require.Len(t, *captured, 2)
	assert.JSONEq(t, `{"message":"**Alert**\nDisk full","number":"+15550000000","recipients":["+15559876543"],"text_mode":"styled"}`, (*captured)[0].body)
	assert.JSONEq(t, `{"message":"Hello","number":"+15550000000","recipients":["+15551234567"]}`, (*captured)[1].body)
}

// TestSendErrors verifies that a send rejects invalid params, including
// recipients that are not phone numbers, group IDs, or usernames, and a config
// or params without recipients before any request, and reports error statuses
// and transport errors.
func TestSendErrors(t *testing.T) {
	t.Parallel()

	// rejectedSends lists sends that fail before any request.
	rejectedSends := []struct {
		wantErr error
		params  *types.Params
		config  *Config
		name    string
	}{
		{name: "unknown param", params: &types.Params{"unknown": "value"}, wantErr: format.ErrInvalidConfigKey},
		{name: "invalid text mode param", params: &types.Params{"textmode": "markdown"}, wantErr: format.ErrInvalidEnumValue},
		{name: "invalid recipient param", params: &types.Params{"recipients": "alice"}, wantErr: ErrInvalidRecipient},
		{name: "invalid to param", params: &types.Params{"to": testRecipient + ",group.AB!"}, wantErr: ErrInvalidRecipient},
		{name: "empty recipients param", params: &types.Params{"recipients": ""}, wantErr: ErrNoRecipients},
		{
			name:    "no recipients",
			config:  &Config{Host: "localhost", Port: 8080, Source: testSource, Recipients: []string{}},
			wantErr: ErrNoRecipients,
		},
	}

	for _, tt := range rejectedSends {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			service := newTestService(t, testServiceURL)
			if tt.config != nil {
				service.Config = tt.config
			}

			// The mock has no expectations, so a request fails the test.
			service.SetHTTPClient(mocks.NewMockHTTPClient(t))

			require.ErrorIs(t, service.SendContext(t.Context(), "Hello", tt.params), tt.wantErr)
		})
	}

	// failedSends lists responses with the error each should produce.
	failedSends := []struct {
		wantErr   error
		name      string
		response  cannedResponse
		wantError string
	}{
		{
			name:      "bad request",
			response:  cannedResponse{statusCode: http.StatusBadRequest, body: `{"error":"Couldn't process request - please provide a valid number"}`},
			wantErr:   ErrSendFailed,
			wantError: "failed to send Signal message: server returned status 400: Couldn't process request - please provide a valid number",
		},
		{
			name:      "server error without a body",
			response:  cannedResponse{statusCode: http.StatusInternalServerError},
			wantErr:   ErrSendFailed,
			wantError: "failed to send Signal message: server returned status 500",
		},
		{
			name:      "redirect",
			response:  cannedResponse{statusCode: http.StatusFound},
			wantErr:   ErrSendFailed,
			wantError: "failed to send Signal message: server returned status 302",
		},
		{
			name:      "transport error",
			response:  cannedResponse{err: errTransport},
			wantErr:   errTransport,
			wantError: `sending HTTP request: Post "https://localhost:8080": transport failure`,
		},
	}

	for _, tt := range failedSends {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			service := newTestService(t, testServiceURL)
			client, _ := newRecordingClient(t, tt.response)
			service.SetHTTPClient(client)

			err := service.SendContext(t.Context(), "Hello", nil)

			require.ErrorIs(t, err, tt.wantErr)
			assert.EqualError(t, err, tt.wantError)
		})
	}

	t.Run("truncated error body", func(t *testing.T) {
		t.Parallel()

		service := newTestService(t, testServiceURL)
		client, _ := newRecordingClient(t, cannedResponse{
			statusCode: http.StatusBadGateway,
			body:       strings.Repeat("a", maxErrorBodySize) + "SECRETtail",
		})
		service.SetHTTPClient(client)

		err := service.SendContext(t.Context(), "Hello", nil)

		require.ErrorIs(t, err, ErrSendFailed)
		assert.EqualError(t, err, "failed to send Signal message: server returned status 502: "+strings.Repeat("a", maxErrorBodySize)+" [truncated]")
	})
}

// TestSendContinuesAfterFailedRequest verifies that a failed request does not
// stop the requests for the other recipient types, and that the send reports
// every failure.
func TestSendContinuesAfterFailedRequest(t *testing.T) {
	t.Parallel()

	service := newTestService(t, testServiceURL+"/u:alice.01/group.Z3JvdXAx")
	client, captured := newRecordingClient(t,
		cannedResponse{statusCode: http.StatusBadRequest, body: `{"error":"Failed to send message"}`},
		accepted,
		cannedResponse{err: errTransport},
	)
	service.SetHTTPClient(client)

	err := service.SendContext(t.Context(), "Hello", nil)

	require.ErrorIs(t, err, ErrSendFailed)
	require.ErrorIs(t, err, errTransport)
	require.EqualError(t, err, "failed to send Signal message: server returned status 400: Failed to send message\n"+
		`sending HTTP request: Post "https://localhost:8080": transport failure`)
	assert.Len(t, *captured, 3)
}

// TestSendContextCancellation verifies that a send makes no request when the
// caller's context has already ended, and skips the remaining recipient
// batches when it ends during a request, reporting its error once.
func TestSendContextCancellation(t *testing.T) {
	t.Parallel()

	t.Run("canceled before the send", func(t *testing.T) {
		t.Parallel()

		ctx, cancel := context.WithCancel(t.Context())
		cancel()

		service := newTestService(t, testServiceURL+"/group.Z3JvdXAx")
		// The mock has no expectations, so a request fails the test.
		service.SetHTTPClient(mocks.NewMockHTTPClient(t))

		require.ErrorIs(t, service.SendContext(ctx, "Hello", nil), context.Canceled)
	})

	t.Run("canceled during a request", func(t *testing.T) {
		t.Parallel()

		ctx, cancel := context.WithCancel(t.Context())
		defer cancel()

		service := newTestService(t, testServiceURL+"/group.Z3JvdXAx")
		client := mocks.NewMockHTTPClient(t)
		client.EXPECT().Do(mock.Anything).RunAndReturn(func(req *http.Request) (*http.Response, error) {
			cancel()

			return nil, req.Context().Err()
		}).Once()
		service.SetHTTPClient(client)

		err := service.SendContext(ctx, "Hello", nil)

		require.ErrorIs(t, err, context.Canceled)
		assert.Equal(t, 1, strings.Count(err.Error(), context.Canceled.Error()), err.Error())
	})
}

// TestSendDefaultClient verifies that a send without a custom client, or after
// a nil client is set, uses a default client that honors the caller's
// context. The context is canceled before the send, so the transport returns
// before it dials.
func TestSendDefaultClient(t *testing.T) {
	t.Parallel()

	// tests lists how each case sets up the service's HTTP client.
	tests := []struct {
		configure func(*Service)
		name      string
	}{
		{name: "no client set", configure: func(*Service) {}},
		{name: "nil client", configure: func(s *Service) { s.SetHTTPClient(nil) }},
		{name: "nil *http.Client", configure: func(s *Service) { s.SetHTTPClient((*http.Client)(nil)) }},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			service := newTestService(t, testServiceURL+"?token=SECRETtoken")
			tt.configure(service)

			ctx, cancel := context.WithCancel(t.Context())
			cancel()

			err := service.SendContext(ctx, "Hello", nil)

			require.ErrorIs(t, err, context.Canceled)
			assert.NotContains(t, err.Error(), "SECRETtoken")
		})
	}
}

// TestSendDefaultClientTLS verifies the default clients against a loopback TLS
// server with a self-signed certificate. The send fails certificate
// verification unless skiptlsverify is set in the URL or the send params,
// including by a second Initialize.
func TestSendDefaultClientTLS(t *testing.T) {
	t.Parallel()

	var requests atomic.Int32

	server := httptest.NewUnstartedServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requests.Add(1)

		if r.Method != http.MethodPost || r.URL.Path != "/v2/send" {
			t.Errorf("unexpected request %s %s", r.Method, r.URL.Path)
		}

		w.WriteHeader(http.StatusCreated)
		_, _ = io.WriteString(w, sentBody)
	}))
	// The server logs the rejected handshake to the test output.
	server.Config.ErrorLog = log.New(t.Output(), "", 0)
	server.StartTLS()
	t.Cleanup(server.Close)
	// The certificate check fails before any request reaches the server.
	t.Cleanup(func() { assert.Equal(t, int32(3), requests.Load()) })

	rawURL := "signal://" + server.Listener.Addr().String() + "/" + testSource + "/" + testRecipient

	t.Run("verifies the certificate", func(t *testing.T) {
		t.Parallel()

		service := newTestService(t, rawURL)

		_, isCertErr := errors.AsType[*tls.CertificateVerificationError](service.SendContext(t.Context(), "Hello", nil))
		assert.True(t, isCertErr, "error is not a certificate verification error")
	})

	t.Run("skiptlsverify in the URL", func(t *testing.T) {
		t.Parallel()

		service := newTestService(t, rawURL+"?skiptlsverify=yes")

		require.NoError(t, service.SendContext(t.Context(), "Hello", nil))
	})

	t.Run("skiptlsverify param", func(t *testing.T) {
		t.Parallel()

		service := newTestService(t, rawURL)

		require.NoError(t, service.SendContext(t.Context(), "Hello", &types.Params{"skiptlsverify": "yes"}))
	})

	// A second Initialize that turns skiptlsverify on applies to the next
	// send, with no client left over from the first configuration.
	t.Run("skiptlsverify after initializing again", func(t *testing.T) {
		t.Parallel()

		service := newTestService(t, rawURL)
		require.NoError(t, service.Initialize(parseTestURL(t, rawURL+"?skiptlsverify=yes"), log.New(t.Output(), "", 0)))

		require.NoError(t, service.SendContext(t.Context(), "Hello", nil))
	})
}

// TestSendUsesCustomClientDialer verifies that a custom *http.Client's
// transport, such as one with a guarded dialer, carries the send.
func TestSendUsesCustomClientDialer(t *testing.T) {
	t.Parallel()

	errBlocked := errors.New("destination blocked")

	service := newTestService(t, testServiceURL)
	service.SetHTTPClient(&http.Client{Transport: &http.Transport{
		DialContext: func(context.Context, string, string) (net.Conn, error) {
			return nil, errBlocked
		},
	}})

	require.ErrorIs(t, service.SendContext(t.Context(), "Hello", nil), errBlocked)
}

// TestSendShares verifies that each recipient batch's request may use an equal share of
// the time left, so a batch that does not answer cannot use up the others'
// time. The first of two gets half of the budget, and the last gets the rest.
func TestSendShares(t *testing.T) {
	t.Parallel()

	synctest.Test(t, func(t *testing.T) {
		client, captured := newRecordingClient(t, accepted, accepted)

		service := newTestService(t, testServiceURL+"/group.Z3JvdXAx")
		service.SetHTTPClient(client)

		start := time.Now()
		budget := service.ServiceTimeout(nil)

		require.NoError(t, service.Send("Message", nil))
		assert.Equal(t, []time.Time{start.Add(budget / 2), start.Add(budget)},
			requestDeadlines(*captured))
	})
}

// TestSendSharesFromParams verifies that recipients set by params set the
// send's budget. Two batches from params share the budget for two batches,
// though the service URL names one recipient.
func TestSendSharesFromParams(t *testing.T) {
	t.Parallel()

	synctest.Test(t, func(t *testing.T) {
		client, captured := newRecordingClient(t, accepted, accepted)

		service := newTestService(t, testServiceURL)
		service.SetHTTPClient(client)

		params := types.Params{"recipients": testRecipient + ",group.Z3JvdXAx"}
		start := time.Now()
		budget := service.ServiceTimeout(&params)

		require.NoError(t, service.Send("Message", &params))
		assert.Equal(t, []time.Time{start.Add(budget / 2), start.Add(budget)},
			requestDeadlines(*captured))
	})
}

// TestServiceTimeout verifies that the send budget is one batch budget per
// recipient batch, counting recipients set by params. The stored recipients
// count when a param is invalid, and one batch budget applies before
// Initialize.
func TestServiceTimeout(t *testing.T) {
	t.Parallel()

	// tests lists URLs and params with the budget each should produce.
	tests := []struct {
		params *types.Params
		name   string
		rawURL string
		want   int
	}{
		{name: "phone numbers in one batch", rawURL: testServiceURL + "/+15559876543", want: 1},
		{name: "phone, group, and username batches", rawURL: testServiceURL + "/group.Z3JvdXAx/u:alice.01", want: 3},
		{
			name:   "recipients param",
			rawURL: testServiceURL,
			params: &types.Params{"recipients": testRecipient + ",group.Z3JvdXAx"},
			want:   2,
		},
		{
			name:   "invalid param",
			rawURL: testServiceURL + "/group.Z3JvdXAx",
			params: &types.Params{"recipients": testRecipient, "unknown": "value"},
			want:   2,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			service := newTestService(t, tt.rawURL)

			assert.Equal(t, batchBudget*time.Duration(tt.want), service.ServiceTimeout(tt.params))
		})
	}

	t.Run("before Initialize", func(t *testing.T) {
		t.Parallel()

		assert.Equal(t, batchBudget, (&Service{}).ServiceTimeout(nil))
	})

	t.Run("no recipients", func(t *testing.T) {
		t.Parallel()

		service := newTestService(t, testServiceURL)
		service.Config.Recipients = nil

		assert.Equal(t, batchBudget, service.ServiceTimeout(nil))
	})
}

// TestSendPlaintextWarning verifies that the first send over HTTP logs the
// shared warning, later sends do not, and initializing the service again lets
// it appear again. An injected client gets the warning too, because the URL
// decides the scheme.
func TestSendPlaintextWarning(t *testing.T) {
	t.Parallel()

	var logs bytes.Buffer

	client, _ := newRecordingClient(t, accepted, accepted, accepted)
	rawURL := testServiceURL + "?disabletls=yes"

	service := &Service{}
	service.SetHTTPClient(client)
	require.NoError(t, service.Initialize(parseTestURL(t, rawURL), log.New(&logs, "", 0)))

	require.NoError(t, service.Send("Hello", nil))
	require.NoError(t, service.Send("Hello", nil))
	assert.Equal(t, []string{"WARN sending without TLS"}, warningLines(logs.String()))

	require.NoError(t, service.Initialize(parseTestURL(t, rawURL), log.New(&logs, "", 0)))
	require.NoError(t, service.Send("Hello", nil))
	assert.Equal(t, []string{"WARN sending without TLS", "WARN sending without TLS"}, warningLines(logs.String()))
}

// warningLines returns the logged lines that are warnings, leaving out the
// log of each message sent.
//
// Parameters:
//   - logs: the log output.
//
// Returns:
//   - []string: the warning lines in order.
func warningLines(logs string) []string {
	var lines []string

	for line := range strings.Lines(logs) {
		if strings.HasPrefix(line, "WARN ") {
			lines = append(lines, strings.TrimSuffix(line, "\n"))
		}
	}

	return lines
}

// TestSetHTTPClient verifies that SetHTTPClient stores a custom client, and
// that nil and a nil *http.Client restore the default client.
func TestSetHTTPClient(t *testing.T) {
	t.Parallel()

	// tests lists the clients that restore the default.
	tests := []struct {
		client types.HTTPClient
		name   string
	}{
		{name: "nil", client: nil},
		{name: "nil *http.Client", client: (*http.Client)(nil)},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			service := &Service{}

			custom := mocks.NewMockHTTPClient(t)
			service.SetHTTPClient(custom)
			assert.Same(t, custom, service.client.Client())

			service.SetHTTPClient(tt.client)
			assertDefaultClient(t, service.client.Client(), false)
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
