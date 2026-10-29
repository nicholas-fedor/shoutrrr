package mqtt

import (
	"context"
	"crypto/tls"
	"fmt"
	"io"
	"net"
	"net/url"
	"strconv"
	"sync"
	"time"

	"github.com/eclipse/paho.golang/autopaho"
	"github.com/eclipse/paho.golang/packets"
	"github.com/eclipse/paho.golang/paho"

	"github.com/nicholas-fedor/shoutrrr/internal/httpclient"
	"github.com/nicholas-fedor/shoutrrr/internal/logging"
	"github.com/nicholas-fedor/shoutrrr/internal/transport"
	"github.com/nicholas-fedor/shoutrrr/pkg/format"
	"github.com/nicholas-fedor/shoutrrr/pkg/services/standard"
	"github.com/nicholas-fedor/shoutrrr/pkg/types"
	"github.com/nicholas-fedor/shoutrrr/pkg/util"
)

// Service implements the notification service interface for MQTT brokers.
// It manages the connection lifecycle and message publishing to MQTT topics.
//
// A connection the service creates lives until [Service.Close], or until no send
// has used it for [idleTimeout], and the next send reconnects. A connection
// manager set with [Service.SetConnectionManager] belongs to the caller and is
// never torn down for idleness or replaced.
type Service struct {
	// Standard provides base service functionality including logging.
	standard.Standard

	// connectionManager is the underlying MQTT connection manager for broker communication.
	connectionManager ConnectionManager

	// Config holds the MQTT connection and publishing settings.
	Config *Config
	// dialContext, if non-nil, is used for the MQTT TCP dial via AttemptConnection.
	dialContext types.DialContextFunc
	// connect creates a connection manager. Nil uses [autopaho.NewConnection].
	connect connectFunc
	// cancelConnection ends the lifetime of a connection the service created.
	cancelConnection context.CancelFunc
	// connectionSends tracks the sends using connectionManager, so tearing the
	// connection down waits for them to finish.
	connectionSends *sync.WaitGroup
	// idleTimer tears down a connection the service created once it is idle.
	idleTimer *time.Timer
	// warnings logs each warning about the service's settings once.
	warnings logging.Once
	// pkr resolves property keys for configuration updates from URL parameters.
	pkr format.PropKeyResolver
	// activeSends counts the sends in progress, which keeps the idle timer stopped.
	activeSends int
	// idleGeneration identifies the current idle period, so a timer that fires
	// after a send has started does nothing.
	idleGeneration uint64
	// clientMutex guards the connection state below.
	clientMutex sync.Mutex
	// ownedConnection reports whether the service created connectionManager, so it
	// may replace the connection once it ends and tear it down when idle.
	ownedConnection bool
}

// connectFunc creates a connection manager whose lifetime ends with ctx. The
// service replaces a connection once it ends only when the manager exposes
// Done() <-chan struct{}, as autopaho's does. A manager without Done is reused
// until Close or an idle teardown.
type connectFunc func(ctx context.Context, cfg *autopaho.ClientConfig) (ConnectionManager, error)

// keepAliveInterval defines the interval in seconds between keep-alive packets.
// This maintains the connection and detects dead peers.
const keepAliveInterval = 20

// sessionExpiryInterval defines the session expiry interval in seconds for MQTT v5.
// After this period, the broker will discard session state.
const sessionExpiryInterval = 60

// disconnectTimeout defines the maximum time in seconds to wait for graceful disconnection.
// This prevents indefinite blocking when the broker is unresponsive during close operations.
const disconnectTimeout = 5

// idleTimeout is how long a connection the service created stays open without
// sends. Tearing it down bounds the goroutines and sockets left behind by
// one-shot senders that never call Close.
const idleTimeout = 60 * time.Second

// Compile-time checks that Service implements the interfaces the router relies on.
var (
	_ types.Service           = (*Service)(nil)
	_ types.ContextSender     = (*Service)(nil)
	_ types.DialContextSetter = (*Service)(nil)
	_ types.ServiceTimeout    = (*Service)(nil)
	_ io.Closer               = (*Service)(nil)
)

// Close disconnects from the broker and ends the connection's lifetime. It first
// waits for sends already using the connection to finish, then waits at most
// [disconnectTimeout] seconds for the disconnect. Sends that start meanwhile open
// a new connection. Calling it again is safe.
//
// Returns:
//   - error: the disconnect failure, or nil when there was no connection.
func (s *Service) Close() error {
	s.clientMutex.Lock()
	manager, cancel, sends := s.detachConnectionLocked()
	s.clientMutex.Unlock()

	if manager == nil {
		return nil
	}

	if err := disconnect(manager, cancel, sends); err != nil {
		return fmt.Errorf("disconnecting from MQTT broker: %w", err)
	}

	return nil
}

// GetID returns the service identifier used for registration and URL scheme matching.
// This identifier is used to route URLs to the correct service implementation.
//
// Returns:
//   - string: the MQTT scheme constant [Scheme], "mqtt".
func (s *Service) GetID() string {
	return Scheme
}

// Initialize configures the MQTT service with settings from a URL and sets up logging.
// The URL format is: mqtt://[username:password@]host[:port]/topic[?options]
//
// The MQTT client is lazily initialized on the first call to Send, allowing runtime
// configuration changes via UpdateConfigFromParams to take effect before the client
// connection is established. This means Host, Port, Username, Password, and TLS
// settings can be modified through params in the first Send call.
//
// It warns once about each deprecated key the URL uses.
//
// Parameters:
//   - serviceURL: The configuration URL containing broker address, credentials, and topic
//   - logger: The logger instance for recording service events and errors
//
// Returns:
//   - error: the error from applying the default props or reading the URL,
//     such as [ErrTopicRequired], [ErrInvalidTopic], or [ErrInvalidQoS].
func (s *Service) Initialize(serviceURL *url.URL, logger types.StdLogger) error {
	// Set up logging for the service
	s.SetLogger(logger)
	s.warnings.Reset()

	// A connection the service created belongs to the previous configuration.
	s.closeOwnedConnection()

	// Initialize the configuration struct with default values
	s.Config = &Config{}

	// Create a property key resolver for mapping URL parameters to config fields
	s.pkr = format.NewPropKeyResolver(s.Config)

	// Apply default values from struct tags to the configuration
	if err := s.pkr.SetDefaultProps(s.Config); err != nil {
		return fmt.Errorf("setting default config properties: %w", err)
	}

	// Parse the configuration URL and populate the config struct
	if err := s.Config.setURL(&s.pkr, serviceURL); err != nil {
		return fmt.Errorf("parsing configuration URL: %w", err)
	}

	s.warnings.DeprecatedKeys(context.Background(), s, s.pkr.DeprecatedKeys())

	// Note: The MQTT client is lazily initialized on the first Send call,
	// allowing runtime configuration changes to take effect before connection.

	return nil
}

// Send delivers a message to the configured MQTT topic.
//
// It delegates to [Service.SendContext] with [context.Background].
//
// Parameters:
//   - message: The notification message to publish to the topic
//   - params: Optional runtime parameters to override config settings
//
// Returns:
//   - error: [types.ErrServiceNotInitialized] before Initialize, or the
//     params, connection, or publish error.
func (s *Service) Send(message string, params *types.Params) error {
	return s.SendContext(context.Background(), message, params)
}

// SendContext delivers a message to the configured MQTT topic.
// It handles connection establishment if needed and publishes the message
// with the configured QoS level and retention settings.
//
// Params apply to this send only. A send that opens a connection uses its own
// settings, including Host, Port, Username, Password, and TLS settings, for that
// connection. Sends that reuse the connection apply only their message settings
// (Topic, QoS, Retained).
//
// ctx bounds the wait for the connection and the publish. A context without a
// deadline is bounded by the budget that [Service.ServiceTimeout] reports. The
// connection itself outlives the send, so it is not bound by ctx.
//
// Parameters:
//   - ctx: cancellation and deadline for the send.
//   - message: The notification message to publish to the topic
//   - params: Optional runtime parameters to override config settings
//
// Returns:
//   - error: [types.ErrServiceNotInitialized] before Initialize, or the
//     params, connection, or publish error, which matches ctx's error when
//     ctx ends the wait.
func (s *Service) SendContext(ctx context.Context, message string, params *types.Params) error {
	if s.Config == nil {
		return types.ErrServiceNotInitialized
	}

	ctx, cancel := httpclient.BoundContext(ctx, s.ServiceTimeout(params))
	defer cancel()

	config := *s.Config

	deprecatedFields, err := format.SyncDeprecatedFields(&config)
	if err != nil {
		return fmt.Errorf("syncing deprecated config fields: %w", err)
	}

	s.warnings.DeprecatedFields(ctx, s, deprecatedFields)
	s.warnings.DeprecatedKeys(ctx, s, s.pkr.DeprecatedParams(params))

	// Apply any runtime parameter overrides to this send's configuration
	if err := s.pkr.UpdateConfigFromParams(&config, params); err != nil {
		return fmt.Errorf("updating config from params: %w", err)
	}

	// Validate QoS value is within MQTT protocol range (0-2)
	if !config.QoS.IsValid() {
		return fmt.Errorf(
			"validating QoS value %d: %w",
			config.QoS,
			ErrInvalidQoS,
		)
	}

	manager, sends, err := s.acquireConnection(ctx, &config)
	if err != nil {
		return fmt.Errorf("initializing MQTT client: %w", err)
	}
	defer s.releaseConnection(sends) //nolint:contextcheck // An idle teardown runs after the send, with its own timeout.

	// Wait for connection to be established
	if err := manager.AwaitConnection(ctx); err != nil {
		return fmt.Errorf("connecting to MQTT broker: %w", err)
	}

	// Publish the message to the configured topic with QoS and retention settings
	//nolint:exhaustruct_v5 // paho.Publish PacketID is auto-generated, Properties optional for MQTT v5
	resp, err := manager.Publish(
		ctx,
		&paho.Publish{
			Topic:   config.Topic,
			QoS:     byte(config.QoS), //nolint:gosec // QoS validated to 0-2
			Retain:  config.Retained,
			Payload: []byte(message),
		},
	)
	// Per MQTT v5 spec, reason codes >= 0x80 indicate failures that are
	// returned as a PublishError. The client returns a rejected QoS 2 PUBREC as
	// a response alone and a rejected QoS 1 PUBACK as a response with an error,
	// so the response is checked before the error. Packet Identifier not found
	// without an error comes only from a QoS 2 PUBCOMP, which answers a PUBREL
	// the client resent after a reconnect. The spec does not treat it as an
	// error during recovery, and the broker accepted the message in its PUBREC.
	recovered := err == nil && resp != nil &&
		resp.ReasonCode == packets.PubcompPacketIdentifierNotFound
	if resp != nil && IsFailureCode(resp.ReasonCode) && !recovered {
		publishErr := PublishError{ReasonCode: resp.ReasonCode, ReasonString: ""}
		if resp.Properties != nil {
			publishErr.ReasonString = resp.Properties.ReasonString
		}

		return publishErr
	}

	if err != nil {
		return fmt.Errorf(
			"publishing to MQTT topic %q: %w",
			config.Topic,
			err,
		)
	}

	// Non-fatal codes (> 0 but < 0x80) and a recovered PUBCOMP are logged as
	// warnings but don't fail
	if resp != nil && resp.ReasonCode != 0 {
		s.Warn(ctx, "publish completed with a reason code", logging.KeyCode, resp.ReasonCode)
	}

	s.Debug(ctx, "message published", logging.KeyTopic, config.Topic)

	return nil
}

// SetConnectionManager sets the connection manager for the service. The caller
// owns manager: the service never tears it down for idleness or replaces it, and
// Close disconnects it. A nil manager makes the next send open its own connection.
//
// Parameters:
//   - manager: The ConnectionManager implementation to use
//
// This method should only be called before any Send operations to avoid
// race conditions with lazy initialization.
func (s *Service) SetConnectionManager(manager ConnectionManager) {
	s.closeOwnedConnection()

	s.clientMutex.Lock()
	defer s.clientMutex.Unlock()

	s.connectionManager = manager
	s.ownedConnection = false
	s.cancelConnection = nil
	s.connectionSends = &sync.WaitGroup{}
}

// SetDialContext sets a custom dial function for MQTT TCP connections.
//
// TLS wrapping for mqtts still happens after the TCP dial.
// A nil dial restores the default autopaho dialer, including all_proxy handling.
// This method should only be called before any Send operations to avoid
// race conditions with lazy initialization.
//
// Parameters:
//   - dial: The dial function. Must be safe for concurrent use when non-nil.
func (s *Service) SetDialContext(dial types.DialContextFunc) {
	s.dialContext = dial
}

// acquireConnection returns the connection for a send, opening one with config
// when there is none or when the connection the service created has ended. It
// stops the idle timer until [Service.releaseConnection] is called.
//
// Parameters:
//   - ctx: the context of the send, passed to the warnings of a connection it
//     opens. The connection's lifetime does not derive from it.
//   - config: the send's configuration, used when a connection is opened.
//
// Returns:
//   - ConnectionManager: the connection to publish through.
//   - *sync.WaitGroup: the connection's sends, to pass to [Service.releaseConnection].
//   - error: the failure to open a connection, which the next send retries.
func (s *Service) acquireConnection(ctx context.Context, config *Config) (ConnectionManager, *sync.WaitGroup, error) {
	s.clientMutex.Lock()
	defer s.clientMutex.Unlock()

	if s.ownedConnection && connectionEnded(s.connectionManager) {
		_, cancel, _ := s.detachConnectionLocked()
		cancel()
	}

	if s.connectionManager == nil {
		if err := s.openConnectionLocked(ctx, config); err != nil {
			return nil, nil, err
		}
	}

	s.activeSends++
	s.connectionSends.Add(1)
	s.stopIdleTimerLocked()

	return s.connectionManager, s.connectionSends, nil
}

// attemptConnection dials the MQTT broker using [Service.dialContext] and
// optionally wraps the connection in TLS.
//
// Parameters:
//   - ctx: Context that bounds dialing and the TLS handshake.
//   - cfg: Autopaho client configuration providing TLS settings.
//   - serverURL: Broker URL whose host and port are dialed.
//
// Returns:
//   - net.Conn: a connected [net.Conn], TLS-wrapped when cfg.TlsCfg is set,
//     always wrapped for thread-safe writes.
//   - error: an error if the custom dialer is unset, dialing fails, or the
//     handshake fails.
func (s *Service) attemptConnection(
	ctx context.Context,
	cfg autopaho.ClientConfig, //nolint:gocritic // hugeParam: matches autopaho.AttemptConnection
	serverURL *url.URL,
) (net.Conn, error) {
	if s.dialContext == nil {
		return nil, ErrNoDialContext
	}

	conn, err := s.dialContext(ctx, "tcp", serverURL.Host)
	if err != nil {
		return nil, fmt.Errorf("dialing MQTT broker %q: %w", serverURL.Host, err)
	}

	if cfg.TlsCfg == nil {
		return packets.NewThreadSafeConn(conn), nil
	}

	tlsCfg := cfg.TlsCfg.Clone()
	if tlsCfg.ServerName == "" {
		tlsCfg.ServerName = serverURL.Hostname()
	}

	tlsConn := tls.Client(conn, tlsCfg)
	if err := tlsConn.HandshakeContext(ctx); err != nil {
		_ = conn.Close()

		return nil, fmt.Errorf("TLS handshake with MQTT broker %q: %w", serverURL.Host, err)
	}

	return packets.NewThreadSafeConn(tlsConn), nil
}

// closeIdleConnection tears down the connection the service created when the
// idle period identified by generation is still current.
//
// Parameters:
//   - generation: the idle period the timer was started for.
func (s *Service) closeIdleConnection(generation uint64) {
	s.clientMutex.Lock()

	if generation != s.idleGeneration || s.activeSends > 0 || !s.ownedConnection {
		s.clientMutex.Unlock()

		return
	}

	manager, cancel, sends := s.detachConnectionLocked()
	s.clientMutex.Unlock()

	// The idle timer runs after the sends, so no send's context applies.
	if err := disconnect(manager, cancel, sends); err != nil {
		s.Warn(context.Background(), "disconnecting the idle connection failed", logging.KeyError, err)
	}
}

// closeOwnedConnection tears down a connection the service created and leaves a
// connection manager set with [Service.SetConnectionManager] in place.
func (s *Service) closeOwnedConnection() {
	s.clientMutex.Lock()

	if !s.ownedConnection {
		s.clientMutex.Unlock()

		return
	}

	manager, cancel, sends := s.detachConnectionLocked()
	s.clientMutex.Unlock()

	if err := disconnect(manager, cancel, sends); err != nil {
		s.Warn(context.Background(), "disconnecting the connection failed", logging.KeyError, err)
	}
}

// createTLSConfig builds the TLS settings of a connection from the shared
// helpers, which require TLS 1.2 or later. It skips certificate verification
// when SkipTLSVerify is set, such as for a self-signed certificate, and warns
// about it once. The dial fills in the server name from the broker URL.
//
// Parameters:
//   - ctx: the context of the send that opens the connection, passed to the
//     warning.
//   - config: the configuration of the connection being opened.
//
// Returns:
//   - *tls.Config: new settings for the connection.
func (s *Service) createTLSConfig(ctx context.Context, config *Config) *tls.Config {
	if config.SkipTLSVerify {
		s.warnings.Insecure(ctx, s)

		return transport.InsecureTLSConfig("")
	}

	return transport.TLSConfig("")
}

// detachConnectionLocked clears the connection state and stops the idle timer.
// The caller holds clientMutex.
//
// Returns:
//   - ConnectionManager: the detached connection, or nil when there was none.
//   - context.CancelFunc: ends the detached connection's lifetime. It is never nil.
//   - *sync.WaitGroup: the sends still using the detached connection, or nil.
func (s *Service) detachConnectionLocked() (ConnectionManager, context.CancelFunc, *sync.WaitGroup) {
	manager, cancel, sends := s.connectionManager, s.cancelConnection, s.connectionSends
	if cancel == nil {
		cancel = func() {}
	}

	s.connectionManager = nil
	s.cancelConnection = nil
	s.connectionSends = nil
	s.ownedConnection = false
	s.stopIdleTimerLocked()

	return manager, cancel, sends
}

// getDefaultPortForScheme returns the standard port number for a given MQTT scheme.
// This is used for automatic scheme detection when the port matches a known default.
//
// Parameters:
//   - scheme: The MQTT scheme ("mqtt" or "mqtts")
//
// Returns:
//   - int: [DefaultTLSPort] (8883) for mqtts, otherwise [DefaultPort] (1883).
func (s *Service) getDefaultPortForScheme(scheme string) int {
	switch scheme {
	case SchemeTLS:
		// Return the secure MQTT port for mqtts scheme
		return DefaultTLSPort
	default:
		// Return the standard MQTT port for all other schemes
		return DefaultPort
	}
}

// openConnectionLocked creates a connection manager for config. The caller holds
// clientMutex.
//
// The connection is configured with:
//   - Broker URL (with automatic scheme detection based on TLS settings)
//   - Authentication credentials if provided
//   - Connection timeout and callbacks for logging
//   - TLS configuration for secure connections
//
// Parameters:
//   - ctx: the context of the send that opens the connection, passed to its
//     warnings. The connection's lifetime does not derive from it.
//   - config: the configuration of the send that opens the connection.
//
// Returns:
//   - error: the failure to create the connection manager.
//
//nolint:contextcheck // The connection outlives the send, so its lifetime starts from the background context.
func (s *Service) openConnectionLocked(ctx context.Context, config *Config) error {
	// Determine the connection scheme based on URL scheme and port configuration.
	// The URL scheme is the primary determinant, with port-based defaults as fallback.
	// User-specified port overrides the default; scheme determines TLS handling.
	scheme := config.scheme
	port := config.Port

	// Apply default scheme if URL scheme is not set or is invalid
	if scheme == "" {
		scheme = Scheme
	}

	// Apply default port based on scheme if user didn't specify one
	// Default: 1883 for mqtt, 8883 for mqtts
	defaultPort := s.getDefaultPortForScheme(scheme)
	if port == 0 {
		port = defaultPort
	}

	// Handle scheme/port mismatches with warnings (but don't change the scheme).
	// Each is logged once, since reconnecting after an idle teardown opens the
	// connection again.
	if scheme == SchemeTLS && !config.DisableTLS && port == DefaultPort {
		// MQTTS scheme on non-TLS port (1883) - warn but keep MQTTS scheme
		s.warnings.Do("mqtts-port", func() {
			s.Warn(ctx, "mqtts scheme on the plain MQTT port, attempting TLS", logging.KeyPort, port)
		})
	} else if scheme == Scheme && port == DefaultTLSPort {
		// MQTT scheme on TLS port (8883) - warn but keep MQTT scheme
		s.warnings.Do("mqtt-port", func() {
			s.Warn(ctx, "mqtt scheme on the TLS port, not using TLS", logging.KeyPort, port)
		})
	}

	// Enable TLS when using secure scheme and TLS is not disabled
	useTLS := scheme == SchemeTLS && !config.DisableTLS
	if !useTLS {
		s.warnings.Plaintext(ctx, s)
	}

	// Build the broker URL
	server := util.JoinURLHost(config.Host, strconv.Itoa(port))
	brokerURL := (&url.URL{Scheme: scheme, Host: server}).String()

	// Parse the broker URL for autopaho
	serverURL, err := url.Parse(brokerURL)
	if err != nil {
		return fmt.Errorf(
			"parsing broker URL %q: %w",
			brokerURL,
			err,
		)
	}

	// The connection lives until Close, an idle teardown, or a replacement after
	// it ends, so its lifetime does not derive from the send's context. Its
	// callbacks run during that lifetime, so they log with it.
	lifetime, cancel := context.WithCancel(context.Background())

	// Create autopaho client configuration
	//nolint:exhaustruct_v5 // autopaho.ClientConfig has many optional fields with library defaults
	cliCfg := autopaho.ClientConfig{
		ServerUrls:                    []*url.URL{serverURL},
		KeepAlive:                     keepAliveInterval,
		CleanStartOnInitialConnection: config.CleanSession,
		SessionExpiryInterval:         sessionExpiryInterval,
		//nolint:exhaustruct_v5 // remaining fields use library defaults
		ClientID: config.ClientID,
		OnServerDisconnect: func(disconnect *paho.Disconnect) {
			s.Warn(lifetime, "server disconnected", logging.KeyCode, disconnect.ReasonCode)
		},
		OnConnectionUp: func(_ *autopaho.ConnectionManager, _ *paho.Connack) {
			s.Debug(lifetime, "connected", logging.KeyServer, server)
		},
		// A send that cannot connect returns only its context's error, so this
		// warning is the only report of why, such as a broker rejecting the
		// credentials. Each failed attempt is retried.
		OnConnectError: func(err error) {
			s.Warn(lifetime, "connection attempt failed", logging.KeyError, err)
		},
	}

	// Set the credentials the URL carries. MQTT v5 allows a password without a
	// username, which brokers use for credentials such as tokens.
	if config.Username != "" {
		cliCfg.ConnectUsername = config.Username
	}

	if config.Password != "" {
		cliCfg.ConnectPassword = []byte(config.Password)
	}

	// Configure TLS based on scheme and DisableTLS setting
	if useTLS {
		cliCfg.TlsCfg = s.createTLSConfig(ctx, config)
	}

	if s.dialContext != nil {
		cliCfg.AttemptConnection = s.attemptConnection
	}

	connect := s.connect
	if connect == nil {
		connect = newAutopahoConnection
	}

	connectionManager, err := connect(lifetime, &cliCfg)
	if err != nil {
		cancel()

		return fmt.Errorf("creating MQTT connection: %w", err)
	}

	s.connectionManager = connectionManager
	s.ownedConnection = true
	s.cancelConnection = cancel
	s.connectionSends = &sync.WaitGroup{}

	return nil
}

// releaseConnection ends a send started with [Service.acquireConnection] and
// starts the idle timer when no other send uses a connection the service created.
//
// Parameters:
//   - sends: the connection's sends, as returned by [Service.acquireConnection].
func (s *Service) releaseConnection(sends *sync.WaitGroup) {
	sends.Done()

	s.clientMutex.Lock()
	defer s.clientMutex.Unlock()

	s.activeSends--
	if s.activeSends > 0 || !s.ownedConnection || s.connectionManager == nil {
		return
	}

	s.stopIdleTimerLocked()

	generation := s.idleGeneration
	s.idleTimer = time.AfterFunc(idleTimeout, func() { s.closeIdleConnection(generation) })
}

// stopIdleTimerLocked stops the idle timer and starts a new idle generation, so
// a timer that already fired does nothing. The caller holds clientMutex.
func (s *Service) stopIdleTimerLocked() {
	s.idleGeneration++

	if s.idleTimer != nil {
		s.idleTimer.Stop()
		s.idleTimer = nil
	}
}

// newAutopahoConnection creates an autopaho connection manager.
//
// Parameters:
//   - ctx: the connection's lifetime.
//   - cfg: the client configuration.
//
// Returns:
//   - ConnectionManager: the connection manager.
//   - error: the failure to create it.
func newAutopahoConnection(ctx context.Context, cfg *autopaho.ClientConfig) (ConnectionManager, error) {
	manager, err := autopaho.NewConnection(ctx, *cfg)
	if err != nil {
		return nil, fmt.Errorf("starting autopaho connection: %w", err)
	}

	return manager, nil
}

// connectionEnded reports whether a connection manager that exposes Done, such as
// autopaho's, has shut down.
//
// Parameters:
//   - manager: the connection manager, which may be nil.
//
// Returns:
//   - bool: true when manager's Done channel is closed.
func connectionEnded(manager ConnectionManager) bool {
	ender, ok := manager.(interface{ Done() <-chan struct{} })
	if !ok {
		return false
	}

	select {
	case <-ender.Done():
		return true
	default:
		return false
	}
}

// disconnect waits for the sends still using manager, disconnects it, waiting at
// most [disconnectTimeout] seconds, and then calls cancel to end its lifetime.
//
// Parameters:
//   - manager: the connection to disconnect.
//   - cancel: ends the connection's lifetime.
//   - sends: the sends using the connection, or nil.
//
// Returns:
//   - error: the disconnect failure.
func disconnect(manager ConnectionManager, cancel context.CancelFunc, sends *sync.WaitGroup) error {
	defer cancel()

	if sends != nil {
		sends.Wait()
	}

	ctx, stop := context.WithTimeout(context.Background(), disconnectTimeout*time.Second)
	defer stop()

	if err := manager.Disconnect(ctx); err != nil {
		return fmt.Errorf("disconnecting: %w", err)
	}

	return nil
}
