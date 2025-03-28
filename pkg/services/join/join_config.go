package join

import (
	"errors"
	"fmt"
	"net/url"

	"github.com/nicholas-fedor/shoutrrr/pkg/format"
	"github.com/nicholas-fedor/shoutrrr/pkg/types"
)

// Scheme identifies this service in configuration URLs.
const Scheme = "join"

// ErrDevicesMissing indicates that no devices are specified in the configuration.
var (
	ErrDevicesMissing = errors.New("devices missing from config URL")
	ErrAPIKeyMissing  = errors.New("API key missing from config URL")
)

// Config holds settings for the Join notification service.
type Config struct {
	APIKey  string   `url:"pass"`
	Devices []string `           desc:"Comma separated list of device IDs" key:"devices"`
	Title   string   `           desc:"If set creates a notification"      key:"title"   optional:""`
	Icon    string   `           desc:"Icon URL"                           key:"icon"    optional:""`
}

// Enums returns the fields that should use an EnumFormatter for their values.
func (config *Config) Enums() map[string]types.EnumFormatter {
	return map[string]types.EnumFormatter{}
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
		User:       url.UserPassword("Token", config.APIKey),
		Host:       "join",
		Scheme:     Scheme,
		ForceQuery: true,
		RawQuery:   format.BuildQuery(resolver),
	}
}

func (config *Config) setURL(resolver types.ConfigQueryResolver, url *url.URL) error {
	password, _ := url.User.Password()
	config.APIKey = password

	for key, vals := range url.Query() {
		if err := resolver.Set(key, vals[0]); err != nil {
			return fmt.Errorf("setting config property %q from URL query: %w", key, err)
		}
	}

	if url.String() != "join://dummy@dummy.com" {
		if len(config.Devices) < 1 {
			return ErrDevicesMissing
		}

		if len(config.APIKey) < 1 {
			return ErrAPIKeyMissing
		}
	}

	return nil
}
