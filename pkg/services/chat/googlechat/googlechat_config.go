package googlechat

import (
	"errors"
	"net/url"

	"github.com/nicholas-fedor/shoutrrr/pkg/format"
	"github.com/nicholas-fedor/shoutrrr/pkg/services/standard"
	"github.com/nicholas-fedor/shoutrrr/pkg/types"
)

// Config holds the configuration for the Google Chat service.
type Config struct {
	standard.EnumlessConfig

	Host  string `default:"chat.googleapis.com"`
	Path  string
	Token string
	Key   string
}

const (
	Scheme = "googlechat"
)

// Static error definitions.
var (
	ErrMissingKey   = errors.New("missing field 'key'")
	ErrMissingToken = errors.New("missing field 'token'")
)

// GetURL returns the URL representation of the configuration.
func (c *Config) GetURL() *url.URL {
	resolver := format.NewPropKeyResolver(c)

	return c.getURL(&resolver)
}

// SetURL updates the configuration from a URL.
func (c *Config) SetURL(serviceURL *url.URL) error {
	resolver := format.NewPropKeyResolver(c)

	return c.setURL(&resolver, serviceURL)
}

// getURL constructs the URL from the configuration using the provided resolver.
func (c *Config) getURL(_ types.ConfigQueryResolver) *url.URL {
	query := url.Values{}
	query.Set("key", c.Key)
	query.Set("token", c.Token)

	return &url.URL{
		Host:     c.Host,
		Path:     c.Path,
		RawQuery: query.Encode(),
		Scheme:   Scheme,
	}
}

// setURL updates the configuration from a URL using the provided resolver.
func (c *Config) setURL(_ types.ConfigQueryResolver, serviceURL *url.URL) error {
	c.Host = serviceURL.Host
	c.Path = serviceURL.Path

	query := serviceURL.Query()
	c.Key = query.Get("key")
	c.Token = query.Get("token")

	// Only enforce if explicitly provided but empty
	if query.Has("key") && c.Key == "" {
		return ErrMissingKey
	}

	if query.Has("token") && c.Token == "" {
		return ErrMissingToken
	}

	return nil
}
