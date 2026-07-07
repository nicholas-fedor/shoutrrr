package types

import "time"

// SenderOptions controls creation of senders and routers.
//
// HTTPClient, if non-nil, will be used for all outbound HTTP requests
// made by the resulting sender/router. This allows callers to supply
// custom transports, dialers, TLS configuration, timeouts, etc.
//
// Timeout, if > 0, overrides the default per-service timeout.
type SenderOptions struct {
	// HTTPClient is the client used for all HTTP operations.
	// If nil, a default client with reasonable settings is used.
	HTTPClient HTTPClient

	// Timeout overrides the default operation timeout when > 0.
	Timeout time.Duration
}
