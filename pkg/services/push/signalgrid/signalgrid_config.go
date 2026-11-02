package signalgrid

import (
	"fmt"
	"net/url"
	"strings"

	"github.com/nicholas-fedor/shoutrrr/pkg/format"
	"github.com/nicholas-fedor/shoutrrr/pkg/types"
	"github.com/nicholas-fedor/shoutrrr/pkg/util"
)

// Config holds settings for the Signalgrid notification service.
type Config struct {
	// ClientKey is the Signalgrid client key used to authenticate Push API requests.
	ClientKey string `desc:"Signalgrid client key" sensitive:"true" url:"user"`
	// Channel is the destination channel token.
	Channel string `desc:"Signalgrid channel token" sensitive:"true" url:"host"`
	// Title is an optional notification title. It is omitted from the API request when empty.
	Title string `desc:"Notification title" key:"title" optional:""`
	// Type is the visual severity of the notification. Default: INFO.
	Type notificationType `default:"INFO" desc:"Notification severity (CRIT, WARN, INFO, SUCCESS)" key:"type"`
	// Critical delivers the notification as a critical alert when true.
	Critical bool `default:"No" desc:"Deliver as a critical alert" key:"critical"`
}

// Service identity.
const (
	// Scheme identifies this service in configuration URLs.
	Scheme = "signalgrid"
)

// Enums returns the fields that use an EnumFormatter for their values.
//
// Returns:
//   - A map of config field names to their enum formatters.
func (*Config) Enums() map[string]types.EnumFormatter {
	return map[string]types.EnumFormatter{
		"Type": NotificationType.Enum,
	}
}

// GetURL returns a URL representation of the current configuration.
//
// Returns:
//   - The service URL encoding the current field values.
func (c *Config) GetURL() *url.URL {
	resolver := format.NewPropKeyResolver(c)

	return c.getURL(&resolver)
}

// SetURL updates the configuration from a URL representation.
//
// Parameters:
//   - serviceURL: The service URL to parse.
//
// Returns:
//   - An error if the URL is invalid or required fields are missing.
func (c *Config) SetURL(serviceURL *url.URL) error {
	resolver := format.NewPropKeyResolver(c)

	return c.setURL(&resolver, serviceURL)
}

// getURL constructs a service URL from the current configuration.
//
// Parameters:
//   - resolver: Resolver used to encode query parameters from config fields.
//
// Returns:
//   - The service URL with the client key as userinfo and the channel as host.
func (c *Config) getURL(resolver types.ConfigQueryResolver) *url.URL {
	return &url.URL{
		Scheme:   Scheme,
		User:     url.User(c.ClientKey),
		Host:     c.Channel,
		RawQuery: format.BuildQuery(resolver),
	}
}

// setURL updates the configuration from a service URL.
//
// Parameters:
//   - resolver: Resolver used to apply query parameters to config fields.
//   - serviceURL: The service URL to parse.
//
// Returns:
//   - An error if the query is malformed ([util.ErrInvalidQuery]), a query
//     parameter is invalid, or the config's checks fail, such as for a channel
//     token with a colon or brackets ([util.ErrInvalidHost]).
func (c *Config) setURL(resolver types.ConfigQueryResolver, serviceURL *url.URL) error {
	if serviceURL.User != nil {
		c.ClientKey = serviceURL.User.Username()
	} else {
		c.ClientKey = ""
	}

	c.Channel = serviceURL.Host

	query, err := util.ParseURLQuery(serviceURL)
	if err != nil {
		return err
	}

	if err := format.ApplyQuery(resolver, query); err != nil {
		return err
	}

	return c.validate()
}

// validate checks that the config has a client key and a channel token.
// Reading a service URL and each send run it, so a config built in code fails
// before a request.
//
// Returns:
//   - error: [ErrClientKeyMissing] or [ErrChannelMissing] for a missing
//     field, an error wrapping [util.ErrInvalidHost] for a channel token with
//     a colon or brackets, or nil.
func (c *Config) validate() error {
	if c.ClientKey == "" {
		return ErrClientKeyMissing
	}

	if c.Channel == "" {
		return ErrChannelMissing
	}

	// The channel token is the URL host as written, which cannot hold a port
	// or brackets that url.URL.Hostname would strip.
	if strings.ContainsAny(c.Channel, ":[]") {
		return fmt.Errorf("channel token: %w", util.ErrInvalidHost)
	}

	return nil
}
