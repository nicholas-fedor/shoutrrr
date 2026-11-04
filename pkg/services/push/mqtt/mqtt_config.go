package mqtt

import (
	"fmt"
	"net/url"
	"strings"
	"unicode/utf8"

	"github.com/nicholas-fedor/shoutrrr/pkg/format"
	"github.com/nicholas-fedor/shoutrrr/pkg/types"
	"github.com/nicholas-fedor/shoutrrr/pkg/util"
)

// Config holds all configuration settings for the MQTT service.
// Each field can be set via URL parameters or struct tags, with defaults
// applied when values are not explicitly provided.
type Config struct {
	// Host is the MQTT broker hostname or IP address.
	// Default: "localhost"
	Host string `default:"localhost" desc:"MQTT broker hostname" url:"host"`

	// Topic is the MQTT topic path to publish messages to.
	// This is a required field and must be provided in the URL path.
	// Example: "notifications/alerts" for topic notifications/alerts
	Topic string `desc:"Target topic name" required:"" url:"path"`

	// Username for MQTT broker authentication. It is optional, and empty for
	// anonymous connections or password-only authentication.
	Username string `desc:"Auth username" optional:"" url:"user"`

	// Password for MQTT broker authentication. It is optional, and empty for
	// anonymous connections or username-only authentication.
	Password string `desc:"Auth password" optional:"" sensitive:"true" url:"password"`

	// ClientID is the unique identifier for this MQTT client connection.
	// The broker uses this to identify the client across sessions and for
	// persistent session management. Must be unique among all clients connecting
	// to the same broker.
	// Default: "shoutrrr"
	ClientID string `default:"shoutrrr" desc:"MQTT client identifier" key:"clientid"`

	// scheme is the URL scheme the config was parsed from, mqtt or mqtts. It selects
	// TLS for the connection and is kept by GetURL.
	scheme string

	// Port is the MQTT broker port number.
	// Default: 1883 (standard MQTT), 8883 (MQTT over TLS)
	Port int `default:"1883" desc:"MQTT broker port" url:"port"`

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

	// SkipTLSVerify skips TLS certificate validation.
	// When true, the client accepts any certificate from the broker.
	// WARNING: This makes the connection vulnerable to man-in-the-middle attacks.
	// Only use for testing or with self-signed certificates in trusted environments.
	// Default: false (verification enabled)
	SkipTLSVerify bool `default:"no" deprecatedkeys:"disabletlsverification" desc:"Skip TLS certificate verification" key:"skiptlsverify"`

	// DisableTLSVerification is the old name of SkipTLSVerify. When
	// SkipTLSVerify is off, setting it skips verification too.
	//
	// Deprecated: Use [Config.SkipTLSVerify], which the skiptlsverify key sets.
	DisableTLSVerification bool `deprecated:"SkipTLSVerify"`
}

// QoS represents the Quality of Service (QoS) level for MQTT message delivery.
// Higher QoS levels provide stronger delivery guarantees but with increased overhead.
type QoS int

// qosVals holds the QoS enum value constants and formatter.
// This struct enables type-safe QoS value handling with flexible parsing.
type qosVals struct {
	// Enum provides parsing and formatting for QoS values.
	Enum types.EnumFormatter
	// AtMostOnce is the fire-and-forget QoS level (0).
	AtMostOnce QoS
	// AtLeastOnce is the acknowledged delivery QoS level (1).
	AtLeastOnce QoS
	// ExactlyOnce is the guaranteed delivery QoS level (2).
	ExactlyOnce QoS
}

// Scheme identifies the URL scheme for MQTT service URLs.
const Scheme = "mqtt"

// SchemeTLS identifies the URL scheme for MQTT over TLS service URLs.
const SchemeTLS = "mqtts"

// DefaultHost is the broker host a URL without a host uses.
const DefaultHost = "localhost"

// maxTopicLength is the longest topic name in bytes. MQTT encodes a topic name
// as a UTF-8 string with a two-byte length prefix.
const maxTopicLength = 65535

// DefaultPort is the standard MQTT port (1883).
const DefaultPort = 1883

// DefaultTLSPort is the standard MQTT over TLS port (8883).
const DefaultTLSPort = 8883

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
		},
	),
}

// Enums returns a map of enum type names to their formatters.
// This enables proper parsing and formatting of enum values like QoS.
//
// Returns:
//   - map[string]types.EnumFormatter: the QoS formatter by field name, which
//     converts between string representations and enum values.
func (c *Config) Enums() map[string]types.EnumFormatter {
	return map[string]types.EnumFormatter{
		"QoS": QoSValues.Enum,
	}
}

// GetURL constructs a configuration URL from the current config settings.
// The URL contains all configuration values and can be used to recreate
// an identical service configuration. A value set through a deprecated field
// is written under its replacement's key.
//
// Returns:
//   - *url.URL: the URL in the format mqtt://[user][:pass]@host:port/topic?options.
func (c *Config) GetURL() *url.URL {
	synced := *c

	_, _ = format.SyncDeprecatedFields(&synced) //nolint:errcheck // The tags are valid, and a failed sync changes nothing.

	// Create a new property key resolver for building the query string
	resolver := format.NewPropKeyResolver(&synced)

	return synced.getURL(&resolver)
}

// QueryFields returns a list of all configurable query parameter names.
// This is used for documentation generation and validation of URL parameters.
//
// Returns:
//   - []string: the query parameter field names.
func (c *Config) QueryFields() []string {
	return format.GetConfigQueryResolver(c).QueryFields()
}

// SetURL parses a configuration URL and populates the config fields.
// This is the public entry point for URL-based configuration.
//
// Parameters:
//   - serviceURL: The configuration URL to parse
//
// Returns:
//   - error: a host error such as [util.ErrInvalidHost], a port error, a
//     malformed query ([util.ErrInvalidQuery]) or query parameter error,
//     [ErrTopicRequired], [ErrInvalidTopic], or [ErrInvalidQoS].
func (c *Config) SetURL(serviceURL *url.URL) error {
	// Create a new property key resolver for parsing query parameters
	resolver := format.NewPropKeyResolver(c)

	return c.setURL(&resolver, serviceURL)
}

// ValidateCredentials reports whether the configuration has a password without
// a username. It is kept for compatibility, and the service never calls it.
//
// Deprecated: MQTT v5 allows a password without a username, which the service
// sends and [Config.GetURL] keeps, so callers do not need to check for it.
//
// Returns:
//   - error: [ErrPasswordWithoutUsername] for a password without a username,
//     kept for compatibility even though the service accepts one, otherwise nil.
func (c *Config) ValidateCredentials() error {
	if c.Password != "" && c.Username == "" {
		return ErrPasswordWithoutUsername
	}

	return nil
}

// brokerScheme returns the scheme the connection uses. A config read from an
// mqtts URL uses mqtts, and any other config uses mqtt.
//
// Returns:
//   - string: [SchemeTLS] or [Scheme].
func (c *Config) brokerScheme() string {
	if c.scheme == SchemeTLS {
		return SchemeTLS
	}

	return Scheme
}

// getURL is the internal implementation of GetURL that accepts a resolver interface.
// This allows for easier testing and decoupling from the concrete resolver type.
//
// Parameters:
//   - resolver: The query resolver to use for building the URL query string
//
// Returns:
//   - *url.URL: the URL representing the current configuration.
func (c *Config) getURL(resolver types.ConfigQueryResolver) *url.URL {
	// A zero port selects the default port for the scheme. The URL leaves it
	// out, and a URL without a port reads back as port 0.
	result := &url.URL{
		Scheme:     c.brokerScheme(),
		Host:       util.JoinURLHostPort(c.Host, c.Port),
		ForceQuery: true, // Always include the query string separator (?)
		// SetURL strips the path's first slash, so a topic's own leading slash,
		// which makes it a distinct MQTT topic, follows the path's slash.
		Path:     "/" + c.Topic,
		RawQuery: format.BuildQuery(resolver),
	}

	// Add user credentials if authentication is configured. A password may
	// stand alone, since MQTT v5 allows a password without a username.
	if c.Password != "" {
		result.User = url.UserPassword(c.Username, c.Password)
	} else if c.Username != "" {
		result.User = url.User(c.Username)
	}

	return result
}

// prepare applies the defaults and checks the config.
//
// A config without a host gets [DefaultHost], and one without a port gets
// [DefaultTLSPort] for mqtts and [DefaultPort] otherwise, as a URL without them
// does. The checks then cover the host, the topic, the QoS, and the port.
// Reading a service URL and each send run it, so a config built in code
// reaches the broker its URL would, and fails before the connection is opened
// or a message is published.
//
// Returns:
//   - error: an error wrapping [util.ErrInvalidHost] for a host that holds a
//     port or brackets. [ErrTopicRequired] for an empty topic, and
//     [ErrInvalidTopic] for a topic an MQTT topic name cannot be.
//     [ErrInvalidQoS], also wrapping [format.ErrInvalidEnumValue], for a QoS
//     that is not one of its values.
//     An error wrapping [util.ErrInvalidPort] for a port outside 1 to 65535.
//     Otherwise nil.
func (c *Config) prepare() error {
	if c.Host == "" {
		c.Host = DefaultHost
	}

	if c.Port == 0 {
		c.Port = DefaultPort
		if c.brokerScheme() == SchemeTLS {
			c.Port = DefaultTLSPort
		}
	}

	if err := util.ValidateHostName(c.Host); err != nil {
		return fmt.Errorf("host: %w", err)
	}

	if c.Topic == "" {
		return ErrTopicRequired
	}

	// MQTT forbids wildcards and the null character in the topic name of a
	// PUBLISH packet, so a broker would reject every message to such a topic.
	if strings.ContainsAny(c.Topic, "+#\x00") {
		return fmt.Errorf("%w: %q must not contain +, #, or a null character", ErrInvalidTopic, c.Topic)
	}

	// A topic name is a UTF-8 string with a two-byte length prefix
	// ([MQTT-3.3.2-1]).
	if !utf8.ValidString(c.Topic) {
		return fmt.Errorf("%w: %q is not valid UTF-8", ErrInvalidTopic, c.Topic)
	}

	if len(c.Topic) > maxTopicLength {
		return fmt.Errorf("%w: %d bytes is longer than %d", ErrInvalidTopic, len(c.Topic), maxTopicLength)
	}

	if err := format.CheckEnumValue(QoSValues.Enum, int(c.QoS)); err != nil {
		return fmt.Errorf("QoS: %w: %w", ErrInvalidQoS, err)
	}

	if err := util.ValidatePort(c.Port); err != nil {
		return err
	}

	return nil
}

// setURL is the internal implementation of SetURL that accepts a resolver interface.
// It extracts all configuration values from the URL including:
//   - User credentials from the URL user info (username:password@host)
//   - Host and port from the URL host component, with an IPv6 host in brackets
//     and [DefaultHost] when the URL has no host
//   - Topic from the URL path (with leading slash removed)
//   - Additional options from URL query parameters
//
// Parameters:
//   - resolver: The query resolver to use for parsing query parameters
//   - serviceURL: The configuration URL to parse
//
// Returns:
//   - error: an error wrapping a host error such as [util.ErrInvalidHost] for
//     a host the URL cannot hold. A port that is not 1
//     to 65535 returns an error wrapping [util.ErrInvalidPort]. A URL with a
//     fragment returns [ErrInvalidTopic]. A malformed query returns
//     [util.ErrInvalidQuery], and an invalid query parameter returns its error.
//     A config that fails its checks returns their error, such as
//     [ErrTopicRequired].
func (c *Config) setURL(resolver types.ConfigQueryResolver, serviceURL *url.URL) error {
	c.scheme = serviceURL.Scheme

	// Extract authentication credentials from URL user info if present
	if serviceURL.User != nil {
		// Get the password (may be empty if only username is provided)
		password, _ := serviceURL.User.Password()
		c.Password = password
		c.Username = serviceURL.User.Username()
	} else {
		// Clear credentials if not present in URL
		c.Password = ""
		c.Username = ""
	}

	// Extract the hostname and port from the URL. An IPv6 host must be in brackets.
	// A URL without a port reads as port 0, which the defaults turn into the
	// port for the scheme.
	host, parsedPort, err := util.SplitURLHostPort(serviceURL)
	if err != nil {
		return fmt.Errorf("parsing host: %w", err)
	}

	c.Host = host

	c.Port = int(parsedPort)

	// Extract topic from URL path, removing the leading slash
	// Example: "/notifications/alerts" becomes "notifications/alerts"
	c.Topic = strings.TrimPrefix(serviceURL.Path, "/")

	// An unescaped # starts the URL fragment, which would cut the intended topic
	// short. A # in a topic is a wildcard, which a topic name must not contain.
	if serviceURL.Fragment != "" {
		return fmt.Errorf("%w: %q must not contain #", ErrInvalidTopic, c.Topic+"#"+serviceURL.Fragment)
	}

	// Process all query parameters through the resolver
	// This handles optional parameters like qos, retained, clientid, etc.
	query, err := util.ParseURLQuery(serviceURL)
	if err != nil {
		return err
	}

	if err := format.ApplyQuery(resolver, query); err != nil {
		return err
	}

	return c.prepare()
}

// IsValid reports whether the QoS value is one of the MQTT QoS levels.
//
// MQTT supports QoS levels 0 (at most once), 1 (at least once), and 2 (exactly
// once), the values [QoSValues] names. Any other value is invalid, and reading
// a service URL rejects it.
//
// Returns:
//   - bool: true if q is between QoSAtMostOnce (0) and QoSExactlyOnce (2)
//     inclusive.
func (q QoS) IsValid() bool {
	return format.CheckEnumValue(QoSValues.Enum, int(q)) == nil
}

// String returns the human-readable name of the QoS level.
// This implements the fmt.Stringer interface for pretty printing.
//
// Returns:
//   - string: one of "AtMostOnce", "AtLeastOnce", or "ExactlyOnce", or
//     "Invalid" for any other value.
func (q QoS) String() string {
	return QoSValues.Enum.Print(int(q))
}
