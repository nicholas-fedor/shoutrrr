package smtp

import (
	"context"
	"net/url"
	"time"

	"github.com/nicholas-fedor/shoutrrr/pkg/format"
	"github.com/nicholas-fedor/shoutrrr/pkg/services/standard"
	"github.com/nicholas-fedor/shoutrrr/pkg/types"
)

// Service sends notifications to given email addresses via SMTP.
type Service struct {
	// Standard provides logging and other base service behavior.
	standard.Standard
	// Templater provides named message templates for plain and HTML parts.
	standard.Templater

	// Config holds SMTP server, authentication, and message options.
	Config *Config
	// propKeyResolver applies URL and send-parameter overrides to [Config].
	propKeyResolver format.PropKeyResolver
}

const (
	// DefaultSMTPPort is the standard port for SMTP communication.
	DefaultSMTPPort = 25
	// defaultTimeout is the default SMTP timeout.
	defaultTimeout = 10 * time.Second
)

// GetID returns the service identifier.
//
// Returns:
//   - The scheme name [Scheme].
func (s *Service) GetID() string {
	return Scheme
}

// Initialize loads [Config] from serviceURL and sets logger for this [Service].
//
// It applies SMTP defaults, parses the configuration URL, and infers Auth as
// [AuthPlain] when a username is present or [AuthNone] otherwise if
// Auth is [AuthUnknown].
//
// Parameters:
//   - serviceURL: The SMTP configuration URL.
//   - logger: Logger used for service output ([types.StdLogger]).
//
// Returns:
//   - An error if the configuration URL is invalid.
func (s *Service) Initialize(serviceURL *url.URL, logger types.StdLogger) error {
	s.SetLogger(logger)
	s.Config = &Config{
		Port:        DefaultSMTPPort,
		ToAddresses: nil,
		Subject:     "",
		Auth:        AuthTypes.Unknown,
		UseStartTLS: true,
		UseHTML:     false,
		Encryption:  EncMethods.Auto,
		ClientHost:  "localhost",
		Timeout:     defaultTimeout,
	}

	pkr := format.NewPropKeyResolver(s.Config)

	if err := s.Config.setURL(&pkr, serviceURL); err != nil {
		return err
	}

	if s.Config.Auth == AuthTypes.Unknown {
		if s.Config.Username != "" {
			s.Config.Auth = AuthTypes.Plain
		} else {
			s.Config.Auth = AuthTypes.None
		}
	}

	s.propKeyResolver = pkr

	return nil
}

// Send sends a notification message to email recipients.
//
// It clones the service configuration, applies optional runtime params, opens
// an SMTP client, and delivers the message. A timeout from [Config.Timeout]
// bounds the connection and session. Non-positive timeouts use [defaultTimeout].
//
// Parameters:
//   - message: The notification body sent as the email message.
//   - params: Optional runtime overrides for configuration fields ([types.Params]).
//
// Returns:
//   - An error if configuration updates, connection, or delivery fail.
func (s *Service) Send(message string, params *types.Params) error {
	config := s.Config.Clone()
	if err := s.propKeyResolver.UpdateConfigFromParams(&config, params); err != nil {
		return fail(FailApplySendParams, err)
	}

	if config.SkipTLSVerify {
		s.Log("Warning: TLS verification is disabled, making connections insecure")
	}

	ctx, cancel := context.WithTimeout(
		context.Background(),
		effectiveTimeout(config.Timeout),
	)
	defer cancel()

	client, err := dialClient(ctx, &config)
	if err != nil {
		return fail(FailGetSMTPClient, err)
	}

	return (&session{
		client: client,
		config: &config,
		svc:    s,
		closed: false,
	}).run(message)
}

// effectiveTimeout returns a positive SMTP timeout.
//
// Non-positive values fall back to [defaultTimeout] so [context.WithTimeout]
// always bounds the connection and session.
//
// Parameters:
//   - timeout: The configured session timeout.
//
// Returns:
//   - timeout when it is positive; otherwise [defaultTimeout].
func effectiveTimeout(timeout time.Duration) time.Duration {
	if timeout <= 0 {
		return defaultTimeout
	}

	return timeout
}
