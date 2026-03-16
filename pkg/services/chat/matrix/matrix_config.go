package matrix

import (
	"fmt"
	"net/url"

	"github.com/nicholas-fedor/shoutrrr/pkg/format"
	"github.com/nicholas-fedor/shoutrrr/pkg/services/standard"
	"github.com/nicholas-fedor/shoutrrr/pkg/types"
)

// Config is the configuration for the matrix service.
type Config struct {
	standard.EnumlessConfig

	User string `desc:"Username or empty when using access token" optional:"" url:"user"`

	Password   string   `desc:"Password or access token"                 url:"password"`
	DisableTLS bool     `                                                               default:"No" key:"disableTLS"`
	Host       string   `                                                url:"host"`
	Rooms      []string `desc:"Room aliases, or with ! prefix, room IDs"                             key:"rooms,room" optional:""`
	Title      string   `                                                               default:""   key:"title"`
}

// GetURL returns a URL representation of it's current field values.
func (c *Config) GetURL() *url.URL {
	resolver := format.NewPropKeyResolver(c)

	return c.getURL(&resolver)
}

// SetURL updates a ServiceConfig from a URL representation of it's field values.
func (c *Config) SetURL(serviceURL *url.URL) error {
	resolver := format.NewPropKeyResolver(c)

	return c.setURL(&resolver, serviceURL)
}

func (c *Config) getURL(resolver types.ConfigQueryResolver) *url.URL {
	return &url.URL{
		User:       url.UserPassword(c.User, c.Password),
		Host:       c.Host,
		Scheme:     Scheme,
		ForceQuery: true,
		RawQuery:   format.BuildQuery(resolver),
	}
}

func (c *Config) setURL(resolver types.ConfigQueryResolver, serviceURL *url.URL) error {
	// Skip credential validation for dummy URLs used in docs generation
	if serviceURL.Host == "dummy.com" {
		c.User = serviceURL.User.Username()
		if password, ok := serviceURL.User.Password(); ok {
			c.Password = password
		}

		c.Host = serviceURL.Host

		return nil
	}

	c.User = serviceURL.User.Username()

	password, ok := serviceURL.User.Password()
	if !ok {
		return ErrMissingCredentials
	}

	c.Password = password
	c.Host = serviceURL.Host

	if c.Host == "" {
		return ErrMissingHost
	}

	for key, vals := range serviceURL.Query() {
		if err := resolver.Set(key, vals[0]); err != nil {
			return fmt.Errorf("setting query parameter %q to %q: %w", key, vals[0], err)
		}
	}

	for r, room := range c.Rooms {
		// If room does not begin with a '#' let's prepend it
		if room != "" && room[0] != '#' && room[0] != '!' {
			c.Rooms[r] = "#" + room
		}
	}

	return nil
}
