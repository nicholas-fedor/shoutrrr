package signal

import (
	"bytes"
	"crypto/tls"
	"errors"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/url"
	"strings"
	"testing"
	"testing/iotest"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	"github.com/nicholas-fedor/shoutrrr/internal/httpbody"
	"github.com/nicholas-fedor/shoutrrr/internal/meta"
	"github.com/nicholas-fedor/shoutrrr/internal/redact"
	"github.com/nicholas-fedor/shoutrrr/pkg/types/mocks"
)

// TestBuildAPIURL verifies that the send endpoint uses HTTPS unless TLS is
// disabled, and brackets an IPv6 host.
func TestBuildAPIURL(t *testing.T) {
	t.Parallel()

	// tests lists configs with the endpoint each should produce.
	tests := []struct {
		name   string
		config Config
		want   string
	}{
		{name: "TLS", config: Config{Host: "localhost", Port: 8080}, want: testEndpoint},
		{name: "TLS disabled", config: Config{Host: "localhost", Port: 8080, DisableTLS: true}, want: "http://localhost:8080/v2/send"},
		{name: "IPv6 host", config: Config{Host: "::1", Port: 9922}, want: "https://[::1]:9922/v2/send"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			assert.Equal(t, tt.want, (&Service{}).buildAPIURL(&tt.config))
		})
	}
}

// TestCreateRequest verifies that createRequest builds a JSON POST with the
// User-Agent and authentication headers, bounded by the request timeout.
func TestCreateRequest(t *testing.T) {
	t.Parallel()

	config := &Config{Host: "localhost", Port: 8080, Token: "SECRETtoken"}
	payload := &sendMessagePayload{Message: "Hello", Number: testSource, Recipients: []string{testRecipient}}

	req, cancel, err := (&Service{}).createRequest(t.Context(), config, payload)
	require.NoError(t, err)
	t.Cleanup(cancel)

	assert.Equal(t, http.MethodPost, req.Method)
	assert.Equal(t, testEndpoint, req.URL.String())
	assert.Equal(t, "application/json", req.Header.Get("Content-Type"))
	assert.Equal(t, meta.UserAgent(), req.Header.Get("User-Agent"))
	assert.Equal(t, "Bearer SECRETtoken", req.Header.Get("Authorization"))

	deadline, ok := req.Context().Deadline()
	require.True(t, ok, "request has no deadline")
	assert.WithinDuration(t, deadline, time.Now().Add(defaultHTTPTimeout), time.Second)

	body, err := io.ReadAll(req.Body)
	require.NoError(t, err)
	assert.JSONEq(t, `{"message":"Hello","number":"+15550000000","recipients":["+15551234567"]}`, string(body))
}

// TestSendInvalidHost verifies that a host that makes the endpoint
// unparsable fails that request with a redacted URL error, without sending
// it.
func TestSendInvalidHost(t *testing.T) {
	t.Parallel()

	service := newTestService(t, testServiceURL)
	service.Config.Host = "bad host"
	// The mock has no expectations, so a request fails the test.
	service.SetHTTPClient(mocks.NewMockHTTPClient(t))

	err := service.SendContext(t.Context(), "Hello", nil)

	_, isURLErr := errors.AsType[*url.Error](err)
	require.True(t, isURLErr, "error %v is not a *url.Error", err)
	require.ErrorIs(t, err, redact.ErrInvalidURL)
	assert.NotContains(t, err.Error(), "bad host")
}

// TestParseSendResults verifies that a successful response is read as the
// array of send results signal-cli-rest-api returns, or as a single result
// object, with timestamps as quoted or plain numbers and any failed
// recipients.
func TestParseSendResults(t *testing.T) {
	t.Parallel()

	// stamp returns a pointer to the timestamp n.
	stamp := func(n int64) *timestamp {
		ts := timestamp(n)

		return &ts
	}

	// tests lists response bodies with the results each should produce.
	tests := []struct {
		name    string
		body    string
		want    []sendResult
		wantErr bool
	}{
		{name: "array", body: `[{"timestamp":"1760500000000"}]`, want: []sendResult{{Timestamp: stamp(1760500000000)}}},
		{
			name: "array with several results",
			body: `[{"timestamp":"1760500000000"},{"timestamp":1760500000001}]`,
			want: []sendResult{{Timestamp: stamp(1760500000000)}, {Timestamp: stamp(1760500000001)}},
		},
		{name: "empty array", body: `[]`, want: []sendResult{}},
		{name: "object", body: ` {"timestamp":1760500000000}`, want: []sendResult{{Timestamp: stamp(1760500000000)}}},
		{name: "zero", body: `[{"timestamp":"0"}]`, want: []sendResult{{Timestamp: stamp(0)}}},
		{name: "missing timestamp", body: `[{}]`, want: []sendResult{{}}},
		{name: "null timestamp", body: `[{"timestamp":null}]`, want: []sendResult{{}}},
		{
			name: "failed recipients",
			body: `[{"timestamp":"1760500000000","errors":{"recipients":[{"number":"+15551234567","uuid":"0b9f9c4e","reason":"UNREGISTERED_FAILURE"},{"username":"alice.01","reason":"RATE_LIMIT_FAILURE"}]}}]`,
			want: []sendResult{{
				Timestamp: stamp(1760500000000),
				Errors: &sendResultErrors{Recipients: []recipientFailure{
					{Number: "+15551234567", UUID: "0b9f9c4e", Reason: "UNREGISTERED_FAILURE"},
					{Username: "alice.01", Reason: "RATE_LIMIT_FAILURE"},
				}},
			}},
		},
		{name: "timestamp not a number", body: `[{"timestamp":"soon"}]`, wantErr: true},
		{name: "timestamp not a string or number", body: `[{"timestamp":true}]`, wantErr: true},
		{name: "not JSON", body: `sent`, wantErr: true},
		{name: "empty body", body: ``, wantErr: true},
		{name: "truncated object", body: `{"timestamp":`, wantErr: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			results, err := parseSendResults([]byte(tt.body))

			if tt.wantErr {
				require.Error(t, err)

				return
			}

			require.NoError(t, err)
			assert.Equal(t, tt.want, results)
		})
	}
}

// TestSendLogsResults verifies that a send logs the timestamp of each send
// result in the response, and a warning when the response has none or cannot
// be read, without failing the send.
func TestSendLogsResults(t *testing.T) {
	t.Parallel()

	// tests lists response bodies with the log each should start with.
	tests := []struct {
		name    string
		body    string
		wantLog string
	}{
		{name: "result array", body: sentBody, wantLog: "Message sent successfully at timestamp 1760500000000\n"},
		{
			name:    "several results",
			body:    `[{"timestamp":"1760500000000"},{"timestamp":"1760500000001"}]`,
			wantLog: "Message sent successfully at timestamp 1760500000000\nMessage sent successfully at timestamp 1760500000001\n",
		},
		{name: "result object", body: `{"timestamp":1760500000000}`, wantLog: "Message sent successfully at timestamp 1760500000000\n"},
		{name: "no timestamp", body: `[{}]`, wantLog: "Warning: response has no timestamp\n"},
		{name: "not JSON", body: `sent`, wantLog: "Warning: failed to parse response: "},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			var logs bytes.Buffer

			service := &Service{}
			require.NoError(t, service.Initialize(parseTestURL(t, testServiceURL), log.New(&logs, "", 0)))

			client, _ := newRecordingClient(t, cannedResponse{statusCode: http.StatusCreated, body: tt.body})
			service.SetHTTPClient(client)

			require.NoError(t, service.Send("Hello", nil))
			assert.True(t, strings.HasPrefix(logs.String(), tt.wantLog), "log %q does not start with %q", logs.String(), tt.wantLog)
		})
	}
}

// TestSendReportsFailedRecipients verifies that a send fails when a
// successful response lists recipients that could not be reached, naming
// each by number, username, or UUID with its reason, without logging the
// send as successful, and that the requests for the other recipient types are
// still sent.
func TestSendReportsFailedRecipients(t *testing.T) {
	t.Parallel()

	var logs bytes.Buffer

	service := &Service{}
	require.NoError(t, service.Initialize(parseTestURL(t, testServiceURL+"/+15559876543/+15550001111/u:alice.01"), log.New(&logs, "", 0)))

	client, captured := newRecordingClient(t,
		cannedResponse{
			statusCode: http.StatusCreated,
			body: `[{"timestamp":"1760500000000","errors":{"recipients":[` +
				`{"number":"+15559876543","uuid":"0b9f9c4e","reason":"UNREGISTERED_FAILURE"},` +
				`{"uuid":"7d1e2a90","reason":"NETWORK_FAILURE"}]}}]`,
		},
		cannedResponse{
			statusCode: http.StatusCreated,
			body:       `[{"timestamp":"1760500000001","errors":{"recipients":[{"username":"alice.01","reason":"RATE_LIMIT_FAILURE"}]}}]`,
		},
	)
	service.SetHTTPClient(client)

	err := service.SendContext(t.Context(), "Hello", nil)

	require.ErrorIs(t, err, ErrSendFailed)
	require.ErrorIs(t, err, ErrRecipientsFailed)
	require.EqualError(t, err,
		"failed to send Signal message: delivery failed for recipients: +15559876543 (UNREGISTERED_FAILURE), 7d1e2a90 (NETWORK_FAILURE)\n"+
			"failed to send Signal message: delivery failed for recipients: alice.01 (RATE_LIMIT_FAILURE)")
	assert.Len(t, *captured, 2)
	assert.Empty(t, logs.String())
}

// TestSendLargeResponse verifies that a send reads a successful response
// large enough to hold the failed recipients of a full Signal group, and that
// a response beyond the read limit is accepted with a warning that the
// recipient results were not checked.
func TestSendLargeResponse(t *testing.T) {
	t.Parallel()

	// failures returns a result array with n failed recipients.
	failures := func(n int) string {
		entries := make([]string, n)
		for i := range entries {
			entries[i] = fmt.Sprintf(`{"number":"+1555%07d","uuid":"0b9f9c4e-6a1d-4e8f-9c55-%012d","reason":"UNREGISTERED_FAILURE"}`, i, i)
		}

		return `[{"timestamp":"1760500000000","errors":{"recipients":[` + strings.Join(entries, ",") + `]}}]`
	}

	t.Run("failed recipients of a full group", func(t *testing.T) {
		t.Parallel()

		service := newTestService(t, "signal://localhost:8080/"+testSource+"/group.Z3JvdXAx")
		client, _ := newRecordingClient(t, cannedResponse{statusCode: http.StatusCreated, body: failures(1000)})
		service.SetHTTPClient(client)

		err := service.SendContext(t.Context(), "Hello", nil)

		require.ErrorIs(t, err, ErrRecipientsFailed)
		assert.Contains(t, err.Error(), "+15550000999 (UNREGISTERED_FAILURE)")
	})

	t.Run("beyond the read limit", func(t *testing.T) {
		t.Parallel()

		var logs bytes.Buffer

		service := &Service{}
		require.NoError(t, service.Initialize(parseTestURL(t, testServiceURL), log.New(&logs, "", 0)))

		body := failures(12000)
		require.Greater(t, len(body), httpbody.MaxSize)

		client, _ := newRecordingClient(t, cannedResponse{statusCode: http.StatusCreated, body: body})
		service.SetHTTPClient(client)

		require.NoError(t, service.SendContext(t.Context(), "Hello", nil))
		assert.Equal(t, "Warning: response exceeds 1048576 bytes, so the recipient results were not checked\n", logs.String())
	})
}

// TestSendBodyReadError verifies that a body that fails to read after a
// success status is logged without failing the send, because the API accepted
// the message, while after an error status the read error fails the send.
func TestSendBodyReadError(t *testing.T) {
	t.Parallel()

	errRead := errors.New("connection reset")

	// tests lists response statuses with whether the send should fail.
	tests := []struct {
		name       string
		wantLog    string
		statusCode int
		wantErr    bool
	}{
		{
			name:       "success status",
			statusCode: http.StatusCreated,
			wantLog:    "Warning: failed to read response body, so the recipient results were not checked: connection reset\n",
		},
		{name: "error status", statusCode: http.StatusBadRequest, wantErr: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			var logs bytes.Buffer

			service := &Service{}
			require.NoError(t, service.Initialize(parseTestURL(t, testServiceURL), log.New(&logs, "", 0)))

			client := mocks.NewMockHTTPClient(t)
			client.EXPECT().Do(mock.Anything).Return(&http.Response{
				StatusCode: tt.statusCode,
				Body:       io.NopCloser(iotest.ErrReader(errRead)),
			}, nil).Once()
			service.SetHTTPClient(client)

			err := service.SendContext(t.Context(), "Hello", nil)

			if tt.wantErr {
				require.ErrorIs(t, err, errRead)
				assert.Empty(t, logs.String())

				return
			}

			require.NoError(t, err)
			assert.Equal(t, tt.wantLog, logs.String())
		})
	}
}

// TestRecipientFailureDescribe verifies that a failed recipient is named by
// number, then username, then UUID, with the reason.
func TestRecipientFailureDescribe(t *testing.T) {
	t.Parallel()

	// tests lists failures with the description each should produce.
	tests := []struct {
		name    string
		want    string
		failure recipientFailure
	}{
		{
			name:    "number",
			failure: recipientFailure{Number: testRecipient, Username: "alice.01", UUID: "0b9f9c4e", Reason: "UNREGISTERED_FAILURE"},
			want:    testRecipient + " (UNREGISTERED_FAILURE)",
		},
		{
			name:    "username",
			failure: recipientFailure{Username: "alice.01", UUID: "0b9f9c4e", Reason: "RATE_LIMIT_FAILURE"},
			want:    "alice.01 (RATE_LIMIT_FAILURE)",
		},
		{name: "UUID", failure: recipientFailure{UUID: "0b9f9c4e", Reason: "NETWORK_FAILURE"}, want: "0b9f9c4e (NETWORK_FAILURE)"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			assert.Equal(t, tt.want, tt.failure.describe())
		})
	}
}

// TestFormatSendError verifies the error for a failed send: the status, the
// API error with its account and challenge tokens, or else the trimmed body,
// and a marker when the body was truncated.
func TestFormatSendError(t *testing.T) {
	t.Parallel()

	// tests lists error responses with the message each should produce.
	tests := []struct {
		name       string
		body       string
		want       string
		statusCode int
		truncated  bool
	}{
		{
			name:       "API error",
			statusCode: http.StatusBadRequest,
			body:       `{"error":"Couldn't process request - invalid request"}`,
			want:       "failed to send Signal message: server returned status 400: Couldn't process request - invalid request",
		},
		{
			name:       "rate limit with challenge tokens",
			statusCode: http.StatusTooManyRequests,
			body:       `{"error":"rate limited","challenge_tokens":["abc","def"],"account":"+15550000000"}`,
			want:       "failed to send Signal message: server returned status 429: rate limited (account +15550000000; challenge tokens: abc,def)",
		},
		{
			name:       "account only",
			statusCode: http.StatusTooManyRequests,
			body:       `{"error":"rate limited","account":"+15550000000"}`,
			want:       "failed to send Signal message: server returned status 429: rate limited (account +15550000000)",
		},
		{
			name:       "challenge tokens only",
			statusCode: http.StatusTooManyRequests,
			body:       `{"error":"rate limited","challenge_tokens":["abc"]}`,
			want:       "failed to send Signal message: server returned status 429: rate limited (challenge tokens: abc)",
		},
		{
			name:       "truncated API error",
			statusCode: http.StatusTooManyRequests,
			body:       `{"error":"rate limited","account":"+15550000000"}`,
			truncated:  true,
			want:       "failed to send Signal message: server returned status 429: rate limited (account +15550000000) [truncated]",
		},
		{
			name:       "plain body",
			statusCode: http.StatusBadGateway,
			body:       "  upstream unavailable\n",
			want:       "failed to send Signal message: server returned status 502: upstream unavailable",
		},
		{
			name:       "JSON without an error",
			statusCode: http.StatusInternalServerError,
			body:       `{"message":"oops"}`,
			want:       `failed to send Signal message: server returned status 500: {"message":"oops"}`,
		},
		{
			name:       "empty body",
			statusCode: http.StatusInternalServerError,
			want:       "failed to send Signal message: server returned status 500",
		},
		{
			name:       "truncated empty body",
			statusCode: http.StatusInternalServerError,
			truncated:  true,
			want:       "failed to send Signal message: server returned status 500 [truncated]",
		},
		{
			name:       "long plain body",
			statusCode: http.StatusBadGateway,
			body:       strings.Repeat("a", maxErrorBodySize) + "tail",
			want:       "failed to send Signal message: server returned status 502: " + strings.Repeat("a", maxErrorBodySize) + " [truncated]",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			err := formatSendError(tt.statusCode, []byte(tt.body), tt.truncated)

			require.ErrorIs(t, err, ErrSendFailed)
			assert.EqualError(t, err, tt.want)
		})
	}
}

// TestHTTPClientFor verifies that a custom client serves every send, and that
// otherwise the stored default client serves sends whose skiptlsverify
// matches the stored config, with a new default client for the others.
func TestHTTPClientFor(t *testing.T) {
	t.Parallel()

	t.Run("custom client", func(t *testing.T) {
		t.Parallel()

		client := mocks.NewMockHTTPClient(t)
		service := &Service{Config: &Config{}}
		service.SetHTTPClient(client)

		assert.Same(t, client, service.httpClientFor(&Config{SkipTLSVerify: true}))
	})

	// tests lists stored and per-send skiptlsverify values with whether the
	// stored default client should serve the send.
	tests := []struct {
		name       string
		storedSkip bool
		sendSkip   bool
		wantStored bool
	}{
		{name: "both verify", wantStored: true},
		{name: "both skip", storedSkip: true, sendSkip: true, wantStored: true},
		{name: "send skips", sendSkip: true},
		{name: "send verifies", storedSkip: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			service := &Service{Config: &Config{SkipTLSVerify: tt.storedSkip}}
			service.SetLogger(log.New(t.Output(), "", 0))

			stored := service.newHTTPClient(tt.storedSkip)
			service.httpClient = stored

			client := service.httpClientFor(&Config{SkipTLSVerify: tt.sendSkip})

			if tt.wantStored {
				assert.Same(t, stored, client)

				return
			}

			assert.NotSame(t, stored, client)
			assertSkipsVerify(t, client, tt.sendSkip)
		})
	}

	t.Run("no stored client", func(t *testing.T) {
		t.Parallel()

		service := &Service{}
		service.SetLogger(log.New(t.Output(), "", 0))

		assertSkipsVerify(t, service.httpClientFor(nil), false)
		assertSkipsVerify(t, service.httpClientFor(&Config{SkipTLSVerify: true}), true)
	})
}

// TestNewHTTPClient verifies that the default client logs a warning when it
// skips certificate verification.
func TestNewHTTPClient(t *testing.T) {
	t.Parallel()

	var logs bytes.Buffer

	service := &Service{}
	service.SetLogger(log.New(&logs, "", 0))

	assertSkipsVerify(t, service.newHTTPClient(false), false)
	assert.Empty(t, logs.String())

	assertSkipsVerify(t, service.newHTTPClient(true), true)
	assert.Equal(t, "Warning: TLS verification is disabled, making connections insecure\n", logs.String())
}

// assertSkipsVerify asserts that client is a default client with the request
// timeout, TLS 1.2 or later, and the given certificate verification setting.
//
// Parameters:
//   - t: the test.
//   - client: the client to check.
//   - wantSkip: whether the client should skip certificate verification.
func assertSkipsVerify(t *testing.T, client any, wantSkip bool) {
	t.Helper()

	httpClient, ok := client.(*http.Client)
	require.True(t, ok, "client is %T", client)
	assert.Equal(t, defaultHTTPTimeout, httpClient.Timeout)

	transport, ok := httpClient.Transport.(*http.Transport)
	require.True(t, ok, "transport is %T", httpClient.Transport)
	require.NotNil(t, transport.TLSClientConfig)
	assert.Equal(t, uint16(tls.VersionTLS12), transport.TLSClientConfig.MinVersion)
	assert.Equal(t, wantSkip, transport.TLSClientConfig.InsecureSkipVerify)
}
