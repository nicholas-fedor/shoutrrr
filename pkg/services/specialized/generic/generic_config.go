package generic

import (
	"fmt"
	"net/url"

	"github.com/nicholas-fedor/shoutrrr/pkg/format"
	"github.com/nicholas-fedor/shoutrrr/pkg/services/standard"
	"github.com/nicholas-fedor/shoutrrr/pkg/types"
)

// Scheme identifies this service in configuration URLs.
const (
	Scheme               = "generic"
	DefaultWebhookScheme = "https"
)

// Config holds settings for the generic notification service.
type Config struct {
	standard.EnumlessConfig
	// The webhook URL to send notifications to
	webhookURL *url.URL
	// Custom HTTP headers to include in requests
	headers map[string]string
	// Additional data to include in JSON payloads
	extraData     map[string]string
	ContentType   string `default:"application/json" desc:"The default value of the Content-Type header (dynamically set based on payload format)" key:"contenttype"`
	DisableTLS    bool   `default:"No"                                                                                                             key:"disabletls"`
	Template      string `                           desc:"The template used for creating the request payload"                                     key:"template"    optional:""`
	Title         string `default:""                                                                                                               key:"title"`
	TitleKey      string `default:"title"            desc:"The key that will be used for the title value"                                          key:"titlekey"`
	MessageKey    string `default:"message"          desc:"The key that will be used for the message value"                                        key:"messagekey"`
	RequestMethod string `default:"POST"                                                                                                           key:"method"`
}

// DefaultConfig creates a new Config with default values and its associated PropKeyResolver.
func DefaultConfig() (*Config, format.PropKeyResolver) {
	// Initialize empty config
	config := &Config{}
	// Create property key resolver
	pkr := format.NewPropKeyResolver(config)
	// Set default properties from struct tags
	_ = pkr.SetDefaultProps(config)

	return config, pkr
}

// ConfigFromWebhookURL constructs a Config from a parsed webhook URL.
func ConfigFromWebhookURL(webhookURL url.URL) (*Config, format.PropKeyResolver, error) {
	// Get default config and resolver
	config, pkr := DefaultConfig()

	// Extract query parameters
	webhookQuery := webhookURL.Query()
	// Extract custom headers and extra data
	headers, extraData := stripCustomQueryValues(webhookQuery)

	// Set config properties from remaining query params
	_, err := format.SetConfigPropsFromQuery(&pkr, webhookQuery)
	if err != nil {
		return nil, pkr, fmt.Errorf("setting config properties from query: %w", err)
	}

	// Update URL with modified query
	webhookURL.RawQuery = webhookQuery.Encode()
	// Assign webhook URL
	config.webhookURL = &webhookURL
	// Assign extracted headers
	config.headers = headers
	// Assign extracted extra data
	config.extraData = extraData
	// Set TLS based on scheme
	config.DisableTLS = webhookURL.Scheme == "http"

	return config, pkr, nil
}

// WebhookURL returns the configured webhook URL, adjusted for TLS settings.
func (config *Config) WebhookURL() *url.URL {
	// Copy the URL to modify
	webhookURL := *config.webhookURL
	// Set default HTTPS scheme
	webhookURL.Scheme = DefaultWebhookScheme

	if config.DisableTLS {
		// Use HTTP if TLS is disabled
		webhookURL.Scheme = "http"
	}

	return &webhookURL
}

// GetURL generates a URL from the current configuration values.
func (config *Config) GetURL() *url.URL {
	// Create resolver for this config
	resolver := format.NewPropKeyResolver(config)

	// Generate URL using resolver
	return config.getURL(&resolver)
}

// SetURL updates the configuration from a service URL.
func (config *Config) SetURL(serviceURL *url.URL) error {
	// Create resolver for this config
	resolver := format.NewPropKeyResolver(config)

	// Parse and set URL
	return config.setURL(&resolver, serviceURL)
}

// getURL generates a service URL from the configuration using the provided resolver.
func (config *Config) getURL(resolver types.ConfigQueryResolver) *url.URL {
	// Copy webhook URL
	serviceURL := *config.webhookURL
	// Get existing query params
	webhookQuery := config.webhookURL.Query()
	// Build query with config fields
	serviceQuery := format.BuildQueryWithCustomFields(resolver, webhookQuery)
	// Add custom headers and extra data
	appendCustomQueryValues(serviceQuery, config.headers, config.extraData)
	// Encode the query
	serviceURL.RawQuery = serviceQuery.Encode()
	// Set service scheme
	serviceURL.Scheme = Scheme

	return &serviceURL
}

// setURL updates the configuration from a service URL using the provided resolver.
func (config *Config) setURL(resolver types.ConfigQueryResolver, serviceURL *url.URL) error {
	// Copy service URL
	webhookURL := *serviceURL
	// Extract query parameters
	serviceQuery := serviceURL.Query()
	// Extract custom headers and extra data
	headers, extraData := stripCustomQueryValues(serviceQuery)

	// Set config properties from query
	customQuery, err := format.SetConfigPropsFromQuery(resolver, serviceQuery)
	if err != nil {
		return fmt.Errorf("setting config properties from service URL query: %w", err)
	}

	// Update URL with remaining query
	webhookURL.RawQuery = customQuery.Encode()
	// Assign webhook URL
	config.webhookURL = &webhookURL
	// Assign extracted headers
	config.headers = headers
	// Assign extracted extra data
	config.extraData = extraData

	return nil
}
