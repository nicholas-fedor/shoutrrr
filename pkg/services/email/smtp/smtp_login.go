package smtp

import (
	"fmt"
	"net/smtp"
)

// loginAuth implements the SASL LOGIN authentication mechanism for SMTP.
//
// Challenge prompt text from the server is ignored.
// The username and password are sent in order as successive client responses.
type loginAuth struct {
	// username is the SMTP account username sent as the first LOGIN response.
	username string
	// password is the SMTP account password sent as the second LOGIN response.
	password string
	// host is the expected SMTP server hostname checked in Start.
	host string
	// respStep tracks which LOGIN response to send next.
	respStep uint8
}

// newLoginAuth returns an [smtp.Auth] that implements SASL LOGIN authentication.
//
// LOGIN is a multi-step mechanism commonly offered by servers that do not
// support AUTH PLAIN:
//  1. The client sends AUTH LOGIN.
//  2. The server issues a challenge (often a username prompt) and the client sends the username.
//  3. The server issues a challenge (often a password prompt) and the client sends the password.
//
// Challenge contents are ignored per draft-murchison-sasl-login-00 so that
// common server variants (different wording, casing, or trailing NULs) work.
//
// Credentials are sent only when the connection uses TLS or the server name is
// localhost. Otherwise authentication fails without transmitting credentials,
// matching [smtp.PlainAuth].
//
// Parameters:
//   - username: The SMTP account username.
//   - password: The SMTP account password.
//   - host: The expected SMTP server hostname, which must match [smtp.ServerInfo.Name] at [loginAuth.Start].
//
// Returns:
//   - An [smtp.Auth] that performs LOGIN authentication for the given credentials and host.
func newLoginAuth(username, password, host string) smtp.Auth {
	return &loginAuth{
		username: username,
		password: password,
		host:     host,
		respStep: 0,
	}
}

// isLocalhost reports whether name is a loopback host identity.
//
// Parameters:
//   - name: The server name to check (for example from [smtp.ServerInfo.Name]).
//
// Returns:
//   - true if name is "localhost", "127.0.0.1", or "::1"; otherwise, false.
func isLocalhost(name string) bool {
	return name == "localhost" || name == "127.0.0.1" || name == "::1"
}

// Next continues the LOGIN challenge-response exchange.
//
// When more is true, the next credential is returned by step order (username,
// then password). Challenge bytes from the server are ignored. An extra
// challenge after the password step returns an error so net/smtp aborts AUTH
// instead of sending a dummy client response.
//
// Parameters:
//   - fromServer: The decoded server challenge payload (ignored for LOGIN).
//   - more: Whether the server expects another client response (SMTP 334).
//
// Returns:
//   - The next client response (username or password bytes), or nil when the exchange is finished.
//   - A non-nil error if the server issues an unexpected additional challenge.
func (a *loginAuth) Next(fromServer []byte, more bool) ([]byte, error) {
	if !more {
		return nil, nil
	}

	// Answer by step order; do not parse challenge text.
	switch a.respStep {
	case 0:
		a.respStep++

		return []byte(a.username), nil
	case 1:
		a.respStep++

		return []byte(a.password), nil
	default:
		return nil, fmt.Errorf("%w: %q", errUnexpectedServerChallenge, fromServer)
	}
}

// Start begins LOGIN authentication after validating TLS and host identity.
//
// Parameters:
//   - server: Connection metadata from the SMTP client, including Name and TLS ([smtp.ServerInfo]).
//
// Returns:
//   - The authentication protocol name ("LOGIN").
//   - An empty initial client response (LOGIN has no initial SASL payload).
//   - A non-nil error if the connection is unencrypted to a non-local host or the host name does not match.
func (a *loginAuth) Start(server *smtp.ServerInfo) (string, []byte, error) {
	// Require TLS unless the server name is localhost. Without TLS, [smtp.ServerInfo]
	// cannot be trusted and an attacker could advertise LOGIN to harvest credentials.
	if !server.TLS && !isLocalhost(server.Name) {
		return "", nil, errUnencryptedConnection
	}

	if server.Name != a.host {
		return "", nil, errWrongHostName
	}

	a.respStep = 0

	return "LOGIN", nil, nil
}
