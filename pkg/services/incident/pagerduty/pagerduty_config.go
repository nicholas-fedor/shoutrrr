package pagerduty

import (
	"fmt"
	"net/url"
	"regexp"
	"strconv"

	"github.com/nicholas-fedor/shoutrrr/pkg/format"
	"github.com/nicholas-fedor/shoutrrr/pkg/types"
)

// Config holds the configuration for the PagerDuty service.
type Config struct {
	IntegrationKey string `desc:"The PagerDuty API integration key"                                                            url:"path"`
	Host           string `desc:"The PagerDuty API host."                                                                      url:"host" default:"events.pagerduty.com"`
	Port           uint16 `desc:"The PagerDuty API port."                                                                      url:"port" default:"443"`
	Severity       string `desc:"The perceived severity of the status the event (critical, error, warning, or info); required"            default:"error"                key:"severity"`
	Source         string `desc:"The unique location of the affected system, preferably a hostname or FQDN; required"                     default:"default"              key:"source"`
	Action         string `desc:"The type of event (trigger, acknowledge, or resolve)"                                                    default:"trigger"              key:"action"`
	DedupKey       string `desc:"A unique key used for incident deduplication"                                                                                           key:"dedup_key"`
	Details        string `desc:"Additional details about the incident (JSON string that will be parsed into an object)"                                                 key:"details"`
	Contexts       string `desc:"Additional context links or images"                                                                                                     key:"contexts"`
	Client         string `desc:"The name of the monitoring client that is triggering this event"                                                                        key:"client"`
	ClientURL      string `desc:"The URL of the monitoring client that is triggering this event"                                                                         key:"client_url"`
}

const (
	// Scheme is the identifying part of this service's configuration URL.
	Scheme = "pagerduty"
	// integrationKeyRegex validates that integration keys are 32-character hexadecimal strings.
	integrationKeyRegex = `^[a-fA-F0-9]{32}$`
)

// Enums returns an empty map because the PagerDuty service doesn't use Enums.
func (c *Config) Enums() map[string]types.EnumFormatter {
	return map[string]types.EnumFormatter{}
}

// GetURL returns a URL representation of the Config's current field values.
func (c *Config) GetURL() *url.URL {
	resolver := format.NewPropKeyResolver(c)

	return c.getURL(&resolver)
}

// SetURL updates the Config from a URL representation of its field values.
func (c *Config) SetURL(serviceURL *url.URL) error {
	resolver := format.NewPropKeyResolver(c)

	return c.setURL(&resolver, serviceURL)
}

// getURL constructs a URL from the Config's fields using the provided resolver.
func (c *Config) getURL(resolver types.ConfigQueryResolver) *url.URL {
	var host string
	if c.Port > 0 {
		host = fmt.Sprintf("%s:%d", c.Host, c.Port)
	} else {
		host = c.Host
	}

	return &url.URL{
		Host:     host,
		Path:     "/" + c.IntegrationKey,
		Scheme:   Scheme,
		RawQuery: format.BuildQuery(resolver),
	}
}

// setURL updates the Config from a URL using the provided resolver.
func (c *Config) setURL(resolver types.ConfigQueryResolver, serviceURL *url.URL) error {
	// Skip validation for dummy URLs used in docs generation
	if serviceURL.Host == "dummy.com" {
		if len(serviceURL.Path) > 1 {
			c.IntegrationKey = serviceURL.Path[1:]
		}

		return nil
	}

	if len(serviceURL.Path) <= 1 {
		return errMissingIntegrationKey
	}

	c.IntegrationKey = serviceURL.Path[1:]

	// Validate integration key format
	if matched, err := regexp.MatchString(integrationKeyRegex, c.IntegrationKey); err != nil {
		return fmt.Errorf("failed to validate integration key: %w", err)
	} else if !matched {
		return errInvalidIntegrationKey
	}

	if serviceURL.Hostname() != "" {
		c.Host = serviceURL.Hostname()
	}

	if serviceURL.Port() != "" {
		port, err := strconv.ParseUint(serviceURL.Port(), 10, 16)
		if err != nil {
			return fmt.Errorf("failed to parse port: %w", err)
		}

		c.Port = uint16(port)
	}

	for key, vals := range serviceURL.Query() {
		if err := resolver.Set(key, vals[0]); err != nil {
			return fmt.Errorf("failed to set query parameter %q: %w", key, err)
		}
	}

	return nil
}
