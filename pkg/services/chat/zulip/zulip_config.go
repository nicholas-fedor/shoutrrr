package zulip

import (
	"fmt"
	"net/url"

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
//   - error: [ErrMissingBotMail], [ErrMissingAPIKey], [ErrMissingHost], a
//     query wrapping [util.ErrInvalidQuery], or the error of
//     [format.ApplyQuery], such as an unknown key or a read_by_sender value
//     that is not a boolean.
func (c *Config) setURL(resolver *format.PropKeyResolver, serviceURL *url.URL) error {
	var isSet bool

	c.BotMail = serviceURL.User.Username()
	c.BotKey, isSet = serviceURL.User.Password()
	c.Host = serviceURL.Host

	if c.BotMail == "" {
		return ErrMissingBotMail
	}

	if !isSet {
		return ErrMissingAPIKey
	}

	if c.Host == "" {
		return ErrMissingHost
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

	return format.ApplyQuery(resolver, query)
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
