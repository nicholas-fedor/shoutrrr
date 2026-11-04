package util

import (
	"errors"
	"fmt"
	"net"
	"net/netip"
	"net/url"
	"strconv"
	"strings"
	"unicode/utf8"
)

// maxPort is the highest TCP port number.
const maxPort = 65535

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
	// as one segment, or a path with a "." or ".." segment. A value for one
	// segment is invalid when it is empty or holds a slash, and the values "."
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
// leaves out, and brackets may enclose only an IPv6 address, as [url.Parse]
// requires. [url.Parse] accepts an unbracketed IPv6 address such as ::1 but
// splits it at its last colon, and [url.URL.Hostname] and [url.URL.Port] keep a
// non-numeric port of a hand-built URL in the hostname, so both are rejected
// here rather than read as a different host.
//
// A port must be a number from 1 to 65535, as [ParseURLPort] reads it. Port 0
// is reserved and a port is a 16-bit number, so no other port reaches a
// server.
//
// Parameters:
//   - serviceURL: the URL whose Host to split.
//
// Returns:
//   - host: the hostname, without brackets, or "" when the URL has none.
//   - port: the decimal port, or "" when the URL has none.
//   - err: [ErrInvalidHost] for an unbracketed host with several colons or a
//     character a URL host cannot hold, a malformed bracket, or brackets
//     around anything but an IPv6 address. An error wrapping [ErrInvalidPort]
//     for a port that is not a number from 1 to 65535.
func SplitURLHost(serviceURL *url.URL) (host, port string, err error) { //nolint:nonamedreturns // Same-type returns need names.
	host, port, err = splitAuthority(serviceURL.Host)
	if err != nil {
		return "", "", err
	}

	if _, err := ParseURLPort(port); err != nil {
		return "", "", err
	}

	return host, port, nil
}

// SplitURLHostPort splits the authority of serviceURL into its hostname and
// port number.
//
// It reads the authority as [SplitURLHost] does, for a service that keeps its
// port as a number.
//
// Parameters:
//   - serviceURL: the URL whose Host to split.
//
// Returns:
//   - string: the hostname, without brackets, or "" when the URL has none.
//   - uint16: the port, or 0 when the URL has none.
//   - error: the error [SplitURLHost] returns for the authority.
func SplitURLHostPort(serviceURL *url.URL) (string, uint16, error) {
	host, port, err := splitAuthority(serviceURL.Host)
	if err != nil {
		return "", 0, err
	}

	number, err := ParseURLPort(port)
	if err != nil {
		return "", 0, err
	}

	return host, number, nil
}

// ParseURLPort reads the text of a URL port as a TCP port number.
//
// [SplitURLHost] reads a URL's port with it, and a service whose config holds
// the port as text checks a port set in code with it. The number must pass
// [ValidatePort].
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
	if err != nil {
		return 0, fmt.Errorf("%w: %q must be 1 to 65535", ErrInvalidPort, port)
	}

	if err := ValidatePort(int(number)); err != nil {
		return 0, err
	}

	return uint16(number), nil
}

// ValidatePort checks that a port number can reach a server.
//
// A port is a 16-bit number and port 0 is reserved. A port must therefore be
// 1 to 65535. A service whose config holds the port as a number checks a port set
// in code with it, as reading a URL checks one.
//
// Parameters:
//   - port: the port.
//
// Returns:
//   - error: an error wrapping [ErrInvalidPort] for a port outside 1 to 65535,
//     or nil.
func ValidatePort(port int) error {
	if port < 1 || port > maxPort {
		return fmt.Errorf("%w: %d must be 1 to 65535", ErrInvalidPort, port)
	}

	return nil
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

// SplitEscapedPath splits an escaped URL path, such as the one
// [url.URL.EscapedPath] returns, at its slashes and decodes each segment. An
// escaped slash, %2F, stays inside its segment, where splitting the decoded
// path would read it as a separator. A leading slash gives an empty first
// segment, as [strings.Split] does.
//
// Parameters:
//   - escapedPath: the escaped path.
//
// Returns:
//   - []string: the decoded segments. A segment with a malformed escape is
//     kept as written.
func SplitEscapedPath(escapedPath string) []string {
	segments := strings.Split(escapedPath, "/")

	for i, segment := range segments {
		// A segment without a % has nothing to decode.
		if strings.IndexByte(segment, '%') < 0 {
			continue
		}

		if decoded, err := url.PathUnescape(segment); err == nil {
			segments[i] = decoded
		}
	}

	return segments
}

// JoinEscapedPath writes each value as one segment of a URL path, after a
// slash. It returns the path both decoded and escaped. Set them as the Path
// and RawPath of a [url.URL], so a slash in a value goes out as %2F and
// [SplitEscapedPath] reads the value back whole. No values give the path "/".
//
// Parameters:
//   - values: the values, one per segment.
//
// Returns:
//   - path: the decoded path, for [url.URL.Path].
//   - rawPath: the path with each value escaped by [url.PathEscape], for
//     [url.URL.RawPath].
func JoinEscapedPath(values []string) (path, rawPath string) { //nolint:nonamedreturns // Same-type returns need names.
	if len(values) == 0 {
		return "/", "/"
	}

	size := len(values)
	for _, value := range values {
		size += len(value)
	}

	var decoded, escaped strings.Builder

	decoded.Grow(size)
	escaped.Grow(size)

	for _, value := range values {
		decoded.WriteByte('/')
		decoded.WriteString(value)
		escaped.WriteByte('/')
		escaped.WriteString(url.PathEscape(value))
	}

	return decoded.String(), escaped.String()
}

// ValidatePathSegments checks a path that a service puts into a request path
// as several segments, such as a server's base path. Each "." or ".." segment
// would walk the request path, so the path may hold neither. Empty segments,
// such as those a leading or trailing slash gives, are accepted.
//
// Parameters:
//   - path: the path, with its segments separated by slashes.
//
// Returns:
//   - error: [ErrInvalidPathSegment] for a path with a "." or ".." segment, or
//     nil.
func ValidatePathSegments(path string) error {
	// A path without a dot has no "." or ".." segment.
	if strings.IndexByte(path, '.') < 0 {
		return nil
	}

	for path != "" {
		segment, rest, _ := strings.Cut(path, "/")
		if segment == "." || segment == ".." {
			return ErrInvalidPathSegment
		}

		path = rest
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

// ValidateHostName checks a hostname that a config keeps apart from its port.
//
// Such a hostname is written into a URL with its port by [JoinURLHost], which
// brackets any host holding a colon. So the hostname is an IPv6 address, or a
// name that reads back unchanged as a URL host, as [ValidateHostToken]
// describes. Any other hostname would name another host and port, or fail to
// parse. An empty hostname passes, for the config's own check of a missing
// host.
//
// Parameters:
//   - host: the hostname, without brackets.
//
// Returns:
//   - error: [ErrInvalidHost] for a hostname with a bracket, a colon outside
//     an IPv6 address, or another character a URL host cannot hold.
//     Otherwise nil.
func ValidateHostName(host string) error {
	if isIPv6(host) {
		return nil
	}

	return ValidateHostToken(host)
}

// JoinURLHostPort writes host and a port number as a URL authority, as
// [JoinURLHost] does.
//
// Port 0 stands for no port, so a config without one writes a URL that reads
// back with its default port.
//
// Parameters:
//   - host: the hostname or IP address, without brackets.
//   - port: the port, or 0 for none.
//
// Returns:
//   - string: the authority, such as example.com:8080 or [::1].
func JoinURLHostPort(host string, port int) string {
	if port == 0 {
		return JoinURLHost(host, "")
	}

	return JoinURLHost(host, strconv.Itoa(port))
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

// splitAuthority splits a URL authority into its hostname and port text, as
// [SplitURLHost] describes, without reading the port as a number.
//
// Parameters:
//   - authority: the URL's Host.
//
// Returns:
//   - host: the hostname, without brackets.
//   - port: the text after the port colon, or "".
//   - err: [ErrInvalidHost] for an unbracketed host with several colons or a
//     character a URL host cannot hold, a malformed bracket, or brackets
//     around anything but an IPv6 address.
func splitAuthority(authority string) (host, port string, err error) { //nolint:nonamedreturns // Same-type returns need names.
	if rest, bracketed := strings.CutPrefix(authority, "["); bracketed {
		var found bool
		if host, rest, found = strings.Cut(rest, "]"); !found {
			return "", "", ErrInvalidHost
		}

		// RFC 3986 brackets only an IP literal, and url.Parse accepts only an
		// IPv6 address there, so a host built in code is held to the same rule.
		if !isIPv6(host) {
			return "", "", ErrInvalidHost
		}

		if rest != "" {
			if port, found = strings.CutPrefix(rest, ":"); !found {
				return "", "", ErrInvalidHost
			}
		}

		return host, port, nil
	}

	if strings.Count(authority, ":") > 1 {
		return "", "", ErrInvalidHost
	}

	host, port, _ = strings.Cut(authority, ":")

	// url.Parse never puts a slash, an at sign, or a space in a host, so a host
	// built in code is held to the same rule.
	if err := ValidateHostToken(host); err != nil {
		return "", "", err
	}

	return host, port, nil
}

// isIPv6 reports whether host is an IPv6 address, with an optional zone, that
// a URL host can hold between brackets.
//
// Parameters:
//   - host: the host, without brackets.
//
// Returns:
//   - bool: whether host is such an IPv6 address.
func isIPv6(host string) bool {
	addr, err := netip.ParseAddr(host)
	if err != nil || !addr.Is6() {
		return false
	}

	// netip accepts any text as a zone, but url.Parse reads back only a zone of
	// ASCII characters that a host holds unescaped. Those include colons, as in
	// an interface alias such as eth0:1. Spaces read back too, since Windows
	// interface names hold them. A zone with a slash, a bracket, or a character
	// outside ASCII makes the URL fail to parse, or ends the brackets early.
	zone := addr.Zone()
	for i := range len(zone) {
		if c := zone[i]; c >= utf8.RuneSelf || c != ' ' && c != ':' && !isHostTokenByte(c) {
			return false
		}
	}

	return true
}
