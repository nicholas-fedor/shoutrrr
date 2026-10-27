package types

import "time"

// SenderOptions controls creation of senders and routers.
//
// HTTPClient, if non-nil, will be used for all outbound HTTP requests
// made by the resulting sender/router. This allows callers to supply
// custom transports, dialers, TLS configuration, timeouts, etc.
//
// DialContext, if non-nil, will be used for outbound TCP connections
// made by services that implement [DialContextSetter]. Implementations
// must be safe for concurrent use. TLS wrapping remains the service's
// responsibility after the TCP dial. A nil value uses the service default.
//
// Timeout, if > 0, is the exact fixed timeout for every service on the sender.
// Zero leaves each service's own send budget in effect.
type SenderOptions struct {
	// HTTPClient is the client used for all HTTP operations.
	// If nil, or a nil *http.Client, each service uses its default client.
	// A nil value of another type, such as a nil pointer to a custom client,
	// is used as given. A runtime panic from it fails the request with
	// [ErrNilHTTPClient].
	HTTPClient HTTPClient

	// DialContext is the dial function used for non-HTTP TCP connections.
	// If nil, services use their default dialer.
	DialContext DialContextFunc

	// Timeout, when set, is the exact fixed timeout for every service on the sender.
	// Zero leaves each service's own send budget in effect.
	Timeout time.Duration
}
