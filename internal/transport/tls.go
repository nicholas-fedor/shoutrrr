package transport

import "crypto/tls"

// TLSConfig returns TLS settings that verify the server's certificate and
// require TLS 1.2 or later.
//
// Parameters:
//   - serverName: the host name to verify the certificate against. An HTTP
//     transport fills it in from each request URL, so HTTP clients pass "".
//
// Returns:
//   - *tls.Config: new settings that are not shared with other callers.
func TLSConfig(serverName string) *tls.Config {
	return &tls.Config{
		ServerName: serverName,
		MinVersion: tls.VersionTLS12,
	}
}

// InsecureTLSConfig returns TLS settings that skip verifying the server's
// certificate and require TLS 1.2 or later. A service uses them only when the
// user asks to skip verification, and warns about it through internal/logging.
//
// Parameters:
//   - serverName: the host name to send in the handshake. HTTP clients pass "".
//
// Returns:
//   - *tls.Config: new settings that are not shared with other callers.
func InsecureTLSConfig(serverName string) *tls.Config {
	config := TLSConfig(serverName)
	config.InsecureSkipVerify = true

	return config
}
