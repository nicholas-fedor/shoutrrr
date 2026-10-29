package smtp

import (
	"context"
	"errors"
	"net/smtp"
	"os"
	"strings"

	"github.com/nicholas-fedor/shoutrrr/internal/failures"
	"github.com/nicholas-fedor/shoutrrr/internal/logging"
)

// session is one SMTP conversation after a client has been dialed.
type session struct {
	// client is the connected SMTP client for this conversation.
	client *smtp.Client
	// config is the send-time configuration clone.
	config *Config
	// svc provides logging and message templates.
	svc *Service
	// closed is true after the connection has been closed by Quit or Close.
	closed bool
}

// Server reply matching.
const (
	// shortResponseErrorSubstring matches textproto short-response errors sometimes seen on QUIT.
	shortResponseErrorSubstring = "short response"
)

// auth authenticates the SMTP client using the configured mechanism.
//
// When the server rejects a refreshed OAuth2 access token, the cached token is
// dropped, so the next send gets a new one.
//
// Returns:
//   - An error if authentication cannot be configured or the server rejects it.
func (s *session) auth() error {
	mechanism, err := newAuth(s.config)
	if err != nil {
		return err
	}

	if mechanism != nil {
		if err := s.client.Auth(mechanism); err != nil {
			if s.config.usesTokenRefresh() {
				s.svc.forgetToken(s.config)
			}

			return fail(FailAuthenticating, err)
		}
	}

	return nil
}

// closeClient closes the SMTP client and logs a warning if Close fails.
//
// Parameters:
//   - ctx: the context of the send, passed to the warning.
func (s *session) closeClient(ctx context.Context) {
	if closeErr := s.client.Close(); closeErr != nil {
		s.svc.Warn(ctx, "closing the SMTP connection failed", logging.KeyError, closeErr)
	}
}

// closeIfOpen closes the client when [session.quit] has not already done so.
//
// Parameters:
//   - ctx: the context of the send, passed to its warning.
func (s *session) closeIfOpen(ctx context.Context) {
	if s.closed {
		return
	}

	s.closeClient(ctx)
	s.closed = true
}

// deliver sends the message to each configured recipient.
//
// The SMTP session is reset between recipients. A failed reset stops the
// remaining deliveries. A context that ended also stops them before the next
// reset, and its error is reported.
//
// Parameters:
//   - ctx: the context of the send, passed to its log records.
//   - message: The notification body.
//   - boundary: Multipart boundary used when HTML mail is enabled.
//
// Returns:
//   - Failures encountered while sending to recipients or resetting the session.
func (s *session) deliver(ctx context.Context, message, boundary string) []error {
	var errs []error

	for i, toAddress := range s.config.ToAddresses {
		if err := ctx.Err(); err != nil {
			errs = append(errs, err)

			break
		}

		if i > 0 {
			if err := s.client.Reset(); err != nil {
				errs = append(errs, fail(FailResetSession, err))

				break
			}
		}

		if err := s.sendMessage(toAddress, message, boundary); err != nil {
			errs = append(errs, fail(FailSendRecipient, err, toAddress))
		} else {
			// The address is personal data, so the record leaves it out.
			s.svc.Debug(ctx, "mail sent")
		}
	}

	return errs
}

// ehloName returns the hostname sent in the SMTP EHLO/HELO command.
//
// When [Config.ClientHost] is "auto", the local hostname is used, falling back to
// localhost if it cannot be determined.
//
// Parameters:
//   - ctx: the context of the send, passed to its warning.
//
// Returns:
//   - string: the EHLO hostname.
func (s *session) ehloName(ctx context.Context) string {
	if s.config.ClientHost != "auto" {
		return s.config.ClientHost
	}

	hostname, err := os.Hostname()
	if err != nil {
		s.svc.Warn(ctx, "hostname unavailable, using localhost", logging.KeyError, err)

		return "localhost"
	}

	return hostname
}

// quit ends the SMTP session with QUIT.
//
// A successful [smtp.Client.Quit] already closes the connection, so Close is
// not called again. When QUIT fails, the client is closed explicitly. Once ctx
// has ended, the connection's I/O fails at once, so the client is closed
// without sending QUIT.
// Errors matching [shortResponseErrorSubstring] are logged and ignored because
// they do not affect delivery. Other QUIT errors are appended to errs.
//
// Parameters:
//   - ctx: the context of the send, passed to its warnings.
//   - errs: Accumulated delivery errors to extend if QUIT fails.
//
// Returns:
//   - The original error slice, possibly with a session-closure failure appended.
func (s *session) quit(ctx context.Context, errs []error) []error {
	if ctx.Err() != nil {
		s.closeIfOpen(ctx)

		return errs
	}

	err := s.client.Quit()
	if err == nil {
		s.closed = true

		return errs
	}

	if strings.Contains(err.Error(), shortResponseErrorSubstring) {
		// The error quotes the server's reply, which can name recipients, so
		// the record leaves it out.
		s.svc.Warn(ctx, "session closure error ignored, because delivery succeeded")
	} else {
		errs = append(errs, fail(FailClosingSession, err))
	}

	s.closeIfOpen(ctx)

	return errs
}

// run conducts the SMTP session: handshake, optional STARTTLS, authentication,
// delivery, and QUIT. Pre-delivery failures close the client without sending QUIT.
// An empty recipient list or sender fails before the handshake, so a blank
// sender is never sent as the null reverse-path.
//
// Parameters:
//   - ctx: the context of the send, passed to its log records.
//   - message: The notification body.
//
// Returns:
//   - An error if there are no recipients or no sender, or if handshake, STARTTLS, authentication, or delivery fails.
func (s *session) run(ctx context.Context, message string) error {
	s.config.restorePlusAddresses()

	defer s.closeIfOpen(ctx)

	if len(s.config.ToAddresses) == 0 {
		return fail(FailSendRecipient, ErrToAddressMissing, "")
	}

	if s.config.FromAddress == "" {
		return fail(FailSetSender, ErrFromAddressMissing)
	}

	if err := s.client.Hello(s.ehloName(ctx)); err != nil {
		return fail(FailHandshake, err)
	}

	var boundary string

	if s.config.UseHTML {
		generated, err := generateBoundary()
		if err != nil {
			return fail(FailUnknown, err)
		}

		boundary = generated
	}

	if err := s.startTLS(ctx); err != nil {
		return err
	}

	s.warnConnection(ctx)

	if err := s.auth(); err != nil {
		return err
	}

	errs := s.deliver(ctx, message, boundary)
	errs = s.quit(ctx, errs)

	if len(errs) > 0 {
		return failures.Wrap(
			"failed to send to some recipients",
			FailSendRecipient,
			errors.Join(errs...),
		)
	}

	return nil
}

// sendMessage sends an email to a single recipient using MAIL, RCPT, and DATA.
//
// Sender and recipient addresses that contain CR or LF are rejected before any
// SMTP command is issued.
//
// Parameters:
//   - toAddress: The recipient address.
//   - message: The notification body.
//   - boundary: Multipart boundary used when HTML mail is enabled.
//
// Returns:
//   - An error if an address is unsafe or any SMTP command or message write fails.
func (s *session) sendMessage(toAddress, message, boundary string) error {
	if strings.ContainsAny(s.config.FromAddress, "\r\n") {
		return fail(FailSetSender, errHeaderBreak)
	}

	if strings.ContainsAny(toAddress, "\r\n") {
		return fail(FailSetRecipient, errHeaderBreak)
	}

	if err := s.client.Mail(s.config.FromAddress); err != nil {
		return fail(FailSetSender, err)
	}

	if err := s.client.Rcpt(toAddress); err != nil {
		return fail(FailSetRecipient, err)
	}

	writeCloser, err := s.client.Data()
	if err != nil {
		return fail(FailOpenDataStream, err)
	}

	if err := writeHeaders(writeCloser, headers(s.config, toAddress, boundary)); err != nil {
		return err
	}

	var writeErr error
	if s.config.UseHTML {
		writeErr = writeAlternative(writeCloser, message, boundary, s.svc)
	} else {
		writeErr = writePart(writeCloser, message, templatePlain, s.svc)
	}

	if writeErr != nil {
		return writeErr
	}

	if err = writeCloser.Close(); err != nil {
		return fail(FailCloseDataStream, err)
	}

	return nil
}

// startTLS upgrades the connection with STARTTLS when configured.
//
// Implicit TLS sessions skip this step. If STARTTLS is unsupported and
// [Config.RequireStartTLS] is false, a warning is logged once and the session
// continues unencrypted.
//
// Parameters:
//   - ctx: the context of the send, passed to its warning.
//
// Returns:
//   - An error if STARTTLS is required but unsupported or the upgrade fails.
func (s *session) startTLS(ctx context.Context) error {
	if !s.config.UseStartTLS || useImplicitTLS(s.config.Encryption, s.config.Port) {
		return nil
	}

	if supported, _ := s.client.Extension("StartTLS"); !supported {
		if s.config.RequireStartTLS {
			return fail(FailEnableStartTLS, ErrServerNoStartTLS)
		}

		s.svc.warnings.Do("starttls-unsupported", func() {
			s.svc.Warn(ctx, "server does not support STARTTLS")
		})

		return nil
	}

	if err := s.client.StartTLS(newTLSConfig(s.config)); err != nil {
		return fail(FailEnableStartTLS, err)
	}

	return nil
}

// warnConnection logs, once per service instance, that the session sends
// without TLS, or that its TLS connection skips certificate verification. It
// runs after the TLS setup, so it reports what the session actually uses.
//
// Parameters:
//   - ctx: the context of the send, passed to the warnings.
func (s *session) warnConnection(ctx context.Context) {
	if _, usesTLS := s.client.TLSConnectionState(); !usesTLS {
		s.svc.warnings.Plaintext(ctx, s.svc)

		return
	}

	if s.config.SkipTLSVerify {
		s.svc.warnings.Insecure(ctx, s.svc)
	}
}
