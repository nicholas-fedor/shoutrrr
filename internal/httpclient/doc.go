// Package httpclient holds the HTTP client of a notification service.
//
// A service sends through the client a caller injects with SetHTTPClient, or
// through a default client. A [Holder] keeps both. It treats a nil client,
// including a nil *http.Client, as no client. Each service sets its default's
// timeout and TLS settings in Initialize. The Holder builds the default from
// them once, when a send first needs it, and reuses it after that. When the
// user turns off TLS certificate verification, the service sends through
// [Holder.InsecureClient] instead, which keeps a second default client.
//
// [BoundContext] bounds a send whose context has no deadline, such as a direct
// Send call without the router.
package httpclient
