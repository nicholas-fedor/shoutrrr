package notifiarr

import (
	"fmt"
	"net/url"
	"strings"

	"github.com/nicholas-fedor/shoutrrr/pkg/format"
	"github.com/nicholas-fedor/shoutrrr/pkg/services/standard"
	"github.com/nicholas-fedor/shoutrrr/pkg/types"
	"github.com/nicholas-fedor/shoutrrr/pkg/util"
)

// Config holds settings for the Notifiarr notification service.
type Config struct {
	// EnumlessConfig provides the empty enum set, since Notifiarr has no enum
	// fields.
	standard.EnumlessConfig

	// APIKey is the API key for Notifiarr authentication.
	APIKey string `desc:"The Notifiarr API key" key:"apikey" required:"true" sensitive:"true"`
	// Name is the name of the app or script the notifications come from.
	Name string `default:"Shoutrrr" desc:"Name of the app/script for notifications" key:"name" optional:""`
	// Channel is the Discord channel ID for Discord notifications.
	Channel string `desc:"Discord channel ID for notifications" key:"channel" optional:""`
	// Thumbnail is the thumbnail URL for Discord notifications.
	Thumbnail string `desc:"Thumbnail URL for Discord notifications" key:"thumbnail" optional:""`
	// Image is the image URL for Discord notifications.
	Image string `desc:"Image URL for Discord notifications" key:"image" optional:""`
	// Color is the embed color for Discord notifications.
	Color string `desc:"Color for Discord notifications" key:"color" optional:""`
}

// Service identity and webhook URL parsing.
const (
	// Scheme identifies this service in configuration URLs.
	Scheme = "notifiarr"
	// expectedPartsAfterSplit is the expected number of parts after splitting on "/passthrough/".
	expectedPartsAfterSplit = 2
	// defaultName is the app name of a config without one, the default of the
	// name key.
	defaultName = "Shoutrrr"
)

// DefaultConfig creates a new Config with default values and its associated PropKeyResolver.
//
// Returns:
//   - *Config: the config with its defaults.
//   - format.PropKeyResolver: the resolver bound to the config.
func DefaultConfig() (*Config, format.PropKeyResolver) {
	config := &Config{}
	pkr := format.NewPropKeyResolver(config)
	// Panic on error since failure indicates a programming error in struct tags
	// that would be caught during development
	if err := pkr.SetDefaultProps(config); err != nil {
		panic(fmt.Sprintf("setting default properties: %v", err))
	}

	return config, pkr
}

// ConfigFromWebhookURL constructs a Config from a parsed webhook URL.
//
// Parameters:
//   - webhookURL: the webhook or service URL.
//
// Returns:
//   - *Config: the config, or nil on error.
//   - format.PropKeyResolver: the resolver bound to the config.
//   - error: an error wrapping [util.ErrInvalidQuery] for a malformed query, or
//     the error of [format.ApplyQuery], such as a key that is not a Notifiarr
//     setting.
func ConfigFromWebhookURL(webhookURL *url.URL) (*Config, format.PropKeyResolver, error) {
	config, pkr := DefaultConfig()

	webhookQuery, err := util.ParseURLQuery(webhookURL)
	if err != nil {
		return nil, pkr, err
	}

	// The passthrough API reads no query parameters, so a key that is not a
	// setting is an error rather than something to forward.
	if err := format.ApplyQuery(&pkr, webhookQuery); err != nil {
		return nil, pkr, fmt.Errorf("setting config properties from query: %w", err)
	}

	// Handle service URLs (scheme = notifiarr)
	if webhookURL.Scheme == Scheme {
		config.APIKey = webhookURL.Host
	} else if strings.Contains(webhookURL.Path, "/passthrough/") {
		// The webhook URL path should contain the API key after "/passthrough/"
		parts := strings.Split(webhookURL.Path, "/passthrough/")
		if len(parts) == expectedPartsAfterSplit {
			config.APIKey = strings.Split(parts[1], "/")[0]
		}
	}

	return config, pkr, nil
}

// GetURL generates a URL from the current configuration values.
//
// Returns:
//   - *url.URL: the service URL.
func (c *Config) GetURL() *url.URL {
	resolver := format.NewPropKeyResolver(c)

	return c.getURL(&resolver)
}

// SetURL updates the configuration from a service URL.
//
// Parameters:
//   - serviceURL: the service URL.
//
// Returns:
//   - error: the error from reading the URL, such as [ErrAPIKeyMissing].
func (c *Config) SetURL(serviceURL *url.URL) error {
	resolver := format.NewPropKeyResolver(c)

	return c.setURL(&resolver, serviceURL)
}

// appName returns the app name a notification carries. Notifiarr requires a
// name, so an empty name uses Shoutrrr, the default a URL without one gets.
//
// Returns:
//   - string: the config's name, or Shoutrrr when it is empty.
func (c *Config) appName() string {
	if c.Name == "" {
		return defaultName
	}

	return c.Name
}

// getURL generates a service URL from the configuration using the provided resolver.
//
// Parameters:
//   - resolver: the resolver that builds the config props into the query.
//
// Returns:
//   - *url.URL: the service URL, with the API key as the host and the config
//     props, without the API key, in its query.
func (c *Config) getURL(resolver types.ConfigQueryResolver) *url.URL {
	query := format.BuildQueryWithCustomFields(resolver, url.Values{})
	query.Del("apikey")
	serviceURL := &url.URL{
		Scheme:   Scheme,
		Host:     c.APIKey,
		RawQuery: query.Encode(),
	}

	return serviceURL
}

// setURL updates the configuration from a service URL using the provided resolver.
//
// The API key is the URL host, unless the apikey query parameter replaces it.
//
// Parameters:
//   - resolver: the resolver that sets the config props from the query.
//   - serviceURL: the service URL.
//
// Returns:
//   - error: an error wrapping [util.ErrInvalidQuery] for a malformed query, or
//     the error of [format.ApplyQuery], such as a key that is not a Notifiarr
//     setting. Otherwise the error from the config's checks, such as
//     [ErrAPIKeyMissing] when neither the host, other than a port alone, nor
//     the query sets an API key.
func (c *Config) setURL(resolver types.ConfigQueryResolver, serviceURL *url.URL) error {
	c.APIKey = serviceURL.Host

	serviceQuery, err := util.ParseURLQuery(serviceURL)
	if err != nil {
		return err
	}

	if err := format.ApplyQuery(resolver, serviceQuery); err != nil {
		return fmt.Errorf("setting config properties from service URL query: %w", err)
	}

	return c.validate()
}

// validate checks that the config has the API key every request needs.
// Reading a service URL and each send run it, so a config built in code fails
// before a request.
//
// Returns:
//   - error: [ErrAPIKeyMissing] for a config without an API key. A port
//     alone, such as :8080, counts as no API key. A key that a URL host cannot
//     hold gives an error wrapping [util.ErrInvalidHost], and a key of "." or
//     "..", which would walk the request path, gives an error wrapping
//     [util.ErrInvalidPathSegment]. Otherwise nil.
func (c *Config) validate() error {
	// The key is read from the URL host, so a port alone, as in
	// notifiarr://:8080, is not an API key.
	if (&url.URL{Host: c.APIKey}).Hostname() == "" {
		return ErrAPIKeyMissing
	}

	if err := util.ValidateHostToken(c.APIKey); err != nil {
		return fmt.Errorf("API key: %w", err)
	}

	if err := util.ValidatePathSegment(c.APIKey); err != nil {
		return fmt.Errorf("API key: %w", err)
	}

	return nil
}
