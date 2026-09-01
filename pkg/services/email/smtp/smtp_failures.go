package smtp

import (
	"github.com/nicholas-fedor/shoutrrr/internal/failures"
)

const (
	// FailUnknown is the default [failures.FailureID].
	FailUnknown failures.FailureID = iota
	// FailGetSMTPClient is returned when a SMTP client could not be created.
	FailGetSMTPClient
	// FailEnableStartTLS is returned when failing to enable StartTLS.
	FailEnableStartTLS
	// FailAuthType is returned when the [Config.Auth] method could not be identified.
	FailAuthType
	// FailAuthenticating is returned when the authentication fails.
	FailAuthenticating
	// FailSendRecipient is returned when sending to a recipient fails.
	FailSendRecipient
	// FailClosingSession is returned when the server doesn't accept the QUIT command.
	FailClosingSession
	// FailPlainHeader is returned when the text/plain multipart header could not be set.
	FailPlainHeader
	// FailHTMLHeader is returned when the text/html multipart header could not be set.
	FailHTMLHeader
	// FailMultiEndHeader is returned when the multipart end header could not be set.
	FailMultiEndHeader
	// FailMessageTemplate is returned when the message template could not be written to the stream.
	FailMessageTemplate
	// FailMessageRaw is returned when a non-templated message could not be written to the stream.
	FailMessageRaw
	// FailSetSender is returned when the server did not accept MAIL FROM.
	FailSetSender
	// FailSetRecipient is returned when the server didn't accept the recipient address.
	FailSetRecipient
	// FailOpenDataStream is returned when the server did not accept DATA.
	FailOpenDataStream
	// FailWriteHeaders is returned when the headers could not be written to the data stream.
	FailWriteHeaders
	// FailCloseDataStream is returned when the server didn't accept the data stream contents.
	FailCloseDataStream
	// FailConnectToServer is returned when the TCP connection to the server failed.
	FailConnectToServer
	// FailCreateSMTPClient is returned when SMTP client initialization failed.
	FailCreateSMTPClient
	// FailApplySendParams is returned when updating the send [Config] failed.
	FailApplySendParams
	// FailHandshake is returned when the initial HELLO handshake returned an error.
	FailHandshake
	// FailResetSession is returned when the server rejects RSET between recipients.
	FailResetSession
)

// fail creates an SMTP-specific failure with a descriptive message and ID.
//
// The message is selected from failureID.
// Optional args are interpolated into the message when it contains format verbs.
//
// Parameters:
//   - failureID: Identifier selecting the failure message ([failures.FailureID]).
//   - err: The underlying error to wrap, which may be nil.
//   - args: Optional values interpolated into the failure message.
//
// Returns:
//   - A failure wrapping err with the selected message and ID.
func fail(failureID failures.FailureID, err error, args ...any) failures.Failure {
	var msg string

	switch failureID {
	case FailGetSMTPClient:
		msg = "error getting SMTP client"
	case FailConnectToServer:
		msg = "error connecting to server"
	case FailCreateSMTPClient:
		msg = "error creating smtp client"
	case FailEnableStartTLS:
		msg = "error enabling StartTLS"
	case FailAuthenticating:
		msg = "error authenticating"
	case FailAuthType:
		msg = "invalid authentication method '%s'"
	case FailSendRecipient:
		msg = "error sending message to recipient %q"
	case FailClosingSession:
		msg = "error closing session"
	case FailPlainHeader:
		msg = "error writing plain header"
	case FailHTMLHeader:
		msg = "error writing HTML header"
	case FailMultiEndHeader:
		msg = "error writing multipart end header"
	case FailMessageTemplate:
		msg = "error applying message template"
	case FailMessageRaw:
		msg = "error writing message"
	case FailSetSender:
		msg = "error setting MAIL FROM"
	case FailSetRecipient:
		msg = "error setting RCPT"
	case FailOpenDataStream:
		msg = "error starting DATA"
	case FailWriteHeaders:
		msg = "error writing message headers"
	case FailCloseDataStream:
		msg = "error closing message stream"
	case FailApplySendParams:
		msg = "error applying params to send config"
	case FailHandshake:
		msg = "server did not accept the handshake"
	case FailResetSession:
		msg = "error resetting session between recipients"
	// case FailUnknown:
	default:
		msg = "an unknown error occurred"
	}

	return failures.Wrap(msg, failureID, err, args...)
}
