package pagerduty

import (
	"fmt"
	"net/url"
	"strconv"

	"github.com/nicholas-fedor/shoutrrr/pkg/format"
	"github.com/nicholas-fedor/shoutrrr/pkg/types"
)

const (
	// Scheme is the identifying part of this service's configuration URL
	Scheme = "pagerduty"
)

// Config for use within the pagerduty service
type Config struct {
	IntegrationKey string `url:"path" desc:"The PagerDuty API integration key"`
	Host           string `url:"host" desc:"The PagerDuty API host." default:"events.pagerduty.com"`
	Port           uint16 `url:"port" desc:"The PagerDuty API port." default:"443"`
	Severity       string `key:"severity" desc:"The perceived severity of the status the event (critical, error, warning, or info); required" default:"error"`
	Source         string `key:"source" desc:"The unique location of the affected system, preferably a hostname or FQDN; required" default:"default"`
	Action         string `key:"action" desc:"The type of event (trigger, acknowledge, or resolve)" default:"trigger"`
}

// Enums returns an empty map because the PagerDuty service doesn't use Enums
func (config Config) Enums() map[string]types.EnumFormatter {
	return map[string]types.EnumFormatter{}
}

// GetURL returns a URL representation of the Config's current field values.
func (config *Config) GetURL() *url.URL {
	resolver := format.NewPropKeyResolver(config)
	return config.getURL(&resolver)
}

// getURL constructs a URL from the Config's fields using the provided resolver.
func (config *Config) getURL(resolver types.ConfigQueryResolver) *url.URL {
	host := ""
	if config.Port > 0 {
		host = fmt.Sprintf("%s:%d", config.Host, config.Port)
	} else {
		host = config.Host
	}

	result := &url.URL{
		Host:     host,
		Path:     fmt.Sprintf("/%s", config.IntegrationKey),
		Scheme:   Scheme,
		RawQuery: format.BuildQuery(resolver),
	}

	return result
}

// SetURL updates the Config from a URL representation of its field values.
func (config *Config) SetURL(url *url.URL) error {
	resolver := format.NewPropKeyResolver(config)
	return config.setURL(&resolver, url)
}

// setURL updates the Config from a URL using the provided resolver.
func (config *Config) setURL(resolver types.ConfigQueryResolver, url *url.URL) error {
	config.IntegrationKey = url.Path[1:]

	if url.Hostname() != "" {
		config.Host = url.Hostname()
	}

	if url.Port() != "" {
		port, err := strconv.ParseUint(url.Port(), 10, 16)
		if err != nil {
			return err
		}
		config.Port = uint16(port)
	}

	for key, vals := range url.Query() {
		if err := resolver.Set(key, vals[0]); err != nil {
			return err
		}
	}

	return nil
}
