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
// Timeout, if > 0, overrides the default per-service timeout.
type SenderOptions struct {
	// HTTPClient is the client used for all HTTP operations.
	// If nil, a default client with reasonable settings is used.
	HTTPClient HTTPClient

	// DialContext is the dial function used for non-HTTP TCP connections.
	// If nil, services use their default dialer.
	DialContext DialContextFunc

	// Timeout overrides the default operation timeout when > 0.
	Timeout time.Duration
}
