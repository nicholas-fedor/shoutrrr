package httpclient

import (
	"errors"
	"fmt"
	"net/http"
)

// maxRedirects is the number of requests after which a default client stops
// following redirects, as [http.Client] does without a redirect policy.
const maxRedirects = 10

// Redirect errors.
var (
	// ErrRedirectRefused indicates a redirect that a default client does not
	// follow.
	ErrRedirectRefused = errors.New("redirect not followed")
	// errTooManyRedirects indicates a request redirected more often than a
	// default client follows.
	errTooManyRedirects = errors.New("too many redirects")
)

// checkRedirect is the redirect policy of the default clients.
//
// A redirect is followed only when it stays on the first request's host name
// and does not go from HTTPS to HTTP. A redirect from HTTP to HTTPS on the
// same host, as servers send to upgrade a connection, is followed.
//
// A followed redirect can carry a service's credentials elsewhere in three
// ways. [http.Client] forwards every header of the first request, except
// Authorization, WWW-Authenticate, and Cookie on a redirect to another domain.
// Services send credentials in headers of their own, such as Gotify's
// X-Gotify-Key. The client also sends the previous URL as the Referer, and
// that URL can hold a token in its path or query. A 307 or 308 redirect sends
// the request body again too. Removing headers would leave the other two
// ways, so such a redirect fails instead.
//
// Parameters:
//   - req: the request the redirect asks for.
//   - via: the requests made so far, oldest first.
//
// Returns:
//   - error: an error wrapping [ErrRedirectRefused] for a redirect to another
//     host or from HTTPS to HTTP. An error after ten requests, as the default
//     policy of [http.Client] returns. Otherwise nil.
func checkRedirect(req *http.Request, via []*http.Request) error {
	if len(via) >= maxRedirects {
		return fmt.Errorf("%w: stopped after %d redirects", errTooManyRedirects, maxRedirects)
	}

	if !sameHostName(req.URL.Hostname(), via[0].URL.Hostname()) {
		return fmt.Errorf("%w: the target is on another host", ErrRedirectRefused)
	}

	if req.URL.Scheme == "http" && via[len(via)-1].URL.Scheme == "https" {
		return fmt.Errorf("%w: the target uses HTTP after HTTPS", ErrRedirectRefused)
	}

	return nil
}

// sameHostName reports whether two host names are the same, ignoring the case
// of ASCII letters only. Unicode case folding would match names that IDNA
// encodes as different hosts, such as one with σ and one with ς, so any other
// byte must match exactly.
//
// Parameters:
//   - name: a host name, as [url.URL.Hostname] returns it.
//   - other: the other host name.
//
// Returns:
//   - bool: true when the names differ at most in the case of ASCII letters.
func sameHostName(name, other string) bool {
	if len(name) != len(other) {
		return false
	}

	for i := range len(name) {
		if lowerASCII(name[i]) != lowerASCII(other[i]) {
			return false
		}
	}

	return true
}

// lowerASCII returns the lowercase form of an ASCII capital letter, and any
// other byte unchanged.
//
// Parameters:
//   - c: the byte.
//
// Returns:
//   - byte: c in lowercase when it is an ASCII capital letter, otherwise c.
func lowerASCII(c byte) byte {
	if 'A' <= c && c <= 'Z' {
		return c + 'a' - 'A'
	}

	return c
}
