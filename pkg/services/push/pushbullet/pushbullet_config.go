package pushbullet

import (
	"errors"
	"fmt"
	"net/url"
	"strings"

	"github.com/nicholas-fedor/shoutrrr/pkg/format"
	"github.com/nicholas-fedor/shoutrrr/pkg/services/standard"
	"github.com/nicholas-fedor/shoutrrr/pkg/types"
	"github.com/nicholas-fedor/shoutrrr/pkg/util"
)

// Config holds the configuration for the Pushbullet service.
type Config struct {
	// EnumlessConfig provides the empty enum set, since Pushbullet has no
	// enum fields.
	standard.EnumlessConfig

	// Token is the access token, written as the host of the service URL.
	Token string `sensitive:"true" url:"host"`
	// Title is the push title.
	Title string `default:"Shoutrrr notification" desc:"Notification title, optionally set by the sender" key:"title"`

	// Targets lists the devices, e-mail addresses, and #channels to push to,
	// written as the path of the service URL. A URL without a path gives one
	// empty target, which pushes to all of the token user's devices.
	Targets []string `url:"path"`
}

// Scheme is the scheme part of the service configuration URL.
const Scheme = "pushbullet"

// ExpectedTokenLength is the required length for a valid Pushbullet token.
const ExpectedTokenLength = 34

// ErrTokenIncorrectSize indicates that the token has an incorrect size.
var ErrTokenIncorrectSize = errors.New("token has incorrect size")

// GetURL returns a URL representation of the Config's current field values.
//
// Returns:
//   - *url.URL: the service URL.
func (c *Config) GetURL() *url.URL {
	resolver := format.NewPropKeyResolver(c)

	return c.getURL(&resolver)
}

// SetURL updates the Config from a URL representation of its field values.
//
// Parameters:
//   - serviceURL: the service URL.
//
// Returns:
//   - error: the error from reading the URL, such as [ErrTokenIncorrectSize].
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
//   - *url.URL: the service URL, with the token as the host and the targets
//     as the path.
func (c *Config) getURL(resolver types.ConfigQueryResolver) *url.URL {
	return &url.URL{
		Host:       c.Token,
		Path:       "/" + strings.Join(c.Targets, "/"),
		Scheme:     Scheme,
		ForceQuery: false,
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
//   - error: [util.ErrInvalidHost] for an access token with a colon or
//     brackets, a token error, an error wrapping [util.ErrInvalidQuery] for a
//     malformed query, or a query prop error.
func (c *Config) setURL(resolver types.ConfigQueryResolver, serviceURL *url.URL) error {
	path := serviceURL.Path
	if path != "" && path[0] == '/' {
		path = path[1:]
	}

	if serviceURL.Fragment != "" {
		path += "/#" + serviceURL.Fragment
	}

	targets := strings.Split(path, "/")

	// The token is the host as written, which cannot hold a port or brackets
	// that url.URL.Hostname would strip.
	token := serviceURL.Host
	if strings.ContainsAny(token, ":[]") {
		return fmt.Errorf("access token: %w", util.ErrInvalidHost)
	}

	if err := validateToken(token); err != nil {
		return err
	}

	c.Token = token
	c.Targets = targets

	query, err := util.ParseURLQuery(serviceURL)
	if err != nil {
		return err
	}

	return format.ApplyQuery(resolver, query)
}

// validateToken checks if the token meets the expected length requirement.
//
// Parameters:
//   - token: the access token.
//
// Returns:
//   - error: [ErrTokenIncorrectSize] for a token that is not
//     [ExpectedTokenLength] characters long.
func validateToken(token string) error {
	if len(token) != ExpectedTokenLength {
		return ErrTokenIncorrectSize
	}

	return nil
}
