package googlechat

import (
	"errors"
	"net/url"

	"github.com/nicholas-fedor/shoutrrr/pkg/format"
	"github.com/nicholas-fedor/shoutrrr/pkg/services/standard"
	"github.com/nicholas-fedor/shoutrrr/pkg/types"
	"github.com/nicholas-fedor/shoutrrr/pkg/util"
)

// Config holds the configuration for the Google Chat service.
type Config struct {
	// EnumlessConfig provides the empty enum set, since Google Chat has no
	// enum fields.
	standard.EnumlessConfig

	// Host is the webhook host.
	Host string `default:"chat.googleapis.com"`
	// Path is the webhook path, such as /v1/spaces/SPACE/messages.
	Path string
	// Token is the webhook's token query parameter.
	Token string `sensitive:"true"`
	// Key is the webhook's key query parameter.
	Key string `sensitive:"true"`
}

// Service identity and URL defaults.
const (
	// Scheme identifies this service in configuration URLs.
	Scheme = "googlechat"
	// DefaultHost is the Google Chat API host a URL without a host uses.
	DefaultHost = "chat.googleapis.com"
)

// Static error definitions.
var (
	// ErrMissingKey indicates a service URL without the webhook key.
	ErrMissingKey = errors.New("missing field 'key'")
	// ErrMissingToken indicates a service URL without the webhook token.
	ErrMissingToken = errors.New("missing field 'token'")
)

// GetURL returns the URL representation of the configuration.
//
// Returns:
//   - *url.URL: the service URL.
func (c *Config) GetURL() *url.URL {
	resolver := format.NewPropKeyResolver(c)

	return c.getURL(&resolver)
}

// SetURL updates the configuration from a URL.
//
// Parameters:
//   - serviceURL: the service URL.
//
// Returns:
//   - error: the error from reading the URL, such as [ErrMissingKey] or
//     [ErrMissingToken].
func (c *Config) SetURL(serviceURL *url.URL) error {
	resolver := format.NewPropKeyResolver(c)

	return c.setURL(&resolver, serviceURL)
}

// getURL constructs the URL from the configuration using the provided resolver.
//
// Parameters:
//   - types.ConfigQueryResolver: unused.
//
// Returns:
//   - *url.URL: the service URL, with the key and token in its query.
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
//
// Google Chat webhook URLs always carry a key and a token, so both are
// required. A URL without a host uses [DefaultHost].
//
// Parameters:
//   - types.ConfigQueryResolver: unused.
//   - serviceURL: the service URL.
//
// Returns:
//   - error: an error wrapping [util.ErrInvalidQuery] for a malformed query,
//     or [ErrMissingKey] or [ErrMissingToken] when the URL lacks the key or the
//     token, or has it empty.
func (c *Config) setURL(_ types.ConfigQueryResolver, serviceURL *url.URL) error {
	c.Host = serviceURL.Host
	c.Path = serviceURL.Path

	query, err := util.ParseURLQuery(serviceURL)
	if err != nil {
		return err
	}

	c.Key = query.Get("key")
	c.Token = query.Get("token")

	if c.Host == "" {
		c.Host = DefaultHost
	}

	if c.Key == "" {
		return ErrMissingKey
	}

	if c.Token == "" {
		return ErrMissingToken
	}

	return nil
}
