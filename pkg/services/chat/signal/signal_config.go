package signal

import (
	"fmt"
	"net"
	"net/url"
	"regexp"
	"strconv"
	"strings"

	"github.com/nicholas-fedor/shoutrrr/pkg/format"
	"github.com/nicholas-fedor/shoutrrr/pkg/services/standard"
	"github.com/nicholas-fedor/shoutrrr/pkg/types"
)

// Config holds settings for the Signal notification service.
//
//nolint:gosec // Password field has sensitive tag to prevent accidental logging
type Config struct {
	standard.EnumlessConfig

	Host       string   `default:"localhost" desc:"Signal REST API server hostname or IP"      key:"host"`
	Port       int      `default:"8080"      desc:"Signal REST API server port"                key:"port"`
	User       string   `                    desc:"Username for HTTP Basic Auth"               key:"user"`
	Password   string   `                    desc:"Password for HTTP Basic Auth"               key:"password"      sensitive:"true"`
	Token      string   `                    desc:"API token for Bearer authentication"        key:"token,apikey"`
	Source     string   `                    desc:"Source phone number (with country code)"    key:"source"`
	Recipients []string `                    desc:"Recipient phone numbers or group IDs"       key:"recipients,to"`
	DisableTLS bool     `default:"No"        desc:"Disable TLS for Signal REST API connection" key:"disabletls"`
}

// Scheme identifies this service in configuration URLs.
const (
	Scheme = "signal"
	// minPathParts is the minimum number of path parts required (source + at least one recipient).
	minPathParts = 2
)

// phoneRegex validates phone number format (with or without + prefix).
var phoneRegex = regexp.MustCompile(`^\+?[0-9\s)(+-]+$`)

// groupRegex validates group ID format.
var groupRegex = regexp.MustCompile(`^group\.[a-zA-Z0-9_+/=-]+$`)

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
		Host:     fmt.Sprintf("%s:%d", c.Host, c.Port),
		Path:     fmt.Sprintf("/%s/%s", c.Source, recipients),
		RawQuery: format.BuildQuery(resolver),
	}

	// Add user:password if authentication is configured
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
//
// Returns:
//   - error: if extraction fails, nil otherwise
func (c *Config) parseAuth(serviceURL *url.URL) error {
	if serviceURL.User != nil {
		c.User = serviceURL.User.Username()
		if password, ok := serviceURL.User.Password(); ok {
			c.Password = password
		}
	}

	return nil
}

// parseHostPort extracts host and port from the URL.
//
// Parameters:
//   - serviceURL: the URL to extract host and port from
//
// Returns:
//   - error: if parsing fails, nil otherwise
func (c *Config) parseHostPort(serviceURL *url.URL) error {
	host, portStr, err := net.SplitHostPort(serviceURL.Host)
	if err != nil {
		// If no port specified, use default
		host = serviceURL.Host
		portStr = "8080"
	}

	c.Host = host

	if portStr != "" {
		if port, err := strconv.Atoi(portStr); err == nil {
			c.Port = port
		}
	}

	return nil
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

	// First part is source phone number
	source := pathParts[0]
	if !isValidPhoneNumber(source) {
		return fmt.Errorf("%w: %s", ErrInvalidPhoneNumber, source)
	}

	c.Source = source

	// Parse recipients from remaining path parts
	recipients, err := parseRecipients(pathParts[1:])
	if err != nil {
		return err
	}

	c.Recipients = recipients

	return nil
}

// parseRecipients parses recipient phone numbers and group IDs from URL path segments.
// It handles group IDs that may contain "/" characters by accumulating consecutive segments.
//
// Parameters:
//   - pathParts: the URL path segments to parse
//
// Returns:
//   - []string: the parsed recipients
//   - error: if parsing fails, nil otherwise
func parseRecipients(pathParts []string) ([]string, error) {
	if len(pathParts) == 0 {
		return nil, ErrNoRecipients
	}

	var (
		recipients     []string
		currentGroupID strings.Builder
	)

	inGroupID := false

	for _, part := range pathParts {
		switch {
		case strings.HasPrefix(part, "group."):
			// Finalize any previous group ID
			if inGroupID {
				recipients = append(recipients, currentGroupID.String())
			}
			// Start new group ID
			currentGroupID.Reset()
			currentGroupID.WriteString(part)

			inGroupID = true

		case inGroupID && !strings.HasPrefix(part, "+"):
			// Continue building group ID (not a phone number)
			currentGroupID.WriteString("/" + part)

		default:
			// Finalize any group ID in progress
			if inGroupID {
				recipients = append(recipients, currentGroupID.String())
				inGroupID = false
			}
			// Add phone number or other recipient
			recipients = append(recipients, part)
		}
	}

	// Finalize any remaining group ID
	if inGroupID {
		recipients = append(recipients, currentGroupID.String())
	}

	// Validate all recipients
	for _, recipient := range recipients {
		if !isValidPhoneNumber(recipient) && !isValidGroupID(recipient) {
			return nil, fmt.Errorf("%w: %s", ErrInvalidRecipient, recipient)
		}
	}

	return recipients, nil
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
	// Handle dummy URL used for documentation generation
	if serviceURL.String() == "signal://dummy@dummy.com" {
		c.Host = "localhost"
		c.Port = 8080
		c.Source = "+1234567890"
		c.Recipients = []string{"+0987654321"}
		c.DisableTLS = false

		return nil
	}

	if err := c.parseAuth(serviceURL); err != nil {
		return err
	}

	if err := c.parseHostPort(serviceURL); err != nil {
		return err
	}

	if err := c.parsePath(serviceURL); err != nil {
		return err
	}

	if err := c.parseQuery(resolver, serviceURL); err != nil {
		return err
	}

	return nil
}

// isValidPhoneNumber checks if the string is a valid phone number.
//
// Parameters:
//   - phone: the phone number string to validate
//
// Returns:
//   - bool: true if valid, false otherwise
func isValidPhoneNumber(phone string) bool {
	return phoneRegex.MatchString(phone)
}

// isValidGroupID checks if the string is a valid group ID.
//
// Parameters:
//   - groupID: the group ID string to validate
//
// Returns:
//   - bool: true if valid, false otherwise
func isValidGroupID(groupID string) bool {
	return groupRegex.MatchString(groupID)
}
