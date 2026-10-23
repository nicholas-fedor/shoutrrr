package smtp

import "errors"

// Errors returned when reading the configuration, authenticating, or sending.
var (
	// ErrServerNoStartTLS is returned when STARTTLS is required but the server does not support it.
	ErrServerNoStartTLS = errors.New("server does not support StartTLS")

	// ErrFromAddressMissing is returned when the configuration URL omits [Config.FromAddress].
	ErrFromAddressMissing = errors.New("fromAddress missing from config URL")

	// ErrToAddressMissing is returned when the configuration URL omits [Config.ToAddresses].
	ErrToAddressMissing = errors.New("toAddress missing from config URL")

	// ErrHostMissing is returned when the configuration URL omits the SMTP server host.
	ErrHostMissing = errors.New("host missing from config URL")

	// ErrInvalidOAuth2Refresh is returned when the OAuth2 refresh settings are
	// incomplete, are set without OAuth2 authentication, or name a token URL
	// that is not HTTPS or a loopback HTTP URL.
	ErrInvalidOAuth2Refresh = errors.New("invalid OAuth2 refresh settings")

	// errUnencryptedConnection is returned when authentication is attempted
	// on a non-local server without TLS.
	errUnencryptedConnection = errors.New("unencrypted connection")

	// errWrongHostName is returned when the server name does not match the expected host.
	errWrongHostName = errors.New("wrong host name")

	// errUnexpectedServerChallenge is returned when the server issues an additional
	// challenge after the client has finished sending credentials.
	errUnexpectedServerChallenge = errors.New("unexpected server challenge")

	// errHeaderBreak is returned when a sender or recipient address contains CR or LF.
	errHeaderBreak = errors.New("address contains CR or LF")
)
