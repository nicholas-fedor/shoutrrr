package wecom

import (
	"errors"
	"net/url"
	"strings"

	"github.com/nicholas-fedor/shoutrrr/pkg/format"
	"github.com/nicholas-fedor/shoutrrr/pkg/types"
	"github.com/nicholas-fedor/shoutrrr/pkg/util"
)

// Config represents the configuration for the WeCom service.
type Config struct {
	// Key is the bot's webhook key, written as the host of the service URL.
	Key string `desc:"Bot webhook key" key:"key" sensitive:"true"`
	// MentionedList is the comma-separated user IDs to mention.
	MentionedList string `desc:"Users to mention (comma-separated)" key:"mentioned_list"`
	// MentionedMobileList is the comma-separated mobile numbers of the users
	// to mention.
	MentionedMobileList string `desc:"Mobile numbers to mention (comma-separated)" key:"mentioned_mobile_list"`
}

// Scheme is the identifier for the WeCom service protocol.
const Scheme = "wecom"

// Error variables for the WeCom service.
var (
	// ErrEmptyKey indicates a service URL without a webhook key.
	ErrEmptyKey = errors.New("WeCom webhook key cannot be empty")
	// ErrInvalidKey indicates a webhook key with characters other than
	// letters, digits, hyphens, and underscores.
	ErrInvalidKey = errors.New("invalid WeCom webhook key format")
)

// Enums returns a map of enum formatters (none for this service).
//
// Returns:
//   - map[string]types.EnumFormatter: an empty map.
func (c *Config) Enums() map[string]types.EnumFormatter {
	return map[string]types.EnumFormatter{}
}

// GetURL constructs a URL from the Config fields.
//
// Returns:
//   - *url.URL: the service URL.
func (c *Config) GetURL() *url.URL {
	resolver := format.NewPropKeyResolver(c)

	return c.getURL(&resolver)
}

// SetURL updates the Config from a URL.
//
// Parameters:
//   - serviceURL: the service URL.
//
// Returns:
//   - error: the error from reading the URL, such as [ErrEmptyKey] or
//     [ErrInvalidKey].
func (c *Config) SetURL(serviceURL *url.URL) error {
	resolver := format.NewPropKeyResolver(c)

	return c.setURL(&resolver, serviceURL)
}

// getURL constructs a URL with the key as the host and the mention lists as
// query parameters.
//
// Parameters:
//   - types.ConfigQueryResolver: unused.
//
// Returns:
//   - *url.URL: the service URL.
func (c *Config) getURL(_ types.ConfigQueryResolver) *url.URL {
	query := url.Values{}

	if c.MentionedList != "" {
		query.Set("mentioned_list", c.MentionedList)
	}

	if c.MentionedMobileList != "" {
		query.Set("mentioned_mobile_list", c.MentionedMobileList)
	}

	return &url.URL{
		Scheme:     Scheme,
		Host:       c.Key,
		ForceQuery: false,
		RawQuery:   query.Encode(),
	}
}

// setURL updates the Config from a URL using the provided resolver. The key is
// the host, unless the key query parameter replaces it, and it is validated
// after the query is applied.
//
// Parameters:
//   - resolver: the resolver that sets the query parameters.
//   - serviceURL: the service URL.
//
// Returns:
//   - error: an error wrapping [util.ErrInvalidQuery] for a malformed query, a
//     query parameter error, [ErrEmptyKey], or [ErrInvalidKey].
func (c *Config) setURL(resolver types.ConfigQueryResolver, serviceURL *url.URL) error {
	// Extract key from host
	c.Key = serviceURL.Host

	// Handle query parameters
	query, err := util.ParseURLQuery(serviceURL)
	if err != nil {
		return err
	}

	if err := format.ApplyQuery(resolver, query); err != nil {
		return err
	}

	if c.Key == "" {
		return ErrEmptyKey
	}

	if !isValidKey(c.Key) {
		return ErrInvalidKey
	}

	return nil
}

// isValidKey reports whether key has only the letters, digits, hyphens, and
// underscores a webhook key consists of, so it is safe as a URL host and as
// the API URL's key query value.
//
// Parameters:
//   - key: the webhook key.
//
// Returns:
//   - bool: true when key is not empty and has only those characters.
func isValidKey(key string) bool {
	return key != "" && !strings.ContainsFunc(key, func(r rune) bool {
		return (r < 'a' || r > 'z') && (r < 'A' || r > 'Z') && (r < '0' || r > '9') && r != '-' && r != '_'
	})
}
