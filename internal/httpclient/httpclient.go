package httpclient

import (
	"crypto/tls"
	"net/http"
	"sync"
	"time"

	"github.com/nicholas-fedor/shoutrrr/internal/transport"
	"github.com/nicholas-fedor/shoutrrr/pkg/types"
)

// Holder holds the injected HTTP client of a service and its default clients.
// The zero value holds no client and builds defaults without a timeout. A
// Holder must not be copied after first use.
type Holder struct {
	// injected is the client from Set, or nil.
	injected types.HTTPClient
	// fallback is the default client, built by the first Client call that
	// needs it, or nil.
	fallback types.HTTPClient
	// insecure is the default client that skips TLS certificate verification,
	// built by the first InsecureClient call that needs it, or nil.
	insecure types.HTTPClient
	// tlsConfig is the TLS settings of the default client, or nil for the
	// settings of [http.DefaultTransport].
	tlsConfig *tls.Config
	// timeout bounds each request of the default client, or 0 for no bound.
	timeout time.Duration
	// mu guards the other fields.
	mu sync.Mutex
}

// Client returns the client to send through.
//
// Without an injected client, it returns the default client, which it builds
// from the SetDefault settings on first use and reuses after that.
//
// Returns:
//   - types.HTTPClient: the injected client, or the default client.
func (h *Holder) Client() types.HTTPClient {
	h.mu.Lock()
	defer h.mu.Unlock()

	if h.injected != nil {
		return h.injected
	}

	if h.fallback == nil {
		h.fallback = newDefault(h.timeout, h.tlsConfig)
	}

	return h.fallback
}

// Injected reports whether a caller injected a client, so a service can skip
// warnings about default client settings that the injected client does not
// use.
//
// Returns:
//   - bool: whether Set stored a client that is not nil.
func (h *Holder) Injected() bool {
	h.mu.Lock()
	defer h.mu.Unlock()

	return h.injected != nil
}

// InsecureClient returns the client to send through when the user turned off
// TLS certificate verification.
//
// Without an injected client, it returns a default client that skips
// verification. It builds that client with the SetDefault timeout on first use
// and reuses it after that. An injected client is returned unchanged, because
// its owner controls its TLS settings.
//
// Returns:
//   - types.HTTPClient: the injected client, or the insecure default client.
func (h *Holder) InsecureClient() types.HTTPClient {
	h.mu.Lock()
	defer h.mu.Unlock()

	if h.injected != nil {
		return h.injected
	}

	if h.insecure == nil {
		h.insecure = newDefault(h.timeout, transport.InsecureTLSConfig(""))
	}

	return h.insecure
}

// Set stores the client to send through. A nil client, including a nil
// *http.Client, clears it, so sends use the default client.
//
// Parameters:
//   - client: the HTTP client, or nil for the default.
func (h *Holder) Set(client types.HTTPClient) {
	if c, ok := client.(*http.Client); ok && c == nil {
		client = nil
	}

	h.mu.Lock()
	defer h.mu.Unlock()

	h.injected = client
}

// SetDefault sets how the default clients are built. A service calls it from
// Initialize. Defaults built with earlier settings are dropped, so the next
// Client or InsecureClient call builds one with these. The injected client is
// kept.
//
// Parameters:
//   - timeout: bounds each request of the default clients, or 0 for no bound.
//   - tlsConfig: the TLS settings of the default client, or nil to send
//     through [http.DefaultTransport]. The insecure default client ignores
//     them.
func (h *Holder) SetDefault(timeout time.Duration, tlsConfig *tls.Config) {
	h.mu.Lock()
	defer h.mu.Unlock()

	h.timeout = timeout
	h.tlsConfig = tlsConfig
	h.fallback = nil
	h.insecure = nil
}

// newDefault builds a default client.
//
// Without TLS settings, the client sends through [http.DefaultTransport], so
// it honors the proxy environment variables and any transport a program
// installs there. With TLS settings, it sends through a clone of that
// transport with those settings.
//
// Parameters:
//   - timeout: bounds each request, or 0 for no bound.
//   - tlsConfig: the TLS settings, or nil.
//
// Returns:
//   - types.HTTPClient: the new client.
func newDefault(timeout time.Duration, tlsConfig *tls.Config) types.HTTPClient {
	client := &http.Client{Timeout: timeout}
	if tlsConfig != nil {
		client.Transport = transport.New(tlsConfig)
	}

	return client
}
