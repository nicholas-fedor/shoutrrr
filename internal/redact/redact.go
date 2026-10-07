package redact

import (
	"errors"
	"net/http"
	"net/url"
	"regexp"

	"github.com/nicholas-fedor/shoutrrr/pkg/types"
)

// client wraps a [types.HTTPClient] and redacts the URLs in its transport errors.
type client struct {
	// inner is the wrapped client.
	inner types.HTTPClient
}

// redactedError is an error cause whose message has had its URLs redacted. It
// unwraps to the original cause, so errors.Is and errors.As still match it.
type redactedError struct {
	// err is the original cause.
	err error
	// msg is the cause's message with its URLs redacted.
	msg string
}

// Placeholder replaces a URL that cannot be reduced to its scheme and host.
const Placeholder = "[redacted]"

// ErrInvalidURL replaces the detail of a URL parse error, which can quote the URL.
var ErrInvalidURL = errors.New("invalid URL")

// Patterns for URLs that error messages can quote.
var (
	// urlPattern matches an absolute URL in free text.
	urlPattern = regexp.MustCompile(`[A-Za-z][A-Za-z0-9+.-]*://[^\s"'\\]+`)
	// quotedPatterns match the values that net/http and net/url quote in parse
	// errors, which can be relative URLs or URL fragments. Each is replaced with
	// its label and a quoted [Placeholder].
	quotedPatterns = []struct {
		pattern *regexp.Regexp
		label   string
	}{
		{regexp.MustCompile(`Location header "(?:[^"\\]|\\.)*"`), "Location header"},
		{regexp.MustCompile(`parse "(?:[^"\\]|\\.)*"`), "parse"},
		{regexp.MustCompile(`invalid port "(?:[^"\\]|\\.)*"`), "invalid port"},
	}
)

// RequestURL reduces rawURL to its scheme and host, dropping userinfo, path, query
// and fragment, which can carry credentials.
//
// Parameters:
//   - rawURL: the URL to redact.
//
// Returns:
//   - string: "scheme://host", or [Placeholder] when rawURL has no parsable host.
func RequestURL(rawURL string) string {
	parsed, err := url.Parse(rawURL)
	if err != nil || parsed.Scheme == "" || parsed.Host == "" {
		return Placeholder
	}

	return parsed.Scheme + "://" + parsed.Host
}

// URLError redacts the URL in a *url.Error, such as the errors returned by
// [http.Client.Do] and [url.Parse].
//
// The operation and underlying error are kept, so errors.Is, errors.As, Timeout
// and Temporary behave as before. Parse errors also replace the underlying error with
// [ErrInvalidURL], because net/url quotes parts of the URL in them. Only an
// outermost *url.Error is rebuilt; any other error is returned unchanged.
//
// Parameters:
//   - err: the error to redact, which may be nil.
//
// Returns:
//   - error: the redacted error, or err when it is not a *url.Error.
func URLError(err error) error {
	urlErr, ok := err.(*url.Error) //nolint:errorlint // Only an outermost *url.Error can be rebuilt.
	if !ok {
		return err
	}

	if urlErr.Op == "parse" {
		return &url.Error{Op: urlErr.Op, URL: Placeholder, Err: ErrInvalidURL}
	}

	return &url.Error{Op: urlErr.Op, URL: RequestURL(urlErr.URL), Err: redactCause(urlErr.Err)}
}

// redactCause redacts the URLs quoted in a *url.Error cause, such as both copies of
// the Location header that net/http quotes for a malformed redirect.
//
// Parameters:
//   - cause: the cause to redact, which may be nil.
//
// Returns:
//   - error: cause itself when its message quotes no URL, otherwise a wrapper
//     with a redacted message that unwraps to cause.
func redactCause(cause error) error {
	if cause == nil {
		return nil
	}

	msg := cause.Error()

	redacted := msg
	for _, quoted := range quotedPatterns {
		redacted = quoted.pattern.ReplaceAllLiteralString(redacted, quoted.label+` "`+Placeholder+`"`)
	}

	redacted = urlPattern.ReplaceAllStringFunc(redacted, RequestURL)

	if redacted == msg {
		return cause
	}

	return &redactedError{err: cause, msg: redacted}
}

// HTTPClient wraps httpClient so that the URLs in its transport errors are redacted.
//
// Parameters:
//   - httpClient: the client to wrap, which may be nil.
//
// Returns:
//   - types.HTTPClient: the wrapped client, httpClient itself when it is already
//     wrapped, or nil when httpClient is nil.
func HTTPClient(httpClient types.HTTPClient) types.HTTPClient {
	if httpClient == nil {
		return nil
	}

	if _, wrapped := httpClient.(*client); wrapped {
		return httpClient
	}

	return &client{inner: httpClient}
}

// Do sends req with the wrapped client and redacts the URL in any returned error.
//
// Parameters:
//   - req: the request to send.
//
// Returns:
//   - *http.Response: the response from the wrapped client.
//   - error: the wrapped client's error, with its URL redacted.
func (c *client) Do(req *http.Request) (*http.Response, error) {
	res, err := c.inner.Do(req)
	if err != nil {
		return res, URLError(err)
	}

	return res, nil
}

// Error returns the redacted message.
//
// Returns:
//   - string: the cause's message with its URLs redacted.
func (e *redactedError) Error() string {
	return e.msg
}

// Temporary reports whether the original cause, or any error it wraps, is
// temporary, so that url.Error.Temporary keeps working on redacted errors.
//
// Returns:
//   - bool: true when the cause chain reports a temporary failure.
func (e *redactedError) Temporary() bool {
	var temporary interface{ Temporary() bool }

	return errors.As(e.err, &temporary) && temporary.Temporary()
}

// Timeout reports whether the original cause, or any error it wraps, is a timeout,
// so that url.Error.Timeout keeps working on redacted errors.
//
// Returns:
//   - bool: true when the cause chain reports a timeout.
func (e *redactedError) Timeout() bool {
	var timeout interface{ Timeout() bool }

	return errors.As(e.err, &timeout) && timeout.Timeout()
}

// Unwrap returns the original cause.
//
// Returns:
//   - error: the original, unredacted cause.
func (e *redactedError) Unwrap() error {
	return e.err
}
