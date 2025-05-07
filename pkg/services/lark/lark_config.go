package lark

import (
	"fmt"
	"net/url"
	"strings"

	"github.com/nicholas-fedor/shoutrrr/pkg/format"
	"github.com/nicholas-fedor/shoutrrr/pkg/types"
)

// Scheme is the identifier for the Lark service protocol.
const Scheme = "lark"

// Config represents the configuration for the Lark service.
type Config struct {
	Host   string `default:"open.larksuite.com" desc:"Custom bot URL Host" url:"Host"`
	Secret string `default:""                   desc:"Custom bot secret"              key:"secret"`
	Path   string `                             desc:"Custom bot token"    url:"Path"`
	Title  string `default:""                   desc:"Message Title"                  key:"title"`
	Link   string `default:""                   desc:"Optional link URL"              key:"link"`
}

// Enums returns a map of enum formatters (none for this service).
func (config *Config) Enums() map[string]types.EnumFormatter {
	return map[string]types.EnumFormatter{}
}

// GetURL constructs a URL from the Config fields.
func (config *Config) GetURL() *url.URL {
	resolver := format.NewPropKeyResolver(config)

	return config.getURL(&resolver)
}

// getURL constructs a URL using the provided resolver.
func (config *Config) getURL(resolver types.ConfigQueryResolver) *url.URL {
	return &url.URL{
		Host:       config.Host,
		Path:       "/" + config.Path,
		Scheme:     Scheme,
		ForceQuery: true,
		RawQuery:   format.BuildQuery(resolver),
	}
}

// SetURL updates the Config from a URL.
func (config *Config) SetURL(url *url.URL) error {
	resolver := format.NewPropKeyResolver(config)

	return config.setURL(&resolver, url)
}

// setURL updates the Config from a URL using the provided resolver.
// It sets the host, path, and query parameters, validating host and path, and returns an error if parsing or validation fails.
func (config *Config) setURL(resolver types.ConfigQueryResolver, url *url.URL) error {
	config.Host = url.Host
	// Use default host if none provided (e.g., during documentation generation)
	if config.Host == "" {
		config.Host = "open.larksuite.com"
	} else if config.Host != larkHost && config.Host != feishuHost {
		return ErrInvalidHost
	}

	config.Path = strings.Trim(url.Path, "/")
	if config.Path == "" {
		return ErrNoPath
	}

	for key, vals := range url.Query() {
		if err := resolver.Set(key, vals[0]); err != nil {
			return fmt.Errorf("setting query parameter %q: %w", key, err)
		}
	}

	return nil
}
