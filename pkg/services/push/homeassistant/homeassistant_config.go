package homeassistant

import (
	"fmt"
	"net"
	"net/url"
	"path"
	"strconv"
	"strings"

	"github.com/nicholas-fedor/shoutrrr/pkg/format"
	"github.com/nicholas-fedor/shoutrrr/pkg/types"
)

// Config holds settings for the Home Assistant notification service.
type Config struct {
	// Token is the long-lived access token sent as a Bearer credential.
	Token string `desc:"Long-lived access token" url:"user"`
	// Host is the Home Assistant hostname.
	Host string `desc:"Home Assistant hostname" url:"host"`
	// Port is the Home Assistant port. When omitted, HTTPS uses 443.
	Port int `desc:"Home Assistant port" optional:"" url:"port"`
	// Path is an optional reverse-proxy URL prefix.
	Path string `desc:"Reverse-proxy path prefix" optional:"" url:"path"`
	// Title is an optional notification title. It is omitted from the API request when empty.
	Title string `desc:"Notification title" key:"title" optional:""`
	// Service is the Home Assistant action to call. Empty defaults to persistent_notification.create.
	Service string `desc:"Home Assistant action (domain.action or notify target)" key:"service" optional:""`
	// Targets are notify-platform destination names. They are omitted for persistent notifications.
	Targets []string `desc:"Notify targets" key:"targets" optional:""`
	// Nid is the persistent notification ID. It is omitted when empty.
	Nid string `desc:"Persistent notification ID" key:"nid" optional:""`
}

const (
	// Scheme identifies this service in configuration URLs.
	Scheme = "homeassistant"

	// dummyServiceURL is the placeholder URL used by documentation generation.
	dummyServiceURL = "homeassistant://dummy@dummy.example"

	// defaultTLSPort is used when the port is omitted.
	defaultTLSPort = 443

	// persistentDomain is the Home Assistant domain for persistent notifications.
	persistentDomain = "persistent_notification"

	// persistentService is the Home Assistant action that creates a persistent notification.
	persistentService = "create"

	// notifyDomain is the default Home Assistant domain when service has no dot.
	notifyDomain = "notify"
)

// Enums returns the fields that use an EnumFormatter for their values.
//
// Returns:
//   - An empty map because this service has no enum fields.
func (*Config) Enums() map[string]types.EnumFormatter {
	return map[string]types.EnumFormatter{}
}

// GetURL returns a URL representation of the current configuration.
//
// Returns:
//   - The service URL encoding the current field values.
func (c *Config) GetURL() *url.URL {
	resolver := format.NewPropKeyResolver(c)

	return c.getURL(&resolver)
}

// SetURL updates the configuration from a URL representation.
//
// Parameters:
//   - serviceURL: The service URL to parse.
//
// Returns:
//   - An error if the URL is invalid or required fields are missing.
func (c *Config) SetURL(serviceURL *url.URL) error {
	resolver := format.NewPropKeyResolver(c)

	return c.setURL(&resolver, serviceURL)
}

// apiURL returns the absolute REST URL for a Home Assistant service call.
//
// Parameters:
//   - domain: The Home Assistant domain.
//   - service: The Home Assistant action.
//
// Returns:
//   - The absolute POST URL.
func (c *Config) apiURL(domain, service string) string {
	apiPath := path.Join(c.Path, "api", "services", domain, service)
	if apiPath == "" || apiPath[0] != '/' {
		apiPath = "/" + apiPath
	}

	return (&url.URL{
		Scheme: "https",
		Host:   net.JoinHostPort(c.Host, strconv.Itoa(c.requestPort())),
		Path:   apiPath,
	}).String()
}

// domainService resolves the Home Assistant domain and action from Service.
//
// Returns:
//   - domain: The Home Assistant domain.
//   - service: The Home Assistant action.
//   - err: An error if Service is set but not a valid domain.action pair.
func (c *Config) domainService() (string, string, error) {
	if c.Service == "" {
		return persistentDomain, persistentService, nil
	}

	if !strings.Contains(c.Service, ".") {
		if !validHASlug(c.Service) {
			return "", "", fmt.Errorf("%w: %q", ErrInvalidService, c.Service)
		}

		return notifyDomain, c.Service, nil
	}

	domain, service, _ := strings.Cut(c.Service, ".")
	if !validHASlug(domain) || !validHASlug(service) {
		return "", "", fmt.Errorf("%w: %q", ErrInvalidService, c.Service)
	}

	return domain, service, nil
}

// getURL constructs a service URL from the current configuration.
//
// Parameters:
//   - resolver: Resolver used to encode query parameters from config fields.
//
// Returns:
//   - The service URL with the token as userinfo and the host as hostname.
func (c *Config) getURL(resolver types.ConfigQueryResolver) *url.URL {
	host := c.Host
	if c.Port != 0 && c.Port != c.impliedPort() {
		host = net.JoinHostPort(c.Host, strconv.Itoa(c.Port))
	}

	return &url.URL{
		Scheme:   Scheme,
		User:     url.User(c.Token),
		Host:     host,
		Path:     c.Path,
		RawQuery: format.BuildQuery(resolver),
	}
}

// impliedPort returns the port used when the URL omits one.
//
// Returns:
//   - 443, the default HTTPS port.
func (*Config) impliedPort() int {
	return defaultTLSPort
}

// isPersistent reports whether the resolved action creates a persistent notification.
//
// Returns:
//   - true when the action is persistent_notification.create.
func (c *Config) isPersistent() bool {
	domain, service, err := c.domainService()
	if err != nil {
		return false
	}

	return domain == persistentDomain && service == persistentService
}

// requestPort returns the TCP port used for the API request.
//
// Returns:
//   - The configured port, or the implied default when the port was omitted.
func (c *Config) requestPort() int {
	if c.Port != 0 {
		return c.Port
	}

	return c.impliedPort()
}

// setPort parses an optional TCP port from the URL.
//
// Parameters:
//   - rawPort: The port string from the URL, or empty when omitted.
//
// Returns:
//   - An error if the port is not a valid integer in 1–65535.
func (c *Config) setPort(rawPort string) error {
	if rawPort == "" {
		c.Port = 0

		return nil
	}

	port, err := strconv.Atoi(rawPort)
	if err != nil || port < 1 || port > 65535 {
		return fmt.Errorf("%w: %q", ErrInvalidPort, rawPort)
	}

	c.Port = port

	return nil
}

// setURL updates the configuration from a service URL.
//
// Parameters:
//   - resolver: Resolver used to apply query parameters to config fields.
//   - serviceURL: The service URL to parse.
//
// Returns:
//   - An error if a query parameter is invalid or required fields are missing.
func (c *Config) setURL(resolver types.ConfigQueryResolver, serviceURL *url.URL) error {
	isDummy := serviceURL.String() == dummyServiceURL

	if serviceURL.User != nil {
		c.Token = serviceURL.User.Username()
	} else {
		c.Token = ""
	}

	c.Host = serviceURL.Hostname()

	if err := c.setPort(serviceURL.Port()); err != nil {
		return err
	}

	c.Path = normalizePath(serviceURL.Path)

	for key, vals := range serviceURL.Query() {
		if len(vals) == 0 {
			continue
		}

		if err := resolver.Set(key, vals[0]); err != nil {
			return fmt.Errorf("setting query parameter %q to %q: %w", key, vals[0], err)
		}
	}

	if isDummy {
		return nil
	}

	if c.Token == "" {
		return ErrTokenMissing
	}

	if c.Host == "" {
		return ErrHostMissing
	}

	if _, _, err := c.domainService(); err != nil {
		return err
	}

	return nil
}

// normalizePath returns a leading-slash path without a trailing slash.
//
// Parameters:
//   - raw: The URL path from the service URL.
//
// Returns:
//   - The normalized path, or empty when the path is missing or only slashes.
func normalizePath(raw string) string {
	trimmed := strings.Trim(raw, "/")
	if trimmed == "" {
		return ""
	}

	return "/" + trimmed
}

// validHASlug reports whether s is a Home Assistant domain or action name.
//
// Parameters:
//   - s: The candidate domain or action.
//
// Returns:
//   - true when s is non-empty and contains only ASCII letters, digits, and underscores.
func validHASlug(s string) bool {
	if s == "" {
		return false
	}

	for _, r := range s {
		switch {
		case r >= 'a' && r <= 'z', r >= 'A' && r <= 'Z', r >= '0' && r <= '9', r == '_':
			continue
		default:
			return false
		}
	}

	return true
}
