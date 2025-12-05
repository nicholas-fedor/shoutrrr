package gotify

import (
	"crypto/tls"
	"net/http"
	"time"

	"github.com/nicholas-fedor/shoutrrr/pkg/util/jsonclient"
)

const (
	// HTTPTimeout defines the HTTP client timeout in seconds.
	HTTPTimeout = 10
)

// HTTPClientManager handles HTTP client creation and configuration.
type HTTPClientManager interface {
	CreateTransport(config *Config) *http.Transport
	CreateClient(transport *http.Transport) *http.Client
}

// DefaultHTTPClientManager provides the default implementation of HTTPClientManager.
type DefaultHTTPClientManager struct{}

// CreateTransport sets up the HTTP transport with TLS configuration and proxy settings.
// This method configures the underlying HTTP transport layer with appropriate security settings
// based on the service configuration, particularly handling TLS verification preferences.
// Returns: *http.Transport configured with TLS settings for secure or insecure connections.
func (m *DefaultHTTPClientManager) CreateTransport(config *Config) *http.Transport {
	return &http.Transport{
		TLSClientConfig: &tls.Config{
			MinVersion: tls.VersionTLS12,
			InsecureSkipVerify: config.DisableTLS || //nolint:gosec // Intentionally allow insecure connections when TLS is disabled or explicitly configured
				config.InsecureSkipVerify,
		},
	}
}

// CreateClient creates an HTTP client with timeout and transport.
// This method assembles an HTTP client with the configured transport and timeout settings
// to ensure reliable API communication with appropriate performance characteristics.
// Parameters:
//   - transport: The HTTP transport with TLS and proxy configuration
//
// Returns: *http.Client ready for making API requests with proper timeout and transport settings.
func (m *DefaultHTTPClientManager) CreateClient(transport *http.Transport) *http.Client {
	return &http.Client{
		Transport: transport,                 // Use the configured transport for TLS and proxy handling
		Timeout:   HTTPTimeout * time.Second, // Set timeout to prevent hanging requests
	}
}

// initClient initializes the HTTP client and related components.
// This method ensures that the transport, HTTP client, JSON client,
// and TLS warning logging are performed when needed, allowing re-initialization if the client becomes nil.
// This function is called by the Service and modifies its fields.
func initClient(service *Service, manager HTTPClientManager) {
	service.mu.Lock()
	defer service.mu.Unlock()

	if service.httpClient == nil || service.client == nil {
		transport := manager.CreateTransport(service.Config)
		service.httpClient = manager.CreateClient(transport)

		service.client = jsonclient.NewWithHTTPClient(service.httpClient)
		if service.Config.DisableTLS {
			service.Log("Warning: TLS is disabled, using insecure HTTP connections")
		}

		if service.Config.InsecureSkipVerify {
			service.Log("Warning: TLS certificate verification is disabled")
		}
	}
}
