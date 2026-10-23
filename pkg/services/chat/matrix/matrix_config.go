package matrix

import (
	"fmt"
	"net/url"

	"github.com/nicholas-fedor/shoutrrr/pkg/format"
	"github.com/nicholas-fedor/shoutrrr/pkg/services/standard"
	"github.com/nicholas-fedor/shoutrrr/pkg/types"
	"github.com/nicholas-fedor/shoutrrr/pkg/util"
)

// Config is the configuration for the matrix service.
type Config struct {
	// EnumlessConfig provides the empty enum set, since Matrix has no enum
	// fields.
	standard.EnumlessConfig

	// User is the user name for a password login, or empty to use Password
	// as an access token.
	User string `desc:"Username or empty when using access token" optional:"" url:"user"`

	// Password is the user's password, or the access token when User is
	// empty.
	Password string `desc:"Password or access token" sensitive:"true" url:"password"`
	// Host is the homeserver host, with an optional port.
	Host string `url:"host"`
	// Title is prepended to the message, followed by a blank line.
	Title string `default:"" key:"title"`
	// Rooms lists the room aliases, or room IDs starting with !, to send to.
	// Without rooms the message goes to every joined room.
	Rooms []string `desc:"Room aliases, or with ! prefix, room IDs" key:"rooms,room" optional:""`
	// DisableTLS connects to the homeserver over HTTP instead of HTTPS.
	DisableTLS bool `default:"No" key:"disableTLS"`
}

// GetURL returns a URL representation of it's current field values.
//
// Returns:
//   - *url.URL: the service URL.
func (c *Config) GetURL() *url.URL {
	resolver := format.NewPropKeyResolver(c)

	return c.getURL(&resolver)
}

// SetURL updates a ServiceConfig from a URL representation of it's field values.
//
// Parameters:
//   - serviceURL: the service URL.
//
// Returns:
//   - error: the error from reading the URL, such as [ErrMissingCredentials]
//     or [ErrMissingHost].
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
//   - *url.URL: the service URL, with the user and password as its userinfo.
func (c *Config) getURL(resolver types.ConfigQueryResolver) *url.URL {
	return &url.URL{
		User:       url.UserPassword(c.User, c.Password),
		Host:       c.Host,
		Scheme:     Scheme,
		ForceQuery: true,
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
//   - error: [ErrMissingCredentials] for a URL without a password,
//     [ErrMissingHost] for a URL without a host, an error wrapping
//     [util.ErrInvalidQuery] for a malformed query, or a query prop error.
func (c *Config) setURL(resolver types.ConfigQueryResolver, serviceURL *url.URL) error {
	// Skip credential validation and query parameter parsing for dummy URLs
	// used in docs generation. Query params (rooms, title, disableTLS) are
	// intentionally ignored for dummy.com to avoid validation errors.
	if serviceURL.Hostname() == "dummy.com" {
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

	query, err := util.ParseURLQuery(serviceURL)
	if err != nil {
		return err
	}

	for key, vals := range query {
		if err := resolver.Set(key, vals[0]); err != nil {
			return fmt.Errorf("setting query parameter %q: %w", key, err)
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
