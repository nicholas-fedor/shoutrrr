package signal

import (
	"fmt"
	"net"
	"net/url"
	"strconv"
	"strings"

	"github.com/nicholas-fedor/shoutrrr/pkg/format"
	"github.com/nicholas-fedor/shoutrrr/pkg/types"
)

// Config holds settings for the Signal notification service.
type Config struct {
	Host          string   `default:"localhost" desc:"Signal REST API server hostname or IP"                            key:"host"`
	Port          int      `default:"8080"      desc:"Signal REST API server port"                                      key:"port"`
	User          string   `                    desc:"Username for HTTP Basic Auth"                                     key:"user"               optional:""`
	Password      string   `                    desc:"Password for HTTP Basic Auth"                                     key:"password"           optional:"" sensitive:"true"`
	Token         string   `                    desc:"API token for Bearer authentication"                              key:"token,apikey"       optional:"" sensitive:"true"`
	Source        string   `                    desc:"Source phone number (with country code)"                          key:"source"`
	Recipients    []string `                    desc:"Recipient phone numbers, group IDs, or u: usernames"              key:"recipients,to"`
	Title         string   `                    desc:"Optional title prepended to the message body"                     key:"title"              optional:""`
	Attachments   string   `                    desc:"Comma-separated raw base64 attachments; a data: value is one URI" key:"attachments"        optional:""`
	TextMode      textMode `default:"None"      desc:"Message text mode (None omits text_mode; Styled enables markup)"  key:"textmode,text_mode"`
	DisableTLS    bool     `default:"No"        desc:"Disable TLS for Signal REST API connection"                       key:"disabletls"`
	SkipTLSVerify bool     `default:"No"        desc:"Skip TLS certificate verification"                                key:"skiptlsverify"`
	NotifySelf    bool     `default:"Yes"       desc:"Notify the sender's devices. When No, sends notify_self=false."   key:"notifyself"`
}

const (
	// Scheme identifies this service in configuration URLs.
	Scheme = "signal"
	// minPathParts is the minimum number of path parts required (source + at least one recipient).
	minPathParts = 2
)

// Enums returns the fields that use an EnumFormatter for their values.
//
// Returns:
//   - A map of config field names to their enum formatters.
func (*Config) Enums() map[string]types.EnumFormatter {
	return map[string]types.EnumFormatter{
		"TextMode": TextModes.Enum,
	}
}

// GetURL generates a URL from the current configuration values.
//
// Returns:
//   - *url.URL: the generated URL representing the configuration.
func (c *Config) GetURL() *url.URL {
	resolver := format.NewPropKeyResolver(c)

	return c.getURL(&resolver)
}

// SetURL updates the configuration from a URL representation.
//
// Parameters:
//   - serviceURL: the URL to parse configuration from
//
// Returns:
//   - error: if parsing fails, nil otherwise
func (c *Config) SetURL(serviceURL *url.URL) error {
	resolver := format.NewPropKeyResolver(c)

	return c.setURL(&resolver, serviceURL)
}

// getURL constructs a URL from the Config's fields using the provided resolver.
//
// Parameters:
//   - resolver: the configuration query resolver for property resolution
//
// Returns:
//   - *url.URL: the constructed URL
func (c *Config) getURL(resolver types.ConfigQueryResolver) *url.URL {
	recipients := strings.Join(c.Recipients, "/")

	result := &url.URL{
		Scheme:   Scheme,
		Host:     net.JoinHostPort(c.Host, strconv.Itoa(c.Port)),
		Path:     fmt.Sprintf("/%s/%s", c.Source, recipients),
		RawQuery: format.BuildQuery(resolver),
	}

	if c.User != "" {
		if c.Password != "" {
			result.User = url.UserPassword(c.User, c.Password)
		} else {
			result.User = url.User(c.User)
		}
	}

	return result
}

// parseAuth extracts user and password from the URL.
//
// Parameters:
//   - serviceURL: the URL to extract authentication from
func (c *Config) parseAuth(serviceURL *url.URL) {
	if serviceURL.User != nil {
		c.User = serviceURL.User.Username()
		if password, ok := serviceURL.User.Password(); ok {
			c.Password = password
		}
	}
}

// parseHostPort extracts host and port from the URL.
//
// Parameters:
//   - serviceURL: the URL to extract host and port from
func (c *Config) parseHostPort(serviceURL *url.URL) {
	host := serviceURL.Hostname()
	if host == "" {
		host = serviceURL.Host
	}

	c.Host = host

	portStr := serviceURL.Port()
	if portStr == "" {
		portStr = "8080"
	}

	if port, err := strconv.Atoi(portStr); err == nil {
		c.Port = port
	}
}

// parsePath extracts source phone number and recipients from the URL path.
//
// Parameters:
//   - serviceURL: the URL to extract path from
//
// Returns:
//   - error: if parsing fails, nil otherwise
func (c *Config) parsePath(serviceURL *url.URL) error {
	pathParts := strings.Split(strings.Trim(serviceURL.Path, "/"), "/")
	if len(pathParts) < minPathParts {
		return ErrNoRecipients
	}

	source := pathParts[0]
	if !isValidPhoneNumber(source) {
		return fmt.Errorf("%w: %s", ErrInvalidPhoneNumber, source)
	}

	c.Source = source

	recipients, err := parseRecipients(pathParts[1:])
	if err != nil {
		return err
	}

	c.Recipients = recipients

	return nil
}

// parseQuery processes query parameters using the resolver.
//
// Parameters:
//   - resolver: the configuration query resolver
//   - serviceURL: the URL containing query parameters
//
// Returns:
//   - error: if parsing fails, nil otherwise
func (c *Config) parseQuery(resolver types.ConfigQueryResolver, serviceURL *url.URL) error {
	for key, vals := range serviceURL.Query() {
		if err := resolver.Set(key, vals[0]); err != nil {
			return fmt.Errorf("setting config property %q from URL query: %w", key, err)
		}
	}

	return nil
}

// setURL updates the Config from a URL using the provided resolver.
//
// Parameters:
//   - resolver: the configuration query resolver
//   - serviceURL: the URL to parse
//
// Returns:
//   - error: if parsing fails, nil otherwise
func (c *Config) setURL(resolver types.ConfigQueryResolver, serviceURL *url.URL) error {
	if serviceURL.String() == "signal://dummy@dummy.com" {
		c.Host = "localhost"
		c.Port = 8080
		c.Source = "+1234567890"
		c.Recipients = []string{"+0987654321"}
		c.DisableTLS = false

		return nil
	}

	c.parseAuth(serviceURL)
	c.parseHostPort(serviceURL)

	if err := c.parsePath(serviceURL); err != nil {
		return err
	}

	if err := c.parseQuery(resolver, serviceURL); err != nil {
		return err
	}

	return nil
}
