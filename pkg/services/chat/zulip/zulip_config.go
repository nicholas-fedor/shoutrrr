package zulip

import (
	"fmt"
	"net/url"
	"strings"

	"github.com/nicholas-fedor/shoutrrr/pkg/format"
	"github.com/nicholas-fedor/shoutrrr/pkg/services/standard"
	"github.com/nicholas-fedor/shoutrrr/pkg/types"
	"github.com/nicholas-fedor/shoutrrr/pkg/util"
)

// MessageType indicates the type of Zulip message to send.
type MessageType string

// Config for the zulip service.
type Config struct {
	// EnumlessConfig provides the empty enum set, since Zulip has no enum
	// fields.
	standard.EnumlessConfig

	// BotMail is the bot's e-mail address, written as the URL username.
	BotMail string `desc:"Bot e-mail address" url:"user"`
	// BotKey is the bot's API key, written as the URL password.
	BotKey string `desc:"API key" sensitive:"true" url:"pass"`
	// Host is the Zulip server host, with an optional port.
	Host string `desc:"API server hostname (with optional port)" url:"host"`
	// Type selects a channel or a direct message. Empty sends to a channel.
	Type MessageType `desc:"Message type (channel or direct)" key:"type" optional:""`
	// Stream is the channel that a channel message goes to. A direct message
	// uses it as the recipient list when To is empty.
	Stream string `desc:"Target stream name" key:"stream" optional:""`
	// Topic is the topic of a channel message. A send without a topic uses
	// Title as the topic.
	Topic string `desc:"Stream topic" key:"topic" optional:""`
	// Title is prepended to the message when Topic is also set. A send without
	// a topic uses it as the topic instead.
	Title string `desc:"Notification title prepended to message" key:"title" optional:""`
	// To lists the recipients of a direct message as comma-separated user IDs
	// or e-mail addresses. A list cannot mix the two.
	To string `desc:"Comma-separated user IDs or emails for DMs" key:"to" optional:""`
	// ReadBySender marks the message as read by its sender.
	ReadBySender bool `default:"No" desc:"Mark the message read by its sender" key:"read_by_sender" optional:""`
	// SkipTLSVerify skips verification of the server's TLS certificate.
	SkipTLSVerify bool `default:"No" desc:"Skip TLS certificate verification" key:"skiptlsverify"`
}

// Message types that [Config.Type] accepts.
const (
	// MessageTypeChannel sends a message to a channel (stream).
	MessageTypeChannel MessageType = "channel"
	// MessageTypeDirect sends a direct message to users.
	MessageTypeDirect MessageType = "direct"
)

// Scheme is the identifying part of this service's configuration URL.
const Scheme = "zulip"

// maxLabelLength is the longest DNS label of a host name, in characters.
const maxLabelLength = 63

// Clone creates a copy of the Config.
//
// Returns:
//   - *Config: a copy with the same field values.
func (c *Config) Clone() *Config {
	// Every field is a value, so a struct copy is a full copy.
	clone := *c

	return &clone
}

// GetURL returns a URL representation of its current field values.
//
// Returns:
//   - *url.URL: the service URL, with the bot credentials as userinfo and the
//     options as query parameters.
func (c *Config) GetURL() *url.URL {
	resolver := format.NewPropKeyResolver(c)

	return c.getURL(&resolver)
}

// SetURL updates a ServiceConfig from a URL representation of its field values.
//
// Parameters:
//   - serviceURL: the service URL.
//
// Returns:
//   - error: the reason the URL is not a valid Zulip URL, or nil.
func (c *Config) SetURL(serviceURL *url.URL) error {
	resolver := format.NewPropKeyResolver(c)

	return c.setURL(&resolver, serviceURL)
}

// getURL constructs a URL from the Config's fields.
//
// Parameters:
//   - resolver: the resolver that builds the query from the options. Options
//     at their defaults are left out.
//
// Returns:
//   - *url.URL: the service URL, with the bot credentials as userinfo and the
//     options as query parameters.
func (c *Config) getURL(resolver types.ConfigQueryResolver) *url.URL {
	return &url.URL{
		User:     url.UserPassword(c.BotMail, c.BotKey),
		Host:     c.Host,
		RawQuery: format.BuildQuery(resolver),
		Scheme:   Scheme,
	}
}

// setURL updates the Config from a URL.
//
// Parameters:
//   - resolver: the resolver that sets the options from the query.
//   - serviceURL: the service URL.
//
// Returns:
//   - error: a query wrapping [util.ErrInvalidQuery], or the error of
//     [format.ApplyQuery], such as an unknown key or a read_by_sender value
//     that is not a boolean. Otherwise the error from the config's checks,
//     such as [ErrMissingBotMail].
func (c *Config) setURL(resolver *format.PropKeyResolver, serviceURL *url.URL) error {
	c.BotMail = serviceURL.User.Username()
	c.BotKey, _ = serviceURL.User.Password()

	// A host that splits is stored without an empty port, so a URL ending in
	// a colon reads as the host alone. One that does not split is kept as
	// written for the config's checks to reject.
	c.Host = serviceURL.Host
	if host, port, err := util.SplitURLHost(serviceURL); err == nil {
		c.Host = util.JoinURLHost(host, port)
	}

	query, err := util.ParseURLQuery(serviceURL)
	if err != nil {
		return err
	}

	// The options start from their defaults, so an option the URL leaves out
	// does not keep the value an earlier URL set.
	if err := resolver.SetDefaultProps(c); err != nil {
		return fmt.Errorf("setting default props: %w", err)
	}

	if err := format.ApplyQuery(resolver, query); err != nil {
		return err
	}

	return c.validate()
}

// validate checks the bot's credentials, the host, and the message type.
//
// Zulip authenticates each request with the bot's e-mail address and API key.
// Reading a service URL and each send run it, so a config built in code or
// changed by a param fails before a request.
//
// Returns:
//   - error: [ErrMissingBotMail], [ErrMissingAPIKey], or [ErrMissingHost] for
//     a missing field. A host that is not a host name or address with an
//     optional port gives an error wrapping [ErrInvalidHost], and also
//     [util.ErrInvalidPort] for a port outside 1 to 65535. An unsupported
//     message type gives [ErrInvalidMessageType]. Otherwise nil.
func (c *Config) validate() error {
	if c.BotMail == "" {
		return ErrMissingBotMail
	}

	if c.BotKey == "" {
		return ErrMissingAPIKey
	}

	if c.Host == "" {
		return ErrMissingHost
	}

	if err := validateHost(c.Host); err != nil {
		return fmt.Errorf("%w: %q: %w", ErrInvalidHost, c.Host, err)
	}

	return ValidateMessageType(c.Type)
}

// CreateConfigFromURL creates a new Config from a URL for use within the zulip service.
//
// Parameters:
//   - serviceURL: the service URL.
//
// Returns:
//   - *Config: the config read from the URL. It is partly set when the error
//     is not nil.
//   - error: the reason the URL is not a valid Zulip URL, or nil.
func CreateConfigFromURL(serviceURL *url.URL) (*Config, error) {
	config := Config{}
	resolver := format.NewPropKeyResolver(&config)
	err := config.setURL(&resolver, serviceURL)

	return &config, err
}

// validateHost checks that the host is a host name, an IPv4 address, or a
// bracketed IPv6 address, with an optional port.
//
// The port follows the URL port rule, a number from 1 to 65535.
//
// Parameters:
//   - host: the host, as in a URL authority.
//
// Returns:
//   - error: an error wrapping [util.ErrInvalidPort] for a port outside 1 to
//     65535, [util.ErrInvalidHost] for any other invalid host, or nil.
func validateHost(host string) error {
	name, _, err := util.SplitURLHost(&url.URL{Host: host})
	if err != nil {
		return err
	}

	// SplitURLHost accepts brackets only around an IPv6 address, and an
	// unbracketed host with a colon only before its port.
	if strings.HasPrefix(host, "[") {
		return nil
	}

	// An IPv4 address reads as a host name of digit labels.
	if !isHostName(name) {
		return util.ErrInvalidHost
	}

	return nil
}

// isHostName reports whether name is a host name of one or more DNS labels
// separated by dots. Each label holds 1 to 63 ASCII letters, digits, and
// hyphens, and starts and ends with a letter or digit.
//
// Parameters:
//   - name: the name to check.
//
// Returns:
//   - bool: true for a host name.
func isHostName(name string) bool {
	start := 0

	for i := 0; i <= len(name); i++ {
		if i < len(name) && name[i] != '.' {
			switch c := name[i]; {
			case 'a' <= c && c <= 'z', 'A' <= c && c <= 'Z', '0' <= c && c <= '9', c == '-':
			default:
				return false
			}

			continue
		}

		label := name[start:i]
		if label == "" || len(label) > maxLabelLength || label[0] == '-' || label[len(label)-1] == '-' {
			return false
		}

		start = i + 1
	}

	return true
}
