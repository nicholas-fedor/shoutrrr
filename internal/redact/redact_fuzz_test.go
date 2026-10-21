package redact

import (
	"errors"
	"fmt"
	"net/url"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// errFuzzCause is the sentinel that the causes built by FuzzURLError wrap.
var errFuzzCause = errors.New("fuzz cause")

// FuzzRequestURL checks that a redacted request URL is either [Placeholder] or
// parses back to the input's scheme and host alone, with no userinfo, path,
// query, or fragment. The seed corpus covers credentials in the userinfo,
// path, query, and fragment, ports, IPv6 hosts, relative and opaque URLs, and
// malformed escapes.
func FuzzRequestURL(f *testing.F) {
	for _, seed := range []string{
		"",
		"https://user:" + testSecret + "@example.com/path?token=" + testSecret + "#frag",
		"https://api.telegram.org/bot" + testSecret + "/sendMessage",
		"http://[::1]:8080/" + testSecret,
		"mailto:" + testSecret + "@example.com",
		"/relative/" + testSecret,
		"https://example.com/%zz",
		"https://example.com:notaport/",
	} {
		f.Add(seed)
	}

	f.Fuzz(func(t *testing.T, rawURL string) {
		got := RequestURL(rawURL)
		if got == Placeholder {
			return
		}

		want, err := url.Parse(rawURL)
		require.NoError(t, err, "RequestURL(%q) = %q for a URL that does not parse", rawURL, got)

		parsed, err := url.Parse(got)
		require.NoError(t, err, "RequestURL(%q) = %q does not parse", rawURL, got)
		assert.Equal(t, want.Scheme, parsed.Scheme)
		assert.Equal(t, want.Host, parsed.Host)
		assert.Nil(t, parsed.User)
		assert.Empty(t, parsed.Opaque)
		assert.Empty(t, parsed.Path)
		assert.Empty(t, parsed.RawQuery)
		assert.Empty(t, parsed.Fragment)
	})
}

// FuzzURLError checks that redaction removes a password quoted in a *url.Error
// or in any other error's message, wherever the URL sits in the text, that every
// URL left in the message is already reduced to its scheme and host, and that the
// redacted error keeps its operation, carries the reduced URL, or [Placeholder]
// for a parse error, and still matches its cause with errors.Is.
// The URL's password is a fixed secret, and the fuzzer varies the operation, the
// host, the path and query after it, and the text around the URL, skipping
// inputs that repeat the secret anywhere else. The seed corpus covers request
// and parse operations, quoted and unquoted URLs, and the Location header and
// parse messages that net/http and net/url quote.
func FuzzURLError(f *testing.F) {
	for _, seed := range []struct {
		op, host, rest, prefix, suffix string
	}{
		{"Post", "example.com", "/hook?token=abc", "", ""},
		{"Get", "api.telegram.org", "/bot123/getMe", "dial tcp: lookup ", ": no such host"},
		{"parse", "example.com", "/%zz", `parse "`, `": invalid URL escape "%zz"`},
		{"Get", "example.com:8080", "/a b", `Location header "`, `"`},
		{"Get", "[::1]", "", "invalid port ", ""},
		{"Put", "", "/path", "'", "'"},
	} {
		f.Add(seed.op, seed.host, seed.rest, seed.prefix, seed.suffix)
	}

	f.Fuzz(func(t *testing.T, op, host, rest, prefix, suffix string) {
		rawURL := "https://user:" + testSecret + "@" + host + rest

		// Redaction keeps the operation, the host, and text outside URLs, so the
		// secret may appear only as the password.
		if strings.Contains(op, testSecret) || strings.Count(prefix+rawURL+suffix, testSecret) != 1 {
			t.Skip()
		}

		cause := fmt.Errorf("%s%s%s: %w", prefix, rawURL, suffix, errFuzzCause)

		redacted := URLError(&url.Error{Op: op, URL: rawURL, Err: cause})
		requireRedacted(t, redacted)

		urlErr, ok := errors.AsType[*url.Error](redacted)
		require.True(t, ok, "URLError returned %T, want a *url.Error", redacted)
		assert.Equal(t, op, urlErr.Op)

		if op == "parse" {
			assert.Equal(t, Placeholder, urlErr.URL)
			require.ErrorIs(t, redacted, ErrInvalidURL)
		} else {
			assert.Equal(t, RequestURL(rawURL), urlErr.URL)
			require.ErrorIs(t, redacted, errFuzzCause)
		}

		redacted = URLError(cause)
		requireRedacted(t, redacted)
		require.ErrorIs(t, redacted, errFuzzCause)
	})
}

// requireRedacted requires err's message to omit [testSecret] and every URL in
// it to be reduced to its scheme and host.
//
// Parameters:
//   - t: the test.
//   - err: the redacted error.
func requireRedacted(t *testing.T, err error) {
	t.Helper()

	require.Error(t, err)

	msg := err.Error()
	require.NotContains(t, msg, testSecret)

	for _, match := range urlPattern.FindAllString(msg, -1) {
		assert.Equal(t, RequestURL(match), match, "message %q keeps an unreduced URL", msg)
	}
}
