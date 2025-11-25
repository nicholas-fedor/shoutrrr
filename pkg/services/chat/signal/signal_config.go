package signal

import (
	"errors"
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

// ErrInvalidPhoneNumber indicates an invalid phone number format.
var (
	ErrInvalidPhoneNumber = errors.New("invalid phone number format")
	ErrInvalidGroupID     = errors.New("invalid group ID format")
	ErrNoRecipients       = errors.New("no recipients specified")
	ErrInvalidRecipient   = errors.New("invalid recipient: must be phone number or group ID")
)

// Config holds settings for the Signal notification service.
type Config struct {
	standard.EnumlessConfig
	Host       string   `default:"localhost" desc:"Signal REST API server hostname or IP"      key:"host"`
	Port       int      `default:"8080"      desc:"Signal REST API server port"                key:"port"`
	User       string   `                    desc:"Username for HTTP Basic Auth"               key:"user"`
	Password   string   `                    desc:"Password for HTTP Basic Auth"               key:"password"`
	Token      string   `                    desc:"API token for Bearer authentication"        key:"token,apikey"`
	Source     string   `                    desc:"Source phone number (with country code)"    key:"source"`
	Recipients []string `                    desc:"Recipient phone numbers or group IDs"       key:"recipients,to"`
	DisableTLS bool     `default:"No"        desc:"Disable TLS for Signal REST API connection" key:"disabletls"`
}

// GetURL generates a URL from the current configuration values.
func (config *Config) GetURL() *url.URL {
	resolver := format.NewPropKeyResolver(config)

	return config.getURL(&resolver)
}

// SetURL updates the configuration from a URL representation.
func (config *Config) SetURL(url *url.URL) error {
	resolver := format.NewPropKeyResolver(config)

	return config.setURL(&resolver, url)
}

// getURL constructs a URL from the Config's fields using the provided resolver.
func (config *Config) getURL(resolver types.ConfigQueryResolver) *url.URL {
	recipients := strings.Join(config.Recipients, "/")

	result := &url.URL{
		Scheme:   Scheme,
		Host:     fmt.Sprintf("%s:%d", config.Host, config.Port),
		Path:     fmt.Sprintf("/%s/%s", config.Source, recipients),
		RawQuery: format.BuildQuery(resolver),
	}

	// Add user:password if authentication is configured
	if config.User != "" {
		if config.Password != "" {
			result.User = url.UserPassword(config.User, config.Password)
		} else {
			result.User = url.User(config.User)
		}
	}

	return result
}

// setURL updates the Config from a URL using the provided resolver.
func (config *Config) setURL(resolver types.ConfigQueryResolver, serviceURL *url.URL) error {
	// Handle dummy URL used for documentation generation
	if serviceURL.String() == "signal://dummy@dummy.com" {
		config.Host = "localhost"
		config.Port = 8080
		config.Source = "+1234567890"
		config.Recipients = []string{"+0987654321"}
		config.DisableTLS = false

		return nil
	}

	if err := config.parseAuth(serviceURL); err != nil {
		return err
	}

	if err := config.parseHostPort(serviceURL); err != nil {
		return err
	}

	if err := config.parsePath(serviceURL); err != nil {
		return err
	}

	if err := config.parseQuery(resolver, serviceURL); err != nil {
		return err
	}

	return nil
}

// parseAuth extracts user and password from the URL.
func (config *Config) parseAuth(serviceURL *url.URL) error {
	if serviceURL.User != nil {
		config.User = serviceURL.User.Username()
		if password, ok := serviceURL.User.Password(); ok {
			config.Password = password
		}
	}

	return nil
}

// parseHostPort extracts host and port from the URL.
func (config *Config) parseHostPort(serviceURL *url.URL) error {
	host, portStr, err := net.SplitHostPort(serviceURL.Host)
	if err != nil {
		// If no port specified, use default
		host = serviceURL.Host
		portStr = "8080"
	}

	config.Host = host

	if portStr != "" {
		if port, err := strconv.Atoi(portStr); err == nil {
			config.Port = port
		}
	}

	return nil
}

// parsePath extracts source phone number and recipients from the URL path.
func (config *Config) parsePath(serviceURL *url.URL) error {
	pathParts := strings.Split(strings.Trim(serviceURL.Path, "/"), "/")
	if len(pathParts) < minPathParts {
		return ErrNoRecipients
	}

	// First part is source phone number
	source := pathParts[0]
	if !isValidPhoneNumber(source) {
		return fmt.Errorf("%w: %s", ErrInvalidPhoneNumber, source)
	}

	config.Source = source

	// Parse recipients from remaining path parts
	recipients, err := parseRecipients(pathParts[1:])
	if err != nil {
		return err
	}

	config.Recipients = recipients

	return nil
}

// parseRecipients parses recipient phone numbers and group IDs from URL path segments.
// It handles group IDs that may contain "/" characters by accumulating consecutive segments.
func parseRecipients(pathParts []string) ([]string, error) {
	var recipients []string

	for i := 0; i < len(pathParts); i++ {
		part := pathParts[i]
		if strings.HasPrefix(part, "group.") {
			// Accumulate group ID parts until next phone number or group ID
			groupID := part

			i++

			var groupIDBuilder strings.Builder

			// Iterate through the remaining parts to build the group ID
			for i < len(pathParts) && !strings.HasPrefix(pathParts[i], "+") && !strings.HasPrefix(pathParts[i], "group.") {
				groupIDBuilder.WriteString("/" + pathParts[i])
				i++
			}

			groupID += groupIDBuilder.String()

			i-- // compensate for the outer loop increment

			recipients = append(recipients, groupID)
		} else {
			recipients = append(recipients, part)
		}
	}

	// Check for empty recipients
	if len(recipients) == 0 {
		return nil, ErrNoRecipients
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
func (config *Config) parseQuery(resolver types.ConfigQueryResolver, serviceURL *url.URL) error {
	for key, vals := range serviceURL.Query() {
		if err := resolver.Set(key, vals[0]); err != nil {
			return fmt.Errorf("setting config property %q from URL query: %w", key, err)
		}
	}

	return nil
}

// isValidPhoneNumber checks if the string is a valid phone number.
func isValidPhoneNumber(phone string) bool {
	return phoneRegex.MatchString(phone)
}

// isValidGroupID checks if the string is a valid group ID.
func isValidGroupID(groupID string) bool {
	return groupRegex.MatchString(groupID)
}
