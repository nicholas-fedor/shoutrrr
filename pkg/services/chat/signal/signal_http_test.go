package signal

import (
	"bytes"
	"crypto/tls"
	"errors"
	"io"
	"log"
	"net/http"
	"net/url"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

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

// TestParseTimestamp verifies that a timestamp is read as a JSON number or a
// quoted number, and that a missing or invalid timestamp is an error.
func TestParseTimestamp(t *testing.T) {
	t.Parallel()

	// tests lists response bodies with the timestamp each should produce.
	tests := []struct {
		name    string
		body    string
		want    int64
		wantErr bool
	}{
		{name: "number", body: `{"timestamp":1760500000000}`, want: 1760500000000},
		{name: "quoted number", body: `{"timestamp":"1760500000000"}`, want: 1760500000000},
		{name: "zero", body: `{"timestamp":0}`},
		{name: "missing", body: `{}`, wantErr: true},
		{name: "null", body: `{"timestamp":null}`, wantErr: true},
		{name: "not a number", body: `{"timestamp":"soon"}`, wantErr: true},
		{name: "not JSON", body: `sent`, wantErr: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			timestamp, err := parseTimestamp([]byte(tt.body))

			if tt.wantErr {
				require.Error(t, err)

				return
			}

			require.NoError(t, err)
			assert.Equal(t, tt.want, timestamp)
		})
	}
}

// TestParseResponse verifies that the timestamp of a successful send is
// logged, or else a warning with the parse error.
func TestParseResponse(t *testing.T) {
	t.Parallel()

	// tests lists response bodies with the log each should produce.
	tests := []struct {
		name    string
		body    string
		wantLog string
	}{
		{name: "timestamp", body: `{"timestamp":"1760500000000"}`, wantLog: "Message sent successfully at timestamp 1760500000000\n"},
		{name: "no timestamp", body: `{}`, wantLog: "Warning: failed to parse response: "},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			var logs bytes.Buffer

			service := &Service{}
			service.SetLogger(log.New(&logs, "", 0))
			service.parseResponse([]byte(tt.body))

			assert.True(t, strings.HasPrefix(logs.String(), tt.wantLog), "log %q does not start with %q", logs.String(), tt.wantLog)
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
