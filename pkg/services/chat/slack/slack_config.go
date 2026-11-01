package slack

import (
	"net/url"

	"github.com/nicholas-fedor/shoutrrr/pkg/format"
	"github.com/nicholas-fedor/shoutrrr/pkg/services/standard"
	"github.com/nicholas-fedor/shoutrrr/pkg/types"
	"github.com/nicholas-fedor/shoutrrr/pkg/util"
)

// Config for the slack service.
type Config struct {
	// EnumlessConfig provides the empty enum set, since Slack has no enum
	// fields.
	standard.EnumlessConfig

	// BotName overrides the name the message is posted as.
	BotName string `desc:"Bot name" key:"botname,username" optional:"uses bot default"`
	// Icon overrides the icon the message is posted with, as a URL when it
	// starts with http:// or https:// and as an emoji name otherwise.
	Icon string `default:"" desc:"Use emoji or URL as icon (based on presence of http(s):// prefix)" key:"icon,icon_emoji,icon_url" optional:""`
	// Token is the bot API token or the webhook token, written as the
	// userinfo of the service URL.
	Token Token `desc:"API Bot token" sensitive:"true" url:"user,pass"`
	// Color is the left-hand border color of the message's attachments.
	Color string `desc:"Message left-hand border color" key:"color" optional:"default border color"`
	// Title is the message text shown above the attachments.
	Title string `desc:"Prepended text above the message" key:"title" optional:"omitted"`
	// Channel is the channel ID to post to, or "webhook" for a webhook token.
	Channel string `desc:"Channel to send messages to in Cxxxxxxxxxx format" url:"host"`
	// ThreadTS is the ts value of a parent message, so the message is posted
	// as a reply in its thread.
	ThreadTS string `desc:"ts value of the parent message (to send message as reply in thread)" key:"thread_ts" optional:""`
}

// Service identity.
const (
	// Scheme is the identifying part of this service's configuration URL.
	Scheme = "slack"
)

// GetURL returns a URL representation of it's current field values.
//
// Returns:
//   - *url.URL: the service URL.
func (c *Config) GetURL() *url.URL {
	resolver := format.NewPropKeyResolver(c)

	return c.getURL(&resolver)
}

// MaskedURL returns the config URL with the secret part of the token replaced
// by [format.MaskedValue]. The token type stays, so the URL still shows whether
// the token is a bot token or a webhook.
//
// Returns:
//   - *url.URL: the config URL with the token masked.
func (c *Config) MaskedURL() *url.URL {
	serviceURL := c.GetURL()
	if serviceURL.User != nil {
		serviceURL.User = url.UserPassword(serviceURL.User.Username(), format.MaskedValue)
	}

	return serviceURL
}

// SetURL updates a ServiceConfig from a URL representation of it's field values.
//
// Parameters:
//   - serviceURL: the service URL.
//
// Returns:
//   - error: the error from reading the URL, such as [ErrInvalidToken].
func (c *Config) SetURL(serviceURL *url.URL) error {
	resolver := format.NewPropKeyResolver(c)

	return c.setURL(&resolver, serviceURL)
}

// getURL constructs a URL from the Config's fields using the provided resolver.
//
// Parameters:
//   - resolver: the resolver that builds the query from the config props.
//
// Returns:
//   - *url.URL: the service URL, with the token as its userinfo and the
//     channel as its host.
func (c *Config) getURL(resolver types.ConfigQueryResolver) *url.URL {
	return &url.URL{
		User:       c.Token.UserInfo(),
		Host:       c.Channel,
		Scheme:     Scheme,
		ForceQuery: false,
		RawQuery:   format.BuildQuery(resolver),
	}
}

// setURL updates the Config from a URL using the provided resolver.
//
// Parameters:
//   - resolver: the resolver that sets the config props from the query.
//   - serviceURL: the service URL.
//
// Returns:
//   - error: a token error, an error wrapping [util.ErrInvalidQuery] for a
//     malformed query, or a query prop error.
func (c *Config) setURL(resolver types.ConfigQueryResolver, serviceURL *url.URL) error {
	var token string

	if len(serviceURL.Path) > 1 {
		// Reading legacy config URL format
		token = serviceURL.Hostname() + serviceURL.Path
		c.Channel = "webhook"
		c.BotName = serviceURL.User.Username()
	} else {
		token = serviceURL.User.String()
		c.Channel = serviceURL.Hostname()
	}

	if err := c.Token.SetFromProp(token); err != nil {
		return err
	}

	query, err := util.ParseURLQuery(serviceURL)
	if err != nil {
		return err
	}

	return format.ApplyQuery(resolver, query)
}

// validate checks that the config has a token. Reading a service URL rejects a
// malformed token as it parses it. Each send runs this check, so a config built
// in code without a token fails before a request.
//
// Returns:
//   - error: [ErrInvalidToken] for a config without a token, or nil.
func (c *Config) validate() error {
	if c.Token.String() == "" {
		return ErrInvalidToken
	}

	return nil
}

// CreateConfigFromURL to use within the slack service.
//
// Parameters:
//   - serviceURL: the service URL.
//
// Returns:
//   - *Config: the configuration, set as far as the URL could be read.
//   - error: the error from reading the URL, such as [ErrInvalidToken].
func CreateConfigFromURL(serviceURL *url.URL) (*Config, error) {
	config := Config{}
	err := config.SetURL(serviceURL)

	return &config, err
}
