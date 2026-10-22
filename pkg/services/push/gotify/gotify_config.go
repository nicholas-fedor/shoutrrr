package gotify

import (
	"encoding/json"
	"fmt"
	"log"
	"net/url"
	"strings"

	"github.com/nicholas-fedor/shoutrrr/pkg/format"
	"github.com/nicholas-fedor/shoutrrr/pkg/services/standard"
	"github.com/nicholas-fedor/shoutrrr/pkg/types"
	"github.com/nicholas-fedor/shoutrrr/pkg/util"
)

// Config holds settings for the Gotify notification service.
// This struct contains all configuration parameters needed to connect to and authenticate
// with a Gotify server, including connection details, authentication credentials,
// notification defaults, and additional metadata.
type Config struct {
	standard.EnumlessConfig // Embeds standard configuration functionality without enum handling

	Extras map[string]any // Additional extras parsed from JSON - custom key-value pairs sent with notifications

	Token string `desc:"Application token"                     required:"" url:"path2"`     // Gotify application token for authentication (gtfya. token from Gotify 3.0 or later, or 15 chars starting with 'A' from earlier versions)
	Host  string `desc:"Server hostname (and optionally port)" required:"" url:"host,port"` // Gotify server hostname and optional port number
	Path  string `desc:"Server subpath"                        optional:"" url:"path1"`     // Optional subpath for Gotify installation (e.g., "/gotify")
	Title string `default:"Shoutrrr notification"              key:"title"`                 // Default notification title when none provided

	// Date is accepted in URLs and params but not sent.
	//
	// Deprecated: Gotify stamps each message with the server time, so a date has no effect.
	Date string `default:"" desc:"Deprecated: not sent, because Gotify sets the date of each message" key:"date"`

	Priority           int  `default:"0"  key:"priority"`                                            // Notification priority level (-2 to 10, where higher numbers are more important; negative values have special meanings in some clients)
	DisableTLS         bool `default:"No" key:"disabletls"`                                          // Disable TLS in URL scheme only (use HTTP instead of HTTPS)
	InsecureSkipVerify bool `default:"No" key:"insecureskipverify"`                                  // Skip TLS certificate verification (insecure, use with caution)
	UseHeader          bool `default:"No" desc:"Enable header-based authentication" key:"useheader"` // Send token in X-Gotify-Key header instead of URL query parameter

	// prioritySet is true when the URL or the send params set the priority, so
	// a priority of 0 is sent instead of leaving the application default.
	prioritySet bool
}

// Scheme identifies this service in configuration URLs.
const (
	Scheme = "gotify"
)

// GetURL returns a URL representation of the current configuration.
func (c *Config) GetURL() *url.URL {
	resolver := format.NewPropKeyResolver(c)

	return c.getURL(&resolver)
}

// SetURL updates the configuration from a URL representation.
func (c *Config) SetURL(serviceURL *url.URL) error {
	resolver := format.NewPropKeyResolver(c)

	return c.setURL(&resolver, serviceURL)
}

// getURL generates a URL from the current configuration values.
// This internal method constructs a URL representation of the configuration,
// including all settings as query parameters and extras as JSON in the query string.
// Used for serialization and URL reconstruction.
// Parameters:
//   - resolver: Configuration resolver for building query parameters from config fields
//
// Returns: *url.URL containing the complete configuration as a URL.
func (c *Config) getURL(resolver types.ConfigQueryResolver) *url.URL {
	// Build base query string from configuration fields using the resolver
	query := format.BuildQuery(resolver)

	// Handle extras serialization if present
	if c.Extras != nil {
		// Marshal extras map to JSON string
		extrasJSON, err := json.Marshal(c.Extras)
		if err != nil {
			// Skip adding extras when Extras cannot be serialized
			log.Printf("Failed to marshal Extras %v: %v, skipping extras", c.Extras, err)
		} else {
			// Append extras to query string with proper URL encoding
			if query != "" {
				query += "&"
			}

			query += "extras=" + url.QueryEscape(string(extrasJSON))
		}
	}

	// Construct and return the complete URL
	return &url.URL{
		Host:       c.Host,        // Server hostname and port
		Scheme:     Scheme,        // URL scheme (gotify)
		ForceQuery: false,         // Don't force query string presence
		Path:       c.tokenPath(), // Path with token appended
		RawQuery:   query,         // Query parameters including extras
	}
}

// processQueryParameters processes query parameters from URL.
// This function handles both standard configuration parameters and the special 'extras'
// JSON parameter, setting appropriate config fields through the resolver or direct assignment,
// and notes a priority the query sets.
//
// Parameters:
//   - resolver: Configuration resolver for setting standard config properties
//   - query: URL query parameters to process
//
// Returns:
//   - error: [ErrExtrasParseFailed] or [ErrConfigPropertyFailed] with the key
//     and the cause, or nil.
func (c *Config) processQueryParameters(
	resolver types.ConfigQueryResolver,
	query url.Values,
) error {
	// Iterate through all query parameters
	for key := range query {
		if key == "extras" {
			// Special handling for extras JSON parameter
			if query.Get(key) != "" {
				// Initialize extras map
				c.Extras = make(map[string]any)
				// Parse JSON string into map
				if err := json.Unmarshal([]byte(query.Get(key)), &c.Extras); err != nil {
					return fmt.Errorf("%w: %w", ErrExtrasParseFailed, err)
				}
			}
		} else {
			// Standard parameter handling through resolver
			if err := resolver.Set(key, query.Get(key)); err != nil {
				return fmt.Errorf("%w %q: %w", ErrConfigPropertyFailed, key, err)
			}

			if strings.EqualFold(key, "priority") {
				c.prioritySet = true
			}
		}
	}

	return nil
}

// setURL updates the configuration from a URL representation.
// This internal method parses a URL and extracts configuration values including
// host, path, token, and query parameters, populating the config struct fields.
// Used for deserialization from URL format.
// Parameters:
//   - resolver: Configuration resolver for setting config fields from query parameters
//   - url: The URL to parse configuration values from
//
// Returns:
//   - error: [ErrHostMissing] for a URL without a host, [ErrInvalidToken] for a
//     missing token or one in neither Gotify token format, an error wrapping
//     [util.ErrInvalidQuery] for a malformed query, or a query parameter error.
func (c *Config) setURL(resolver types.ConfigQueryResolver, serviceURL *url.URL) error {
	// Extract and clean the path from the URL
	path := serviceURL.Path
	if path != "" && path[len(path)-1] == '/' {
		path = path[:len(path)-1] // Remove trailing slash if present
	}

	// Find the last slash to separate path from token
	tokenIndex := strings.LastIndex(path, "/") + 1

	// Extract path component (everything before the token)
	c.Path = path[:tokenIndex]
	if c.Path == "/" {
		c.Path = c.Path[1:] // Remove leading slash to normalize empty path
	}

	// Set host and token from URL components. A Gotify server has no default host.
	if serviceURL.Hostname() == "" {
		return ErrHostMissing
	}

	c.Host = serviceURL.Host
	c.Token = path[tokenIndex:]
	c.prioritySet = false

	// The documentation dummy URL has no token.
	if serviceURL.String() != "gotify://dummy@dummy.com" && !(&DefaultValidator{}).ValidateToken(c.Token) {
		return ErrInvalidToken
	}

	query, err := util.ParseURLQuery(serviceURL)
	if err != nil {
		return err
	}

	// Process query parameters to set remaining configuration fields
	if err := c.processQueryParameters(resolver, query); err != nil {
		return fmt.Errorf("failed to process query parameters: %w", err)
	}

	return nil
}

// tokenPath returns the URL path of the subpath and the token, separated by a
// slash when the subpath has none at its end.
//
// Returns:
//   - string: the URL path.
func (c *Config) tokenPath() string {
	if c.Path == "" || strings.HasSuffix(c.Path, "/") {
		return c.Path + c.Token
	}

	return c.Path + "/" + c.Token
}
