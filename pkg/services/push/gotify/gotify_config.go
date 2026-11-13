package gotify

import (
	"encoding/json/v2"
	"fmt"
	"maps"
	"net/url"
	"strings"

	"github.com/nicholas-fedor/shoutrrr/internal/jsonopts"
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
	// EnumlessConfig provides the empty enum set, since Gotify has no enum
	// fields.
	standard.EnumlessConfig

	// Extras holds the custom key-value pairs sent with each message, parsed
	// from JSON.
	Extras map[string]any

	// Token is the application token. Gotify 3.0 and later issue gtfya.
	// tokens. Earlier versions issue 15-character tokens starting with A,
	// which Gotify 3 still accepts. A client token is rejected, because
	// Gotify sends a message for one only with an application ID.
	Token string `desc:"Application token" required:"" sensitive:"true" url:"path2"`
	// Host is the server host, with an optional port.
	Host string `desc:"Server hostname (and optionally port)" required:"" url:"host,port"`
	// Path is the subpath of a Gotify installation, such as /gotify.
	Path string `desc:"Server subpath" optional:"" url:"path1"`
	// Title is the message title.
	Title string `default:"Shoutrrr notification" desc:"Notification title" key:"title"`

	// Date is accepted in URLs and params but not sent.
	//
	// Deprecated: Gotify stamps each message with the server time, so a date has no effect.
	Date string `default:"" desc:"Deprecated: not sent, because Gotify sets the date of each message" key:"date"`

	// Priority is the message priority, from -2 to 10, higher being more
	// important. It is sent only when the URL or the send sets it, or the
	// level param chooses it.
	Priority int `default:"0" desc:"Message priority, from -2 to 10" key:"priority"`
	// DisableTLS sends the request over HTTP instead of HTTPS.
	DisableTLS bool `default:"No" desc:"Send over HTTP instead of HTTPS" key:"disabletls"`
	// SkipTLSVerify skips verification of the server's TLS certificate.
	SkipTLSVerify bool `default:"No" deprecatedkeys:"insecureskipverify" desc:"Skip TLS certificate verification" key:"skiptlsverify"`
	// InsecureSkipVerify is the old name of SkipTLSVerify. When SkipTLSVerify
	// is off, setting it skips verification too.
	//
	// Deprecated: Use [Config.SkipTLSVerify], which the skiptlsverify key sets.
	InsecureSkipVerify bool `deprecated:"SkipTLSVerify"`
	// UseHeader sends the token in the X-Gotify-Key header instead of the
	// URL query, where access logs record it. A URL without the useheader key
	// turns it on, and a config built in code turns it on by setting it.
	UseHeader bool `default:"Yes" desc:"Send the token in the X-Gotify-Key header instead of the URL query" key:"useheader"`

	// prioritySet is true when the URL, the send params, or the level set the
	// priority, so a priority of 0 is sent instead of leaving the application
	// default. Reading a URL records whether that URL set it, and each send
	// adds its params and level.
	prioritySet bool
}

// Service identity.
const (
	// Scheme identifies this service in configuration URLs.
	Scheme = "gotify"
)

// extrasKey names the extras JSON in the URL query and the send params. It is
// not a config key, so the query and the params are read for it directly.
const extrasKey = "extras"

// GetURL returns a URL representation of the current configuration. A value
// set through a deprecated field is written under its replacement's key.
//
// Extras that cannot be written as JSON, such as a channel, are left out of
// the URL. A send with those extras fails with the JSON error, so the problem
// still surfaces.
//
// Returns:
//   - *url.URL: the service URL.
func (c *Config) GetURL() *url.URL {
	synced := *c

	_, _ = format.SyncDeprecatedFields(&synced) //nolint:errcheck // The tags are valid, and a failed sync changes nothing.
	resolver := format.NewPropKeyResolver(&synced)

	return synced.getURL(&resolver)
}

// SetURL updates the configuration from a URL representation.
//
// Parameters:
//   - serviceURL: the service URL.
//
// Returns:
//   - error: the error from reading the URL, such as [ErrInvalidToken].
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
		// Extras that cannot be written as JSON, such as a channel, are left
		// out of the URL.
		extrasJSON, err := json.Marshal(c.Extras, jsonopts.Default)
		if err == nil {
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
// JSON parameter, setting appropriate config fields through the resolver or direct assignment.
//
// Parameters:
//   - resolver: Configuration resolver for setting standard config properties
//   - query: URL query parameters to process
//
// Returns:
//   - error: [ErrExtrasParseFailed], or [ErrConfigPropertyFailed] wrapping
//     the error of [format.ApplyQuery], which names the key, or nil.
func (c *Config) processQueryParameters(
	resolver *format.PropKeyResolver,
	query url.Values,
) error {
	// extras holds JSON, which the config sets directly rather than through a
	// config key.
	if extras := query.Get(extrasKey); extras != "" {
		c.Extras = make(map[string]any)

		if err := json.Unmarshal([]byte(extras), &c.Extras, jsonopts.UserInput); err != nil {
			return fmt.Errorf("%w: %w", ErrExtrasParseFailed, err)
		}
	}

	configQuery := maps.Clone(query)
	delete(configQuery, extrasKey)

	if err := format.ApplyQuery(resolver, configQuery); err != nil {
		return fmt.Errorf("%w: %w", ErrConfigPropertyFailed, err)
	}

	// A priority this URL sets under any spelling, even 0, is sent.
	c.prioritySet = !resolver.KeepsDefault(c, nil, "priority")

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
//   - error: an error wrapping [util.ErrInvalidQuery] for a malformed query or
//     a query parameter error. Otherwise the error from the config's checks,
//     such as [ErrInvalidToken] for a missing token.
func (c *Config) setURL(resolver *format.PropKeyResolver, serviceURL *url.URL) error {
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

	c.Host = serviceURL.Host
	c.Token = path[tokenIndex:]

	query, err := util.ParseURLQuery(serviceURL)
	if err != nil {
		return err
	}

	// The options start from their defaults, so an option the URL leaves out
	// does not keep the value an earlier URL set. The extras have no default
	// tag, so they are cleared here.
	if err := resolver.SetDefaultProps(c); err != nil {
		return fmt.Errorf("setting default props: %w", err)
	}

	c.Extras = nil

	// Process query parameters to set remaining configuration fields
	if err := c.processQueryParameters(resolver, query); err != nil {
		return fmt.Errorf("failed to process query parameters: %w", err)
	}

	return c.validate()
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

// validate checks the host and the token that every request needs.
//
// Reading a service URL and each send run it, so a config built in code fails
// before a request. A Gotify server has no default host.
//
// Returns:
//   - error: [ErrHostMissing] for a config without a host name. An error
//     wrapping [util.ErrInvalidHost] or [util.ErrInvalidPort] for a host that
//     a URL cannot hold. An error wrapping [util.ErrInvalidPathSegment] for a
//     subpath with a "." or ".." segment. [ErrInvalidToken] for a missing
//     token, a client token, or a token in neither application token format.
//     Otherwise nil.
func (c *Config) validate() error {
	// The host may carry a port, so a port alone names no host.
	host, _, err := util.SplitURLHost(&url.URL{Host: c.Host})
	if err != nil {
		return fmt.Errorf("host: %w", err)
	}

	if host == "" {
		return ErrHostMissing
	}

	if err := util.ValidatePathSegments(c.Path); err != nil {
		return fmt.Errorf("subpath: %w", err)
	}

	switch {
	case (&DefaultValidator{}).ValidateToken(c.Token):
		return nil
	case hasTokenForm(c.Token, clientTokenPrefix, legacyClientPrefix):
		return fmt.Errorf("%w: client tokens need an application ID, so use an application token", ErrInvalidToken)
	default:
		return ErrInvalidToken
	}
}
