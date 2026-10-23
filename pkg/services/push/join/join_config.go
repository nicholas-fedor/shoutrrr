package join

import (
	"fmt"
	"net/url"

	"github.com/nicholas-fedor/shoutrrr/pkg/format"
	"github.com/nicholas-fedor/shoutrrr/pkg/types"
	"github.com/nicholas-fedor/shoutrrr/pkg/util"
)

// Config holds settings for the Join notification service.
type Config struct {
	APIKey  string   `sensitive:"true"                          url:"pass"`
	Title   string   `desc:"If set creates a notification"      key:"title"   optional:""`
	Icon    string   `desc:"Icon URL"                           key:"icon"    optional:""`
	Devices []string `desc:"Comma separated list of device IDs" key:"devices"`
}

// Scheme identifies this service in configuration URLs.
const Scheme = "join"

// Enums returns the fields that should use an EnumFormatter for their values.
func (c *Config) Enums() map[string]types.EnumFormatter {
	return map[string]types.EnumFormatter{}
}

// GetURL generates a URL from the current configuration values.
func (c *Config) GetURL() *url.URL {
	resolver := format.NewPropKeyResolver(c)

	return c.getURL(&resolver)
}

// SetURL updates the configuration from a URL representation.
func (c *Config) SetURL(serviceURL *url.URL) error {
	resolver := format.NewPropKeyResolver(c)

	return c.setURL(&resolver, serviceURL)
}

func (c *Config) getURL(resolver types.ConfigQueryResolver) *url.URL {
	return &url.URL{
		User:       url.UserPassword("Token", c.APIKey),
		Host:       "join",
		Scheme:     Scheme,
		ForceQuery: true,
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
//   - error: an error wrapping [util.ErrInvalidQuery] for a malformed query, a
//     query prop error, or [ErrDevicesMissing] or [ErrAPIKeyMissing] when the
//     devices or the API key are missing.
func (c *Config) setURL(resolver types.ConfigQueryResolver, serviceURL *url.URL) error {
	password, _ := serviceURL.User.Password()
	c.APIKey = password

	query, err := util.ParseURLQuery(serviceURL)
	if err != nil {
		return err
	}

	for key, vals := range query {
		if err := resolver.Set(key, vals[0]); err != nil {
			return fmt.Errorf("setting config property %q from URL query: %w", key, err)
		}
	}

	if serviceURL.String() != "join://dummy@dummy.com" {
		if len(c.Devices) < 1 {
			return ErrDevicesMissing
		}

		if len(c.APIKey) < 1 {
			return ErrAPIKeyMissing
		}
	}

	return nil
}
