package xmpp

import (
	"context"
	"fmt"
	"net"
	"net/url"
	"strconv"
	"time"

	"github.com/nicholas-fedor/shoutrrr/pkg/format"
	"github.com/nicholas-fedor/shoutrrr/pkg/services/standard"
	"github.com/nicholas-fedor/shoutrrr/pkg/types"
)

// Service sends fire-and-forget XMPP chat and MUC notifications.
type Service struct {
	standard.Standard

	// Config holds the parsed XMPP service settings.
	Config *Config
	// pkr applies URL query keys and Send params onto Config.
	pkr format.PropKeyResolver
	// dialContext, when set, is used instead of [net.Dialer] for the TCP dial.
	dialContext types.DialContextFunc
	// newSession constructs the XMPP session after the TCP dial succeeds.
	newSession SessionFactory
}

// defaultTimeout bounds dial, session negotiation, and send.
const defaultTimeout = 30 * time.Second

var (
	_ types.Service           = (*Service)(nil)
	_ types.ContextSender     = (*Service)(nil)
	_ types.DialContextSetter = (*Service)(nil)
)

// GetID returns the service identifier.
//
// Returns:
//   - The scheme name [Scheme].
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
//   - An error if the configuration URL is invalid.
func (s *Service) Initialize(serviceURL *url.URL, logger types.StdLogger) error {
	s.SetLogger(logger)
	s.Config = &Config{}
	s.pkr = format.NewPropKeyResolver(s.Config)

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
//   - An error if configuration updates, connection, or delivery fail.
func (s *Service) Send(message string, params *types.Params) error {
	return s.SendContext(context.Background(), message, params)
}

// SendContext sends a notification message over XMPP.
//
// It clones the service configuration, applies optional runtime params, dials
// the server, negotiates a session, and sends chat and MUC messages. The
// session is closed after the send.
//
// Parameters:
//   - ctx: Parent context for cancellation and deadlines.
//   - message: The notification body.
//   - params: Optional runtime overrides for configuration fields ([types.Params]).
//
// Returns:
//   - An error if configuration updates, connection, or delivery fail.
func (s *Service) SendContext(ctx context.Context, message string, params *types.Params) error {
	config := s.Config.Clone()
	if err := s.pkr.UpdateConfigFromParams(&config, params); err != nil {
		return fmt.Errorf("updating config from params: %w", err)
	}

	config.To = compactStrings(config.To)
	config.Rooms = compactStrings(config.Rooms)

	if err := config.validate(); err != nil {
		return err
	}

	if config.SkipTLSVerify {
		s.Log("Warning: TLS verification is disabled, making connections insecure")
	}

	ctx, cancel := context.WithTimeout(ctx, defaultTimeout)
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

	body := composeMessage(config.Title, message)

	for _, to := range config.To {
		if err := session.SendChat(ctx, to, body); err != nil {
			return err
		}
	}

	nick := config.mucNick()
	for _, room := range config.Rooms {
		if err := session.SendToRoom(ctx, room, nick, config.RoomPassword, body); err != nil {
			return err
		}
	}

	return nil
}

// SetDialContext sets a custom dial function for XMPP TCP connections.
//
// TLS wrapping for xmpps and STARTTLS still happens after the TCP dial.
// A nil dial restores the default [net.Dialer].
//
// Parameters:
//   - dial: The dial function. Must be safe for concurrent use when non-nil.
func (s *Service) SetDialContext(dial types.DialContextFunc) {
	s.dialContext = dial
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

// dial opens a TCP connection to cfg.Host:cfg.Port.
//
// Parameters:
//   - ctx: Parent context for the dial.
//   - cfg: Configuration supplying the dial address.
//
// Returns:
//   - The connected TCP socket.
//   - An error if the dial fails.
func (s *Service) dial(ctx context.Context, cfg *Config) (net.Conn, error) {
	addr := net.JoinHostPort(cfg.Host, strconvPort(cfg.Port))

	dial := s.dialContext
	if dial == nil {
		var d net.Dialer

		dial = d.DialContext
	}

	conn, err := dial(ctx, "tcp", addr)
	if err != nil {
		return nil, fmt.Errorf("dialing %s: %w", addr, err)
	}

	return conn, nil
}

// strconvPort formats port as a decimal string for [net.JoinHostPort].
//
// Parameters:
//   - port: The TCP port.
//
// Returns:
//   - The port as a decimal string.
func strconvPort(port uint16) string {
	return strconv.FormatUint(uint64(port), 10)
}
