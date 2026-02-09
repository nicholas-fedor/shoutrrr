package mqtt

import (
	"errors"
	"fmt"
	"log"
	"net/url"
	"strconv"
	"strings"

	"github.com/nicholas-fedor/shoutrrr/pkg/format"
	"github.com/nicholas-fedor/shoutrrr/pkg/types"
)

// Scheme identifies the URL scheme for MQTT service URLs.
const Scheme = "mqtt"

// QoS constants define the three MQTT Quality of Service levels.
const (
	// QoSAtMostOnce (fire-and-forget): The broker delivers the message at most once.
	// No acknowledgment is required. Messages may be lost if the client disconnects
	// or the broker fails. Best for high-throughput, non-critical data.
	QoSAtMostOnce QoS = 0

	// QoSAtLeastOnce: The broker delivers the message at least once.
	// Requires acknowledgment from the subscriber. Duplicates may occur if
	// acknowledgments are lost. Suitable for important data where occasional
	// duplicates are acceptable.
	QoSAtLeastOnce QoS = 1

	// QoSExactlyOnce: The broker delivers the message exactly once.
	// Uses a two-phase commit handshake to guarantee exactly-once delivery.
	// Highest overhead but strongest guarantee. Best for critical data where
	// duplicates must be avoided.
	QoSExactlyOnce QoS = 2
)

// ErrTopicRequired is returned when a configuration URL lacks a required topic path.
// The topic is mandatory for publishing messages and must be provided in the URL path.
var ErrTopicRequired = errors.New("topic is required")

// ErrInvalidQoS is returned when a QoS value is outside the valid range (0-2).
// MQTT protocol only supports QoS levels 0, 1, and 2.
var ErrInvalidQoS = errors.New("invalid QoS value: must be 0, 1, or 2")

// QoS represents the Quality of Service (QoS) level for MQTT message delivery.
// Higher QoS levels provide stronger delivery guarantees but with increased overhead.
type QoS int

// Config holds all configuration settings for the MQTT service.
// Each field can be set via URL parameters or struct tags, with defaults
// applied when values are not explicitly provided.
type Config struct {
	// Host is the MQTT broker hostname or IP address.
	// Default: "localhost"
	Host string `default:"localhost" desc:"MQTT broker hostname" url:"host"`

	// Port is the MQTT broker port number.
	// Default: 1883 (standard MQTT), 8883 (MQTT over TLS)
	Port int `default:"1883" desc:"MQTT broker port" url:"port"`

	// Topic is the MQTT topic path to publish messages to.
	// This is a required field and must be provided in the URL path.
	// Example: "notifications/alerts" for topic notifications/alerts
	Topic string `desc:"Target topic name" required:"" url:"path"`

	// Username for MQTT broker authentication.
	// Optional: Leave empty for anonymous connections or when password-only auth is used.
	Username string `desc:"Auth username" optional:"" url:"user"`

	// Password for MQTT broker authentication.
	// Optional: Leave empty for anonymous connections or username-only auth.
	Password string `desc:"Auth password" optional:"" url:"password"`

	// ClientID is the unique identifier for this MQTT client connection.
	// The broker uses this to identify the client across sessions and for
	// persistent session management. Must be unique among all clients connecting
	// to the same broker.
	// Default: "shoutrrr"
	ClientID string `default:"shoutrrr" desc:"MQTT client identifier" key:"clientid"`

	// QoS (Quality of Service) determines the message delivery guarantee level.
	// See QoS constants for available levels and their semantics.
	// Default: 0 (QoSAtMostOnce)
	QoS QoS `default:"0" desc:"Quality of Service level (0, 1, or 2)" key:"qos"`

	// Retained controls whether the broker stores the last message on this topic.
	// When true, new subscribers immediately receive the last retained message.
	// Useful for status updates or configuration data that should be available
	// immediately when clients subscribe.
	// Default: false
	Retained bool `default:"no" desc:"Retain message on broker" key:"retained"`

	// CleanSession determines whether to start a fresh session on connect.
	// When true, the broker discards any previous session state (subscriptions,
	// queued messages). When false, the broker resumes the previous session
	// if the ClientID matches.
	// Default: true
	CleanSession bool `default:"yes" desc:"Start with a clean session" key:"cleansession"`

	// DisableTLS turns off TLS encryption for the connection.
	// When true, connections use plain TCP which is faster but insecure.
	// Should only be used on trusted networks or for testing.
	// Default: false (TLS enabled)
	DisableTLS bool `default:"no" desc:"Disable TLS encryption" key:"disabletls"`

	// DisableTLSVerification skips TLS certificate validation.
	// When true, the client accepts any certificate from the broker.
	// WARNING: This makes the connection vulnerable to man-in-the-middle attacks.
	// Only use for testing or with self-signed certificates in trusted environments.
	// Default: false (verification enabled)
	DisableTLSVerification bool `default:"no" desc:"Disable TLS certificate verification" key:"disabletlsverification"`
}

// qosVals holds the QoS enum value constants and formatter.
// This struct enables type-safe QoS value handling with flexible parsing.
type qosVals struct {
	// AtMostOnce is the fire-and-forget QoS level (0).
	AtMostOnce QoS
	// AtLeastOnce is the acknowledged delivery QoS level (1).
	AtLeastOnce QoS
	// ExactlyOnce is the guaranteed delivery QoS level (2).
	ExactlyOnce QoS
	// Enum provides parsing and formatting for QoS values.
	Enum types.EnumFormatter
}

// GetURL constructs a configuration URL from the current config settings.
// The URL contains all configuration values and can be used to recreate
// an identical service configuration.
//
// Returns a *url.URL in the format: mqtt://[user:pass@]host:port/topic?options
func (c *Config) GetURL() *url.URL {
	// Create a new property key resolver for building the query string
	resolver := format.NewPropKeyResolver(c)

	return c.getURL(&resolver)
}

// getURL is the internal implementation of GetURL that accepts a resolver interface.
// This allows for easier testing and decoupling from the concrete resolver type.
//
// Parameters:
//   - resolver: The query resolver to use for building the URL query string
//
// Returns a complete URL representing the current configuration.
func (c *Config) getURL(resolver types.ConfigQueryResolver) *url.URL {
	// Build the base URL with scheme, host:port, and topic path
	result := &url.URL{
		Scheme:     Scheme,
		Host:       fmt.Sprintf("%s:%d", c.Host, c.Port),
		ForceQuery: true, // Always include the query string separator (?)
		Path:       c.Topic,
		RawQuery:   format.BuildQuery(resolver),
	}

	// Warn if password is provided without username for URL credentials
	if c.Password != "" && c.Username == "" {
		log.Printf(
			"Warning: Password provided without username; password requires a username for URL credentials and will be ignored",
		)
	}

	// Add user credentials if authentication is configured
	if c.Username != "" {
		if c.Password != "" {
			// Include both username and password in the URL
			result.User = url.UserPassword(c.Username, c.Password)
		} else {
			// Username only (no password)
			result.User = url.User(c.Username)
		}
	}

	return result
}

// SetURL parses a configuration URL and populates the config fields.
// This is the public entry point for URL-based configuration.
//
// Parameters:
//   - url: The configuration URL to parse
//
// Returns an error if the URL is malformed or required fields are missing.
func (c *Config) SetURL(url *url.URL) error {
	// Create a new property key resolver for parsing query parameters
	resolver := format.NewPropKeyResolver(c)

	return c.setURL(&resolver, url)
}

// setURL is the internal implementation of SetURL that accepts a resolver interface.
// It extracts all configuration values from the URL including:
//   - User credentials from the URL user info (username:password@host)
//   - Host and port from the URL host component
//   - Topic from the URL path (with leading slash removed)
//   - Additional options from URL query parameters
//
// Parameters:
//   - resolver: The query resolver to use for parsing query parameters
//   - url: The configuration URL to parse
//
// Returns an error if parsing fails or required fields are missing.
func (c *Config) setURL(resolver types.ConfigQueryResolver, url *url.URL) error {
	// Extract authentication credentials from URL user info if present
	if url.User != nil {
		// Get the password (may be empty if only username is provided)
		password, _ := url.User.Password()
		c.Password = password
		c.Username = url.User.Username()
	} else {
		// Clear credentials if not present in URL
		c.Password = ""
		c.Username = ""
	}

	// Extract hostname from the URL
	host := url.Hostname()
	if host != "" {
		c.Host = host
	}

	// Extract and parse port number from the URL
	port := url.Port()
	if port != "" {
		// Parse the port string to an integer
		parsedPort, err := strconv.Atoi(port)
		if err != nil {
			return fmt.Errorf("parsing port: %w", err)
		}

		c.Port = parsedPort
	}

	// Extract topic from URL path, removing the leading slash
	// Example: "/notifications/alerts" becomes "notifications/alerts"
	c.Topic = strings.TrimPrefix(url.Path, "/")

	// Process all query parameters through the resolver
	// This handles optional parameters like qos, retained, clientid, etc.
	for key, vals := range url.Query() {
		if err := resolver.Set(key, vals[0]); err != nil {
			return fmt.Errorf("setting query parameter %q to %q: %w", key, vals[0], err)
		}
	}

	// Validate that a topic was provided (skip for dummy URL used in docs generation)
	if url.String() != "mqtt://dummy@dummy.com" {
		if c.Topic == "" {
			return ErrTopicRequired
		}
	}

	// Validate QoS value is within MQTT protocol range (0-2)
	if !c.QoS.IsValid() {
		return fmt.Errorf("validating QoS value %d: %w", c.QoS, ErrInvalidQoS)
	}

	return nil
}

// QueryFields returns a list of all configurable query parameter names.
// This is used for documentation generation and validation of URL parameters.
//
// Returns a slice of query parameter field names.
func (c *Config) QueryFields() []string {
	return format.GetConfigQueryResolver(c).QueryFields()
}

// Enums returns a map of enum type names to their formatters.
// This enables proper parsing and formatting of enum values like QoS.
//
// Returns a map where keys are enum type names and values are formatters
// that can convert between string representations and enum values.
func (c *Config) Enums() map[string]types.EnumFormatter {
	return map[string]types.EnumFormatter{
		"QoS": QoSValues.Enum,
	}
}

// QoSValues holds the QoS enum values and their associated formatter.
// This provides both the concrete enum values and the machinery for
// parsing and formatting QoS values from strings.
var QoSValues = &qosVals{
	AtMostOnce:  QoSAtMostOnce,
	AtLeastOnce: QoSAtLeastOnce,
	ExactlyOnce: QoSExactlyOnce,
	// Create an enum formatter that accepts multiple string representations:
	// - Standard names: "AtMostOnce", "AtLeastOnce", "ExactlyOnce"
	// - Numeric strings: "0", "1", "2"
	// - Snake case: "at_most_once", "at_least_once", "exactly_once"
	Enum: format.CreateEnumFormatter(
		[]string{
			"AtMostOnce",
			"AtLeastOnce",
			"ExactlyOnce",
		}, map[string]int{
			"0":             int(QoSAtMostOnce),
			"1":             int(QoSAtLeastOnce),
			"2":             int(QoSExactlyOnce),
			"at_most_once":  int(QoSAtMostOnce),
			"at_least_once": int(QoSAtLeastOnce),
			"exactly_once":  int(QoSExactlyOnce),
		}),
}

// String returns the human-readable name of the QoS level.
// This implements the fmt.Stringer interface for pretty printing.
//
// Returns one of "AtMostOnce", "AtLeastOnce", or "ExactlyOnce".
func (q QoS) String() string {
	return QoSValues.Enum.Print(int(q))
}

// IsValid reports whether the QoS value is within the valid MQTT range.
// MQTT protocol only supports QoS levels 0 (at most once), 1 (at least once),
// and 2 (exactly once). Any value outside this range is invalid.
//
// Returns true if q is between QoSAtMostOnce (0) and QoSExactlyOnce (2) inclusive.
func (q QoS) IsValid() bool {
	return q >= QoSAtMostOnce && q <= QoSExactlyOnce
}
