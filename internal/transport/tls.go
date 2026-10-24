package transport

import "crypto/tls"

// Logger receives the warning that TLS certificate verification is off.
// The standard service logger implements it.
type Logger interface {
	// Log writes v to the service's logger.
	Log(v ...any)
}

// insecureWarning is the message logged when a connection skips TLS
// certificate verification.
const insecureWarning = "Warning: TLS verification is disabled, making connections insecure"

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
// user asks to skip verification, and logs [WarnInsecure] when it does.
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

// WarnInsecure logs that TLS certificate verification is off, so connections
// can be intercepted.
//
// Parameters:
//   - logger: the service's logger.
func WarnInsecure(logger Logger) {
	logger.Log(insecureWarning)
}
