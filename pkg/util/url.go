package util

import (
	"errors"
	"net"
	"net/url"
	"strings"
)

// Errors for a URL authority that does not split into a hostname and port.
var (
	// ErrInvalidHost indicates a URL host that is not a hostname, an IPv4
	// address, or an IPv6 address in brackets.
	ErrInvalidHost = errors.New("invalid URL host")
	// ErrInvalidPort indicates a URL port that is not a decimal number.
	ErrInvalidPort = errors.New("invalid URL port")
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

// JoinURLHost writes host and port as a URL authority. An IPv6 host is
// enclosed in brackets whether or not a port follows it, so [SplitURLHost]
// reads the same host and port back.
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
