package httpclient

import (
	"net/http"
	"sync"

	"github.com/nicholas-fedor/shoutrrr/internal/transport"
	"github.com/nicholas-fedor/shoutrrr/pkg/types"
)

// Holder holds the injected HTTP client of a service and its default clients.
// The zero value is ready to use and holds no client. The default clients
// have no timeout of their own, because each send bounds its requests with
// [BoundContext]. A Holder must not be copied after first use.
type Holder struct {
	// injected is the client from Set, or nil.
	injected types.HTTPClient
	// fallback is the default client, built by the first Client call that
	// needs it, or nil.
	fallback types.HTTPClient
	// insecure is the default client that skips TLS certificate verification,
	// built by the first InsecureClient call that needs it, or nil.
	insecure types.HTTPClient
	// mu guards the other fields.
	mu sync.Mutex
}

// Client returns the client to send through.
//
// Without an injected client, it returns the default client, which it builds
// on first use and reuses after that. The default client sends through
// [http.DefaultTransport], so it honors the proxy environment variables and
// any transport a program installs there.
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
		h.fallback = &http.Client{}
	}

	return h.fallback
}

// InsecureClient returns the client to send through when the user turned off
// TLS certificate verification.
//
// Without an injected client, it returns a default client that skips
// verification and calls warn, so the service can log the warning once. It
// builds that client on first use, from a clone of [http.DefaultTransport],
// and reuses it after that. An injected client is returned unchanged, without calling warn,
// because its owner controls its TLS settings.
//
// Parameters:
//   - warn: called, outside the holder's lock, when the insecure default
//     client is returned. It may be nil.
//
// Returns:
//   - types.HTTPClient: the injected client, or the insecure default client.
func (h *Holder) InsecureClient(warn func()) types.HTTPClient {
	h.mu.Lock()

	if h.injected != nil {
		injected := h.injected
		h.mu.Unlock()

		return injected
	}

	if h.insecure == nil {
		h.insecure = &http.Client{Transport: transport.New(transport.InsecureTLSConfig(""))}
	}

	insecure := h.insecure
	h.mu.Unlock()

	if warn != nil {
		warn()
	}

	return insecure
}

// Prefer returns client when it is set, and the result of [Holder.Client]
// otherwise. A nil *http.Client counts as unset.
//
// A service passes the exported client field it keeps for compatibility. Its
// SetHTTPClient calls Set and clears that field, so whichever of the two the
// caller set last carries the send.
//
// Parameters:
//   - client: the service's exported client field, or nil.
//
// Returns:
//   - types.HTTPClient: client, or the holder's client when client is unset.
func (h *Holder) Prefer(client types.HTTPClient) types.HTTPClient {
	if set := normalize(client); set != nil {
		return set
	}

	return h.Client()
}

// Set stores the client to send through. A nil client, including a nil
// *http.Client, clears it, so sends use the default client.
//
// Parameters:
//   - client: the HTTP client, or nil for the default.
func (h *Holder) Set(client types.HTTPClient) {
	client = normalize(client)

	h.mu.Lock()
	defer h.mu.Unlock()

	h.injected = client
}

// normalize returns nil for a nil *http.Client, which is a non-nil
// [types.HTTPClient], and client otherwise.
//
// Parameters:
//   - client: the client to check.
//
// Returns:
//   - types.HTTPClient: client, or nil when it is a nil *http.Client.
func normalize(client types.HTTPClient) types.HTTPClient {
	if c, ok := client.(*http.Client); ok && c == nil {
		return nil
	}

	return client
}
