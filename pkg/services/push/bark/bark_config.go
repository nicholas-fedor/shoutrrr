package bark

import (
	"errors"
	"fmt"
	"net/url"
	"strings"

	"github.com/nicholas-fedor/shoutrrr/pkg/format"
	"github.com/nicholas-fedor/shoutrrr/pkg/services/standard"
	"github.com/nicholas-fedor/shoutrrr/pkg/types"
)

// Config holds configuration settings for the Bark service.
type Config struct {
	standard.EnumlessConfig

	Title     string `default:""      desc:"Notification title, optionally set by the sender"           key:"title"`
	Host      string `                desc:"Server hostname and port"                                                  url:"host"`
	Path      string `default:"/"     desc:"Server path"                                                               url:"path"`
	DeviceKey string `                desc:"The key for each device"                                                   url:"password"`
	Scheme    string `default:"https" desc:"Server protocol, http or https"                             key:"scheme"`
	Sound     string `default:""      desc:"Value from https://github.com/Finb/Bark/tree/master/Sounds" key:"sound"`
	Badge     int64  `default:"0"     desc:"The number displayed next to App icon"                      key:"badge"`
	Icon      string `default:""      desc:"An url to the icon, available only on iOS 15 or later"      key:"icon"`
	Group     string `default:""      desc:"The group of the notification"                              key:"group"`
	URL       string `default:""      desc:"Url that will jump when click notification"                 key:"url"`
	Category  string `default:""      desc:"Reserved field, no use yet"                                 key:"category"`
	Copy      string `default:""      desc:"The value to be copied"                                     key:"copy"`
}

// Scheme is the URL scheme identifier for the Bark service.
const Scheme = "bark"

// ErrSetQueryFailed indicates a failure to set a configuration value from a query parameter.
var ErrSetQueryFailed = errors.New("failed to set query parameter")

// GetAPIURL constructs the full API URL for a given endpoint.
// The endpoint is appended to the configured host and path.
//
// Parameters:
//   - endpoint: The API endpoint to access (e.g., "push", "register").
//
// Returns:
//   - The complete URL string for the API request.
func (c *Config) GetAPIURL(endpoint string) string {
	path := strings.Builder{}
	if !strings.HasPrefix(c.Path, "/") {
		path.WriteByte('/')
	}

	path.WriteString(c.Path)

	if !strings.HasSuffix(path.String(), "/") {
		path.WriteByte('/')
	}

	path.WriteString(endpoint)

	apiURL := url.URL{
		Scheme: c.Scheme,
		Host:   c.Host,
		Path:   path.String(),
	}

	return apiURL.String()
}

// GetURL returns a URL representation of the current configuration.
// This URL can be used to share or persist the service configuration.
//
// Returns:
//   - A URL struct containing the configuration as query parameters.
func (c *Config) GetURL() *url.URL {
	resolver := format.NewPropKeyResolver(c)

	return c.getURL(&resolver)
}

// SetURL updates the configuration from a URL representation.
// The URL should contain the device key as the password and optional query parameters.
//
// Parameters:
//   - serviceURL: URL containing the service configuration.
//
// Returns:
//   - An error if the URL is invalid or missing required parameters.
func (c *Config) SetURL(serviceURL *url.URL) error {
	resolver := format.NewPropKeyResolver(c)

	return c.setURL(&resolver, serviceURL)
}

// getURL generates a URL representation using the provided resolver.
// This internal method handles the actual URL construction logic.
//
// Parameters:
//   - resolver: Configuration query resolver for building URL parameters.
//
// Returns:
//   - A URL struct with embedded credentials and query parameters.
func (c *Config) getURL(resolver types.ConfigQueryResolver) *url.URL {
	return &url.URL{
		User:       url.UserPassword("", c.DeviceKey),
		Host:       c.Host,
		Scheme:     Scheme,
		ForceQuery: true,
		Path:       c.Path,
		RawQuery:   format.BuildQuery(resolver),
	}
}

// setURL parses a service URL and updates the configuration accordingly.
// It extracts the device key from the password field and host from the URL.
//
// Parameters:
//   - resolver: Configuration query resolver for setting query parameters.
//   - serviceURL: URL containing the service configuration.
//
// Returns:
//   - An error if required fields are missing or query parameters are invalid.
func (c *Config) setURL(resolver types.ConfigQueryResolver, serviceURL *url.URL) error {
	password, _ := serviceURL.User.Password()
	c.DeviceKey = password
	c.Host = serviceURL.Host
	c.Path = serviceURL.Path

	// Validate required fields
	if c.DeviceKey == "" {
		return ErrMissingDeviceKey
	}

	if c.Host == "" {
		return ErrMissingHost
	}

	for key, vals := range serviceURL.Query() {
		if err := resolver.Set(key, vals[0]); err != nil {
			return fmt.Errorf("%w '%s': %w", ErrSetQueryFailed, key, err)
		}
	}

	return nil
}
