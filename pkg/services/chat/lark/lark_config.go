package lark

import (
	"fmt"
	"net/url"
	"strings"

	"github.com/nicholas-fedor/shoutrrr/pkg/format"
	"github.com/nicholas-fedor/shoutrrr/pkg/types"
)

// Config represents the configuration for the Lark service.
type Config struct {
	Host string `default:"open.larksuite.com" desc:"Custom bot URL Host" url:"Host"`
	//nolint:gosec // G117: Secret is a configuration field name, not a hardcoded credential
	Secret string `default:"" desc:"Custom bot secret" key:"secret"`
	Path   string `           desc:"Custom bot token"               url:"Path"`
	Title  string `default:"" desc:"Message Title"     key:"title"`
	Link   string `default:"" desc:"Optional link URL" key:"link"`
}

// Scheme is the identifier for the Lark service protocol.
const Scheme = "lark"

// Enums returns a map of enum formatters (none for this service).
func (c *Config) Enums() map[string]types.EnumFormatter {
	return map[string]types.EnumFormatter{}
}

// GetURL constructs a URL from the Config fields.
func (c *Config) GetURL() *url.URL {
	resolver := format.NewPropKeyResolver(c)

	return c.getURL(&resolver)
}

// SetURL updates the Config from a URL.
func (c *Config) SetURL(configURL *url.URL) error {
	resolver := format.NewPropKeyResolver(c)

	return c.setURL(&resolver, configURL)
}

// getURL constructs a URL using the provided resolver.
//

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
// It sets the host, path, and query parameters, validating host and path, and returns an error if parsing or validation fails.
func (c *Config) setURL(resolver types.ConfigQueryResolver, configURL *url.URL) error {
	c.Host = configURL.Host
	// Handle documentation generation or empty host
	if c.Host == "" || (configURL.User != nil && configURL.User.Username() == "dummy") {
		c.Host = "open.larksuite.com"
	} else if c.Host != larkHost && c.Host != feishuHost {
		return ErrInvalidHost
	}

	c.Path = strings.Trim(configURL.Path, "/")
	// Handle documentation generation with empty path
	if c.Path == "" && (configURL.User != nil && configURL.User.Username() == "dummy") {
		c.Path = "token"
	} else if c.Path == "" {
		return ErrNoPath
	}

	for key, vals := range configURL.Query() {
		if err := resolver.Set(key, vals[0]); err != nil {
			return fmt.Errorf("setting query parameter %q: %w", key, err)
		}
	}

	return nil
}
