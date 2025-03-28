package pushover

import (
	"errors"
	"fmt"
	"net/url"

	"github.com/nicholas-fedor/shoutrrr/pkg/format"
	"github.com/nicholas-fedor/shoutrrr/pkg/types"
)

// Scheme is the identifying part of this service's configuration URL.
const Scheme = "pushover"

// Static errors for configuration validation.
var (
	ErrUserMissing  = errors.New("user missing from config URL")
	ErrTokenMissing = errors.New("token missing from config URL")
)

// Config for the Pushover notification service.
type Config struct {
	Token    string   `desc:"API Token/Key" url:"pass"`
	User     string   `desc:"User Key"      url:"host"`
	Devices  []string `                                key:"devices"  optional:""`
	Priority int8     `                                key:"priority"             default:"0"`
	Title    string   `                                key:"title"    optional:""`
}

// Enums returns the fields that should use a corresponding EnumFormatter to Print/Parse their values.
func (config *Config) Enums() map[string]types.EnumFormatter {
	return map[string]types.EnumFormatter{}
}

// GetURL returns a URL representation of its current field values.
func (config *Config) GetURL() *url.URL {
	resolver := format.NewPropKeyResolver(config)

	return config.getURL(&resolver)
}

// SetURL updates the Config from a URL representation of its field values.
func (config *Config) SetURL(url *url.URL) error {
	resolver := format.NewPropKeyResolver(config)

	return config.setURL(&resolver, url)
}

// setURL updates the Config from a URL using the provided resolver.
func (config *Config) setURL(resolver types.ConfigQueryResolver, url *url.URL) error {
	password, _ := url.User.Password()
	config.User = url.Host
	config.Token = password

	for key, vals := range url.Query() {
		if err := resolver.Set(key, vals[0]); err != nil {
			return fmt.Errorf("setting query parameter %q to %q: %w", key, vals[0], err)
		}
	}

	if url.String() != "pushover://dummy@dummy.com" {
		if len(config.User) < 1 {
			return ErrUserMissing
		}

		if len(config.Token) < 1 {
			return ErrTokenMissing
		}
	}

	return nil
}

// getURL constructs a URL from the Config's fields using the provided resolver.
func (config *Config) getURL(resolver types.ConfigQueryResolver) *url.URL {
	return &url.URL{
		User:       url.UserPassword("Token", config.Token),
		Host:       config.User,
		Scheme:     Scheme,
		ForceQuery: true,
		RawQuery:   format.BuildQuery(resolver),
	}
}
