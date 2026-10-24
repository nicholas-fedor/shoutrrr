package gotify

import (
	"crypto/tls"
	"net/http"
	"time"

	"github.com/nicholas-fedor/shoutrrr/internal/transport"
)

// HTTPClientManager handles HTTP client creation and configuration.
//
// Deprecated: The service builds its default client from its TLS settings and
// does not use an HTTPClientManager. Inject a client with
// [Service.SetHTTPClient] instead.
type HTTPClientManager interface {
	// CreateTransport builds the transport for config's TLS settings.
	CreateTransport(config *Config) *http.Transport
	// CreateClient builds a client that sends through transport.
	CreateClient(transport *http.Transport) *http.Client
}

// DefaultHTTPClientManager provides the default implementation of HTTPClientManager.
//
// Deprecated: The service builds its default client from its TLS settings and
// does not use an HTTPClientManager. Inject a client with
// [Service.SetHTTPClient] instead.
type DefaultHTTPClientManager struct{}

// Default client settings.
const (
	// HTTPTimeout defines the HTTP client timeout in seconds.
	HTTPTimeout = 10
)

// CreateClient creates an HTTP client with timeout and transport.
//
// Parameters:
//   - httpTransport: the HTTP transport with TLS and proxy configuration.
//
// Returns:
//   - *http.Client: a client that sends through httpTransport with the [HTTPTimeout] timeout.
func (m *DefaultHTTPClientManager) CreateClient(httpTransport *http.Transport) *http.Client {
	return &http.Client{
		Transport: httpTransport,             // Use the configured transport for TLS and proxy handling
		Timeout:   HTTPTimeout * time.Second, // Set timeout to prevent hanging requests
	}
}

// CreateTransport sets up the HTTP transport with TLS configuration and proxy settings.
// The transport honors the proxy environment variables, enforces TLS 1.2 or later,
// and skips certificate verification when the config disables TLS or verification.
//
// Parameters:
//   - config: the service configuration with the TLS settings.
//
// Returns:
//   - *http.Transport: the transport for the default HTTP client.
func (m *DefaultHTTPClientManager) CreateTransport(config *Config) *http.Transport {
	return transport.New(newTLSConfig(config))
}

// newTLSConfig returns the TLS settings of the default HTTP client: TLS 1.2 or
// later, without certificate verification when config disables TLS or
// verification.
//
// Parameters:
//   - config: the service configuration with the TLS settings.
//
// Returns:
//   - *tls.Config: the TLS settings.
func newTLSConfig(config *Config) *tls.Config {
	return &tls.Config{
		MinVersion:         tls.VersionTLS12,
		InsecureSkipVerify: config.DisableTLS || config.InsecureSkipVerify,
	}
}
