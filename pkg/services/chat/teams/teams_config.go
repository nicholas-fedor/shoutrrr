package teams

import (
	"net/url"

	"github.com/nicholas-fedor/shoutrrr/pkg/format"
	"github.com/nicholas-fedor/shoutrrr/pkg/services/standard"
	"github.com/nicholas-fedor/shoutrrr/pkg/types"
	"github.com/nicholas-fedor/shoutrrr/pkg/util"
)

// Config represents the configuration for the Teams service.
type Config struct {
	// EnumlessConfig provides the empty enum set, since Teams has no enum
	// fields.
	standard.EnumlessConfig

	// Host is the full Power Automate workflow webhook URL.
	Host string `desc:"The full Power Automate workflow incoming webhook URL" key:"host" sensitive:"true"`
	// Title is shown as a bold header above the message in the card.
	Title string `desc:"Title displayed as a bold header in the Adaptive Card" key:"title" optional:""`
	// Color is the title's text color.
	Color string `desc:"Title text color (accent, good, warning, attention, dark, light, or default)" key:"color" optional:""`
}

// Scheme is the identifier for the Teams service protocol.
const Scheme = "teams"

// GetURL constructs a URL from the Config fields.
//
// Returns:
//   - *url.URL: the service URL.
func (c *Config) GetURL() *url.URL {
	resolver := format.NewPropKeyResolver(c)

	return c.getURL(&resolver)
}

// SetURL updates the Config from a URL.
//
// A query key that is not a Teams setting is rejected. That includes a
// parameter of a webhook URL that was not percent-encoded, such as sig, which
// would otherwise be lost from the webhook URL.
//
// Parameters:
//   - serviceURL: the service URL.
//
// Returns:
//   - error: an error wrapping [util.ErrInvalidQuery] for a malformed query,
//     a query prop error, such as [format.ErrInvalidConfigKey] for an unknown
//     key, or an error wrapping [ErrInvalidWebhookURL] for a webhook URL that
//     Shoutrrr cannot send to.
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
//   - *url.URL: the service URL, with the webhook URL in its host query
//     parameter.
func (c *Config) getURL(resolver types.ConfigQueryResolver) *url.URL {
	return &url.URL{
		Scheme:     Scheme,
		Host:       "",
		ForceQuery: true,
		RawQuery:   format.BuildQuery(resolver),
	}
}

// setURL updates the Config from the query of a URL using the provided
// resolver, and then checks it.
//
// Parameters:
//   - resolver: the resolver that sets the config props from the query.
//   - serviceURL: the service URL.
//
// Returns:
//   - error: an error wrapping [util.ErrInvalidQuery] for a malformed query,
//     a query prop error, or an error wrapping [ErrInvalidWebhookURL] for a
//     webhook URL that Shoutrrr cannot send to.
func (c *Config) setURL(resolver types.ConfigQueryResolver, serviceURL *url.URL) error {
	c.Color = ""
	c.Title = ""
	c.Host = ""

	query, err := util.ParseURLQuery(serviceURL)
	if err != nil {
		return err
	}

	if err := format.ApplyQuery(resolver, query); err != nil {
		return err
	}

	return c.validate()
}

// validate checks the config's webhook URL when a URL is read and on each send.
//
// A config without a webhook URL passes, since a send param can set one. A send
// without one fails with [ErrMissingHost].
//
// Returns:
//   - error: an error wrapping [ErrInvalidWebhookURL] for a webhook URL that
//     Shoutrrr cannot send to, or nil.
func (c *Config) validate() error {
	if c.Host == "" {
		return nil
	}

	return ValidateWebhookURL(c.Host)
}
