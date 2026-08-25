package signalgrid

import (
	"fmt"
	"net/url"

	"github.com/nicholas-fedor/shoutrrr/pkg/format"
	"github.com/nicholas-fedor/shoutrrr/pkg/types"
)

const Scheme = "signalgrid"

type Config struct {
	ClientKey string `desc:"Signalgrid client key" url:"user"`
	Channel   string `desc:"Signalgrid channel token" url:"host"`
	Title     string `key:"title" optional:""`
	Type      string `key:"type" optional:"" default:"INFO"`
	Critical  bool   `key:"critical" optional:"" default:"No"`
}

func (c *Config) Enums() map[string]types.EnumFormatter {
	return map[string]types.EnumFormatter{}
}

func (c *Config) GetURL() *url.URL {
	resolver := format.NewPropKeyResolver(c)

	return c.getURL(&resolver)
}

func (c *Config) SetURL(serviceURL *url.URL) error {
	resolver := format.NewPropKeyResolver(c)

	return c.setURL(&resolver, serviceURL)
}

func (c *Config) getURL(resolver types.ConfigQueryResolver) *url.URL {
	return &url.URL{
		Scheme:     Scheme,
		User:       url.User(c.ClientKey),
		Host:       c.Channel,
		ForceQuery: true,
		RawQuery:   format.BuildQuery(resolver),
	}
}

func (c *Config) setURL(
	resolver types.ConfigQueryResolver,
	serviceURL *url.URL,
) error {
	c.ClientKey = serviceURL.User.Username()
	c.Channel = serviceURL.Host

	for key, vals := range serviceURL.Query() {
		if len(vals) == 0 {
			continue
		}

		if err := resolver.Set(key, vals[0]); err != nil {
			return fmt.Errorf(
				"setting query parameter %q to %q: %w",
				key,
				vals[0],
				err,
			)
		}
	}

	if c.ClientKey == "" {
		return fmt.Errorf("Signalgrid client key is missing")
	}

	if c.Channel == "" {
		return fmt.Errorf("Signalgrid channel token is missing")
	}

	return nil
}
