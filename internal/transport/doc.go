// Package transport builds the HTTP transports that services use when no HTTP
// client is injected.
//
// When [net/http.DefaultTransport] is an [*net/http.Transport], each transport is
// a clone of it, so it keeps its dial, idle and handshake timeouts and a service's
// default client behaves like the standard library's client apart from its TLS
// settings. When DefaultTransport has been replaced by a RoundTripper that is not
// an [*net/http.Transport], the transport uses the standard library's default
// settings instead.
//
// Transports read the proxy environment variables (HTTP_PROXY, HTTPS_PROXY and
// NO_PROXY) unless the application has set its own proxy function on
// DefaultTransport, which is kept.
package transport
