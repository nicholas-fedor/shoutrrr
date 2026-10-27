package redact

import (
	"context"
	"errors"
	"fmt"
	"net"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// temporaryError is a cause that classifies itself as temporary.
type temporaryError struct{}

// testSecret is a credential embedded in test URLs that must never survive redaction.
const testSecret = "SECRETredactTOKEN"

// TestRequestURL verifies that only the scheme and host of a request URL are
// kept, and that the result parses back to the same scheme and host, with any
// percent escapes in the host, such as an IPv6 zone, kept escaped.
func TestRequestURL(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name   string
		rawURL string
		want   string
	}{
		{name: "path token", rawURL: "https://api.example.com/bot" + testSecret + "/send", want: "https://api.example.com"},
		{name: "query token", rawURL: "https://api.example.com/hook?token=" + testSecret, want: "https://api.example.com"},
		{name: "userinfo", rawURL: "https://user:" + testSecret + "@api.example.com:8443/", want: "https://api.example.com:8443"},
		{name: "unparsable", rawURL: "https://api.example.com/%zz" + testSecret, want: Placeholder},
		{name: "no host", rawURL: testSecret, want: Placeholder},
		{name: "percent-escaped host", rawURL: "https://user:" + testSecret + "@0%250:/path", want: "https://0%250:"},
		{name: "IPv6 zone", rawURL: "http://[fe80::1%25en0]:8080/" + testSecret, want: "http://[fe80::1%25en0]:8080"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			got := RequestURL(tt.rawURL)
			assert.Equal(t, tt.want, got)

			if got == Placeholder {
				return
			}

			want, err := url.Parse(tt.rawURL)
			require.NoError(t, err)

			parsed, err := url.Parse(got)
			require.NoError(t, err)
			assert.Equal(t, want.Scheme, parsed.Scheme)
			assert.Equal(t, want.Host, parsed.Host)
		})
	}
}

// TestURLError verifies that a *url.Error loses its secrets but keeps the operation,
// the underlying error, and the behavior callers rely on to classify failures.
func TestURLError(t *testing.T) {
	t.Parallel()

	var netErr net.Error = &net.DNSError{Err: "timeout", IsTimeout: true}

	err := URLError(&url.Error{
		Op:  http.MethodPost,
		URL: "https://api.example.com/bot" + testSecret + "/sendMessage",
		Err: netErr,
	})

	require.Error(t, err)
	assert.NotContains(t, err.Error(), testSecret)
	assert.Contains(t, err.Error(), "https://api.example.com")
	require.ErrorIs(t, err, netErr)

	urlErr, ok := errors.AsType[*url.Error](err)
	require.True(t, ok)
	assert.Equal(t, http.MethodPost, urlErr.Op)
	assert.True(t, urlErr.Timeout())
}

// TestURLErrorParse verifies that URL parse errors drop both the URL and the parse
// detail, because net/url quotes parts of the URL, such as an invalid port.
func TestURLErrorParse(t *testing.T) {
	t.Parallel()

	_, parseErr := url.Parse("https://api.example.com:" + testSecret + "/") //nolint:staticcheck // The invalid port is the point of the test.
	require.Error(t, parseErr)

	err := URLError(parseErr)

	assert.NotContains(t, err.Error(), testSecret)
	require.ErrorIs(t, err, ErrInvalidURL)
}

// TestURLErrorPassesThroughOtherErrors verifies that errors without a *url.Error at
// the top level are returned unchanged.
func TestURLErrorPassesThroughOtherErrors(t *testing.T) {
	t.Parallel()

	plain := errors.New("plain failure")

	assert.Same(t, plain, URLError(plain))
	assert.NoError(t, URLError(nil))
}

// TestURLErrorRedactsRedirectLocation verifies that the error of a malformed
// redirect does not leak its Location header. net/http quotes the header value
// in the cause of the *url.Error it returns, so the cause must be redacted as
// well as the URL.
func TestURLErrorRedactsRedirectLocation(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Location", "https://hooks.example.invalid/%zz?token="+testSecret)
		w.WriteHeader(http.StatusFound)
	}))
	t.Cleanup(server.Close)

	req, err := http.NewRequestWithContext(context.Background(), http.MethodGet, server.URL, http.NoBody)
	require.NoError(t, err)

	res, err := server.Client().Do(req)
	if res != nil {
		require.NoError(t, res.Body.Close())
	}

	err = URLError(err)

	require.Error(t, err)
	assert.NotContains(t, err.Error(), testSecret)
	assert.Contains(t, err.Error(), "Location header")
}

// TestURLErrorRedactsURLsInCause verifies that URLs quoted in the cause of a
// *url.Error are reduced to scheme and host, while the cause stays matchable.
func TestURLErrorRedactsURLsInCause(t *testing.T) {
	t.Parallel()

	cause := fmt.Errorf("redirect to https://hooks.example.invalid/%s refused: %w", testSecret, context.DeadlineExceeded)

	err := URLError(&url.Error{Op: http.MethodGet, URL: "https://api.example.com/", Err: cause})

	assert.NotContains(t, err.Error(), testSecret)
	assert.Contains(t, err.Error(), "https://hooks.example.invalid")
	require.ErrorIs(t, err, context.DeadlineExceeded)

	urlErr, ok := errors.AsType[*url.Error](err)
	require.True(t, ok)
	assert.True(t, urlErr.Timeout())
}

// TestURLErrorRedactsRelativeRedirectLocation verifies that the error of a
// malformed relative redirect leaks neither copy of its Location header.
// net/http quotes the value once in its own message and again in the parse
// error it wraps.
func TestURLErrorRedactsRelativeRedirectLocation(t *testing.T) {
	t.Parallel()

	for name, location := range map[string]string{
		"invalid escape": "/hook%zz?token=" + testSecret,
		"invalid port":   "https://hooks.example.invalid:" + testSecret + "/hook",
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
				w.Header().Set("Location", location)
				w.WriteHeader(http.StatusFound)
			}))
			t.Cleanup(server.Close)

			req, err := http.NewRequestWithContext(context.Background(), http.MethodGet, server.URL, http.NoBody)
			require.NoError(t, err)

			res, err := server.Client().Do(req)
			if res != nil {
				require.NoError(t, res.Body.Close())
			}

			err = URLError(err)

			require.Error(t, err)
			assert.NotContains(t, err.Error(), testSecret)
		})
	}
}

// Error returns a message that quotes a URL, so the cause is redacted.
func (temporaryError) Error() string {
	return "retry https://hooks.example.invalid/" + testSecret
}

// Temporary reports that the failure is temporary.
func (temporaryError) Temporary() bool { return true }

// TestURLErrorPreservesTemporary verifies that a redacted cause keeps the temporary
// classification that url.Error.Temporary reports.
func TestURLErrorPreservesTemporary(t *testing.T) {
	t.Parallel()

	err := URLError(&url.Error{Op: http.MethodGet, URL: "https://api.example.com/", Err: temporaryError{}})

	urlErr, ok := errors.AsType[*url.Error](err)
	require.True(t, ok)
	assert.NotContains(t, urlErr.Error(), testSecret)
	assert.True(t, urlErr.Temporary())
}

// TestURLErrorRedactsOtherErrors verifies that errors other than *url.Error, such as
// those from a custom HTTP client, are redacted when they quote a URL.
func TestURLErrorRedactsOtherErrors(t *testing.T) {
	t.Parallel()

	blocked := fmt.Errorf("destination https://internal.example.invalid/%s blocked: %w", testSecret, context.Canceled)

	err := URLError(blocked)

	require.Error(t, err)
	assert.NotContains(t, err.Error(), testSecret)
	assert.Contains(t, err.Error(), "https://internal.example.invalid")
	require.ErrorIs(t, err, context.Canceled)
}

// TestRedactedErrorChainHidesCause verifies that no error reachable through the
// exposed chain carries the original cause or its message, while sentinel
// matching, Timeout and Temporary keep working.
func TestRedactedErrorChainHidesCause(t *testing.T) {
	t.Parallel()

	cause := fmt.Errorf("redirect to https://hooks.example.invalid/%s refused: %w", testSecret, context.DeadlineExceeded)

	err := URLError(&url.Error{Op: http.MethodGet, URL: "https://api.example.com/", Err: cause})

	for _, chained := range errorChain(err) {
		assert.NotContains(t, chained.Error(), testSecret)
		assert.NotEqual(t, cause, chained, "the original cause must not be reachable")
	}

	require.ErrorIs(t, err, context.DeadlineExceeded)

	urlErr, ok := errors.AsType[*url.Error](err)
	require.True(t, ok)
	assert.True(t, urlErr.Timeout())
}

// errorChain returns every error reachable from err through Unwrap.
//
// Parameters:
//   - err: the error to walk.
//
// Returns:
//   - []error: err and every error it wraps, depth first.
func errorChain(err error) []error {
	if err == nil {
		return nil
	}

	chain := []error{err}

	switch wrapped := err.(type) {
	case interface{ Unwrap() error }:
		chain = append(chain, errorChain(wrapped.Unwrap())...)
	case interface{ Unwrap() []error }:
		for _, inner := range wrapped.Unwrap() {
			chain = append(chain, errorChain(inner)...)
		}
	}

	return chain
}
