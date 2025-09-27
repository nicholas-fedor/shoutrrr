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
	webhookURL    *url.URL
	headers       map[string]string
	extraData     map[string]string
	ContentType   string `default:"application/json" desc:"The value of the Content-Type header"               key:"contenttype"`
	DisableTLS    bool   `default:"No"                                                                         key:"disabletls"`
	Template      string `                           desc:"The template used for creating the request payload" key:"template"    optional:""`
	Title         string `default:""                                                                           key:"title"`
	TitleKey      string `default:"title"            desc:"The key that will be used for the title value"      key:"titlekey"`
	MessageKey    string `default:"message"          desc:"The key that will be used for the message value"    key:"messagekey"`
	RequestMethod string `default:"POST"                                                                       key:"method"`
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

	webhookQuery := webhookURL.Query()
	headers, extraData := stripCustomQueryValues(webhookQuery)
	escapedQuery := url.Values{}

	for key, values := range webhookQuery {
		if len(values) > 0 {
			escapedQuery.Set(format.EscapeKey(key), values[0])
		}
	}

	_, err := format.SetConfigPropsFromQuery(&pkr, escapedQuery)
	if err != nil {
		return nil, pkr, fmt.Errorf("setting config properties from query: %w", err)
	}

	webhookURL.RawQuery = webhookQuery.Encode()
	config.webhookURL = &webhookURL
	config.headers = headers
	config.extraData = extraData
	config.DisableTLS = webhookURL.Scheme == "http"

	return config, pkr, nil
}

// WebhookURL returns the configured webhook URL, adjusted for TLS settings.
func (config *Config) WebhookURL() *url.URL {
	webhookURL := *config.webhookURL
	webhookURL.Scheme = DefaultWebhookScheme

	if config.DisableTLS {
		webhookURL.Scheme = "http" // Truncate to "http" if TLS is disabled
	}

	return &webhookURL
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

func (config *Config) getURL(resolver types.ConfigQueryResolver) *url.URL {
	serviceURL := *config.webhookURL
	webhookQuery := config.webhookURL.Query()
	serviceQuery := format.BuildQueryWithCustomFields(resolver, webhookQuery)
	appendCustomQueryValues(serviceQuery, config.headers, config.extraData)
	serviceURL.RawQuery = serviceQuery.Encode()
	serviceURL.Scheme = Scheme

	return &serviceURL
}

func (config *Config) setURL(resolver types.ConfigQueryResolver, serviceURL *url.URL) error {
	webhookURL := *serviceURL
	serviceQuery := serviceURL.Query()
	headers, extraData := stripCustomQueryValues(serviceQuery)

	customQuery, err := format.SetConfigPropsFromQuery(resolver, serviceQuery)
	if err != nil {
		return fmt.Errorf("setting config properties from service URL query: %w", err)
	}

	webhookURL.RawQuery = customQuery.Encode()
	config.webhookURL = &webhookURL
	config.headers = headers
	config.extraData = extraData

	return nil
}
