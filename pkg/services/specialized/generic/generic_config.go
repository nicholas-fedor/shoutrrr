package generic

import (
	"fmt"
	"net/url"

	"github.com/nicholas-fedor/shoutrrr/pkg/format"
	"github.com/nicholas-fedor/shoutrrr/pkg/services/standard"
	"github.com/nicholas-fedor/shoutrrr/pkg/types"
	"github.com/nicholas-fedor/shoutrrr/pkg/util"
)

// Config holds settings for the generic notification service.
type Config struct {
	// EnumlessConfig provides the empty enum set, since the generic service
	// has no enum fields.
	standard.EnumlessConfig

	// webhookURL is the webhook URL to send notifications to.
	webhookURL *url.URL
	// headers holds the custom HTTP headers to include in requests.
	headers map[string]string
	// extraData holds the additional data to include in JSON payloads.
	extraData map[string]string
	// ContentType is the Content-Type header of the request.
	ContentType string `default:"application/json" desc:"The default value of the Content-Type header (dynamically set based on payload format)" key:"contenttype"`
	// Template names the payload template: empty to send the message as is,
	// JSON for a JSON object, or a template loaded into the service.
	Template string `desc:"The template used for creating the request payload" key:"template" optional:""`
	// Title is the notification title, sent under TitleKey.
	Title string `default:"" desc:"Notification title, optionally set by the sender" key:"title"`
	// TitleKey is the payload key of the title.
	TitleKey string `default:"title" desc:"The key that will be used for the title value" key:"titlekey"`
	// MessageKey is the payload key of the message.
	MessageKey string `default:"message" desc:"The key that will be used for the message value" key:"messagekey"`
	// RequestMethod is the HTTP method of the request.
	RequestMethod string `default:"POST" desc:"The HTTP method of the request" key:"method"`
	// DisableTLS sends the request over HTTP instead of HTTPS.
	DisableTLS bool `default:"No" desc:"Send over HTTP instead of HTTPS" key:"disabletls"`
	// SkipTLSVerify skips verification of the webhook server's TLS
	// certificate.
	SkipTLSVerify bool `default:"No" desc:"Skip TLS certificate verification" key:"skiptlsverify"`
}

// Service identity and webhook defaults.
const (
	// Scheme identifies this service in configuration URLs.
	Scheme = "generic"
	// DefaultWebhookScheme is the webhook scheme unless TLS is disabled.
	DefaultWebhookScheme = "https"
)

// DefaultConfig creates a new Config with default values and its associated PropKeyResolver.
//
// It panics if a default prop cannot be applied, which only a broken struct
// tag can cause.
//
// Returns:
//   - *Config: the config with its defaults.
//   - format.PropKeyResolver: the resolver bound to the config.
func DefaultConfig() (*Config, format.PropKeyResolver) {
	// Initialize empty config
	config := &Config{}
	// Create property key resolver
	pkr := format.NewPropKeyResolver(config)
	// Set default properties from struct tags
	if err := pkr.SetDefaultProps(config); err != nil {
		panic(fmt.Sprintf("setting default properties: %v", err))
	}

	return config, pkr
}

// ConfigFromWebhookURL constructs a Config from a parsed webhook URL.
//
// Parameters:
//   - webhookURL: the webhook URL, whose query is rewritten without the
//     config props, custom headers, and extra data.
//
// Returns:
//   - *Config: the config, or nil on error.
//   - format.PropKeyResolver: the resolver bound to the config.
//   - error: an error wrapping [util.ErrInvalidQuery] for a malformed query, or
//     a query prop error.
func ConfigFromWebhookURL(webhookURL *url.URL) (*Config, format.PropKeyResolver, error) {
	// Get default config and resolver
	config, pkr := DefaultConfig()

	// Extract query parameters
	webhookQuery, err := util.ParseURLQuery(webhookURL)
	if err != nil {
		return nil, pkr, err
	}

	// Extract custom headers and extra data
	headers, extraData := stripCustomQueryValues(webhookQuery)

	// Set config properties from remaining query params
	_, err = format.SetConfigPropsFromQuery(&pkr, webhookQuery)
	if err != nil {
		return nil, pkr, fmt.Errorf("setting config properties from query: %w", err)
	}

	// Update URL with modified query
	webhookURL.RawQuery = webhookQuery.Encode()
	// Assign webhook URL
	config.webhookURL = webhookURL
	// Assign extracted headers
	config.headers = headers
	// Assign extracted extra data
	config.extraData = extraData
	// Set TLS based on scheme
	config.DisableTLS = webhookURL.Scheme == "http"

	return config, pkr, nil
}

// GetURL generates a URL from the current configuration values.
//
// Returns:
//   - *url.URL: the service URL.
func (c *Config) GetURL() *url.URL {
	// Create resolver for this config
	resolver := format.NewPropKeyResolver(c)

	// Generate URL using resolver
	return c.getURL(&resolver)
}

// MaskedURL returns the config URL with every part that can carry a secret
// replaced by [format.MaskedValue]. A webhook can carry credentials in any part
// of its URL, so the userinfo, a path other than the root, and the values of
// the webhook query, custom headers, and extra data are masked. The host and
// the config props stay.
//
// Returns:
//   - *url.URL: the masked config URL.
func (c *Config) MaskedURL() *url.URL {
	webhookURL := c.webhook()

	if webhookURL.User != nil {
		webhookURL.User = url.User(format.MaskedValue)
	}

	if webhookURL.Path != "" && webhookURL.Path != "/" {
		webhookURL.Path = "/" + format.MaskedValue
		webhookURL.RawPath = ""
	}

	query := webhookURL.Query()
	for key := range query {
		query.Set(key, format.MaskedValue)
	}

	webhookURL.RawQuery = query.Encode()

	masked := *c
	masked.webhookURL = &webhookURL
	masked.headers = maskedValues(c.headers)
	masked.extraData = maskedValues(c.extraData)

	return masked.GetURL()
}

// SetURL updates the configuration from a service URL.
//
// Parameters:
//   - serviceURL: the service URL.
//
// Returns:
//   - error: the error from reading the URL, such as [ErrHostMissing].
func (c *Config) SetURL(serviceURL *url.URL) error {
	// Create resolver for this config
	resolver := format.NewPropKeyResolver(c)

	// Parse and set URL
	return c.setURL(&resolver, serviceURL)
}

// WebhookURL returns the configured webhook URL, adjusted for TLS settings.
//
// Returns:
//   - *url.URL: a copy of the webhook URL with the http scheme when TLS is
//     disabled and https otherwise.
func (c *Config) WebhookURL() *url.URL {
	// Copy the URL to modify
	webhookURL := c.webhook()
	// Set default HTTPS scheme
	webhookURL.Scheme = DefaultWebhookScheme

	if c.DisableTLS {
		// Use HTTP if TLS is disabled
		webhookURL.Scheme = "http"
	}

	return &webhookURL
}

// getURL generates a service URL from the configuration using the provided resolver.
//
// Parameters:
//   - resolver: the resolver that builds the config props into the query.
//
// Returns:
//   - *url.URL: the service URL, with the webhook query, the config props, and
//     the custom headers and extra data in its query.
func (c *Config) getURL(resolver types.ConfigQueryResolver) *url.URL {
	// Copy webhook URL
	serviceURL := c.webhook()
	// Get existing query params
	webhookQuery := serviceURL.Query()
	// Build query with config fields
	serviceQuery := format.BuildQueryWithCustomFields(resolver, webhookQuery)
	// Add custom headers and extra data
	appendCustomQueryValues(serviceQuery, c.headers, c.extraData)
	// Encode the query
	serviceURL.RawQuery = serviceQuery.Encode()
	// Set service scheme
	serviceURL.Scheme = Scheme

	return &serviceURL
}

// setURL updates the configuration from a service URL using the provided resolver.
//
// Parameters:
//   - resolver: the resolver that sets the config props from the query.
//   - serviceURL: the service URL.
//
// Returns:
//   - error: [ErrHostMissing] for a URL without a host, which a webhook needs,
//     an error wrapping [util.ErrInvalidQuery] for a malformed query, or a
//     query prop error.
func (c *Config) setURL(resolver types.ConfigQueryResolver, serviceURL *url.URL) error {
	if err := validateWebhook(serviceURL); err != nil {
		return err
	}

	// Copy service URL
	webhookURL := *serviceURL
	// Extract query parameters
	serviceQuery, err := util.ParseURLQuery(serviceURL)
	if err != nil {
		return err
	}

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
	c.webhookURL = &webhookURL
	// Assign extracted headers
	c.headers = headers
	// Assign extracted extra data
	c.extraData = extraData

	return nil
}

// webhook returns a copy of the webhook URL. A config built in code without
// one gives an empty URL.
//
// Returns:
//   - url.URL: the webhook URL.
func (c *Config) webhook() url.URL {
	if c.webhookURL == nil {
		return url.URL{}
	}

	return *c.webhookURL
}

// maskedValues returns a copy of values with each value replaced by
// [format.MaskedValue].
//
// Parameters:
//   - values: the values to mask, such as the custom headers.
//
// Returns:
//   - map[string]string: the same keys with masked values.
func maskedValues(values map[string]string) map[string]string {
	masked := make(map[string]string, len(values))
	for key := range values {
		masked[key] = format.MaskedValue
	}

	return masked
}

// validateWebhook checks that a webhook URL has a host to send to. Reading a
// service URL checks the URL first, so a rejected URL leaves the config
// unchanged. Each send checks the config's webhook URL, so a config built in
// code fails before a request.
//
// Parameters:
//   - webhookURL: the webhook URL, or nil for a config built in code.
//
// Returns:
//   - error: [ErrHostMissing] for a webhook URL without a host, or nil.
func validateWebhook(webhookURL *url.URL) error {
	if webhookURL == nil || webhookURL.Hostname() == "" {
		return ErrHostMissing
	}

	return nil
}
