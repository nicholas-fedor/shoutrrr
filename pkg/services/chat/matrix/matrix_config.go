package matrix

import (
	"net/url"
	"time"

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
	Title string `default:"" desc:"Notification title, optionally set by the sender" key:"title"`
	// Rooms lists the room aliases, or room IDs starting with !, to send to.
	// Without rooms the message goes to every joined room.
	Rooms []string `desc:"Room aliases, or with ! prefix, room IDs" key:"rooms,room" optional:""`
	// DisableTLS connects to the homeserver over HTTP instead of HTTPS.
	DisableTLS bool `default:"No" desc:"Connect over HTTP instead of HTTPS" key:"disabletls"`
	// SkipTLSVerify skips verification of the homeserver's TLS certificate.
	SkipTLSVerify bool `default:"No" desc:"Skip TLS certificate verification" key:"skiptlsverify"`
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

// apiScheme returns the URL scheme of the homeserver API.
//
// Returns:
//   - string: http when TLS is disabled, and https otherwise.
func (c *Config) apiScheme() string {
	if c.DisableTLS {
		return schemeHTTP
	}

	return schemeHTTPS
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

// sendBudget returns the send budget that [Service.ServiceTimeout] describes,
// for a config with the send's params applied.
//
// Returns:
//   - time.Duration: the budget for the send.
func (c *Config) sendBudget() time.Duration {
	requests := minRequestsPerSend
	if len(c.Rooms) > 0 {
		requests = requestsPerRoom * len(c.Rooms)
	}

	if c.User != "" {
		requests += loginRequests
	}

	return defaultHTTPTimeout * time.Duration(requests)
}

// setURL updates the Config from a URL using the provided resolver.
//
// Parameters:
//   - resolver: the resolver that sets the config props from the query.
//   - serviceURL: the service URL.
//
// Returns:
//   - error: an error wrapping [util.ErrInvalidQuery] for a malformed query, a
//     query prop error, or the error from the config's checks, such as
//     [ErrMissingCredentials].
func (c *Config) setURL(resolver types.ConfigQueryResolver, serviceURL *url.URL) error {
	c.User = serviceURL.User.Username()
	c.Password, _ = serviceURL.User.Password()
	c.Host = serviceURL.Host

	query, err := util.ParseURLQuery(serviceURL)
	if err != nil {
		return err
	}

	if err := format.ApplyQuery(resolver, query); err != nil {
		return err
	}

	for r, room := range c.Rooms {
		// If room does not begin with a '#' let's prepend it
		if room != "" && room[0] != '#' && room[0] != '!' {
			c.Rooms[r] = "#" + room
		}
	}

	return c.validate()
}

// validate checks that the config has credentials and names a homeserver.
// An empty password or access token cannot authenticate. Reading a service
// URL and each send run it, so a config built in code fails before the login
// or any request.
//
// Returns:
//   - error: [ErrMissingCredentials] for a config without a password or
//     access token, [ErrMissingHost] for a config without a host, or nil.
func (c *Config) validate() error {
	if c.Password == "" {
		return ErrMissingCredentials
	}

	if c.Host == "" {
		return ErrMissingHost
	}

	return nil
}
