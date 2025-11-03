package pagerduty

import (
	"fmt"
	"net/url"
	"regexp"
	"strconv"

	"github.com/nicholas-fedor/shoutrrr/pkg/format"
	"github.com/nicholas-fedor/shoutrrr/pkg/types"
)

const (
	// Scheme is the identifying part of this service's configuration URL.
	Scheme = "pagerduty"
	// integrationKeyRegex validates that integration keys are 32-character hexadecimal strings.
	integrationKeyRegex = `^[a-fA-F0-9]{32}$`
)

// Config holds the configuration for the PagerDuty service.
type Config struct {
	IntegrationKey string `desc:"The PagerDuty API integration key"                                                            url:"path"`
	Host           string `desc:"The PagerDuty API host."                                                                      url:"host" default:"events.pagerduty.com"`
	Port           uint16 `desc:"The PagerDuty API port."                                                                      url:"port" default:"443"`
	Severity       string `desc:"The perceived severity of the status the event (critical, error, warning, or info); required"            default:"error"                key:"severity"`
	Source         string `desc:"The unique location of the affected system, preferably a hostname or FQDN; required"                     default:"default"              key:"source"`
	Action         string `desc:"The type of event (trigger, acknowledge, or resolve)"                                                    default:"trigger"              key:"action"`
	Details        string `desc:"Additional details about the incident"                                                                                                  key:"details"`
	Contexts       string `desc:"Additional context links or images"                                                                                                     key:"contexts"`
	Client         string `desc:"The name of the monitoring client that is triggering this event"                                                                        key:"client"`
	ClientURL      string `desc:"The URL of the monitoring client that is triggering this event"                                                                         key:"clienturl"`
}

// Enums returns an empty map because the PagerDuty service doesn't use Enums.
func (config *Config) Enums() map[string]types.EnumFormatter {
	return map[string]types.EnumFormatter{}
}

// GetURL returns a URL representation of the Config's current field values.
func (config *Config) GetURL() *url.URL {
	resolver := format.NewPropKeyResolver(config)

	return config.getURL(&resolver)
}

// getURL constructs a URL from the Config's fields using the provided resolver.
func (config *Config) getURL(resolver types.ConfigQueryResolver) *url.URL {
	var host string
	if config.Port > 0 {
		host = fmt.Sprintf("%s:%d", config.Host, config.Port)
	} else {
		host = config.Host
	}

	return &url.URL{
		Host:     host,
		Path:     "/" + config.IntegrationKey,
		Scheme:   Scheme,
		RawQuery: format.BuildQuery(resolver),
	}
}

// SetURL updates the Config from a URL representation of its field values.
func (config *Config) SetURL(url *url.URL) error {
	resolver := format.NewPropKeyResolver(config)

	return config.setURL(&resolver, url)
}

// setURL updates the Config from a URL using the provided resolver.
func (config *Config) setURL(resolver types.ConfigQueryResolver, url *url.URL) error {
	if len(url.Path) <= 1 {
		return errMissingIntegrationKey
	}

	config.IntegrationKey = url.Path[1:]

	// Validate integration key format
	if matched, err := regexp.MatchString(integrationKeyRegex, config.IntegrationKey); err != nil {
		return fmt.Errorf("failed to validate integration key: %w", err)
	} else if !matched {
		return errInvalidIntegrationKey
	}

	if url.Hostname() != "" {
		config.Host = url.Hostname()
	}

	if url.Port() != "" {
		port, err := strconv.ParseUint(url.Port(), 10, 16)
		if err != nil {
			return fmt.Errorf("failed to parse port: %w", err)
		}

		config.Port = uint16(port)
	}

	for key, vals := range url.Query() {
		if err := resolver.Set(key, vals[0]); err != nil {
			return fmt.Errorf("failed to set query parameter %q: %w", key, err)
		}
	}

	return nil
}
