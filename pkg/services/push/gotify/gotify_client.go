package gotify

import (
	"crypto/tls"
	"net/http"
	"time"

	"github.com/nicholas-fedor/shoutrrr/internal/transport"
	"github.com/nicholas-fedor/shoutrrr/pkg/util/jsonclient"
)

// HTTPClientManager handles HTTP client creation and configuration.
type HTTPClientManager interface {
	CreateTransport(config *Config) *http.Transport
	CreateClient(transport *http.Transport) *http.Client
}

// DefaultHTTPClientManager provides the default implementation of HTTPClientManager.
type DefaultHTTPClientManager struct{}

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
	return transport.New(&tls.Config{
		MinVersion: tls.VersionTLS12,
		InsecureSkipVerify: config.DisableTLS ||
			config.InsecureSkipVerify,
	})
}

// initClient initializes the HTTP client and related components.
// This method ensures that the transport, HTTP client, JSON client,
// and TLS warning logging are performed when needed, allowing re-initialization if the client becomes nil.
// This function is called by the Service and modifies its fields.
func initClient(service *Service, manager HTTPClientManager) {
	service.mu.Lock()
	defer service.mu.Unlock()

	if service.httpClient == nil || service.client == nil {
		httpTransport := manager.CreateTransport(service.Config)
		service.httpClient = manager.CreateClient(httpTransport)

		service.client = jsonclient.NewWithHTTPClient(service.httpClient)
		if service.Config.DisableTLS {
			service.Log("Warning: TLS is disabled, using insecure HTTP connections")
		}

		if service.Config.InsecureSkipVerify {
			service.Log("Warning: TLS certificate verification is disabled")
		}
	}
}
