package smtp

import (
	"errors"
	"net/smtp"
	"os"
	"strings"

	"github.com/nicholas-fedor/shoutrrr/internal/failures"
)

// session is one SMTP conversation after a client has been dialed.
type session struct {
	// client is the connected SMTP client for this conversation.
	client *smtp.Client
	// config is the send-time configuration clone.
	config *Config
	// svc provides logging and message templates.
	svc *Service
	// closed is true after [session.quit] has closed the client.
	closed bool
}

const (
	// shortResponseErrorSubstring matches textproto short-response errors sometimes seen on QUIT.
	shortResponseErrorSubstring = "short response"
)

// auth authenticates the SMTP client using the configured mechanism.
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
			return fail(FailAuthenticating, err)
		}
	}

	return nil
}

// closeClient closes the SMTP client and logs a warning if Close fails.
func (s *session) closeClient() {
	if closeErr := s.client.Close(); closeErr != nil {
		s.svc.Logf("Warning: Failed to close SMTP client connection: %v", closeErr)
	}
}

// closeIfOpen closes the client when [session.quit] has not already done so.
func (s *session) closeIfOpen() {
	if s.closed {
		return
	}

	s.closeClient()
}

// deliver sends the message to each configured recipient.
//
// The SMTP session is reset between recipients. A failed reset stops remaining
// deliveries.
//
// Parameters:
//   - message: The notification body.
//   - boundary: Multipart boundary used when HTML mail is enabled.
//
// Returns:
//   - Failures encountered while sending to recipients or resetting the session.
func (s *session) deliver(message, boundary string) []error {
	var errs []error

	for i, toAddress := range s.config.ToAddresses {
		if err := s.sendMessage(toAddress, message, boundary); err != nil {
			errs = append(errs, fail(FailSendRecipient, err, toAddress))
			s.svc.Logf("Failed to send to %q: %v", toAddress, err)
		} else {
			s.svc.Logf("Mail successfully sent to %q!", toAddress)
		}

		if i < len(s.config.ToAddresses)-1 {
			if err := s.client.Reset(); err != nil {
				errs = append(errs, fail(FailResetSession, err))

				break
			}
		}
	}

	return errs
}

// ehloName returns the hostname sent in the SMTP EHLO/HELO command.
//
// When [Config.ClientHost] is "auto", the local hostname is used, falling back to
// localhost if it cannot be determined.
func (s *session) ehloName() string {
	if s.config.ClientHost != "auto" {
		return s.config.ClientHost
	}

	hostname, err := os.Hostname()
	if err != nil {
		s.svc.Logf("Failed to get hostname, falling back to localhost: %v", err)

		return "localhost"
	}

	return hostname
}

// quit ends the SMTP session with QUIT and closes the client.
//
// Errors matching [shortResponseErrorSubstring] on QUIT are logged and ignored
// because they do not affect delivery. Other QUIT errors are appended to errs.
//
// Parameters:
//   - errs: Accumulated delivery errors to extend if QUIT fails.
//
// Returns:
//   - The original error slice, possibly with a session-closure failure appended.
func (s *session) quit(errs []error) []error {
	if err := s.client.Quit(); err != nil {
		// Ignore known "short response" errors from quirky servers (e.g., Office 365 on close),
		// as they don't impact delivery.
		if strings.Contains(err.Error(), shortResponseErrorSubstring) {
			s.svc.Logf("Warning: Ignoring session closure error (delivery succeeded): %v", err)
		} else {
			errs = append(errs, fail(FailClosingSession, err))
		}
	}

	s.closed = true
	s.closeClient()

	return errs
}

// run conducts the SMTP session: handshake, optional STARTTLS, authentication,
// delivery, and QUIT. Pre-delivery failures close the client without sending QUIT.
// An empty recipient list fails before the handshake.
//
// Parameters:
//   - message: The notification body.
//
// Returns:
//   - An error if there are no recipients, or if handshake, STARTTLS, authentication, or delivery fails.
func (s *session) run(message string) error {
	s.config.restorePlusAddresses()

	defer s.closeIfOpen()

	if len(s.config.ToAddresses) == 0 {
		return fail(FailSendRecipient, ErrToAddressMissing, "")
	}

	if err := s.client.Hello(s.ehloName()); err != nil {
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

	if err := s.startTLS(); err != nil {
		return err
	}

	if err := s.auth(); err != nil {
		return err
	}

	errs := s.deliver(message, boundary)
	errs = s.quit(errs)

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
// [Config.RequireStartTLS] is false, a warning is logged and the session continues
// unencrypted.
//
// Returns:
//   - An error if STARTTLS is required but unsupported or the upgrade fails.
func (s *session) startTLS() error {
	if !s.config.UseStartTLS || useImplicitTLS(s.config.Encryption, s.config.Port) {
		return nil
	}

	if supported, _ := s.client.Extension("StartTLS"); !supported {
		if s.config.RequireStartTLS {
			return fail(FailEnableStartTLS, ErrServerNoStartTLS)
		}

		s.svc.Logf(
			"Warning: StartTLS enabled, but server does not support it. Connection is unencrypted",
		)

		return nil
	}

	if err := s.client.StartTLS(newTLSConfig(s.config)); err != nil {
		return fail(FailEnableStartTLS, err)
	}

	return nil
}
