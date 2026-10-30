package teams

import (
	"net/url"
	"slices"
	"strings"

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
// Parameters:
//   - serviceURL: the service URL.
//
// Returns:
//   - error: an error wrapping [util.ErrInvalidQuery] for a malformed query,
//     or a query prop error. The webhook URL is validated when sending.
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
// resolver, applying only the config keys.
//
// Parameters:
//   - resolver: the resolver that sets the config props from the query.
//   - serviceURL: the service URL.
//
// Returns:
//   - error: an error wrapping [util.ErrInvalidQuery] for a malformed query,
//     or a query prop error.
func (c *Config) setURL(resolver types.ConfigQueryResolver, serviceURL *url.URL) error {
	c.Color = ""
	c.Title = ""
	c.Host = ""

	query, err := util.ParseURLQuery(serviceURL)
	if err != nil {
		return err
	}

	// The query also carries the workflow webhook's own parameters, such as
	// sig, so only the config keys apply.
	configKeys := resolver.QueryFields()
	configQuery := url.Values{}

	for key, vals := range query {
		if slices.Contains(configKeys, strings.ToLower(key)) {
			configQuery[key] = vals
		}
	}

	return format.ApplyQuery(resolver, configQuery)
}
