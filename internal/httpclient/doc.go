// Package httpclient holds the HTTP client of a notification service.
//
// A service sends through the client a caller injects with SetHTTPClient, or
// through a default client. A [Holder] keeps both. It treats a nil client,
// including a nil *http.Client, as no client. The Holder builds the default
// once, when a send first needs it, and reuses it after that. When the user
// turns off TLS certificate verification, the service sends through
// [Holder.InsecureClient] instead, which keeps a second default client. A
// service that keeps an exported client field for compatibility sends through
// [Holder.Prefer].
//
// [BoundContext] bounds a send whose context has no deadline, such as a direct
// Send call without the router. [RetryAfter] reads the wait that a
// rate-limited response asks for, so services read the header alike.
// [Duration] turns a wait that a response body gives as a count of a unit
// into a [time.Duration] the same way.
package httpclient
