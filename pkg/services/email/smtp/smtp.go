package smtp

import (
	"context"
	"net/url"
	"sync"
	"time"

	"golang.org/x/oauth2"

	"github.com/nicholas-fedor/shoutrrr/internal/httpclient"
	"github.com/nicholas-fedor/shoutrrr/internal/logging"
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
	// dialContext, if non-nil, is used for the SMTP TCP dial.
	dialContext types.DialContextFunc
	// tokens caches the OAuth2 access token of each set of refresh settings.
	tokens map[tokenKey]*oauth2.Token
	// refreshes holds the token request in flight for each set of refresh
	// settings, which concurrent sends with those settings wait for.
	refreshes map[tokenKey]*tokenRefresh
	// warnings logs each warning about the service's settings once.
	warnings logging.Once
	// propKeyResolver applies URL and send-parameter overrides to [Config].
	propKeyResolver format.PropKeyResolver
	// client holds the injected HTTP client and the default client for OAuth2
	// token requests.
	client httpclient.Holder
	// tokenMu guards tokens and refreshes. It is not held during token
	// requests.
	tokenMu sync.Mutex
}

// Service defaults.
const (
	// DefaultSMTPPort is the standard port for SMTP communication.
	DefaultSMTPPort = 25
	// defaultTimeout is the default SMTP timeout.
	defaultTimeout = 10 * time.Second
)

// Compile-time checks that Service implements the interfaces the router relies on.
var (
	_ types.Service           = (*Service)(nil)
	_ types.DialContextSetter = (*Service)(nil)
	_ types.HTTPClientSetter  = (*Service)(nil)
	_ types.ContextSender     = (*Service)(nil)
	_ types.ServiceTimeout    = (*Service)(nil)
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
// It applies SMTP defaults, parses the configuration URL, and infers Auth when
// it is [AuthUnknown]: [AuthOAuth2] when a refresh token is set, [AuthPlain]
// when a username is present, and [AuthNone] otherwise. It then checks the
// OAuth2 refresh settings.
//
// Parameters:
//   - serviceURL: The SMTP configuration URL.
//   - logger: Logger used for service output ([types.StdLogger]).
//
// Returns:
//   - An error if the configuration URL is invalid, or [ErrInvalidOAuth2Refresh]
//     for invalid refresh settings.
func (s *Service) Initialize(serviceURL *url.URL, logger types.StdLogger) error {
	s.SetLogger(logger)
	s.warnings.Reset()
	// The default client has no timeout of its own, because the send's
	// timeout bounds each token request.
	s.client.SetDefault(0, nil)
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
		switch {
		case s.Config.usesTokenRefresh():
			s.Config.Auth = AuthTypes.OAuth2
		case s.Config.Username != "":
			s.Config.Auth = AuthTypes.Plain
		default:
			s.Config.Auth = AuthTypes.None
		}
	}

	if err := validateOAuth2Refresh(s.Config); err != nil {
		return err
	}

	s.propKeyResolver = pkr

	return nil
}

// Send sends a notification message to email recipients.
//
// It delegates to [Service.SendContext] with [context.Background].
//
// Parameters:
//   - message: The notification body sent as the email message.
//   - params: Optional runtime overrides for configuration fields ([types.Params]).
//
// Returns:
//   - An error if configuration updates, connection, or delivery fail.
func (s *Service) Send(message string, params *types.Params) error {
	return s.SendContext(context.Background(), message, params)
}

// SendContext sends a notification message to email recipients.
//
// It clones the service configuration, applies optional runtime params, opens
// an SMTP client, and delivers the message. A timeout from [Config.Timeout]
// bounds the connection and session, derived from ctx so caller cancellation
// and deadlines propagate. Non-positive timeouts use [defaultTimeout].
//
// With a refresh token, a current OAuth2 access token is obtained before the
// connection opens, within the same timeout, and used in place of the
// password. A token the server rejects is dropped so the next send refreshes.
//
// Parameters:
//   - ctx: Parent context for cancellation and deadlines.
//   - message: The notification body sent as the email message.
//   - params: Optional runtime overrides for configuration fields ([types.Params]).
//
// Returns:
//   - An error if configuration updates, connection, or delivery fail, a
//     [FailTokenRefresh] failure when no access token could be obtained, or
//     [ErrInvalidOAuth2Refresh] for invalid refresh settings in params.
func (s *Service) SendContext(ctx context.Context, message string, params *types.Params) error {
	config, err := s.sendConfig(params)
	if err != nil {
		return fail(FailApplySendParams, err)
	}

	if err := validateOAuth2Refresh(&config); err != nil {
		return fail(FailApplySendParams, err)
	}

	ctx, cancel := httpclient.BoundContext(ctx, effectiveTimeout(config.Timeout))
	defer cancel()

	if config.usesTokenRefresh() {
		accessToken, err := s.accessToken(ctx, &config)
		if err != nil {
			return err
		}

		config.Password = accessToken
	}

	client, err := dialClient(ctx, &config, s.dialContext)
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

// ServiceTimeout reports the SMTP session timeout.
//
// Params are applied to a clone of the stored configuration. A param that
// does not apply leaves the configured timeout in place.
//
// Parameters:
//   - params: Optional runtime overrides for configuration fields ([types.Params]).
//
// Returns:
//   - The positive session timeout. Non-positive configured values use [defaultTimeout].
func (s *Service) ServiceTimeout(params *types.Params) time.Duration {
	config, err := s.sendConfig(params)
	if err != nil {
		return effectiveTimeout(s.Config.Timeout)
	}

	return effectiveTimeout(config.Timeout)
}

// SetDialContext sets a custom dial function for SMTP TCP connections.
//
// TLS wrapping for implicit TLS still happens after the TCP dial.
// A nil dial restores the default [net.Dialer].
//
// Parameters:
//   - dial: The dial function. Must be safe for concurrent use when non-nil.
func (s *Service) SetDialContext(dial types.DialContextFunc) {
	s.dialContext = dial
}

// sendConfig returns the configuration for one send.
//
// The stored configuration is cloned before params are applied.
//
// Parameters:
//   - params: Optional runtime overrides for configuration fields ([types.Params]).
//
// Returns:
//   - The cloned configuration.
//   - An error if a param does not apply.
func (s *Service) sendConfig(params *types.Params) (Config, error) {
	config := s.Config.Clone()
	if err := s.propKeyResolver.UpdateConfigFromParams(&config, params); err != nil {
		return Config{}, err
	}

	return config, nil
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
