// Package transport builds the HTTP transports that services use when no HTTP
// client is injected.
//
// Every transport it returns keeps the dial, idle and handshake timeouts of
// [net/http.DefaultTransport], so a service's default client behaves like the
// standard library's client apart from its TLS settings. It reads the proxy
// environment variables (HTTP_PROXY, HTTPS_PROXY and NO_PROXY) unless the
// application has set its own proxy function on DefaultTransport, which is kept.
package transport
