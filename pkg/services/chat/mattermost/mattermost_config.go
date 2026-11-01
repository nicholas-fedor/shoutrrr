package mattermost

import (
	"net/url"
	"strings"

	"github.com/nicholas-fedor/shoutrrr/pkg/format"
	"github.com/nicholas-fedor/shoutrrr/pkg/services/standard"
	"github.com/nicholas-fedor/shoutrrr/pkg/types"
	"github.com/nicholas-fedor/shoutrrr/pkg/util"
)

// Config holds all configuration information for the Mattermost service.
type Config struct {
	// EnumlessConfig provides the empty enum set, since Mattermost has no enum
	// fields.
	standard.EnumlessConfig

	// UserName overrides the webhook's user name. It is written as the
	// userinfo of the service URL.
	UserName string `desc:"Override webhook user" optional:"" url:"user"`
	// Icon overrides the webhook's icon, as a URL when it starts with http://
	// or https:// and as an emoji name otherwise.
	Icon string `default:"" desc:"Use emoji or URL as icon (based on presence of http(s):// prefix)" key:"icon,icon_emoji,icon_url" optional:""`
	// Title is prepended to the message on its own line.
	Title string `default:"" desc:"Notification title, optionally set by the sender" key:"title"`
	// Channel overrides the webhook's channel. It is the second path segment.
	Channel string `desc:"Override webhook channel" optional:"" url:"path2"`
	// Host is the Mattermost server host, with an optional port.
	Host string `desc:"Mattermost server host" url:"host,port"`
	// Token is the incoming webhook token. It is the first path segment.
	Token string `desc:"Webhook token" sensitive:"true" url:"path1"`
	// DisableTLS sends the webhook request over HTTP instead of HTTPS.
	DisableTLS bool `default:"No" desc:"Send over HTTP instead of HTTPS" key:"disabletls"`
	// SkipTLSVerify skips verification of the server's TLS certificate.
	SkipTLSVerify bool `default:"No" desc:"Skip TLS certificate verification" key:"skiptlsverify"`
}

// Scheme is the identifying part of this service's configuration URL.
const Scheme = "mattermost"

// CreateConfigFromURL creates a new Config instance from a URL representation.
//
// Parameters:
//   - serviceURL: the service URL.
//
// Returns:
//   - *Config: the configuration.
//   - error: the error from reading the URL, such as [ErrHostMissing].
func CreateConfigFromURL(serviceURL *url.URL) (*Config, error) {
	// Config has embedded EnumlessConfig and URL-tagged fields are set via SetURL
	config := &Config{}
	if err := config.SetURL(serviceURL); err != nil {
		return nil, err
	}

	return config, nil
}

// GetURL returns a URL representation of the Config's current field values.
//
// Returns:
//   - *url.URL: the service URL.
func (c *Config) GetURL() *url.URL {
	resolver := format.NewPropKeyResolver(c)

	return c.getURL(&resolver) // Pass pointer to resolver
}

// SetURL updates the Config from a URL representation of its field values.
//
// Parameters:
//   - serviceURL: the service URL.
//
// Returns:
//   - error: the error from reading the URL, such as [ErrHostMissing].
func (c *Config) SetURL(serviceURL *url.URL) error {
	resolver := format.NewPropKeyResolver(c)

	return c.setURL(&resolver, serviceURL) // Pass pointer to resolver
}

// getURL constructs a URL from the Config's fields using the provided resolver.
//
// Parameters:
//   - resolver: the resolver that builds the query from the config props.
//
// Returns:
//   - *url.URL: the service URL, with the token and the channel, when set, as
//     its path.
func (c *Config) getURL(resolver types.ConfigQueryResolver) *url.URL {
	paths := []string{"", c.Token, c.Channel}
	if c.Channel == "" {
		paths = paths[:2]
	}

	var user *url.Userinfo
	if c.UserName != "" {
		user = url.User(c.UserName)
	}

	// URL is constructed with only the fields needed for service URL representation
	return &url.URL{
		User:       user,
		Host:       c.Host,
		Path:       strings.Join(paths, "/"),
		Scheme:     Scheme,
		ForceQuery: false,
		RawQuery:   format.BuildQuery(resolver),
	}
}

// setURL updates the Config from a URL using the provided resolver.
//
// Parameters:
//   - resolver: the resolver that sets the query parameters.
//   - serviceURL: the service URL.
//
// Returns:
//   - error: [ErrHostMissing] for a URL without a host, [ErrNotEnoughArguments]
//     for a path without a token, an error wrapping [util.ErrInvalidQuery] for
//     a malformed query, or a query parameter error.
func (c *Config) setURL(resolver types.ConfigQueryResolver, serviceURL *url.URL) error {
	// The path holds the token and an optional channel.
	path := strings.Split(strings.Trim(serviceURL.Path, "/"), "/")

	// The host and token are checked on a copy first, so a URL without them
	// leaves the config unchanged.
	next := *c
	next.Host = serviceURL.Host
	next.Token = path[0]

	if err := next.validate(); err != nil {
		return err
	}

	c.Host = next.Host
	c.UserName = serviceURL.User.Username()
	c.Token = next.Token
	// A URL without a channel clears one an earlier URL set.
	c.Channel = ""

	if len(path) > 1 {
		c.Channel = path[1]
	}

	query, err := util.ParseURLQuery(serviceURL)
	if err != nil {
		return err
	}

	return format.ApplyQuery(resolver, query)
}

// validate checks that the config names a webhook, which needs a server host
// and a token. Reading a service URL and each send run it, so a config built
// in code fails before a request.
//
// Returns:
//   - error: [ErrHostMissing] for a host without a hostname, such as an empty
//     host or a port alone, [ErrNotEnoughArguments] for a config without a
//     token, or nil.
func (c *Config) validate() error {
	// A Mattermost server has no default host.
	if (&url.URL{Host: c.Host}).Hostname() == "" {
		return ErrHostMissing
	}

	if c.Token == "" {
		return ErrNotEnoughArguments
	}

	return nil
}
