package join

import (
	"net/url"

	"github.com/nicholas-fedor/shoutrrr/pkg/format"
	"github.com/nicholas-fedor/shoutrrr/pkg/types"
	"github.com/nicholas-fedor/shoutrrr/pkg/util"
)

// Config holds settings for the Join notification service.
type Config struct {
	// APIKey is the Join API key, written as the password of the service URL.
	APIKey string `sensitive:"true" url:"pass"`
	// Title is the notification title. With a title, Join shows the message
	// as a notification.
	Title string `desc:"If set creates a notification" key:"title" optional:""`
	// Icon is the URL of the notification icon.
	Icon string `desc:"Icon URL" key:"icon" optional:""`
	// Devices lists the IDs of the devices to send to.
	Devices []string `desc:"Comma separated list of device IDs" key:"devices"`
}

// Scheme identifies this service in configuration URLs.
const Scheme = "join"

// Enums returns the fields that should use an EnumFormatter for their values.
//
// Returns:
//   - map[string]types.EnumFormatter: an empty map, since Join has no enum
//     fields.
func (c *Config) Enums() map[string]types.EnumFormatter {
	return map[string]types.EnumFormatter{}
}

// GetURL generates a URL from the current configuration values.
//
// Returns:
//   - *url.URL: the service URL.
func (c *Config) GetURL() *url.URL {
	resolver := format.NewPropKeyResolver(c)

	return c.getURL(&resolver)
}

// SetURL updates the configuration from a URL representation.
//
// Parameters:
//   - serviceURL: the service URL.
//
// Returns:
//   - error: the error from reading the URL, such as [ErrDevicesMissing].
func (c *Config) SetURL(serviceURL *url.URL) error {
	resolver := format.NewPropKeyResolver(c)

	return c.setURL(&resolver, serviceURL)
}

// getURL constructs a URL from the Config's fields using the provided resolver.
//
// Parameters:
//   - resolver: the resolver that builds the query from the config props.
//
// Returns:
//   - *url.URL: the service URL, with the API key as the password.
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

	if err := format.ApplyQuery(resolver, query); err != nil {
		return err
	}

	if len(c.Devices) < 1 {
		return ErrDevicesMissing
	}

	if len(c.APIKey) < 1 {
		return ErrAPIKeyMissing
	}

	return nil
}
