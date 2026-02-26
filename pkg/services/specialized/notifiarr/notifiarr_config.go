package notifiarr

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
	Scheme = "notifiarr"
	// expectedPartsAfterSplit is the expected number of parts after splitting on "/passthrough/".
	expectedPartsAfterSplit = 2
)

// Config holds settings for the Notifiarr notification service.
type Config struct {
	standard.EnumlessConfig

	// The API key for Notifiarr authentication
	APIKey string `desc:"The Notifiarr API key" key:"apikey" required:"true"`
	// Optional name of the app/script for notifications
	Name string `default:"Shoutrrr" desc:"Name of the app/script for notifications" key:"name" optional:""`
	// Optional Discord channel ID for Discord notifications
	Channel string `desc:"Discord channel ID for notifications" key:"channel" optional:""`
	// Optional thumbnail URL for Discord notifications
	Thumbnail string `desc:"Thumbnail URL for Discord notifications" key:"thumbnail" optional:""`
	// Optional image URL for Discord notifications
	Image string `desc:"Image URL for Discord notifications" key:"image" optional:""`
	// Optional color for Discord notifications
	Color string `desc:"Color for Discord notifications" key:"color" optional:""`
	// Custom query parameters from webhook URL
	webhookQuery url.Values
}

// DefaultConfig creates a new Config with default values and its associated PropKeyResolver.
func DefaultConfig() (*Config, format.PropKeyResolver) {
	config := &Config{}
	pkr := format.NewPropKeyResolver(config)
	_ = pkr.SetDefaultProps(config)

	return config, pkr
}

// ConfigFromWebhookURL constructs a Config from a parsed webhook URL.
func ConfigFromWebhookURL(webhookURL url.URL) (*Config, format.PropKeyResolver, error) {
	config, pkr := DefaultConfig()

	// Extract query parameters
	webhookQuery := webhookURL.Query()

	// Set config properties from query params
	customQuery, err := format.SetConfigPropsFromQuery(&pkr, webhookQuery)
	if err != nil {
		return nil, pkr, fmt.Errorf("setting config properties from query: %w", err)
	}

	// Store custom query parameters
	config.webhookQuery = customQuery

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
func (config *Config) GetURL() *url.URL {
	resolver := format.NewPropKeyResolver(config)

	return config.getURL(&resolver)
}

// SetURL updates the configuration from a service URL.
func (config *Config) SetURL(serviceURL *url.URL) error {
	resolver := format.NewPropKeyResolver(config)

	return config.setURL(&resolver, serviceURL)
}

// getURL generates a service URL from the configuration using the provided resolver.
func (config *Config) getURL(resolver types.ConfigQueryResolver) *url.URL {
	var query url.Values
	if config.webhookQuery == nil {
		query = format.BuildQueryWithCustomFields(resolver, url.Values{})
	} else {
		query = format.BuildQueryWithCustomFields(resolver, config.webhookQuery)
	}

	query.Del("apikey")
	serviceURL := &url.URL{
		Scheme:   Scheme,
		Host:     config.APIKey,
		RawQuery: query.Encode(),
	}

	return serviceURL
}

// setURL updates the configuration from a service URL using the provided resolver.
func (config *Config) setURL(resolver types.ConfigQueryResolver, serviceURL *url.URL) error {
	config.APIKey = serviceURL.Host

	// Set config properties from query
	serviceQuery := serviceURL.Query()

	_, err := format.SetConfigPropsFromQuery(resolver, serviceQuery)
	if err != nil {
		return fmt.Errorf("setting config properties from service URL query: %w", err)
	}

	return nil
}
