package smtp

import "errors"

// Errors returned when reading the configuration, authenticating, or sending.
var (
	// ErrServerNoStartTLS is returned when STARTTLS is required but the server does not support it.
	ErrServerNoStartTLS = errors.New("server does not support StartTLS")

	// ErrFromAddressMissing is returned when the config has no [Config.FromAddress].
	ErrFromAddressMissing = errors.New("fromAddress missing from config")

	// ErrToAddressMissing is returned when the config has no [Config.ToAddresses].
	ErrToAddressMissing = errors.New("toAddress missing from config")

	// ErrHostMissing is returned when the config has no SMTP server host.
	ErrHostMissing = errors.New("host missing from config")

	// ErrInvalidAddress is returned when [Config.FromAddress] or an entry of
	// [Config.ToAddresses] is not one RFC 5322 address, such as
	// jane@example.com or Jane Doe <jane@example.com>.
	ErrInvalidAddress = errors.New("invalid e-mail address")

	// ErrInvalidOAuth2Settings is returned when the OAuth2 token settings, for
	// the refresh grant or the client credentials grant, are incomplete or mix
	// the two grants. It is also returned when they are set without OAuth2
	// authentication or name a token URL that is not HTTPS or a loopback HTTP
	// URL.
	ErrInvalidOAuth2Settings = errors.New("invalid OAuth2 token settings")

	// errUnencryptedConnection is returned when authentication is attempted
	// on a non-local server without TLS.
	errUnencryptedConnection = errors.New("unencrypted connection")

	// errWrongHostName is returned when the server name does not match the expected host.
	errWrongHostName = errors.New("wrong host name")

	// errUnexpectedServerChallenge is returned when the server issues an additional
	// challenge after the client has finished sending credentials.
	errUnexpectedServerChallenge = errors.New("unexpected server challenge")
)
