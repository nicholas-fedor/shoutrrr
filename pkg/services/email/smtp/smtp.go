package smtp

import (
	"context"
	"errors"
	"fmt"
	"net/url"
	"os"
	"sync"
	"sync/atomic"
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
	// tokens caches the OAuth2 access token of each set of token settings.
	tokens map[tokenKey]*oauth2.Token
	// refreshes holds the token request in flight for each set of token
	// settings, which concurrent sends with those settings wait for.
	refreshes map[tokenKey]*tokenRefresh
	// mailboxes holds the addresses the last send parsed, which a send with
	// the same address settings uses again.
	mailboxes atomic.Pointer[mailboxes]
	// client holds the injected HTTP client and the default client for OAuth2
	// token requests.
	client httpclient.Holder
	// warnings logs each warning about the service's settings once.
	warnings logging.Once
	// propKeyResolver applies URL and send-parameter overrides to [Config].
	propKeyResolver format.PropKeyResolver
	// tokenMu guards tokens and refreshes. It is not held during token
	// requests.
	tokenMu sync.Mutex
}

// Service defaults.
const (
	// DefaultSMTPPort is the standard port for SMTP communication.
	DefaultSMTPPort = 25
	// defaultClientHost is the hostname sent in the EHLO handshake when the
	// config names none.
	defaultClientHost = "localhost"
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
// It starts from the SMTP defaults and reads the configuration URL. Reading the
// URL infers an [AuthUnknown] method and checks the result, including the
// OAuth2 token settings.
//
// Parameters:
//   - serviceURL: The SMTP configuration URL.
//   - logger: Logger used for service output ([types.StdLogger]).
//
// Returns:
//   - An error if the configuration URL is invalid, or [ErrInvalidOAuth2Settings]
//     for invalid token settings.
func (s *Service) Initialize(serviceURL *url.URL, logger types.StdLogger) error {
	s.SetLogger(logger)
	s.warnings.Reset()
	// The zero values of Auth and Encryption are the defaults a URL without
	// their keys gets, but STARTTLS is on unless the URL turns it off.
	s.Config = &Config{
		ToAddresses: nil,
		Subject:     "",
		UseStartTLS: true,
		UseHTML:     false,
		Timeout:     defaultTimeout,
	}

	pkr := format.NewPropKeyResolver(s.Config)

	if err := s.Config.setURL(&pkr, serviceURL); err != nil {
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
//   - error: [types.ErrServiceNotInitialized] before Initialize, or the
//     params, connection, or delivery error.
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
// With a refresh token or the client credentials grant, a current OAuth2
// access token is obtained before the connection opens, within the same
// timeout, and used in place of the password. A token the server rejects is
// dropped so the next send gets a new one.
//
// Parameters:
//   - ctx: Parent context for cancellation and deadlines.
//   - message: The notification body sent as the email message.
//   - params: Optional runtime overrides for configuration fields ([types.Params]).
//
// Returns:
//   - error: [types.ErrServiceNotInitialized] before Initialize. A param that
//     does not apply or a config that fails its checks returns a
//     [FailApplySendParams] failure, such as one for [ErrFromAddressMissing]
//     or [ErrInvalidOAuth2Settings]. A failed connection or delivery returns
//     its error. A send that obtains no access token returns a
//     [FailGetAccessToken] failure. When ctx ends the session, the error also
//     matches ctx's error.
func (s *Service) SendContext(ctx context.Context, message string, params *types.Params) error {
	if s.Config == nil {
		return types.ErrServiceNotInitialized
	}

	config, err := s.sendConfig(params)
	if err != nil {
		return fail(FailApplySendParams, err)
	}

	// The checks run before the token request and the dial, so an invalid
	// config makes no connection.
	previous := s.mailboxes.Load()

	parsed, err := config.prepare(previous)
	if err != nil {
		return fail(FailApplySendParams, err)
	}

	// A send with unchanged addresses gets previous back, which needs no store.
	if parsed != previous {
		s.mailboxes.Store(parsed)
	}

	ctx, cancel := httpclient.BoundContext(ctx, effectiveTimeout(config.Timeout))
	defer cancel()

	if config.grant() != grantNone {
		accessToken, err := s.accessToken(ctx, &config)
		if err != nil {
			return err
		}

		config.Password = accessToken
	}

	client, stopWatch, err := dialClient(ctx, &config, s.dialContext)
	if err != nil {
		return withContextErr(ctx, fail(FailGetSMTPClient, err))
	}

	defer stopWatch()

	return withContextErr(ctx, (&session{
		client:    client,
		config:    &config,
		mailboxes: parsed,
		svc:       s,
		closed:    false,
	}).run(ctx, message))
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
	if s.Config == nil {
		return defaultTimeout
	}

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
// Non-positive values fall back to 10 seconds, so [context.WithTimeout]
// always bounds the connection and session.
//
// Parameters:
//   - timeout: The configured session timeout.
//
// Returns:
//   - time.Duration: timeout when it is positive, or 10 seconds otherwise.
func effectiveTimeout(timeout time.Duration) time.Duration {
	if timeout <= 0 {
		return defaultTimeout
	}

	return timeout
}

// withContextErr adds ctx's error to a session's error when ctx ended the
// session.
//
// The connection then fails with a network timeout, which does not match
// ctx's error by itself. Any other error keeps its own cause, even when
// ctx ended at the same moment. The connection's deadline is ctx's deadline.
// Once that deadline has passed, ctx counts as ended even before its own timer
// has fired.
//
// Parameters:
//   - ctx: the send's context.
//   - err: the session's error, or nil.
//
// Returns:
//   - error: err, wrapped with ctx's error when err is a network timeout and
//     ctx ended, or nil when err is nil.
func withContextErr(ctx context.Context, err error) error {
	if err == nil || !errors.Is(err, os.ErrDeadlineExceeded) {
		return err
	}

	ctxErr := ctx.Err()
	if deadline, ok := ctx.Deadline(); ctxErr == nil && ok && !time.Now().Before(deadline) {
		ctxErr = context.DeadlineExceeded
	}

	if ctxErr == nil || errors.Is(err, ctxErr) {
		return err
	}

	return fmt.Errorf("%w: %w", err, ctxErr)
}
