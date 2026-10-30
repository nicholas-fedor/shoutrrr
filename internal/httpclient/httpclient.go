package httpclient

import (
	"fmt"
	"net/http"
	"reflect"
	"runtime"
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

// nilClient wraps an injected client that is a nil pointer or another nil
// value. A nil receiver can be valid, so requests go to the client. A runtime
// panic, such as a nil pointer dereference, fails the request with
// [types.ErrNilHTTPClient] instead.
type nilClient struct {
	// client is the injected nil client.
	client types.HTTPClient
	// typeName is the injected client's type, for the error.
	typeName string
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
// otherwise. A nil *http.Client counts as unset. A nil value of another type,
// such as a nil pointer to a custom client, counts as set: requests go to it,
// and a runtime panic from it fails the request with [types.ErrNilHTTPClient].
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

// PreferInsecure returns client when it is set, and the result of
// [Holder.InsecureClient] otherwise. It is [Holder.Prefer] for a send that
// skips TLS certificate verification. A client the caller set through the
// service's exported client field is used unchanged and without calling warn.
// Its owner controls its TLS settings. Which values count as set is the same
// as for Prefer.
//
// Parameters:
//   - client: the service's exported client field, or nil.
//   - warn: called when the insecure default client is returned. It may be nil.
//
// Returns:
//   - types.HTTPClient: client, or the holder's insecure client when client is unset.
func (h *Holder) PreferInsecure(client types.HTTPClient, warn func()) types.HTTPClient {
	if set := normalize(client); set != nil {
		return set
	}

	return h.InsecureClient(warn)
}

// Set stores the client to send through. A nil client, including a nil
// *http.Client, clears it, so sends use the default client. A nil value of
// another type, such as a nil pointer to a custom client, is stored: requests
// go to it, and a runtime panic from it fails the request with
// [types.ErrNilHTTPClient]. It is never replaced by the default client, which
// would bypass whatever the caller meant to inject.
//
// Parameters:
//   - client: the HTTP client, or nil for the default.
func (h *Holder) Set(client types.HTTPClient) {
	client = normalize(client)

	h.mu.Lock()
	defer h.mu.Unlock()

	h.injected = client
}

// Do sends req through the nil client. A runtime panic from the client, such
// as a nil pointer dereference or a write to a nil map, fails the request and
// closes its body, as a client does when it returns an error. Any other panic
// is raised again.
//
// Parameters:
//   - req: the request.
//
// Returns:
//   - res: the client's response, or nil after a runtime panic.
//   - err: the client's error, or [types.ErrNilHTTPClient] wrapping the
//     runtime panic.
//
//nolint:nonamedreturns // The deferred recover sets the results.
func (c nilClient) Do(req *http.Request) (res *http.Response, err error) {
	defer func() {
		recovered := recover()
		if recovered == nil {
			return
		}

		runtimeErr, ok := recovered.(runtime.Error)
		if !ok {
			panic(recovered)
		}

		if req != nil && req.Body != nil {
			_ = req.Body.Close()
		}

		res, err = nil, fmt.Errorf("%w: %s: %w", types.ErrNilHTTPClient, c.typeName, runtimeErr)
	}()

	return c.client.Do(req)
}

// normalize prepares an injected client for sending. A nil *http.Client, which
// is a non-nil [types.HTTPClient], becomes nil, so the default client is used.
// A nil pointer, func, map, channel, or slice of another type
// is wrapped in a [nilClient], so a runtime panic from it fails the request
// instead of the send. It never becomes the default client, which would
// silently bypass whatever the caller meant to inject, such as a client that
// guards outbound requests.
//
// Parameters:
//   - client: the client to check, or nil.
//
// Returns:
//   - types.HTTPClient: nil for no client, a [nilClient] for a nil value of
//     another type, or client unchanged.
func normalize(client types.HTTPClient) types.HTTPClient {
	if client == nil {
		return nil
	}

	if c, ok := client.(*http.Client); ok && c == nil {
		return nil
	}

	// Values of these kinds can be nil inside a non-nil interface. Values of
	// the other kinds never are, and an unsafe pointer cannot have methods.
	switch value := reflect.ValueOf(client); value.Kind() { //nolint:exhaustive // Only the kinds that can be nil matter.
	case reflect.Pointer, reflect.Func, reflect.Map, reflect.Chan, reflect.Slice:
		if value.IsNil() {
			return nilClient{client: client, typeName: fmt.Sprintf("%T", client)}
		}
	default:
	}

	return client
}
