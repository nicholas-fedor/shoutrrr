package pushbullet

import (
	"errors"
	"fmt"
	"net/url"
	"strings"
	"time"

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
	// written as the path of the service URL. An empty target pushes to all of
	// the token user's devices. A URL without a path gives one, and a send
	// without targets pushes to all devices too.
	Targets []string `url:"path"`
}

// Scheme is the scheme part of the service configuration URL.
const Scheme = "pushbullet"

// ExpectedTokenLength is the required length for a valid Pushbullet token.
const ExpectedTokenLength = 34

// ErrTokenIncorrectSize indicates that the token has an incorrect size.
var ErrTokenIncorrectSize = errors.New("token has incorrect size")

// allDevices is the target list of a send without targets. Its one empty
// target pushes to all of the token user's devices. Sends only read it.
var allDevices = []string{""}

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
	// Each target is one escaped segment, so a slash in an email address goes
	// out as %2F and reads back whole.
	path, rawPath := util.JoinEscapedPath(c.Targets)

	return &url.URL{
		Host:       c.Token,
		Path:       path,
		RawPath:    rawPath,
		Scheme:     Scheme,
		ForceQuery: false,
		RawQuery:   format.BuildQuery(resolver),
	}
}

// prepare drops the empty targets and checks the access token.
//
// A trailing slash leaves an empty target, which reading a list setting drops
// too. Reading a service URL and each send run it, so a config built in code
// pushes to the targets its URL would, and fails before a request as its URL
// would.
//
// Returns:
//   - error: the access token error, such as [ErrTokenIncorrectSize].
func (c *Config) prepare() error {
	c.Targets = util.NonEmpty(c.Targets)

	return validateToken(c.Token)
}

// sendBudget returns the send budget that [Service.ServiceTimeout] describes,
// for a config with the send's params applied.
//
// Returns:
//   - time.Duration: the budget for the send.
func (c *Config) sendBudget() time.Duration {
	return defaultHTTPTimeout * time.Duration(max(1, len(util.NonEmpty(c.Targets))))
}

// sendTargets returns the targets a send pushes to. A config without targets
// pushes to all devices, as a URL without a path does.
//
// Returns:
//   - []string: the targets, which the caller must not change.
func (c *Config) sendTargets() []string {
	if len(c.Targets) == 0 {
		return allDevices
	}

	return c.Targets
}

// setURL updates the Config from a URL using the provided resolver.
//
// Parameters:
//   - resolver: the resolver that sets the config props from the query.
//   - serviceURL: the service URL.
//
// Returns:
//   - error: an access token error, an error wrapping [util.ErrInvalidQuery]
//     for a malformed query, or a query prop error.
func (c *Config) setURL(resolver types.ConfigQueryResolver, serviceURL *url.URL) error {
	// An escaped slash stays in its segment, so a target written by GetURL
	// reads back whole.
	targets := util.SplitEscapedPath(strings.TrimPrefix(serviceURL.EscapedPath(), "/"))

	// An unescaped # starts the fragment. It holds a channel tag and any
	// targets after it, which are read the same way as the path.
	if serviceURL.Fragment != "" {
		targets = append(targets, util.SplitEscapedPath("#"+serviceURL.EscapedFragment())...)
	}

	if err := validateToken(serviceURL.Host); err != nil {
		return err
	}

	c.Token = serviceURL.Host
	c.Targets = targets

	query, err := util.ParseURLQuery(serviceURL)
	if err != nil {
		return err
	}

	if err := format.ApplyQuery(resolver, query); err != nil {
		return err
	}

	return c.prepare()
}

// validateToken checks the access token.
//
// Parameters:
//   - token: the access token.
//
// Returns:
//   - error: an error wrapping [util.ErrInvalidHost] for a token that a URL
//     host cannot hold, or [ErrTokenIncorrectSize] for a token that is not
//     [ExpectedTokenLength] characters long.
func validateToken(token string) error {
	if err := util.ValidateHostToken(token); err != nil {
		return fmt.Errorf("access token: %w", err)
	}

	if len(token) != ExpectedTokenLength {
		return ErrTokenIncorrectSize
	}

	return nil
}
