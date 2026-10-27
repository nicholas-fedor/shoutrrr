package types

import "net/http"

// HTTPClient is the interface shoutrrr uses for all outbound HTTP operations.
//
// Implementations must be safe for concurrent use. *http.Client satisfies this interface.
type HTTPClient interface {
	// Do sends an HTTP request and returns its response, following the
	// contract of [http.Client.Do].
	//
	// Parameters:
	//   - req: the request to send.
	//
	// Returns:
	//   - *http.Response: the response. When the error is nil, the caller
	//     closes its body. With an error, the response can be ignored. It is
	//     non-nil only after a failed CheckRedirect, with its body closed.
	//   - error: a client policy failure, such as a failed CheckRedirect, or
	//     a transport failure. It is nil when Do succeeds.
	Do(req *http.Request) (*http.Response, error)
}

// HTTPClientSetter is implemented by services that accept a custom HTTP client
// for all outbound requests. This enables callers to control egress, TLS settings,
// proxies, and timeouts without global side effects.
type HTTPClientSetter interface {
	// SetHTTPClient sets the client the service uses for its HTTP requests.
	//
	// The router calls it after [Service.Initialize] when
	// [SenderOptions.HTTPClient] is set.
	//
	// Parameters:
	//   - client: the client for the service's requests. Nil, or a nil
	//     *http.Client, restores the default client. A nil value of another
	//     type is used as given, and a runtime panic from it fails the request
	//     with [ErrNilHTTPClient].
	SetHTTPClient(client HTTPClient)
}
