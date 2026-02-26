package zulip

import (
	"errors"
	"net/url"

	"github.com/nicholas-fedor/shoutrrr/pkg/format"
	"github.com/nicholas-fedor/shoutrrr/pkg/services/standard"
	"github.com/nicholas-fedor/shoutrrr/pkg/types"
)

// Scheme is the identifying part of this service's configuration URL.
const Scheme = "zulip"

// Static errors for configuration validation.
var (
	ErrMissingBotMail = errors.New("bot mail missing from config URL")
	ErrMissingAPIKey  = errors.New("API key missing from config URL")
	ErrMissingHost    = errors.New("host missing from config URL")
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

// GetURL returns a URL representation of its current field values.
func (config *Config) GetURL() *url.URL {
	resolver := format.NewPropKeyResolver(config)

	return config.getURL(&resolver)
}

// SetURL updates a ServiceConfig from a URL representation of its field values.
func (config *Config) SetURL(url *url.URL) error {
	resolver := format.NewPropKeyResolver(config)

	return config.setURL(&resolver, url)
}

// getURL constructs a URL from the Config's fields using the provided resolver.
func (config *Config) getURL(_ types.ConfigQueryResolver) *url.URL {
	query := &url.Values{}
	if config.Stream != "" {
		query.Set("stream", config.Stream)
	}

	if config.Topic != "" {
		query.Set("topic", config.Topic)
	}

	return &url.URL{
		User:     url.UserPassword(config.BotMail, config.BotKey),
		Host:     config.Host,
		RawQuery: query.Encode(),
		Scheme:   Scheme,
	}
}

// setURL updates the Config from a URL using the provided resolver.
func (config *Config) setURL(_ types.ConfigQueryResolver, serviceURL *url.URL) error {
	var isSet bool

	config.BotMail = serviceURL.User.Username()
	config.BotKey, isSet = serviceURL.User.Password()
	config.Host = serviceURL.Hostname()

	if serviceURL.String() != "zulip://dummy@dummy.com" {
		if config.BotMail == "" {
			return ErrMissingBotMail
		}

		if !isSet {
			return ErrMissingAPIKey
		}

		if config.Host == "" {
			return ErrMissingHost
		}
	}

	config.Stream = serviceURL.Query().Get("stream")
	config.Topic = serviceURL.Query().Get("topic")

	return nil
}

// Clone creates a copy of the Config.
func (config *Config) Clone() *Config {
	return &Config{
		BotMail: config.BotMail,
		BotKey:  config.BotKey,
		Host:    config.Host,
		Stream:  config.Stream,
		Topic:   config.Topic,
	}
}

// CreateConfigFromURL creates a new Config from a URL for use within the zulip service.
func CreateConfigFromURL(serviceURL *url.URL) (*Config, error) {
	config := Config{}
	err := config.setURL(nil, serviceURL)

	return &config, err
}
