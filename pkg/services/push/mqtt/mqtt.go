// Package mqtt provides a notification service for MQTT message brokers.
// It supports both standard MQTT (mqtt://) and TLS-secured MQTT (mqtts://) connections
// with configurable QoS levels, authentication, and message retention.
package mqtt

import (
	"context"
	"crypto/tls"
	"fmt"
	"net/url"
	"sync"
	"time"

	"github.com/eclipse/paho.golang/autopaho"
	"github.com/eclipse/paho.golang/paho"

	"github.com/nicholas-fedor/shoutrrr/pkg/format"
	"github.com/nicholas-fedor/shoutrrr/pkg/services/standard"
	"github.com/nicholas-fedor/shoutrrr/pkg/types"
)

// publishTimeout defines the maximum time in seconds to wait for message publication.
// This prevents indefinite blocking when the broker is unresponsive during publish operations.
const publishTimeout = 10

// keepAliveInterval defines the interval in seconds between keep-alive packets.
// This maintains the connection and detects dead peers.
const keepAliveInterval = 20

// sessionExpiryInterval defines the session expiry interval in seconds for MQTT v5.
// After this period, the broker will discard session state.
const sessionExpiryInterval = 60

// disconnectTimeout defines the maximum time in seconds to wait for graceful disconnection.
// This prevents indefinite blocking when the broker is unresponsive during close operations.
const disconnectTimeout = 5

// SecureScheme identifies the TLS-secured MQTT protocol scheme.
const SecureScheme = "mqtts"

// Default ports for MQTT connections based on security level.
const (
	// defaultMQTTPort is the standard unencrypted MQTT port (1883).
	defaultMQTTPort = 1883
	// defaultMQTTSPort is the standard TLS-encrypted MQTT port (8883).
	defaultMQTTSPort = 8883
)

// Service implements the notification service interface for MQTT brokers.
// It manages the connection lifecycle and message publishing to MQTT topics.
type Service struct {
	// Standard provides base service functionality including logging.
	standard.Standard
	// Config holds the MQTT connection and publishing settings.
	Config *Config
	// pkr resolves property keys for configuration updates from URL parameters.
	pkr format.PropKeyResolver
	// connectionManager is the underlying MQTT connection manager for broker communication.
	connectionManager ConnectionManager
	// clientMutex protects the connection initialization to ensure thread-safe
	// lazy initialization while allowing retry on transient failures.
	clientMutex sync.Mutex
	// connectionInitialized indicates whether the MQTT client has been successfully
	// initialized. This is set to true only after connectionManager is assigned.
	connectionInitialized bool
	// ctx is the context for managing the connection lifecycle.
	// This is required to ensure that cancel() properly terminates the connection
	// used by autopaho. The context is created alongside the cancel function in
	// getCancel() and used in initClient() for the connection manager.
	//
	//nolint:containedctx // Required for proper cancellation wiring with autopaho
	ctx context.Context
	// cancel is the cancel function for cleaning up the connection.
	cancel context.CancelFunc
	// cancelOnce ensures the cancel function is initialized exactly once.
	cancelOnce sync.Once
	// closeOnce ensures the Close method is called exactly once.
	closeOnce sync.Once
	// closeErr stores any error from the close operation for return on subsequent calls.
	closeErr error
}

// Initialize configures the MQTT service with settings from a URL and sets up logging.
// The URL format is: mqtt://[username:password@]host[:port]/topic[?options]
//
// The MQTT client is lazily initialized on the first call to Send, allowing runtime
// configuration changes via UpdateConfigFromParams to take effect before the client
// connection is established. This means Host, Port, Username, Password, and TLS
// settings can be modified through params in the first Send call.
//
// Parameters:
//   - configURL: The configuration URL containing broker address, credentials, and topic
//   - logger: The logger instance for recording service events and errors
//
// Returns an error if URL parsing fails or required fields are missing.
func (s *Service) Initialize(configURL *url.URL, logger types.StdLogger) error {
	// Set up logging for the service
	s.SetLogger(logger)

	// Initialize the configuration struct with default values
	s.Config = &Config{}

	// Create a property key resolver for mapping URL parameters to config fields
	s.pkr = format.NewPropKeyResolver(s.Config)

	// Apply default values from struct tags to the configuration
	if err := s.pkr.SetDefaultProps(s.Config); err != nil {
		return fmt.Errorf("setting default config properties: %w", err)
	}

	// Parse the configuration URL and populate the config struct
	if err := s.Config.setURL(&s.pkr, configURL); err != nil {
		return fmt.Errorf("parsing configuration URL: %w", err)
	}

	// Note: The MQTT client is lazily initialized on the first Send call,
	// allowing runtime configuration changes to take effect before connection.

	return nil
}

// getCancel returns the cancel function, initializing both the context and cancel
// function exactly once. The context is stored in s.ctx for use by initClient,
// ensuring that calling s.cancel() will properly cancel the connection lifecycle.
//
// Returns the cancel function for connection cleanup.
func (s *Service) getCancel() context.CancelFunc {
	s.cancelOnce.Do(func() {
		// Create a cancellable context for managing the connection lifecycle.
		// Both ctx and cancel are stored so that cancel() can terminate
		// operations using this context.
		s.ctx, s.cancel = context.WithCancel(context.Background())
	})

	return s.cancel
}

// initClient creates and configures the MQTT client using a mutex-protected pattern.
// This ensures thread-safe lazy initialization while allowing retry on transient failures.
//
// Unlike sync.Once, this pattern allows initialization to be retried if it fails,
// preventing permanent failure states when transient errors occur during url.Parse
// or autopaho.NewConnection.
//
// The client is lazily initialized on the first call to Send, not during Initialize.
// This allows runtime configuration changes (Host, Port, Username, Password, TLS settings)
// to be applied via UpdateConfigFromParams before the client is created.
//
// Once successfully initialized, the client is reused for all subsequent Send calls.
// The mutex-protected pattern ensures that:
//   - Only one goroutine can attempt initialization at a time
//   - Successful initialization is recorded via connectionInitialized flag
//   - Failed initialization can be retried on subsequent calls
//
// The client is configured with:
//   - Broker URL (with automatic scheme detection based on TLS settings)
//   - Authentication credentials if provided
//   - Connection timeout and callbacks for logging
//   - TLS configuration for secure connections
//
// Returns an error if initialization fails, allowing callers to retry.
func (s *Service) initClient() error {
	// Lock the mutex to ensure thread-safe initialization
	s.clientMutex.Lock()
	defer s.clientMutex.Unlock()

	// Check if already successfully initialized
	if s.connectionInitialized {
		return nil
	}

	// Ensure cancel function and context are initialized.
	// This stores both s.ctx and s.cancel for connection lifecycle management.
	_ = s.getCancel()

	// Determine the connection scheme based on TLS configuration
	scheme := Scheme
	port := s.Config.Port

	// Set secure scheme when TLS is enabled and port matches MQTTS default
	if !s.Config.DisableTLS && s.Config.Port == s.getDefaultPortForScheme(SecureScheme) {
		scheme = SecureScheme
	}

	// Build the broker URL
	brokerURL := fmt.Sprintf("%s://%s:%d", scheme, s.Config.Host, port)

	// Parse the broker URL for autopaho
	serverURL, err := url.Parse(brokerURL)
	if err != nil {
		return fmt.Errorf("parsing broker URL %q: %w", brokerURL, err)
	}

	// Use the service's context for connection lifecycle management.
	// This ensures that calling s.cancel() will properly terminate
	// the connection when shutdown is requested.
	ctx := s.ctx

	// Create autopaho client configuration
	cliCfg := autopaho.ClientConfig{
		ServerUrls:                    []*url.URL{serverURL},
		KeepAlive:                     keepAliveInterval,
		CleanStartOnInitialConnection: s.Config.CleanSession,
		SessionExpiryInterval:         sessionExpiryInterval,
		ClientConfig: paho.ClientConfig{
			ClientID: s.Config.ClientID,
			OnServerDisconnect: func(disconnect *paho.Disconnect) {
				s.Logf("Server disconnected: reason code %d", disconnect.ReasonCode)
			},
		},
		OnConnectionUp: func(_ *autopaho.ConnectionManager, _ *paho.Connack) {
			s.Logf("Connected to MQTT broker at %s", brokerURL)
		},
		OnConnectError: func(err error) {
			s.Logf("Connection error: %v", err)
		},
	}

	// Set authentication credentials if username is provided
	if s.Config.Username != "" {
		cliCfg.ConnectUsername = s.Config.Username

		// Set password if provided alongside username
		if s.Config.Password != "" {
			cliCfg.ConnectPassword = []byte(s.Config.Password)
		}
	} else if s.Config.Password != "" {
		// Password without username creates invalid MQTT auth state
		s.Log("Warning: Password provided without username; skipping password authentication")
	}

	// Configure TLS only when using the secure scheme and TLS is not disabled
	if scheme == SecureScheme && !s.Config.DisableTLS {
		cliCfg.TlsCfg = s.createTLSConfig()
	}

	// Create the connection manager
	connectionManager, err := autopaho.NewConnection(ctx, cliCfg)
	if err != nil {
		return fmt.Errorf("creating MQTT connection: %w", err)
	}

	// Only set connectionManager and mark as initialized on success
	s.connectionManager = connectionManager
	s.connectionInitialized = true

	return nil
}

// createTLSConfig builds a TLS configuration based on the service settings.
// It enforces TLS 1.2 as the minimum version and optionally skips certificate
// verification when DisableTLSVerification is set (useful for testing or
// self-signed certificates).
//
// Returns a *tls.Config ready for use with the MQTT client.
func (s *Service) createTLSConfig() *tls.Config {
	// Start with a base config requiring TLS 1.2 or higher
	config := &tls.Config{
		MinVersion: tls.VersionTLS12,
	}

	// Skip certificate verification if explicitly disabled
	// Warning: This makes the connection vulnerable to man-in-the-middle attacks
	if s.Config.DisableTLSVerification {
		config.InsecureSkipVerify = true

		// Log a warning about the security implications
		s.Log("Warning: TLS verification is disabled, making connections insecure")
	}

	return config
}

// getDefaultPortForScheme returns the standard port number for a given MQTT scheme.
// This is used for automatic scheme detection when the port matches a known default.
//
// Parameters:
//   - scheme: The MQTT scheme ("mqtt" or "mqtts")
//
// Returns the default port number (1883 for mqtt, 8883 for mqtts).
func (s *Service) getDefaultPortForScheme(scheme string) int {
	switch scheme {
	case SecureScheme:
		// Return the secure MQTT port for mqtts scheme
		return defaultMQTTSPort
	default:
		// Return the standard MQTT port for all other schemes
		return defaultMQTTPort
	}
}

// Send delivers a message to the configured MQTT topic.
// It handles connection establishment if needed and publishes the message
// with the configured QoS level and retention settings.
//
// On the first call, this method triggers lazy initialization of the MQTT client,
// applying any configuration changes from params before creating the client. This allows
// Host, Port, Username, Password, and TLS settings to be overridden at runtime.
// Subsequent calls reuse the existing client; config changes after the first Send
// only affect message-related settings (Topic, QoS, Retained), not connection settings.
//
// Parameters:
//   - message: The notification message to publish to the topic
//   - params: Optional runtime parameters to override config settings
//
// Returns an error if connection fails or publishing encounters an error.
func (s *Service) Send(message string, params *types.Params) error {
	// Apply any runtime parameter overrides to the configuration
	if err := s.pkr.UpdateConfigFromParams(s.Config, params); err != nil {
		return fmt.Errorf("updating config from params: %w", err)
	}

	// Validate QoS value is within MQTT protocol range (0-2)
	if !s.Config.QoS.IsValid() {
		return fmt.Errorf("validating QoS value %d: %w", s.Config.QoS, ErrInvalidQoS)
	}

	// Ensure the MQTT client is initialized.
	// This now returns an error that can be retried on transient failures.
	if err := s.initClient(); err != nil {
		return fmt.Errorf("initializing MQTT client: %w", err)
	}

	// Create a context with timeout for the publish operation
	ctx, cancel := context.WithTimeout(context.Background(), publishTimeout*time.Second)
	defer cancel()

	// Wait for connection to be established
	if err := s.connectionManager.AwaitConnection(ctx); err != nil {
		return fmt.Errorf("connecting to MQTT broker: %w", err)
	}

	// Publish the message to the configured topic with QoS and retention settings
	resp, err := s.connectionManager.Publish(ctx, &paho.Publish{
		Topic:   s.Config.Topic,
		QoS:     byte(s.Config.QoS),
		Retain:  s.Config.Retained,
		Payload: []byte(message),
	})
	if err != nil {
		return fmt.Errorf("publishing to MQTT topic %q: %w", s.Config.Topic, err)
	}

	// Handle MQTT v5 reason codes from the publish response.
	// Per MQTT v5 spec, reason codes >= 0x80 indicate failures that should be
	// returned as errors, while codes 0x01-0x7F are non-fatal warnings.
	if resp != nil && resp.ReasonCode != 0 {
		if IsFailureCode(resp.ReasonCode) {
			// Failure codes (>= 0x80) should be returned as errors to the caller
			return PublishError{
				ReasonCode:   resp.ReasonCode,
				ReasonString: resp.Properties.ReasonString,
			}
		}

		// Non-fatal codes (> 0 but < 0x80) are logged as warnings but don't fail
		s.Logf("Warning: Publish completed with reason code %d", resp.ReasonCode)
	}

	// Log successful publication for debugging and monitoring
	s.Logf("Successfully published message to topic %q", s.Config.Topic)

	return nil
}

// GetID returns the service identifier used for registration and URL scheme matching.
// This identifier is used to route URLs to the correct service implementation.
//
// Returns the MQTT scheme constant "mqtt".
func (s *Service) GetID() string {
	return Scheme
}

// Close gracefully shuts down the MQTT service by disconnecting from the broker
// and canceling the connection context.
//
// This method is idempotent - multiple calls are safe and will not cause panics
// or errors. The first call performs the actual cleanup; subsequent calls return
// the same result as the first call.
//
// The shutdown process:
//  1. Disconnects the connection manager with a 5-second timeout
//  2. Cancels the context to signal termination to any goroutines
//
// Returns an error if the disconnect fails, wrapped with context about the failure.
// Returns nil on successful cleanup or if already closed without error.
func (s *Service) Close() error {
	s.closeOnce.Do(func() {
		// Disconnect the connection manager if it was initialized.
		if s.connectionManager != nil {
			// Create a context with timeout for the disconnect operation.
			ctx, cancel := context.WithTimeout(context.Background(), disconnectTimeout*time.Second)
			defer cancel()

			// Attempt to disconnect from the broker.
			if err := s.connectionManager.Disconnect(ctx); err != nil {
				s.closeErr = fmt.Errorf("disconnecting from MQTT broker: %w", err)

				return
			}
		}

		// Cancel the context to signal termination.
		// The cancel function is initialized by getCancel(), which is called
		// during initClient(). If the client was never initialized, cancel is nil.
		if s.cancel != nil {
			s.cancel()
		}
	})

	return s.closeErr
}

// SetConnectionManager sets the connection manager for the service.
//
// Parameters:
//   - cm: The ConnectionManager implementation to use
//
// This method should only be called before any Send operations to avoid
// race conditions with lazy initialization.
func (s *Service) SetConnectionManager(cm ConnectionManager) {
	s.connectionManager = cm
	s.connectionInitialized = cm != nil
}
