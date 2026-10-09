package transport

import (
	"crypto/tls"
	"net"
	"net/http"
	"time"
)

// Defaults that match [net/http.DefaultTransport], used when it has been replaced
// by a value that is not an [*http.Transport].
const (
	// dialTimeout bounds establishing a TCP connection.
	dialTimeout = 30 * time.Second
	// dialKeepAlive is the TCP keep-alive period of each connection.
	dialKeepAlive = 30 * time.Second
	// maxIdleConns limits idle connections across all hosts.
	maxIdleConns = 100
	// idleConnTimeout closes idle connections after this long.
	idleConnTimeout = 90 * time.Second
	// tlsHandshakeTimeout bounds the TLS handshake.
	tlsHandshakeTimeout = 10 * time.Second
	// expectContinueTimeout bounds the wait for a 100-continue response.
	expectContinueTimeout = time.Second
)

// New returns a transport that honors the proxy environment variables and uses
// tlsConfig for TLS connections.
//
// It clones [net/http.DefaultTransport] so the transport keeps its proxy,
// dial and timeout settings. A clone without a proxy function reads the proxy
// environment variables, and a proxy function set on DefaultTransport is kept.
// When DefaultTransport has been replaced by a value that is not an
// [*http.Transport], it builds a transport with the standard settings.
//
// Parameters:
//   - tlsConfig: the TLS settings for HTTPS connections, or nil to keep the
//     default settings.
//
// Returns:
//   - *http.Transport: a new transport that is not shared with other callers.
func New(tlsConfig *tls.Config) *http.Transport {
	var transport *http.Transport

	if base, ok := http.DefaultTransport.(*http.Transport); ok {
		transport = base.Clone()
		if transport.Proxy == nil {
			transport.Proxy = http.ProxyFromEnvironment
		}
	} else {
		transport = &http.Transport{
			Proxy: http.ProxyFromEnvironment,
			//nolint:exhaustruct_v5 // Matches net/http's default dialer, which leaves the other fields at zero.
			DialContext: (&net.Dialer{
				Timeout:   dialTimeout,
				KeepAlive: dialKeepAlive,
			}).DialContext,
			ForceAttemptHTTP2:     true,
			MaxIdleConns:          maxIdleConns,
			IdleConnTimeout:       idleConnTimeout,
			TLSHandshakeTimeout:   tlsHandshakeTimeout,
			ExpectContinueTimeout: expectContinueTimeout,
		}
	}

	if tlsConfig != nil {
		transport.TLSClientConfig = tlsConfig
	}

	return transport
}
