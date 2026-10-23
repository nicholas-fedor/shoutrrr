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
	standard.EnumlessConfig

	Host  string `default:"chat.googleapis.com"`
	Path  string
	Token string `sensitive:"true"`
	Key   string `sensitive:"true"`
}

const (
	Scheme = "googlechat"
	// DefaultHost is the Google Chat API host a URL without a host uses.
	DefaultHost = "chat.googleapis.com"
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
//
// Google Chat webhook URLs always carry a key and a token, so both are
// required. A URL without a host uses [DefaultHost]. The documentation dummy
// URL, <scheme>://dummy@dummy.com, is accepted without them.
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

	if serviceURL.String() == serviceURL.Scheme+"://dummy@dummy.com" {
		return nil
	}

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
