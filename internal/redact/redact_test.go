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
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	"github.com/nicholas-fedor/shoutrrr/pkg/types/mocks"
)

// testSecret is a credential embedded in test URLs that must never survive redaction.
const testSecret = "SECRETredactTOKEN"

// TestRequestURL verifies that only the scheme and host of a request URL are kept.
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
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			assert.Equal(t, tt.want, RequestURL(tt.rawURL))
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

// TestHTTPClient verifies that the wrapper redacts transport errors from the
// wrapped client and passes successful responses through untouched.
func TestHTTPClient(t *testing.T) {
	t.Parallel()

	req, err := http.NewRequestWithContext(
		context.Background(), http.MethodGet, "https://api.example.com/"+testSecret, http.NoBody,
	)
	require.NoError(t, err)

	failing := mocks.NewMockHTTPClient(t)
	failing.EXPECT().Do(mock.Anything).Return(nil, &url.Error{
		Op: http.MethodGet, URL: req.URL.String(), Err: context.DeadlineExceeded,
	})

	_, err = HTTPClient(failing).Do(req)
	require.ErrorIs(t, err, context.DeadlineExceeded)
	assert.NotContains(t, err.Error(), testSecret)

	want := &http.Response{StatusCode: http.StatusOK, Body: http.NoBody}
	succeeding := mocks.NewMockHTTPClient(t)
	succeeding.EXPECT().Do(mock.Anything).Return(want, nil)

	res, err := HTTPClient(succeeding).Do(req)
	require.NoError(t, err)
	assert.Same(t, want, res)

	require.NoError(t, res.Body.Close())
}

// TestHTTPClientWrapsOnce verifies that wrapping is idempotent and that a nil client
// stays nil, so callers can apply it unconditionally.
func TestHTTPClientWrapsOnce(t *testing.T) {
	t.Parallel()

	inner := mocks.NewMockHTTPClient(t)
	wrapped := HTTPClient(inner)

	assert.Same(t, wrapped, HTTPClient(wrapped))
	assert.Nil(t, HTTPClient(nil))
	assert.NotContains(t, Placeholder, testSecret)
}

// TestHTTPClientRedactsRedirectLocation verifies that a malformed redirect does not
// leak its Location header. net/http quotes the header value in the cause of the
// *url.Error it returns, so the cause must be redacted as well as the URL.
func TestHTTPClientRedactsRedirectLocation(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Location", "https://hooks.example.invalid/%zz?token="+testSecret)
		w.WriteHeader(http.StatusFound)
	}))
	t.Cleanup(server.Close)

	req, err := http.NewRequestWithContext(context.Background(), http.MethodGet, server.URL, http.NoBody)
	require.NoError(t, err)

	res, err := HTTPClient(server.Client()).Do(req)
	if res != nil {
		require.NoError(t, res.Body.Close())
	}

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
