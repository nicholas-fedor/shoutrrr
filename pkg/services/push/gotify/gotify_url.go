package gotify

import "net/url"

// URLBuilder handles URL construction for Gotify API endpoints.
type URLBuilder interface {
	BuildURL(config *Config) (string, error)
}

// DefaultURLBuilder provides the default implementation of URLBuilder.
type DefaultURLBuilder struct{}

// BuildURL constructs the Gotify API URL with scheme, host, path, and token.
// This function builds the complete endpoint URL for the Gotify message API, handling
// different authentication methods (header vs query parameter) and TLS settings.
// The URL format depends on whether header authentication is enabled.
// Parameters:
//   - config: Configuration containing host, path, token, and authentication settings
//
// Returns: complete API URL string.
func (b *DefaultURLBuilder) BuildURL(config *Config) (string, error) {
	// Determine URL scheme based on TLS settings
	scheme := "https"
	if config.DisableTLS {
		scheme = "http" // Use HTTP scheme when TLS verification is disabled
	}

	// Construct URL using url.URL for proper encoding
	apiURL := &url.URL{
		Scheme: scheme,
		Host:   config.Host,
		Path:   config.Path + "/message",
	}

	if !config.UseHeader {
		// Query parameter authentication: include token in URL query string
		q := apiURL.Query()
		q.Set("token", config.Token)
		apiURL.RawQuery = q.Encode()
	}

	return apiURL.String(), nil
}
