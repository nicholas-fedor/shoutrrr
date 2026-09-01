package smtp

import (
	"net/smtp"
)

// oauth2Auth implements the SASL XOAUTH2 authentication mechanism for SMTP.
//
// Credentials are sent as a single initial response from Start. See [newOAuth2Auth].
type oauth2Auth struct {
	// username is the SMTP account username.
	username string
	// accessToken is the OAuth2 Bearer token sent in Start.
	accessToken string
	// host is the expected SMTP server hostname checked in Start.
	host string
}

// newOAuth2Auth returns an [smtp.Auth] that implements SASL XOAUTH2 authentication.
//
// The password is treated as a static OAuth2 access token. Token refresh is not
// supported. A 334 challenge is answered with an empty continuation so the
// server can send 535 on failure. See
// https://developers.google.com/gmail/imap/xoauth2-protocol.
//
// Credentials are sent only when the connection uses TLS or the server name is
// localhost. Otherwise authentication fails without transmitting the token,
// matching [smtp.PlainAuth] and [newLoginAuth].
//
// Parameters:
//   - username: The SMTP account username.
//   - accessToken: A valid OAuth2 access token used as the Bearer credential.
//   - host: The expected SMTP server hostname, which must match [smtp.ServerInfo.Name] at [oauth2Auth.Start].
//
// Returns:
//   - An [smtp.Auth] that performs XOAUTH2 authentication for the given credentials and host.
func newOAuth2Auth(username, accessToken, host string) smtp.Auth {
	return &oauth2Auth{
		username:    username,
		accessToken: accessToken,
		host:        host,
	}
}

// Next continues the XOAUTH2 exchange.
//
// Credentials were sent in [oauth2Auth.Start]. When more is true the server
// issued a 334 (typically a JSON error status); an empty continuation is
// returned so net/smtp does not treat AUTH as successful and the server can
// follow with 535.
//
// Parameters:
//   - fromServer: The decoded server challenge payload (unused).
//   - more: Whether the server expects another client response (SMTP 334).
//
// Returns:
//   - An empty slice when the server expects a continuation, or nil when AUTH is finished.
//   - A nil error; failure is reported by the subsequent SMTP status line.
func (a *oauth2Auth) Next(_ []byte, more bool) ([]byte, error) {
	if more {
		return []byte{}, nil
	}

	return nil, nil
}

// Start begins XOAUTH2 authentication with an initial SASL payload.
//
// Parameters:
//   - server: Connection metadata from the SMTP client, including Name and TLS ([smtp.ServerInfo]).
//
// Returns:
//   - The authentication protocol name ("XOAUTH2").
//   - The initial client response containing the user and Bearer token.
//   - A non-nil error if the connection is unencrypted to a non-local host or the host name does not match.
func (a *oauth2Auth) Start(server *smtp.ServerInfo) (string, []byte, error) {
	if !server.TLS && !isLocalhost(server.Name) {
		return "", nil, errUnencryptedConnection
	}

	if server.Name != a.host {
		return "", nil, errWrongHostName
	}

	resp := []byte("user=" + a.username + "\x01auth=Bearer " + a.accessToken + "\x01\x01")

	return "XOAUTH2", resp, nil
}
