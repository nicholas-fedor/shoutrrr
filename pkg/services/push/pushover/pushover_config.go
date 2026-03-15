package pushover

import (
	"fmt"
	"net/url"

	"github.com/nicholas-fedor/shoutrrr/pkg/format"
	"github.com/nicholas-fedor/shoutrrr/pkg/types"
)

// Config for the Pushover notification service.
type Config struct {
	Token    string   `desc:"API Token/Key" url:"pass"`
	User     string   `desc:"User Key"      url:"host"`
	Devices  []string `                                key:"devices"  optional:""`
	Priority int8     `                                key:"priority"             default:"0"`
	Title    string   `                                key:"title"    optional:""`
}

// Scheme is the identifying part of this service's configuration URL.
const Scheme = "pushover"

// Enums returns the fields that should use a corresponding EnumFormatter to Print/Parse their values.
func (c *Config) Enums() map[string]types.EnumFormatter {
	return map[string]types.EnumFormatter{}
}

// GetURL returns a URL representation of its current field values.
func (c *Config) GetURL() *url.URL {
	resolver := format.NewPropKeyResolver(c)

	return c.getURL(&resolver)
}

// SetURL updates the Config from a URL representation of its field values.
func (c *Config) SetURL(serviceURL *url.URL) error {
	resolver := format.NewPropKeyResolver(c)

	return c.setURL(&resolver, serviceURL)
}

// getURL constructs a URL from the Config's fields using the provided resolver.
func (c *Config) getURL(resolver types.ConfigQueryResolver) *url.URL {
	return &url.URL{
		User:       url.UserPassword("Token", c.Token),
		Host:       c.User,
		Scheme:     Scheme,
		ForceQuery: true,
		RawQuery:   format.BuildQuery(resolver),
	}
}

// setURL updates the Config from a URL using the provided resolver.
func (c *Config) setURL(resolver types.ConfigQueryResolver, serviceURL *url.URL) error {
	password, _ := serviceURL.User.Password()
	c.User = serviceURL.Host
	c.Token = password

	for key, vals := range serviceURL.Query() {
		if err := resolver.Set(key, vals[0]); err != nil {
			return fmt.Errorf("setting query parameter %q to %q: %w", key, vals[0], err)
		}
	}

	if serviceURL.String() != "pushover://dummy@dummy.com" {
		if len(c.User) < 1 {
			return ErrUserMissing
		}

		if len(c.Token) < 1 {
			return ErrTokenMissing
		}
	}

	return nil
}
