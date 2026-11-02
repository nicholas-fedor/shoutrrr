package bark

import (
	"errors"
	"fmt"
	"net/url"
	"strings"

	"github.com/nicholas-fedor/shoutrrr/pkg/format"
	"github.com/nicholas-fedor/shoutrrr/pkg/services/standard"
	"github.com/nicholas-fedor/shoutrrr/pkg/types"
	"github.com/nicholas-fedor/shoutrrr/pkg/util"
)

// Config holds configuration settings for the Bark service.
type Config struct {
	// EnumlessConfig provides the empty enum set, since Bark has no enum
	// fields.
	standard.EnumlessConfig

	// Title is the notification title.
	Title string `default:"" desc:"Notification title, optionally set by the sender" key:"title"`
	// Host is the server host, with an optional port.
	Host string `desc:"Server hostname and port" url:"host"`
	// Path is the server path that the API endpoints are appended to.
	Path string `default:"/" desc:"Server path" url:"path"`
	// DeviceKey is the key of the device that receives the notifications.
	// It is the password of the service URL.
	DeviceKey string `desc:"The key for each device" sensitive:"true" url:"password"`
	// Scheme is the server's protocol, http or https. A URL or params that set
	// it to http turn TLS off, and GetURL writes that as disabletls.
	//
	// Deprecated: Use [Config.DisableTLS], which the disabletls key sets.
	Scheme string `default:"https" desc:"Deprecated: use disabletls. Server protocol, http or https" key:"scheme"`
	// Sound is the name of the notification sound.
	Sound string `default:"" desc:"Value from https://github.com/Finb/Bark/tree/master/Sounds" key:"sound"`
	// Icon is the URL of the notification icon, shown on iOS 15 or later.
	Icon string `default:"" desc:"An url to the icon, available only on iOS 15 or later" key:"icon"`
	// Group is the group of the notification.
	Group string `default:"" desc:"The group of the notification" key:"group"`
	// URL is the URL to open when the notification is tapped.
	URL string `default:"" desc:"Url that will jump when click notification" key:"url"`
	// Category is a field that Bark reserves for future use.
	Category string `default:"" desc:"Reserved field, no use yet" key:"category"`
	// Copy is the text to copy to the clipboard when the notification is
	// tapped.
	Copy string `default:"" desc:"The value to be copied" key:"copy"`
	// Badge is the number displayed next to the app icon.
	Badge int64 `default:"0" desc:"The number displayed next to App icon" key:"badge"`
	// DisableTLS sends over HTTP whatever Scheme says.
	DisableTLS bool `default:"no" desc:"Send over HTTP instead of HTTPS" key:"disabletls"`
	// SkipTLSVerify skips verification of the server's TLS certificate.
	SkipTLSVerify bool `default:"no" desc:"Skip TLS certificate verification" key:"skiptlsverify"`
}

// Scheme is the URL scheme identifier for the Bark service.
const Scheme = "bark"

// Protocols that the deprecated scheme accepts.
const (
	// schemeHTTP sends over plain HTTP.
	schemeHTTP = "http"
	// schemeHTTPS sends over HTTPS, the default.
	schemeHTTPS = "https"
)

// ErrSetQueryFailed indicates a failure to set a configuration value from a query parameter.
var ErrSetQueryFailed = errors.New("failed to set query parameter")

// GetAPIURL constructs the full API URL for a given endpoint.
// The endpoint is appended to the configured host and path. The URL uses HTTP
// when TLS is disabled or the deprecated scheme is http, and HTTPS otherwise.
//
// Parameters:
//   - endpoint: the API endpoint, such as "push".
//
// Returns:
//   - string: the API URL for the endpoint.
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

	scheme := schemeHTTPS
	if c.DisableTLS || strings.EqualFold(c.Scheme, schemeHTTP) {
		scheme = schemeHTTP
	}

	apiURL := url.URL{
		Scheme: scheme,
		Host:   c.Host,
		Path:   path.String(),
	}

	return apiURL.String()
}

// GetURL returns a URL representation of the current configuration.
// This URL can be used to share or persist the service configuration. An
// http scheme is written as disabletls, and the config is left unchanged.
//
// Returns:
//   - *url.URL: the service URL, with the options as query parameters.
func (c *Config) GetURL() *url.URL {
	normalized := *c
	normalized.normalizeScheme()
	resolver := format.NewPropKeyResolver(&normalized)

	return normalized.getURL(&resolver)
}

// SetURL updates the configuration from a URL representation.
// The URL should contain the device key as the password and optional query parameters.
//
// Parameters:
//   - serviceURL: URL containing the service configuration.
//
// Returns:
//   - error: the error from reading the URL, such as [ErrMissingDeviceKey]
//     or [ErrInvalidScheme].
func (c *Config) SetURL(serviceURL *url.URL) error {
	resolver := format.NewPropKeyResolver(c)

	return c.setURL(&resolver, serviceURL)
}

// applyScheme checks the deprecated Scheme and turns TLS off when it is http,
// so DisableTLS alone decides the protocol. An empty Scheme counts as https.
//
// Returns:
//   - error: [ErrInvalidScheme] for a scheme other than http or https.
func (c *Config) applyScheme() error {
	switch strings.ToLower(c.Scheme) {
	case schemeHTTP:
		c.DisableTLS = true
	case schemeHTTPS, "":
	default:
		return fmt.Errorf("%w: %q", ErrInvalidScheme, c.Scheme)
	}

	return nil
}

// getURL generates a URL representation using the provided resolver.
// This internal method handles the actual URL construction logic.
//
// Parameters:
//   - resolver: the resolver that builds the query from the config props.
//
// Returns:
//   - *url.URL: the service URL, with the device key as its password.
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

// normalizeScheme writes an http scheme as DisableTLS and resets the scheme to
// its default, so a config URL carries disabletls instead of the deprecated
// scheme key. An invalid scheme is kept, so reading the URL reports it.
func (c *Config) normalizeScheme() {
	switch strings.ToLower(c.Scheme) {
	case schemeHTTP:
		c.DisableTLS = true
		c.Scheme = schemeHTTPS
	case schemeHTTPS, "":
		c.Scheme = schemeHTTPS
	default:
	}
}

// setURL parses a service URL and updates the configuration accordingly.
// It extracts the device key from the password field and host from the URL.
// A scheme of http turns TLS off, and disabletls wins when the query sets
// both.
//
// Parameters:
//   - resolver: the resolver that sets the query parameters.
//   - serviceURL: the service URL.
//
// Returns:
//   - error: a malformed query ([util.ErrInvalidQuery]), a query parameter
//     error, [ErrInvalidScheme], or [ErrMissingDeviceKey] or [ErrMissingHost]
//     for a missing field.
func (c *Config) setURL(resolver types.ConfigQueryResolver, serviceURL *url.URL) error {
	password, _ := serviceURL.User.Password()
	c.DeviceKey = password
	c.Host = serviceURL.Host
	c.Path = serviceURL.Path

	query, err := util.ParseURLQuery(serviceURL)
	if err != nil {
		return err
	}

	if err := format.ApplyQuery(resolver, query); err != nil {
		return fmt.Errorf("%w: %w", ErrSetQueryFailed, err)
	}

	disableTLS := c.DisableTLS

	if err := c.applyScheme(); err != nil {
		return err
	}

	// disabletls replaces scheme, so it wins when the query sets both. The
	// scheme resets, so a send that applies the stored scheme keeps the choice.
	if hasKey(query, schemeKey.Replacement) {
		c.DisableTLS = disableTLS
		c.Scheme = schemeHTTPS
	}

	return c.validate()
}

// validate checks that the config has the device key and the host a push
// needs. Reading a service URL and each send run it, so a config built in code
// fails before a request.
//
// Returns:
//   - error: [ErrMissingDeviceKey] or [ErrMissingHost] for a missing field, or
//     nil.
func (c *Config) validate() error {
	if c.DeviceKey == "" {
		return ErrMissingDeviceKey
	}

	if c.Host == "" {
		return ErrMissingHost
	}

	return nil
}

// hasKey reports whether values set key, matched case-insensitively as the
// config keys are.
//
// Parameters:
//   - values: the URL query or the send params.
//   - key: the key in lower case.
//
// Returns:
//   - bool: whether values hold the key.
func hasKey[V any](values map[string]V, key string) bool {
	for name := range values {
		if strings.EqualFold(name, key) {
			return true
		}
	}

	return false
}
