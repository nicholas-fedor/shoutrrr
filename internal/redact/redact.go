package redact

import (
	"errors"
	"net/http"
	"net/url"

	"github.com/nicholas-fedor/shoutrrr/pkg/types"
)

// client wraps a [types.HTTPClient] and redacts the URLs in its transport errors.
type client struct {
	// inner is the wrapped client.
	inner types.HTTPClient
}

// Placeholder replaces a URL that cannot be reduced to its scheme and host.
const Placeholder = "[redacted]"

// ErrInvalidURL replaces the detail of a URL parse error, which can quote the URL.
var ErrInvalidURL = errors.New("invalid URL")

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
// The operation and underlying error are kept, so errors.Is, errors.As and
// Timeout behave as before. Parse errors also replace the underlying error with
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

	return &url.Error{Op: urlErr.Op, URL: RequestURL(urlErr.URL), Err: urlErr.Err}
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
