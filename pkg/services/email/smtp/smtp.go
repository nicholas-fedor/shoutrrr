package smtp

import (
	"context"
	"crypto/rand"
	"crypto/tls"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"net"
	"net/smtp"
	"net/url"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/nicholas-fedor/shoutrrr/internal/failures"
	"github.com/nicholas-fedor/shoutrrr/pkg/format"
	"github.com/nicholas-fedor/shoutrrr/pkg/services/standard"
	"github.com/nicholas-fedor/shoutrrr/pkg/types"
)

const (
	contentHTML                 = "text/html; charset=\"UTF-8\""
	contentPlain                = "text/plain; charset=\"UTF-8\""
	contentMultipart            = "multipart/alternative; boundary=%s"
	DefaultSMTPPort             = 25               // DefaultSMTPPort is the standard port for SMTP communication.
	boundaryByteLen             = 8                // boundaryByteLen is the number of bytes for the multipart boundary.
	DefaultTimeout              = 10               // DefaultTimeout is the default timout in seconds
	shortResponseErrorSubstring = "short response" // Error substring from textproto indicating a short response that is sometimes received during session closure.
)

// ErrNoAuth is a sentinel error indicating no authentication is required.
var ErrNoAuth = errors.New("no authentication required")

// Static errors for SMTP operations.
var (
	ErrServerNoStartTLS = errors.New("server does not support StartTLS")
)

// Service sends notifications to given email addresses via SMTP.
type Service struct {
	standard.Standard
	standard.Templater
	Config            *Config
	multipartBoundary string
	propKeyResolver   format.PropKeyResolver
}

// Initialize loads ServiceConfig from configURL and sets logger for this Service.
func (service *Service) Initialize(configURL *url.URL, logger types.StdLogger) error {
	service.SetLogger(logger)
	service.Config = &Config{
		Port:        DefaultSMTPPort,
		ToAddresses: nil,
		Subject:     "",
		Auth:        AuthTypes.Unknown,
		UseStartTLS: true,
		UseHTML:     false,
		Encryption:  EncMethods.Auto,
		ClientHost:  "localhost",
		Timeout:     DefaultTimeout * time.Second,
	}

	pkr := format.NewPropKeyResolver(service.Config)

	if err := service.Config.setURL(&pkr, configURL); err != nil {
		return err
	}

	if service.Config.Auth == AuthTypes.Unknown {
		if service.Config.Username != "" {
			service.Config.Auth = AuthTypes.Plain
		} else {
			service.Config.Auth = AuthTypes.None
		}
	}

	service.propKeyResolver = pkr

	return nil
}

// GetID returns the service identifier.
func (service *Service) GetID() string {
	return Scheme
}

// Send sends a notification message to email recipients.
func (service *Service) Send(message string, params *types.Params) error {
	config := service.Config.Clone()
	if err := service.propKeyResolver.UpdateConfigFromParams(&config, params); err != nil {
		return fail(FailApplySendParams, err)
	}

	if config.SkipTLSVerify {
		service.Log("Warning: TLS verification is disabled, making connections insecure")
	}

	ctx, cancel := context.WithTimeout(context.Background(), config.Timeout)
	defer cancel()

	client, err := getClientConnection(ctx, service.Config)
	if err != nil {
		return fail(FailGetSMTPClient, err)
	}

	return service.doSend(client, message, &config)
}

// getClientConnection establishes a connection to the SMTP server using the provided configuration.
func getClientConnection(ctx context.Context, config *Config) (*smtp.Client, error) {
	var (
		conn net.Conn
		err  error
	)

	addr := net.JoinHostPort(config.Host, strconv.FormatUint(uint64(config.Port), 10))

	if useImplicitTLS(config.Encryption, config.Port) {
		dialer := &tls.Dialer{
			Config: &tls.Config{
				ServerName:         config.Host,
				MinVersion:         tls.VersionTLS12,     // Enforce TLS 1.2 or higher
				InsecureSkipVerify: config.SkipTLSVerify, //nolint:gosec
			},
		}
		conn, err = dialer.DialContext(ctx, "tcp", addr)
	} else {
		dialer := &net.Dialer{}
		conn, err = dialer.DialContext(ctx, "tcp", addr)
	}

	if err != nil {
		return nil, fail(FailConnectToServer, err)
	}

	client, err := smtp.NewClient(conn, config.Host)
	if err != nil {
		return nil, fail(FailCreateSMTPClient, err)
	}

	return client, nil
}

// doSend sends an email message using the provided SMTP client and configuration.
func (service *Service) doSend(client *smtp.Client, message string, config *Config) failure {
	config.FixEmailTags()

	clientHost := service.resolveClientHost(config)

	if err := client.Hello(clientHost); err != nil {
		return fail(FailHandshake, err)
	}

	if config.UseHTML {
		b := make([]byte, boundaryByteLen)
		if _, err := rand.Read(b); err != nil {
			return fail(FailUnknown, err) // Fallback error for rare case
		}

		service.multipartBoundary = hex.EncodeToString(b)
	}

	if config.UseStartTLS && !useImplicitTLS(config.Encryption, config.Port) {
		if supported, _ := client.Extension("StartTLS"); !supported {
			if config.RequireStartTLS {
				return fail(FailEnableStartTLS, ErrServerNoStartTLS)
			}

			service.Logf(
				"Warning: StartTLS enabled, but server does not support it. Connection is unencrypted",
			)
		} else {
			if err := client.StartTLS(&tls.Config{
				ServerName:         config.Host,
				MinVersion:         tls.VersionTLS12,
				MaxVersion:         tls.VersionTLS13,
				InsecureSkipVerify: config.SkipTLSVerify, //nolint:gosec
			}); err != nil {
				return fail(FailEnableStartTLS, err)
			}
		}
	}

	if auth, err := service.getAuth(config); err != nil {
		return err
	} else if auth != nil {
		if err := client.Auth(auth); err != nil {
			return fail(FailAuthenticating, err)
		}
	}

	var errs []error

	for _, toAddress := range config.ToAddresses {
		if err := service.sendToRecipient(client, toAddress, config, message); err != nil {
			errs = append(errs, fail(FailSendRecipient, err, toAddress))
			service.Logf("Failed to send to %q: %v", toAddress, err)

			continue
		}

		service.Logf("Mail successfully sent to %q!", toAddress)
	}

	// Send the QUIT command and close the connection.
	if err := client.Quit(); err != nil {
		// Ignore known "short response" errors from quirky servers (e.g., Office 365 on close),
		// as they don't impact delivery.
		if strings.Contains(err.Error(), shortResponseErrorSubstring) {
			service.Logf("Warning: Ignoring session closure error (delivery succeeded): %v", err)
		} else {
			// Bubble up other close errors (e.g., network drops)
			errs = append(errs, fail(FailClosingSession, err))
		}
	}

	// Best-effort cleanup to avoid descriptor leaks
	if closeErr := client.Close(); closeErr != nil {
		service.Logf("Warning: Failed to close SMTP client connection: %v", closeErr)
	}

	if len(errs) > 0 {
		return failures.Wrap(
			"failed to send to some recipients",
			FailSendRecipient,
			errors.Join(errs...),
		)
	}

	return nil
}

// resolveClientHost determines the client hostname to use in the SMTP handshake.
func (service *Service) resolveClientHost(config *Config) string {
	if config.ClientHost != "auto" {
		return config.ClientHost
	}

	hostname, err := os.Hostname()
	if err != nil {
		service.Logf("Failed to get hostname, falling back to localhost: %v", err)

		return "localhost"
	}

	return hostname
}

// getAuth returns the appropriate SMTP authentication mechanism based on the configuration.
//
//nolint:exhaustive,nilnil
func (service *Service) getAuth(config *Config) (smtp.Auth, failure) {
	switch config.Auth {
	case AuthTypes.None:
		return nil, nil // No auth required, proceed without error
	case AuthTypes.Plain:
		return smtp.PlainAuth("", config.Username, config.Password, config.Host), nil
	case AuthTypes.CRAMMD5:
		return smtp.CRAMMD5Auth(config.Username, config.Password), nil
	case AuthTypes.OAuth2:
		return OAuth2Auth(config.Username, config.Password), nil
	case AuthTypes.Unknown:
		return nil, fail(FailAuthType, nil, config.Auth.String())
	default:
		return nil, fail(FailAuthType, nil, config.Auth.String())
	}
}

// sendToRecipient sends an email to a single recipient using the provided SMTP client.
func (service *Service) sendToRecipient(
	client *smtp.Client,
	toAddress string,
	config *Config,
	message string,
) failure {
	// Set the sender and recipient first
	if err := client.Mail(config.FromAddress); err != nil {
		return fail(FailSetSender, err)
	}

	if err := client.Rcpt(toAddress); err != nil {
		return fail(FailSetRecipient, err)
	}

	// Send the email body.
	writeCloser, err := client.Data()
	if err != nil {
		return fail(FailOpenDataStream, err)
	}

	if err := writeHeaders(writeCloser, service.getHeaders(toAddress, config.Subject)); err != nil {
		return err
	}

	var ferr failure
	if config.UseHTML {
		ferr = service.writeMultipartMessage(writeCloser, message)
	} else {
		ferr = service.writeMessagePart(writeCloser, message, "plain")
	}

	if ferr != nil {
		return ferr
	}

	if err = writeCloser.Close(); err != nil {
		return fail(FailCloseDataStream, err)
	}

	return nil
}

// getHeaders constructs email headers for the SMTP message.
func (service *Service) getHeaders(toAddress string, subject string) map[string]string {
	conf := service.Config

	var contentType string
	if conf.UseHTML {
		contentType = fmt.Sprintf(contentMultipart, service.multipartBoundary)
	} else {
		contentType = contentPlain
	}

	return map[string]string{
		"Subject":      subject,
		"Date":         time.Now().Format(time.RFC1123Z),
		"To":           toAddress,
		"From":         fmt.Sprintf("%s <%s>", conf.FromName, conf.FromAddress),
		"MIME-version": "1.0",
		"Content-Type": contentType,
	}
}

// writeMultipartMessage writes a multipart email message to the provided writer.
func (service *Service) writeMultipartMessage(writeCloser io.WriteCloser, message string) failure {
	if err := writeMultipartHeader(
		writeCloser,
		service.multipartBoundary,
		contentPlain,
	); err != nil {
		return fail(FailPlainHeader, err)
	}

	if err := service.writeMessagePart(writeCloser, message, "plain"); err != nil {
		return err
	}

	if err := writeMultipartHeader(
		writeCloser,
		service.multipartBoundary,
		contentHTML,
	); err != nil {
		return fail(FailHTMLHeader, err)
	}

	if err := service.writeMessagePart(writeCloser, message, "HTML"); err != nil {
		return err
	}

	if err := writeMultipartHeader(writeCloser, service.multipartBoundary, ""); err != nil {
		return fail(FailMultiEndHeader, err)
	}

	return nil
}

// writeMessagePart writes a single part of an email message using the specified template.
func (service *Service) writeMessagePart(
	writeCloser io.WriteCloser,
	message string,
	template string,
) failure {
	if tpl, found := service.GetTemplate(template); found {
		data := make(map[string]string)

		data["message"] = message
		if err := tpl.Execute(writeCloser, data); err != nil {
			return fail(FailMessageTemplate, err)
		}
	} else {
		content := message
		if template == "HTML" {
			content = fmt.Sprintf("<pre>%s</pre>", message)
		}

		if _, err := fmt.Fprint(writeCloser, content); err != nil {
			return fail(FailMessageRaw, err)
		}
	}

	return nil
}

// writeMultipartHeader writes a multipart boundary header to the provided writer.
func writeMultipartHeader(writeCloser io.WriteCloser, boundary string, contentType string) error {
	suffix := "\n"
	if len(contentType) < 1 {
		suffix = "--"
	}

	if _, err := fmt.Fprintf(writeCloser, "\n\n--%s%s", boundary, suffix); err != nil {
		return fmt.Errorf("writing multipart boundary: %w", err)
	}

	if len(contentType) > 0 {
		if _, err := fmt.Fprintf(writeCloser, "Content-Type: %s\n\n", contentType); err != nil {
			return fmt.Errorf("writing content type header: %w", err)
		}
	}

	return nil
}

// writeHeaders writes email headers to the provided writer.
func writeHeaders(writeCloser io.WriteCloser, headers map[string]string) failure {
	for key, val := range headers {
		if _, err := fmt.Fprintf(writeCloser, "%s: %s\n", key, val); err != nil {
			return fail(FailWriteHeaders, err)
		}
	}

	_, err := fmt.Fprintln(writeCloser)
	if err != nil {
		return fail(FailWriteHeaders, err)
	}

	return nil
}
