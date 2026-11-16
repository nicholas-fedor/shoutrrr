package xmpp

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net"
	"net/url"
	"strconv"
	"time"

	"github.com/nicholas-fedor/shoutrrr/internal/httpclient"
	"github.com/nicholas-fedor/shoutrrr/internal/logging"
	"github.com/nicholas-fedor/shoutrrr/internal/sequential"
	"github.com/nicholas-fedor/shoutrrr/pkg/format"
	"github.com/nicholas-fedor/shoutrrr/pkg/services/standard"
	"github.com/nicholas-fedor/shoutrrr/pkg/types"
	"github.com/nicholas-fedor/shoutrrr/pkg/util"
)

// Service sends fire-and-forget XMPP chat and MUC notifications.
type Service struct {
	// Standard provides logging and the other shared service functionality.
	standard.Standard

	// Config holds the parsed XMPP service settings.
	Config *Config
	// dialContext, when set, is used instead of [net.Dialer] for the TCP dial.
	dialContext types.DialContextFunc
	// resolver looks up SRV records through dialContext when one is set. Nil
	// uses the system resolver. Concurrent sends share its lookups.
	resolver *net.Resolver
	// newSession constructs the XMPP session after the TCP dial succeeds.
	newSession SessionFactory
	// warnings logs each warning about the service's settings once.
	warnings logging.Once
	// pkr applies URL query keys and Send params onto Config.
	pkr format.PropKeyResolver
}

// Time limits of an XMPP send.
const (
	// defaultTimeout is the send budget, which bounds dial, session
	// negotiation, and send when the caller's context has no deadline.
	defaultTimeout = 30 * time.Second
	// srvAttemptTimeout caps the dial to each server an SRV lookup names but
	// the last, so a server that does not answer leaves time for the next.
	srvAttemptTimeout = 10 * time.Second
	// srvLookupTimeout caps the SRV lookup, so a DNS server that does not
	// answer leaves time to dial the host.
	srvLookupTimeout = 10 * time.Second
)

// Compile-time checks that Service implements the interfaces the router relies on.
var (
	_ types.Service           = (*Service)(nil)
	_ types.DialContextSetter = (*Service)(nil)
	_ types.ContextSender     = (*Service)(nil)
	_ types.ServiceTimeout    = (*Service)(nil)
)

// GetID returns the service identifier.
//
// Returns:
//   - string: the scheme name [Scheme].
func (*Service) GetID() string {
	return Scheme
}

// Initialize loads [Config] from serviceURL and sets logger for this [Service].
//
// Parameters:
//   - serviceURL: The XMPP configuration URL.
//   - logger: Logger used for service output ([types.StdLogger]).
//
// Returns:
//   - error: an error if the configuration URL is invalid.
func (s *Service) Initialize(serviceURL *url.URL, logger types.StdLogger) error {
	s.SetLogger(logger)
	s.Config = &Config{}
	s.pkr = format.NewPropKeyResolver(s.Config)
	s.warnings.Reset()

	if err := s.pkr.SetDefaultProps(s.Config); err != nil {
		return fmt.Errorf("setting default props: %w", err)
	}

	if err := s.Config.setURL(&s.pkr, serviceURL); err != nil {
		return err
	}

	if s.newSession == nil {
		s.newSession = newMelliumSession
	}

	return nil
}

// Send sends a notification message over XMPP.
//
// Parameters:
//   - message: The notification body.
//   - params: Optional runtime overrides for configuration fields ([types.Params]).
//
// Returns:
//   - error: [types.ErrServiceNotInitialized] before Initialize, or the
//     params, connection, or delivery error.
func (s *Service) Send(message string, params *types.Params) error {
	return s.SendContext(context.Background(), message, params)
}

// SendContext sends a notification message over XMPP.
//
// It clones the service configuration, applies optional runtime params, dials
// the server, negotiates a session, and sends chat and MUC messages. Delivery
// continues after a per-target failure. The session is closed after the send.
// A context without a deadline is bounded by the budget that
// [Service.ServiceTimeout] reports. The first send without TLS, or without
// certificate verification, logs a warning.
//
// Parameters:
//   - ctx: Parent context for cancellation and deadlines.
//   - message: The notification body.
//   - params: Optional runtime overrides for configuration fields ([types.Params]).
//
// Returns:
//   - error: [types.ErrServiceNotInitialized] before Initialize, a setup
//     error, or the joined errors of the targets that failed. Once ctx ends,
//     the remaining targets are skipped and ctx's error is reported once.
func (s *Service) SendContext(ctx context.Context, message string, params *types.Params) error {
	if s.Config == nil {
		return types.ErrServiceNotInitialized
	}

	config := s.Config.Clone()
	if err := s.pkr.UpdateConfigFromParams(&config, params); err != nil {
		return fmt.Errorf("updating config from params: %w", err)
	}

	if err := config.prepare(); err != nil {
		return err
	}

	switch {
	case config.DisableTLS:
		s.warnings.Plaintext(ctx, s)
	case config.SkipTLSVerify:
		s.warnings.Insecure(ctx, s)
	default:
	}

	ctx, cancel := httpclient.BoundContext(ctx, s.ServiceTimeout(params))
	defer cancel()

	conn, err := s.dial(ctx, &config)
	if err != nil {
		return err
	}

	session, err := s.newSession(ctx, &config, conn)
	if err != nil {
		_ = conn.Close()

		return err
	}
	defer func() { _ = session.Close() }()

	body := util.TitledMessage(config.Title, message)

	nick := config.mucNick()

	// The chats and then the rooms make one list, so a context that ends
	// partway is reported once.
	sends := make([]func(context.Context) error, 0, len(config.To)+len(config.Rooms))

	for _, to := range config.To {
		sends = append(sends, func(ctx context.Context) error {
			if err := session.SendChat(ctx, to, body); err != nil {
				return fmt.Errorf("sending chat to %s: %w", to, err)
			}

			return nil
		})
	}

	for _, room := range config.Rooms {
		sends = append(sends, func(ctx context.Context) error {
			if err := session.SendToRoom(ctx, room, nick, config.RoomPassword, body); err != nil {
				return fmt.Errorf("sending groupchat to %s: %w", room, err)
			}

			return nil
		})
	}

	return sequential.Send(
		ctx,
		sends,
		sequential.SharedDeadline,
		func(ctx context.Context, send func(context.Context) error) error {
			return send(ctx)
		},
	)
}

// ServiceTimeout returns the session timeout used for an XMPP send.
//
// The budget is [defaultTimeout], which [Service.SendContext] applies to dial,
// negotiation, and send when the caller's context has no deadline.
//
// Parameters:
//   - params: Unused. The budget does not depend on send parameters.
//
// Returns:
//   - time.Duration: [defaultTimeout].
func (*Service) ServiceTimeout(*types.Params) time.Duration {
	return defaultTimeout
}

// SetDialContext sets a custom dial function for XMPP TCP connections.
//
// TLS wrapping for xmpps and STARTTLS still happens after the TCP dial. The
// DNS queries of an SRV lookup go through the dial function too, so a dial
// function that refuses them makes the send dial the host itself. A nil dial
// restores the default [net.Dialer] and the system resolver.
//
// Parameters:
//   - dial: The dial function. Must be safe for concurrent use when non-nil.
func (s *Service) SetDialContext(dial types.DialContextFunc) {
	s.dialContext = dial
	s.resolver = nil

	if dial != nil {
		s.resolver = &net.Resolver{PreferGo: true, StrictErrors: false, Dial: dial}
	}
}

// SetSessionFactory replaces the XMPP session constructor.
//
// Tests use this to inject a mock session. A nil factory restores the default.
//
// Parameters:
//   - factory: The session factory. Must be safe for concurrent use when non-nil.
func (s *Service) SetSessionFactory(factory SessionFactory) {
	if factory == nil {
		s.newSession = newMelliumSession

		return
	}

	s.newSession = factory
}

// dial opens a TCP connection to the XMPP server of cfg.
//
// It dials the addresses that [Service.serverAddresses] gives, in order, and
// stops at the first that connects. Each address but the last gets at most
// 10 seconds.
//
// Parameters:
//   - ctx: Parent context for the lookup and the dials.
//   - cfg: Configuration supplying the server.
//
// Returns:
//   - net.Conn: the connected TCP socket.
//   - error: [ErrServiceNotOffered], or the joined errors of the dials, each
//     naming its address.
func (s *Service) dial(ctx context.Context, cfg *Config) (net.Conn, error) {
	addrs, err := s.serverAddresses(ctx, cfg)
	if err != nil {
		return nil, err
	}

	dial := s.dialContext
	if dial == nil {
		var d net.Dialer

		dial = d.DialContext
	}

	var errs []error

	for i, addr := range addrs {
		attemptCtx, cancel := ctx, context.CancelFunc(func() {})
		if i < len(addrs)-1 {
			attemptCtx, cancel = context.WithTimeout(ctx, srvAttemptTimeout)
		}

		conn, err := dial(attemptCtx, "tcp", addr)

		cancel()

		if err == nil {
			return conn, nil
		}

		errs = append(errs, fmt.Errorf("dialing %s: %w", addr, err))

		if ctx.Err() != nil {
			break
		}
	}

	return nil, errors.Join(errs...)
}

// serverAddresses returns the addresses of the XMPP server of cfg, in the
// order to dial them.
//
// A config that does not look the server up through SRV records gives its
// host address. Otherwise the SRV records of the host give the addresses, in
// the order of their priority and weight. Without records, the host address
// is the only one, as RFC 6120 section 3.2.2 falls back to. Once the domain
// has records, the host itself is not tried, as section 3.2.1 asks. The
// lookup gets at most 10 seconds, so the dial always has time left.
//
// The DNS queries go through the injected dial function when one is set, and
// through the system resolver otherwise.
//
// Parameters:
//   - ctx: bounds the lookup.
//   - cfg: Configuration supplying the server.
//
// Returns:
//   - []string: the addresses, each a host and port.
//   - error: [ErrServiceNotOffered] when every record's target is ".".
func (s *Service) serverAddresses(ctx context.Context, cfg *Config) ([]string, error) {
	hostAddr := cfg.hostAddress()

	if !cfg.usesSRV() {
		return []string{hostAddr}, nil
	}

	lookupCtx, cancel := context.WithTimeout(ctx, srvLookupTimeout)
	defer cancel()

	// The name is queried in ASCII form, and its trailing dot roots it, so the
	// resolver does not try the search domains first. Valid records that come
	// back beside an error for malformed ones are still used. A nil resolver
	// is the zero resolver, which is the system's.
	_, records, err := s.resolver.LookupSRV(lookupCtx, cfg.srvService(), "tcp", asciiDomain(cfg.Host)+".")
	if len(records) == 0 {
		s.LogAttrs(ctx, slog.LevelDebug, "no SRV records, dialing the host",
			slog.String(logging.KeyServer, hostAddr), slog.Any(logging.KeyError, err))

		return []string{hostAddr}, nil
	}

	addrs := make([]string, 0, len(records))

	for _, record := range records {
		// A target of "." says the service is not offered there. Each target
		// keeps its trailing dot, so its own lookup skips the search domains
		// too.
		if record.Target != "." {
			addrs = append(addrs, net.JoinHostPort(record.Target, strconv.FormatUint(uint64(record.Port), 10)))
		}
	}

	if len(addrs) == 0 {
		return nil, fmt.Errorf("%w: %s", ErrServiceNotOffered, cfg.Host)
	}

	return addrs, nil
}
