package wecom

import (
	"errors"
	"fmt"
	"net/url"
	"strings"

	"github.com/nicholas-fedor/shoutrrr/pkg/format"
	"github.com/nicholas-fedor/shoutrrr/pkg/types"
)

// Scheme is the identifier for the WeCom service protocol.
const Scheme = "wecom"

// Error variables for the WeCom service.
var (
	ErrEmptyKey   = errors.New("WeCom webhook key cannot be empty")
	ErrInvalidKey = errors.New("invalid WeCom webhook key format")
)

// Config represents the configuration for the WeCom service.
type Config struct {
	Key                 string `desc:"Bot webhook key"                             key:"key"`
	MentionedList       string `desc:"Users to mention (comma-separated)"          key:"mentioned_list"`
	MentionedMobileList string `desc:"Mobile numbers to mention (comma-separated)" key:"mentioned_mobile_list"`
}

// Enums returns a map of enum formatters (none for this service).
func (config *Config) Enums() map[string]types.EnumFormatter {
	return map[string]types.EnumFormatter{}
}

// GetURL constructs a URL from the Config fields.
func (config *Config) GetURL() *url.URL {
	resolver := format.NewPropKeyResolver(config)

	return config.getURL(&resolver)
}

// getURL constructs a URL using the provided resolver.
func (config *Config) getURL(_ types.ConfigQueryResolver) *url.URL {
	return &url.URL{
		Scheme:     Scheme,
		Host:       config.Key,
		ForceQuery: false,
	}
}

// SetURL updates the Config from a URL.
func (config *Config) SetURL(url *url.URL) error {
	resolver := format.NewPropKeyResolver(config)

	return config.setURL(&resolver, url)
}

// setURL updates the Config from a URL using the provided resolver.
func (config *Config) setURL(resolver types.ConfigQueryResolver, url *url.URL) error {
	// Handle dummy URL used for documentation generation
	if url.String() == "wecom://dummy@dummy.com" {
		config.Key = "dummy-webhook-key"

		return nil
	}

	// Extract key from host
	config.Key = url.Host

	// Validate key format (alphanumeric, hyphens, underscores only)
	if config.Key == "" {
		return ErrEmptyKey
	}

	if strings.ContainsAny(config.Key, "@!#$%^&*()+=[]{}|\\:;\"'<>?,./") {
		return fmt.Errorf("%w: %s", ErrInvalidKey, config.Key)
	}

	// Handle query parameters
	for key, vals := range url.Query() {
		if err := resolver.Set(key, vals[0]); err != nil {
			return fmt.Errorf("setting query parameter %q: %w", key, err)
		}
	}

	return nil
}
