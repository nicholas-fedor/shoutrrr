package gotify

import (
	"fmt"
	"net/url"
	"strings"

	"github.com/nicholas-fedor/shoutrrr/pkg/format"
	"github.com/nicholas-fedor/shoutrrr/pkg/services/standard"
	"github.com/nicholas-fedor/shoutrrr/pkg/types"
)

// Scheme identifies this service in configuration URLs.
const (
	Scheme = "gotify"
)

// Config holds settings for the Gotify notification service.
type Config struct {
	standard.EnumlessConfig
	Token      string `desc:"Application token"                     required:"" url:"path2"`
	Host       string `desc:"Server hostname (and optionally port)" required:"" url:"host,port"`
	Path       string `desc:"Server subpath"                                    url:"path1"     optional:""`
	Priority   int    `                                                                                     default:"0"                     key:"priority"`
	Title      string `                                                                                     default:"Shoutrrr notification" key:"title"`
	DisableTLS bool   `                                                                                     default:"No"                    key:"disabletls"`
}

// GetURL generates a URL from the current configuration values.
func (config *Config) GetURL() *url.URL {
	resolver := format.NewPropKeyResolver(config)

	return config.getURL(&resolver)
}

// SetURL updates the configuration from a URL representation.
func (config *Config) SetURL(url *url.URL) error {
	resolver := format.NewPropKeyResolver(config)

	return config.setURL(&resolver, url)
}

func (config *Config) getURL(resolver types.ConfigQueryResolver) *url.URL {
	return &url.URL{
		Host:       config.Host,
		Scheme:     Scheme,
		ForceQuery: false,
		Path:       config.Path + config.Token,
		RawQuery:   format.BuildQuery(resolver),
	}
}

func (config *Config) setURL(resolver types.ConfigQueryResolver, url *url.URL) error {
	path := url.Path
	if len(path) > 0 && path[len(path)-1] == '/' {
		path = path[:len(path)-1]
	}

	tokenIndex := strings.LastIndex(path, "/") + 1

	config.Path = path[:tokenIndex]
	if config.Path == "/" {
		config.Path = config.Path[1:]
	}

	config.Host = url.Host
	config.Token = path[tokenIndex:]

	for key, vals := range url.Query() {
		if err := resolver.Set(key, vals[0]); err != nil {
			return fmt.Errorf("setting config property %q from URL query: %w", key, err)
		}
	}

	return nil
}
