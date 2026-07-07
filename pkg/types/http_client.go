package types

import "net/http"

// HTTPClient is the interface shoutrrr uses for all outbound HTTP operations.
//
// Implementations must be safe for concurrent use. *http.Client satisfies this interface.
type HTTPClient interface {
	Do(req *http.Request) (*http.Response, error)
}

// HTTPClientSetter is implemented by services that accept a custom HTTP client
// for all outbound requests. This enables callers to control egress, TLS settings,
// proxies, and timeouts without global side effects.
type HTTPClientSetter interface {
	SetHTTPClient(client HTTPClient)
}
