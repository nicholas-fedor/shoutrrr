// Package smtp provides a service for sending email notifications via the Simple Mail Transfer Protocol (SMTP).
// It is part of the shoutrrr notification framework and supports sending notifications to email recipients with configurable
// authentication, encryption, and message formatting options.
//
// The package supports the following features:
//   - Authentication methods: None, Plain, CRAM-MD5, and OAuth2.
//   - Encryption methods: None, ExplicitTLS (using STARTTLS), ImplicitTLS (TLS for the entire session), and Auto (port-based selection).
//   - Message formats: Plain text or HTML with multipart/alternative support.
//   - Configuration via a URL scheme (e.g., `smtp://user:password@host:port/?fromAddress=sender@example.com&toAddresses=recipient@example.com`).
//   - Integration with the shoutrrr framework for extensible notification delivery.
//
// # Usage
//
// To use the SMTP service, you must initialize a [Service] instance with a valid configuration URL and a logger.
// The configuration URL specifies the SMTP server details, authentication credentials, and email parameters.
// Below is an example of how to initialize and send an email notification:
//
//	package main
//
//	import (
//		"log"
//		"net/url"
//		"github.com/nicholas-fedor/shoutrrr/pkg/services/email/smtp"
//		"github.com/nicholas-fedor/shoutrrr/pkg/types"
//	)
//
//	func main() {
//		logger := log.New(log.Writer(), "smtp: ", log.LstdFlags)
//		service := &smtp.Service{}
//
//		configURL, err := url.Parse("smtp://user:password@example.com:587/?fromAddress=sender@example.com&toAddresses=recipient@example.com&subject=Test%20Notification&useStartTLS=yes&useHTML=no")
//		if err != nil {
//			log.Fatalf("Failed to parse URL: %v", err)
//		}
//
//		err = service.Initialize(configURL, logger)
//		if err != nil {
//			log.Fatalf("Failed to initialize service: %v", err)
//		}
//
//		err = service.Send("This is a test notification.", nil)
//		if err != nil {
//			log.Fatalf("Failed to send notification: %v", err)
//		}
//		log.Println("Notification sent successfully!")
//	}
//
// # Configuration
//
// The [Config] struct defines the parameters for the SMTP service, which can be set via a URL or programmatically.
// Key configuration fields include:
//   - Host: The SMTP server hostname or IP address.
//   - Port: The SMTP server port (e.g., 25, 465, 587, or 2525).
//   - Username and Password: Credentials for authentication (if required).
//   - FromAddress and FromName: The sender's email address and display name.
//   - ToAddresses: A list of recipient email addresses.
//   - Subject: The email subject (defaults to "Shoutrrr Notification").
//   - Auth: The authentication method (None, Plain, CRAMMD5, OAuth2, or Unknown).
//   - Encryption: The encryption method (None, ExplicitTLS, ImplicitTLS, or Auto).
//   - UseStartTLS: Whether to use STARTTLS for encryption (default: true).
//   - UseHTML: Whether to send the message as HTML (default: false).
//   - ClientHost: The client hostname used in the SMTP handshake (default: "localhost").
//   - RequireStartTLS: Whether to fail if StartTLS is enabled but unsupported (default: false).
//   - Timeout: Duration for SMTP operations timeout (default: 10 seconds).
//
// The configuration URL follows the format:
//
//	`smtp://<username>:<password>@<host>:<port>/?fromAddress=<email>&toAddresses=<email1>,<email2>&subject=<subject>&auth=<auth>&encryption=<encryption>&useStartTLS=<yes/no>&useHTML=<yes/no>&clientHost=<hostname>&requirestarttls=<yes/no>&timeout=<duration>`
//
// Example URL:
//
//	`smtp://user:pass@example.com:587/?fromAddress=sender@example.com&toAddresses=rec1@example.com,rec2@example.com&subject=Alert&auth=Plain&encryption=Auto&useStartTLS=yes&useHTML=yes&clientHost=localhost&requirestarttls=yes&timeout=10s`
//
// # Error Handling
//
// The package defines a set of failure identifiers in `smtp_failures.go` (e.g., [FailGetSMTPClient], [FailAuthenticating], [FailSendRecipient])
// to categorize errors that may occur during SMTP operations. These are wrapped using the [failures.Failure] interface
// from the shoutrrr framework, providing detailed error messages and IDs for debugging.
//
// # Authentication
//
// The package supports multiple authentication methods, defined in [authType] and [AuthTypes]:
//   - None: No authentication.
//   - Plain: Username and password-based authentication.
//   - CRAMMD5: Challenge-response authentication using CRAM-MD5.
//   - OAuth2: Token-based authentication for services like Gmail (see [OAuth2Auth]).
//     Note that OAuth2 support is limited to static access tokens and does not
//     handle token refresh or complex challenge-response flows.
//   - Unknown: Fallback when the authentication method is not specified or invalid.
//
// # Encryption
//
// Encryption is configured via the [encMethod] type and [EncMethods] helper, supporting:
//   - None: No encryption.
//   - ExplicitTLS: Uses STARTTLS to initiate a secure connection.
//   - ImplicitTLS: Uses TLS for the entire session (typically on port 465).
//   - Auto: Automatically selects ImplicitTLS for port 465, otherwise attempts ExplicitTLS if supported.
//
// The [useImplicitTLS] function determines whether ImplicitTLS should be used based on the encryption method and port.
//
// # Testing
//
// The `smtp_test.go` file includes unit and integration tests using the Ginkgo and Gomega frameworks.
// Tests cover configuration parsing, email sending, error handling, and integration scenarios with mocked SMTP server responses.
// The [testIntegration] and [testSendRecipient] functions simulate SMTP server interactions for testing purposes.
//
// # Notes
//
// - The package uses the standard Go `net/smtp` library for SMTP operations.
// - It supports multipart email messages (plain text and HTML) using a randomly generated boundary.
// - The [Service] struct implements the shoutrrr [standard.Standard] and [standard.Templater] interfaces for logging and message templating.
// - The package handles plus signs (`+`) in email addresses correctly, replacing spaces with plus signs as needed (see [Config.FixEmailTags]).
// - For OAuth2 authentication, the [OAuth2Auth] function implements the SASL XOAUTH2 protocol, suitable for services like Gmail.
package smtp
