package smtp

import "errors"

var (
	// ErrServerNoStartTLS is returned when STARTTLS is required but the server does not support it.
	ErrServerNoStartTLS = errors.New("server does not support StartTLS")

	// ErrFromAddressMissing is returned when the configuration URL omits [Config.FromAddress].
	ErrFromAddressMissing = errors.New("fromAddress missing from config URL")

	// ErrToAddressMissing is returned when the configuration URL omits [Config.ToAddresses].
	ErrToAddressMissing = errors.New("toAddress missing from config URL")

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
