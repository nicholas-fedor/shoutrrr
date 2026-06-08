package zulip

import (
	"net/url"

	"github.com/nicholas-fedor/shoutrrr/pkg/format"
	"github.com/nicholas-fedor/shoutrrr/pkg/services/standard"
	"github.com/nicholas-fedor/shoutrrr/pkg/types"
)

// MessageType indicates the type of Zulip message to send.
type MessageType string

// Config for the zulip service.
type Config struct {
	standard.EnumlessConfig

	BotMail      string      `desc:"Bot e-mail address"                         url:"user"`
	BotKey       string      `desc:"API key"                                    url:"pass"`
	Host         string      `desc:"API server hostname (with optional port)"   url:"host"`
	Type         MessageType `desc:"Message type (channel or direct)"                      key:"type"           optional:""`
	Stream       string      `desc:"Target stream name"                                    key:"stream"         optional:""`
	Topic        string      `desc:"Stream topic"                                          key:"topic"          optional:""`
	Title        string      `desc:"Notification title prepended to message"               key:"title"          optional:""`
	To           string      `desc:"Comma-separated user IDs or emails for DMs"            key:"to"             optional:""`
	ReadBySender bool        `desc:"Mark the message read by its sender"                   key:"read_by_sender" optional:"" default:"No"`
}

const (
	// MessageTypeChannel sends a message to a channel (stream).
	MessageTypeChannel MessageType = "channel"
	// MessageTypeDirect sends a direct message to users.
	MessageTypeDirect MessageType = "direct"
)

// Scheme is the identifying part of this service's configuration URL.
const Scheme = "zulip"

// Clone creates a copy of the Config.
func (c *Config) Clone() *Config {
	return &Config{
		BotMail:      c.BotMail,
		BotKey:       c.BotKey,
		Host:         c.Host,
		Type:         c.Type,
		Stream:       c.Stream,
		Topic:        c.Topic,
		Title:        c.Title,
		To:           c.To,
		ReadBySender: c.ReadBySender,
	}
}

// GetURL returns a URL representation of its current field values.
func (c *Config) GetURL() *url.URL {
	resolver := format.NewPropKeyResolver(c)

	return c.getURL(&resolver)
}

// SetURL updates a ServiceConfig from a URL representation of its field values.
func (c *Config) SetURL(serviceURL *url.URL) error {
	resolver := format.NewPropKeyResolver(c)

	return c.setURL(&resolver, serviceURL)
}

// getURL constructs a URL from the Config's fields using the provided resolver.
func (c *Config) getURL(_ types.ConfigQueryResolver) *url.URL {
	query := &url.Values{}
	if c.Stream != "" {
		query.Set("stream", c.Stream)
	}

	if c.Topic != "" {
		query.Set("topic", c.Topic)
	}

	if c.Title != "" {
		query.Set("title", c.Title)
	}

	if c.Type != "" {
		query.Set("type", string(c.Type))
	}

	if c.To != "" {
		query.Set("to", c.To)
	}

	if c.ReadBySender {
		query.Set("read_by_sender", "true")
	}

	return &url.URL{
		User:     url.UserPassword(c.BotMail, c.BotKey),
		Host:     c.Host,
		RawQuery: query.Encode(),
		Scheme:   Scheme,
	}
}

// setURL updates the Config from a URL using the provided resolver.
func (c *Config) setURL(_ types.ConfigQueryResolver, serviceURL *url.URL) error {
	var isSet bool

	c.BotMail = serviceURL.User.Username()
	c.BotKey, isSet = serviceURL.User.Password()
	c.Host = serviceURL.Host

	// Allow dummy URL during documentation generation
	if !isDummyURL(serviceURL) {
		if c.BotMail == "" {
			return ErrMissingBotMail
		}

		if !isSet {
			return ErrMissingAPIKey
		}

		if c.Host == "" {
			return ErrMissingHost
		}
	}

	c.Stream = serviceURL.Query().Get("stream")
	c.Topic = serviceURL.Query().Get("topic")
	c.Title = serviceURL.Query().Get("title")
	c.Type = MessageType(serviceURL.Query().Get("type"))
	c.To = serviceURL.Query().Get("to")
	c.ReadBySender = serviceURL.Query().Get("read_by_sender") == "true"

	return nil
}

// CreateConfigFromURL creates a new Config from a URL for use within the zulip service.
func CreateConfigFromURL(serviceURL *url.URL) (*Config, error) {
	config := Config{}
	err := config.setURL(nil, serviceURL)

	return &config, err
}

// isDummyURL checks if the given URL is the dummy URL used for documentation generation.
// It compares URL components instead of string comparison to avoid issues with
// URL formatting differences.
func isDummyURL(u *url.URL) bool {
	return u.Host == "dummy.com" && u.User.Username() == "dummy"
}
