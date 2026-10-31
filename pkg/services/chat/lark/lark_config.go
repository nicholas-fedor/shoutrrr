package lark

import (
	"net/url"
	"strings"

	"github.com/nicholas-fedor/shoutrrr/pkg/format"
	"github.com/nicholas-fedor/shoutrrr/pkg/types"
	"github.com/nicholas-fedor/shoutrrr/pkg/util"
)

// Config represents the configuration for the Lark service.
type Config struct {
	// Host is the webhook host, open.larksuite.com for Lark or open.feishu.cn
	// for Feishu.
	Host string `default:"open.larksuite.com" desc:"Custom bot URL Host" url:"Host"`
	// Secret is the custom bot's signing secret. When set, each request is
	// signed.
	Secret string `default:"" desc:"Custom bot secret" key:"secret" sensitive:"true"`
	// Path is the custom bot's webhook token, written as the path of the
	// service URL.
	Path string `desc:"Custom bot token" sensitive:"true" url:"Path"`
	// Title makes the message a post with this title. Without one, the
	// message is sent as text.
	Title string `default:"" desc:"Message Title" key:"title"`
	// Link is a URL added to a post as a "More Info" link.
	Link string `default:"" desc:"Optional link URL" key:"link"`
}

// Scheme is the identifier for the Lark service protocol.
const Scheme = "lark"

// Enums returns a map of enum formatters (none for this service).
//
// Returns:
//   - map[string]types.EnumFormatter: an empty map.
func (c *Config) Enums() map[string]types.EnumFormatter {
	return map[string]types.EnumFormatter{}
}

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
//   - error: the error from reading the URL, such as [ErrInvalidHost] or
//     [ErrNoPath].
func (c *Config) SetURL(serviceURL *url.URL) error {
	resolver := format.NewPropKeyResolver(c)

	return c.setURL(&resolver, serviceURL)
}

// getURL constructs a URL using the provided resolver.
//
// Parameters:
//   - resolver: the resolver that builds the query from the config props.
//
// Returns:
//   - *url.URL: the service URL, with the bot token as its path.
func (c *Config) getURL(resolver types.ConfigQueryResolver) *url.URL {
	return &url.URL{
		Host:       c.Host,
		Path:       "/" + c.Path,
		Scheme:     Scheme,
		ForceQuery: true,
		RawQuery:   format.BuildQuery(resolver),
	}
}

// setURL updates the Config from a URL using the provided resolver.
// It sets the host, path, and query parameters, validating host and path, and
// returns an error if parsing or validation fails.
//
// Parameters:
//   - resolver: the resolver that sets the config props from the query.
//   - serviceURL: the service URL.
//
// Returns:
//   - error: [ErrInvalidHost] for a host other than Lark or Feishu, [ErrNoPath]
//     for a missing path, an error wrapping [util.ErrInvalidQuery] for a
//     malformed query, or a query prop error.
func (c *Config) setURL(resolver types.ConfigQueryResolver, serviceURL *url.URL) error {
	c.Host = serviceURL.Host
	// A URL without a host uses Lark's host.
	if c.Host == "" {
		c.Host = larkHost
	} else if c.Host != larkHost && c.Host != feishuHost {
		return ErrInvalidHost
	}

	c.Path = strings.Trim(serviceURL.Path, "/")
	if c.Path == "" {
		return ErrNoPath
	}

	query, err := util.ParseURLQuery(serviceURL)
	if err != nil {
		return err
	}

	return format.ApplyQuery(resolver, query)
}
