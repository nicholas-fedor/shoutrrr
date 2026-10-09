package transport

import (
	"crypto/tls"
	"net/http"
	"net/url"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// roundTripperFunc adapts a function to [http.RoundTripper], standing in for a
// replaced [http.DefaultTransport].
type roundTripperFunc func(*http.Request) (*http.Response, error)

// RoundTrip calls f.
//
// Parameters:
//   - req: the request.
//
// Returns:
//   - *http.Response: the response from f.
//   - error: the error from f.
func (f roundTripperFunc) RoundTrip(req *http.Request) (*http.Response, error) { return f(req) }

// TestNewClonesDefaultTransport verifies that New starts from a copy of
// http.DefaultTransport, so it keeps the proxy and timeout settings without
// sharing or changing the default transport.
func TestNewClonesDefaultTransport(t *testing.T) {
	t.Parallel()

	base, ok := http.DefaultTransport.(*http.Transport)
	require.True(t, ok, "http.DefaultTransport must be an *http.Transport for this test")

	tlsConfig := &tls.Config{MinVersion: tls.VersionTLS12}
	transport := New(tlsConfig)

	assert.NotSame(t, base, transport)
	assert.NotNil(t, transport.Proxy, "the transport must read the proxy environment variables")
	assert.Equal(t, base.TLSHandshakeTimeout, transport.TLSHandshakeTimeout)
	assert.Equal(t, base.IdleConnTimeout, transport.IdleConnTimeout)
	assert.Same(t, tlsConfig, transport.TLSClientConfig)
	assert.NotSame(t, tlsConfig, base.TLSClientConfig, "the default transport must keep its own TLS settings")
}

// TestNewKeepsDefaultTLSWhenConfigIsNil verifies that a nil TLS config leaves the
// cloned transport's TLS settings unchanged.
func TestNewKeepsDefaultTLSWhenConfigIsNil(t *testing.T) {
	t.Parallel()

	base, ok := http.DefaultTransport.(*http.Transport)
	require.True(t, ok, "http.DefaultTransport must be an *http.Transport for this test")

	assert.Equal(t, base.TLSClientConfig, New(nil).TLSClientConfig)
}

// TestNewBuildsTransportWhenDefaultIsReplaced verifies that New still returns a
// proxy-aware transport with the standard timeouts when http.DefaultTransport has
// been replaced, as HTTP mocking libraries do.
//
//nolint:paralleltest // Replaces http.DefaultTransport for the duration of the test.
func TestNewBuildsTransportWhenDefaultIsReplaced(t *testing.T) {
	original := http.DefaultTransport
	http.DefaultTransport = roundTripperFunc(func(*http.Request) (*http.Response, error) {
		return nil, http.ErrNotSupported
	})

	t.Cleanup(func() { http.DefaultTransport = original })

	tlsConfig := &tls.Config{MinVersion: tls.VersionTLS12}
	transport := New(tlsConfig)

	assert.NotNil(t, transport.Proxy, "the transport must read the proxy environment variables")
	assert.NotNil(t, transport.DialContext)
	assert.True(t, transport.ForceAttemptHTTP2)
	assert.Equal(t, tlsHandshakeTimeout, transport.TLSHandshakeTimeout)
	assert.Equal(t, idleConnTimeout, transport.IdleConnTimeout)
	assert.Same(t, tlsConfig, transport.TLSClientConfig)
}

// TestNewProxySettingsOfReplacedDefault verifies that a cloned DefaultTransport
// without a proxy function reads the proxy environment variables, and that a
// proxy function set on DefaultTransport is kept.
//
//nolint:paralleltest // Replaces http.DefaultTransport for the duration of the test.
func TestNewProxySettingsOfReplacedDefault(t *testing.T) {
	proxyURL := &url.URL{Scheme: "http", Host: "proxy.example.invalid:3128"}
	customProxy := func(*http.Request) (*url.URL, error) { return proxyURL, nil }

	tests := []struct {
		name      string
		base      *http.Transport
		wantProxy *url.URL
	}{
		{name: "without a proxy function", base: &http.Transport{}, wantProxy: nil},
		{name: "with a proxy function", base: &http.Transport{Proxy: customProxy}, wantProxy: proxyURL},
	}

	original := http.DefaultTransport

	t.Cleanup(func() { http.DefaultTransport = original })

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			http.DefaultTransport = tt.base

			transport := New(nil)
			require.NotNil(t, transport.Proxy, "the transport must have a proxy function")

			req, err := http.NewRequestWithContext(t.Context(), http.MethodGet, "https://target.example.invalid/", http.NoBody)
			require.NoError(t, err)

			got, err := transport.Proxy(req)
			require.NoError(t, err)

			if tt.wantProxy != nil {
				assert.Equal(t, tt.wantProxy, got, "the proxy function set on DefaultTransport must be kept")
			}
		})
	}
}
