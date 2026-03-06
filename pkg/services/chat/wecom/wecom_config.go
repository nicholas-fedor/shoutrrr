package wecom

import (
	"errors"
	"fmt"
	"net/url"
	"strings"

	"github.com/nicholas-fedor/shoutrrr/pkg/format"
	"github.com/nicholas-fedor/shoutrrr/pkg/types"
)

// Config represents the configuration for the WeCom service.
type Config struct {
	Key                 string `desc:"Bot webhook key"                             key:"key"`
	MentionedList       string `desc:"Users to mention (comma-separated)"          key:"mentioned_list"`
	MentionedMobileList string `desc:"Mobile numbers to mention (comma-separated)" key:"mentioned_mobile_list"`
}

// Scheme is the identifier for the WeCom service protocol.
const Scheme = "wecom"

// Error variables for the WeCom service.
var (
	ErrEmptyKey   = errors.New("WeCom webhook key cannot be empty")
	ErrInvalidKey = errors.New("invalid WeCom webhook key format")
)

// Enums returns a map of enum formatters (none for this service).
func (c *Config) Enums() map[string]types.EnumFormatter {
	return map[string]types.EnumFormatter{}
}

// GetURL constructs a URL from the Config fields.
func (c *Config) GetURL() *url.URL {
	resolver := format.NewPropKeyResolver(c)

	return c.getURL(&resolver)
}

// SetURL updates the Config from a URL.
func (c *Config) SetURL(serviceURL *url.URL) error {
	resolver := format.NewPropKeyResolver(c)

	return c.setURL(&resolver, serviceURL)
}

// getURL constructs a URL using the provided resolver.
func (c *Config) getURL(_ types.ConfigQueryResolver) *url.URL {
	return &url.URL{
		Scheme:     Scheme,
		Host:       c.Key,
		ForceQuery: false,
	}
}

// setURL updates the Config from a URL using the provided resolver.
func (c *Config) setURL(resolver types.ConfigQueryResolver, serviceURL *url.URL) error {
	// Handle dummy URL used for documentation generation
	if serviceURL.String() == "wecom://dummy@dummy.com" {
		c.Key = "dummy-webhook-key"

		return nil
	}

	// Extract key from host
	c.Key = serviceURL.Host

	// Validate key format (alphanumeric, hyphens, underscores only)
	if c.Key == "" {
		return ErrEmptyKey
	}

	if strings.ContainsAny(c.Key, "@!#$%^&*()+=[]{}|\\:;\"'<>?,./") {
		return fmt.Errorf("%w: %s", ErrInvalidKey, c.Key)
	}

	// Handle query parameters
	for key, vals := range serviceURL.Query() {
		if err := resolver.Set(key, vals[0]); err != nil {
			return fmt.Errorf("setting query parameter %q: %w", key, err)
		}
	}

	return nil
}
