package util

import (
	"errors"
	"fmt"
	"net"
	"net/url"
	"strconv"
	"strings"
	"unicode/utf8"
)

// Errors for a URL whose authority, path, or query does not parse, or for a
// value that does not fit its part of a URL.
var (
	// ErrInvalidHost indicates a URL host that is not a hostname, an IPv4
	// address, or an IPv6 address in brackets.
	ErrInvalidHost = errors.New("invalid URL host")
	// ErrInvalidPort indicates a URL port that is not a decimal number, or one
	// outside 1 to 65535 where a port number is read.
	ErrInvalidPort = errors.New("invalid URL port")
	// ErrInvalidQuery indicates a URL query with a malformed escape or a
	// semicolon separator.
	ErrInvalidQuery = errors.New("invalid URL query")
	// ErrInvalidPathSegment indicates a value that a request path cannot carry
	// as one segment. Such a value is empty or holds a slash. The values "."
	// and ".." are invalid too.
	ErrInvalidPathSegment = errors.New("invalid URL path segment")
)

// URLUserPassword creates a url.Userinfo from user and password strings.
//
// Unlike url.UserPassword, this function treats empty strings as "not specified":
//   - If password is non-empty, returns url.UserPassword(user, password).
//   - If password is empty but user is non-empty, returns url.User(user).
//   - If both are empty, returns nil (which serializes to "" in url.URL).
//
// Parameters:
//   - user: The username, or empty string if no authentication.
//   - password: The password, or empty string if no password.
//
// Returns:
//   - A *url.Userinfo with the appropriate authentication info, or nil if neither user nor password is provided.
func URLUserPassword(user, password string) *url.Userinfo {
	if password != "" {
		return url.UserPassword(user, password)
	} else if user != "" {
		return url.User(user)
	}

	return nil
}

// SplitURLHost splits the authority of serviceURL into its hostname and port.
//
// An IPv6 address must be enclosed in brackets, which the returned hostname
// leaves out. [url.Parse] accepts an unbracketed IPv6 address such as ::1 but
// splits it at its last colon, and [url.URL.Hostname] and [url.URL.Port] keep a
// non-numeric port of a hand-built URL in the hostname, so both are rejected
// here rather than read as a different host.
//
// Parameters:
//   - serviceURL: the URL whose Host to split.
//
// Returns:
//   - host: the hostname, without brackets, or "" when the URL has none.
//   - port: the decimal port, or "" when the URL has none.
//   - err: [ErrInvalidHost] for an unbracketed host with several colons or a
//     malformed bracket, or [ErrInvalidPort] for a port that is not a number.
func SplitURLHost(serviceURL *url.URL) (host, port string, err error) { //nolint:nonamedreturns // Same-type returns need names.
	authority := serviceURL.Host

	if rest, bracketed := strings.CutPrefix(authority, "["); bracketed {
		var found bool
		if host, rest, found = strings.Cut(rest, "]"); !found {
			return "", "", ErrInvalidHost
		}

		if rest != "" {
			if port, found = strings.CutPrefix(rest, ":"); !found {
				return "", "", ErrInvalidHost
			}
		}
	} else {
		if strings.Count(authority, ":") > 1 {
			return "", "", ErrInvalidHost
		}

		host, port, _ = strings.Cut(authority, ":")
	}

	if strings.Trim(port, "0123456789") != "" {
		return "", "", ErrInvalidPort
	}

	return host, port, nil
}

// ParseURLPort reads a URL port as a TCP port number. The port is the decimal
// text [SplitURLHost] returns.
//
// Parameters:
//   - port: the decimal port, or "" when the URL has none.
//
// Returns:
//   - uint16: the port, or 0 when port is empty.
//   - error: an error wrapping [ErrInvalidPort] for a port that is not a number
//     from 1 to 65535.
func ParseURLPort(port string) (uint16, error) {
	if port == "" {
		return 0, nil
	}

	number, err := strconv.ParseUint(port, 10, 16)
	if err != nil || number == 0 {
		return 0, fmt.Errorf("%w: %q must be 1 to 65535", ErrInvalidPort, port)
	}

	return uint16(number), nil
}

// ValidateHostToken checks a token that a service URL carries as its host as
// written, such as an API key or a webhook ID.
//
// The token must read back unchanged from the URL that [url.URL.String] writes
// for it. A colon or a bracket would read back as a port or an IPv6 address.
// Other ASCII characters that a URL host cannot hold, such as a slash or a
// space, make the URL fail to parse. Characters outside ASCII are written
// escaped and read back unchanged.
//
// Parameters:
//   - token: the token.
//
// Returns:
//   - error: [ErrInvalidHost] for a token that would not read back unchanged,
//     or nil.
func ValidateHostToken(token string) error {
	for i := range len(token) {
		if c := token[i]; c < utf8.RuneSelf && !isHostTokenByte(c) {
			return ErrInvalidHost
		}
	}

	return nil
}

// ValidatePathSegment checks a value that a service sends as one segment of a
// request path, such as a webhook token.
//
// Escaping the value with [url.PathEscape] keeps a slash, question mark, or hash
// in the segment, but "." and ".." need no escape and would walk the path.
// A value with a slash is rejected too, since it fills two segments of the
// service URL it came from.
//
// Parameters:
//   - value: the value.
//
// Returns:
//   - error: [ErrInvalidPathSegment] for a value that is not one segment, or
//     nil.
func ValidatePathSegment(value string) error {
	if value == "" || value == "." || value == ".." || strings.IndexByte(value, '/') >= 0 {
		return ErrInvalidPathSegment
	}

	return nil
}

// JoinURLHost writes host and port as a URL authority.
//
// An IPv6 host is enclosed in brackets whether or not a port follows it, so
// [SplitURLHost] reads the same host and port back.
//
// Parameters:
//   - host: the hostname or IP address, without brackets.
//   - port: the decimal port, or "" for none.
//
// Returns:
//   - string: the authority, such as example.com:8080 or [::1].
func JoinURLHost(host, port string) string {
	if port != "" {
		return net.JoinHostPort(host, port)
	}

	if strings.Contains(host, ":") {
		return "[" + host + "]"
	}

	return host
}

// ParseURLQuery parses the query of serviceURL.
//
// [url.URL.Query] silently drops each pair it cannot parse, so a setting with a
// malformed escape would be ignored. ParseURLQuery reports it instead.
//
// Parameters:
//   - serviceURL: the URL whose query to parse.
//
// Returns:
//   - url.Values: the query values.
//   - error: an error wrapping [ErrInvalidQuery] when a pair has a malformed
//     escape or the query uses a semicolon separator.
func ParseURLQuery(serviceURL *url.URL) (url.Values, error) {
	query, err := url.ParseQuery(serviceURL.RawQuery)
	if err != nil {
		return nil, fmt.Errorf("%w: %w", ErrInvalidQuery, err)
	}

	return query, nil
}

// isHostTokenByte reports whether the ASCII byte c reads back unchanged from a
// URL host.
//
// Letters, digits, and the other characters that [url.URL.String] writes
// unescaped in a host do. So does "%", which it writes as "%25". The colon and
// the brackets do not, since they read back as a port or an IPv6 address.
//
// Parameters:
//   - c: an ASCII byte.
//
// Returns:
//   - bool: true when c reads back unchanged.
func isHostTokenByte(c byte) bool {
	switch c {
	case '-', '.', '_', '~', '!', '$', '&', '\'', '(', ')', '*', '+', ',', ';', '=', '<', '>', '"', '%':
		return true
	default:
		return 'a' <= c && c <= 'z' || 'A' <= c && c <= 'Z' || '0' <= c && c <= '9'
	}
}
