package matrix

import (
	"net/url"
	"slices"
	"strings"
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

// loginKey identifies the account a password login is made for. Two configs
// with the same key log in to the same account, so they can share its access
// token.
type loginKey struct {
	// host is the homeserver.
	host string
	// user is the user that logs in.
	user string
	// password is the password the login uses.
	password string
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

// loginKey returns the account a password login with this config is made for.
//
// Returns:
//   - loginKey: the homeserver, user, and password.
func (c *Config) loginKey() loginKey {
	return loginKey{host: c.Host, user: c.User, password: c.Password}
}

// prepare trims each room and starts a room without a # or ! sigil with #, so
// it names an alias. It then checks the config. Reading a service URL and
// each send run it, so a send reaches the rooms that its URL names. A config
// built in code also fails its checks before the login or any request. A
// changed room goes into a new slice, so a send never writes to the rooms of
// the config it was copied from.
//
// Returns:
//   - error: the error from the config's checks, such as
//     [ErrMissingCredentials].
func (c *Config) prepare() error {
	var rooms []string

	for index, room := range c.Rooms {
		trimmed := strings.TrimSpace(room)
		if trimmed != "" && !strings.HasPrefix(trimmed, aliasSigil) && !strings.HasPrefix(trimmed, roomIDSigil) {
			trimmed = aliasSigil + trimmed
		}

		if trimmed == room {
			continue
		}

		if rooms == nil {
			rooms = slices.Clone(c.Rooms)
			c.Rooms = rooms
		}

		rooms[index] = trimmed
	}

	return c.validate()
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

	return c.prepare()
}

// validate checks that the config has credentials and names a homeserver.
// An empty password or access token cannot authenticate.
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
