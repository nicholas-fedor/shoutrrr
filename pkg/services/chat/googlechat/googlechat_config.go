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
func (config *Config) GetURL() *url.URL {
	resolver := format.NewPropKeyResolver(config)

	return config.getURL(&resolver)
}

// SetURL updates the configuration from a URL.
func (config *Config) SetURL(serviceURL *url.URL) error {
	resolver := format.NewPropKeyResolver(config)

	return config.setURL(&resolver, serviceURL)
}

// getURL constructs the URL from the configuration using the provided resolver.
func (config *Config) getURL(_ types.ConfigQueryResolver) *url.URL {
	query := url.Values{}
	query.Set("key", config.Key)
	query.Set("token", config.Token)

	//nolint:exhaustruct // Only required fields are set
	return &url.URL{
		Host:     config.Host,
		Path:     config.Path,
		RawQuery: query.Encode(),
		Scheme:   Scheme,
	}
}

// setURL updates the configuration from a URL using the provided resolver.
func (config *Config) setURL(_ types.ConfigQueryResolver, serviceURL *url.URL) error {
	config.Host = serviceURL.Host
	config.Path = serviceURL.Path

	query := serviceURL.Query()
	config.Key = query.Get("key")
	config.Token = query.Get("token")

	// Only enforce if explicitly provided but empty
	if query.Has("key") && config.Key == "" {
		return ErrMissingKey
	}

	if query.Has("token") && config.Token == "" {
		return ErrMissingToken
	}

	return nil
}
