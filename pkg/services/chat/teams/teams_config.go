package teams

import (
	"fmt"
	"net/url"
	"regexp"
	"strings"

	"github.com/nicholas-fedor/shoutrrr/pkg/format"
	"github.com/nicholas-fedor/shoutrrr/pkg/services/standard"
	"github.com/nicholas-fedor/shoutrrr/pkg/types"
)

// Config represents the configuration for the Teams service.
type Config struct {
	standard.EnumlessConfig

	Group      string `optional:"" url:"user"`
	Tenant     string `optional:"" url:"host"`
	AltID      string `optional:"" url:"path1"`
	GroupOwner string `optional:"" url:"path2"`
	ExtraID    string `optional:"" url:"path3"`

	Title string `key:"title" optional:""`
	Color string `key:"color" optional:""`
	Host  string `key:"host"  optional:""` // Required, no default
}

// Scheme is the identifier for the Teams service protocol.
const Scheme = "teams"

// Config constants.
const (
	DummyURL           = "teams://dummy@dummy.com" // Default placeholder URL
	ExpectedOrgMatches = 2                         // Full match plus organization domain capture group
	MinPathComponents  = 3                         // Minimum required path components: AltID, GroupOwner, ExtraID
)

// GetURL constructs a URL from the Config fields.
func (c *Config) GetURL() *url.URL {
	resolver := format.NewPropKeyResolver(c)

	return c.getURL(&resolver)
}

// SetFromWebhookURL updates the Config from a Teams webhook URL.
func (c *Config) SetFromWebhookURL(webhookURL string) error {
	orgPattern := regexp.MustCompile(
		`https://([a-zA-Z0-9-\.]+)` + WebhookDomain + `/` + Path + `/([0-9a-f\-]{36})@([0-9a-f\-]{36})/` + ProviderName + `/([0-9a-f]{32})/([0-9a-f\-]{36})/([^/]+)`,
	)

	orgGroups := orgPattern.FindStringSubmatch(webhookURL)
	if len(orgGroups) != ExpectedComponents {
		return ErrInvalidWebhookFormat
	}

	c.Host = orgGroups[1] + ".webhook.office.com"

	parts, err := ParseAndVerifyWebhookURL(webhookURL)
	if err != nil {
		return err
	}

	c.setFromWebhookParts(&parts)

	return nil
}

// SetURL updates the Config from a URL.
func (c *Config) SetURL(serviceURL *url.URL) error {
	resolver := format.NewPropKeyResolver(c)

	return c.setURL(&resolver, serviceURL)
}

// WebhookParts returns the webhook components as an array.
func (c *Config) WebhookParts() [5]string {
	return [5]string{c.Group, c.Tenant, c.AltID, c.GroupOwner, c.ExtraID}
}

// getURL constructs a URL using the provided resolver.
func (c *Config) getURL(resolver types.ConfigQueryResolver) *url.URL {
	if c.Host == "" {
		return nil
	}

	return &url.URL{
		User:     url.User(c.Group),
		Host:     c.Tenant,
		Path:     "/" + c.AltID + "/" + c.GroupOwner + "/" + c.ExtraID,
		Scheme:   Scheme,
		RawQuery: format.BuildQuery(resolver),
	}
}

// setFromWebhookParts sets Config fields from webhook parts.
func (c *Config) setFromWebhookParts(parts *[5]string) {
	c.Group = parts[0]
	c.Tenant = parts[1]
	c.AltID = parts[2]
	c.GroupOwner = parts[3]
	c.ExtraID = parts[4]
}

// setQueryParams applies query parameters to the Config using the resolver.
// It resets Color, Host, and Title, then updates them based on query values.
// Returns an error if the resolver fails to set any parameter.
func (c *Config) setQueryParams(resolver types.ConfigQueryResolver, query url.Values) error {
	c.Color = ""
	c.Host = ""
	c.Title = ""

	for key, vals := range query {
		if len(vals) > 0 && vals[0] != "" {
			switch key {
			case "color":
				c.Color = vals[0]
			case "host":
				c.Host = vals[0]
			case "title":
				c.Title = vals[0]
			}

			if err := resolver.Set(key, vals[0]); err != nil {
				return fmt.Errorf(
					"%w: key=%q, value=%q: %w",
					ErrSetParameterFailed,
					key,
					vals[0],
					err,
				)
			}
		}
	}

	return nil
}

// setURL updates the Config from a URL using the provided resolver.
// It parses the URL parts, sets query parameters, and ensures the host is specified.
// Returns an error if the URL is invalid or the host is missing.
func (c *Config) setURL(resolver types.ConfigQueryResolver, serviceURL *url.URL) error {
	parts, err := parseURLParts(serviceURL)
	if err != nil {
		return err
	}

	c.setFromWebhookParts(&parts)

	if err := c.setQueryParams(resolver, serviceURL.Query()); err != nil {
		return err
	}

	// Allow dummy URL during documentation generation
	if c.Host == "" && (serviceURL.User != nil && serviceURL.User.Username() == "dummy") {
		c.Host = "dummy.webhook.office.com"
	} else if c.Host == "" {
		return ErrMissingHostParameter
	}

	return nil
}

// ConfigFromWebhookURL creates a new Config from a parsed Teams webhook URL.
func ConfigFromWebhookURL(webhookURL *url.URL) (*Config, error) {
	webhookURL.RawQuery = ""
	config := &Config{
		EnumlessConfig: standard.EnumlessConfig{},
		Host:           webhookURL.Host,
		Group:          "",
		Tenant:         "",
		AltID:          "",
		GroupOwner:     "",
		ExtraID:        "",
		Title:          "",
		Color:          "",
	}

	if err := config.SetFromWebhookURL(webhookURL.String()); err != nil {
		return nil, err
	}

	return config, nil
}

// parseURLParts extracts and validates webhook components from a URL.
func parseURLParts(serviceURL *url.URL) ([5]string, error) {
	var parts [5]string
	if serviceURL.String() == DummyURL {
		return parts, nil
	}

	pathParts := strings.Split(serviceURL.Path, "/")
	if pathParts[0] == "" {
		pathParts = pathParts[1:]
	}

	if len(pathParts) < MinPathComponents {
		return parts, ErrMissingExtraIDComponent
	}

	parts = [5]string{
		serviceURL.User.Username(),
		serviceURL.Hostname(),
		pathParts[0],
		pathParts[1],
		pathParts[2],
	}
	if err := verifyWebhookParts(&parts); err != nil {
		return parts, fmt.Errorf("invalid URL format: %w", err)
	}

	return parts, nil
}
