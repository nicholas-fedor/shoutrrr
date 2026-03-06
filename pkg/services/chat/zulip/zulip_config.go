package zulip

import (
	"net/url"

	"github.com/nicholas-fedor/shoutrrr/pkg/format"
	"github.com/nicholas-fedor/shoutrrr/pkg/services/standard"
	"github.com/nicholas-fedor/shoutrrr/pkg/types"
)

// Config for the zulip service.
type Config struct {
	standard.EnumlessConfig

	BotMail string `desc:"Bot e-mail address"  url:"user"`
	BotKey  string `desc:"API Key"             url:"pass"`
	Host    string `desc:"API server hostname" url:"host,port"`
	Stream  string `                                           description:"Target stream name" key:"stream"      optional:""`
	Topic   string `                                                                            key:"topic,title"             default:""`
}

// Scheme is the identifying part of this service's configuration URL.
const Scheme = "zulip"

// Clone creates a copy of the Config.
func (c *Config) Clone() *Config {
	return &Config{
		BotMail: c.BotMail,
		BotKey:  c.BotKey,
		Host:    c.Host,
		Stream:  c.Stream,
		Topic:   c.Topic,
	}
}

// GetURL returns a URL representation of its current field values.
func (c *Config) GetURL() *url.URL {
	resolver := format.NewPropKeyResolver(c)

	return c.getURL(&resolver)
}

// SetURL updates a ServiceConfig from a URL representation of its field values.
func (c *Config) SetURL(url *url.URL) error {
	resolver := format.NewPropKeyResolver(c)

	return c.setURL(&resolver, url)
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
	c.Host = serviceURL.Hostname()

	if serviceURL.String() != "zulip://dummy@dummy.com" {
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

	return nil
}

// CreateConfigFromURL creates a new Config from a URL for use within the zulip service.
func CreateConfigFromURL(serviceURL *url.URL) (*Config, error) {
	config := Config{}
	err := config.setURL(nil, serviceURL)

	return &config, err
}
