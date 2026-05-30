package teams

import (
	"errors"
	"fmt"
	"net/url"

	"github.com/nicholas-fedor/shoutrrr/pkg/format"
	"github.com/nicholas-fedor/shoutrrr/pkg/services/standard"
	"github.com/nicholas-fedor/shoutrrr/pkg/types"
)

// Config represents the configuration for the Teams service.
type Config struct {
	standard.EnumlessConfig

	Host  string `desc:"The full Power Automate workflow incoming webhook URL"                   key:"host"`
	Title string `desc:"Title displayed as a bold header in the Adaptive Card"                   key:"title" optional:""`
	Color string `desc:"Hex color code for the title text and card accent (e.g. FF0000 for red)" key:"color" optional:""`
}

// Scheme is the identifier for the Teams service protocol.
const Scheme = "teams"

// GetURL constructs a URL from the Config fields.
func (c *Config) GetURL() *url.URL {
	resolver := format.NewPropKeyResolver(c)

	return c.getURL(&resolver)
}

// SetURL updates the Config from a URL.
func (c *Config) SetURL(serviceURL *url.URL) error {
	resolver := format.NewPropKeyResolver(c)

	return c.setURL(&resolver, serviceURL)
}

func (c *Config) getURL(resolver types.ConfigQueryResolver) *url.URL {
	return &url.URL{
		Scheme:     Scheme,
		Host:       "",
		ForceQuery: true,
		RawQuery:   format.BuildQuery(resolver),
	}
}

func (c *Config) setURL(resolver types.ConfigQueryResolver, serviceURL *url.URL) error {
	c.Color = ""
	c.Title = ""
	c.Host = ""

	for key, vals := range serviceURL.Query() {
		if len(vals) > 0 && vals[0] != "" {
			if err := resolver.Set(key, vals[0]); err != nil {
				if errors.Is(err, format.ErrInvalidConfigKey) {
					continue
				}

				return fmt.Errorf("setting config value for key %q: %w", key, err)
			}
		}
	}

	return nil
}
