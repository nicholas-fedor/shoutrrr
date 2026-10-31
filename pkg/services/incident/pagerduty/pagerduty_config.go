package pagerduty

import (
	"fmt"
	"net/url"
	"regexp"
	"strconv"

	"github.com/nicholas-fedor/shoutrrr/pkg/format"
	"github.com/nicholas-fedor/shoutrrr/pkg/types"
	"github.com/nicholas-fedor/shoutrrr/pkg/util"
)

// Config holds the configuration for the PagerDuty service.
type Config struct {
	// IntegrationKey is the integration's routing key, written as the path of
	// the service URL.
	IntegrationKey string `desc:"The PagerDuty API integration key" sensitive:"true" url:"path"`
	// Host is the Events API host.
	Host string `default:"events.pagerduty.com" desc:"The PagerDuty API host." url:"host"`
	// Severity is the event severity: critical, error, warning, or info.
	// When neither the URL nor the send sets it, the level param chooses it.
	Severity string `default:"error" desc:"The perceived severity of the status the event (critical, error, warning, or info); required" key:"severity"`
	// Source is the location of the affected system, preferably a hostname or
	// FQDN.
	Source string `default:"default" desc:"The unique location of the affected system, preferably a hostname or FQDN; required" key:"source"`
	// Action is the event type: trigger, acknowledge, or resolve.
	Action string `default:"trigger" desc:"The type of event (trigger, acknowledge, or resolve)" key:"action"`
	// DedupKey identifies the alert, so later events with the same key update
	// or resolve it.
	DedupKey string `desc:"A unique key used for incident deduplication" key:"dedup_key"`
	// Details is a JSON object of custom details about the event.
	Details string `desc:"Additional details about the incident (JSON string that will be parsed into an object)" key:"details"`
	// Contexts lists links and images for the event, as a JSON array or as
	// comma-separated type:value entries.
	Contexts string `desc:"Additional context links or images" key:"contexts"`
	// Client is the name of the monitoring client that triggers the event.
	Client string `desc:"The name of the monitoring client that is triggering this event" key:"client"`
	// ClientURL is the URL of the monitoring client that triggers the event.
	ClientURL string `desc:"The URL of the monitoring client that is triggering this event" key:"client_url"`
	// Port is the Events API port.
	Port uint16 `default:"443" desc:"The PagerDuty API port." url:"port"`
}

// Service identity, URL defaults, and the integration key format.
const (
	// Scheme is the identifying part of this service's configuration URL.
	Scheme = "pagerduty"
	// integrationKeyRegex validates that integration keys are 32-character hexadecimal strings.
	integrationKeyRegex = `^[a-fA-F0-9]{32}$`
	// defaultHost is the Events API v2 host used when the URL has no host.
	defaultHost = "events.pagerduty.com"
	// defaultPort is the HTTPS port used when the URL has no port.
	defaultPort = 443
)

// Enums returns an empty map because the PagerDuty service doesn't use Enums.
//
// Returns:
//   - map[string]types.EnumFormatter: an empty map.
func (c *Config) Enums() map[string]types.EnumFormatter {
	return map[string]types.EnumFormatter{}
}

// GetURL returns a URL representation of the Config's current field values.
//
// Returns:
//   - *url.URL: the service URL.
func (c *Config) GetURL() *url.URL {
	resolver := format.NewPropKeyResolver(c)

	return c.getURL(&resolver)
}

// SetURL updates the Config from a URL representation of its field values.
//
// Parameters:
//   - serviceURL: the service URL.
//
// Returns:
//   - error: the error from reading the URL, such as a missing or invalid
//     integration key.
func (c *Config) SetURL(serviceURL *url.URL) error {
	resolver := format.NewPropKeyResolver(c)

	return c.setURL(&resolver, serviceURL)
}

// getURL constructs a URL from the Config's fields using the provided resolver.
//
// The port is left out when it is unset or the default, so a config read from
// a URL without a port writes that URL back.
//
// Parameters:
//   - resolver: Resolver used to encode the query props.
//
// Returns:
//   - *url.URL: The service URL.
func (c *Config) getURL(resolver types.ConfigQueryResolver) *url.URL {
	port := ""
	if c.Port > 0 && c.Port != defaultPort {
		port = strconv.FormatUint(uint64(c.Port), 10)
	}

	return &url.URL{
		Host:     util.JoinURLHost(c.Host, port),
		Path:     "/" + c.IntegrationKey,
		Scheme:   Scheme,
		RawQuery: format.BuildQuery(resolver),
	}
}

// setURL updates the Config from a URL using the provided resolver.
//
// A URL without a host uses the PagerDuty events host, and a URL without a
// port uses port 443.
//
// Parameters:
//   - resolver: Resolver used to apply the query props.
//   - serviceURL: The service URL to parse.
//
// Returns:
//   - error: A missing or invalid integration key, a host that is not a
//     hostname or a bracketed IPv6 address ([util.ErrInvalidHost]), an invalid
//     port, a malformed query ([util.ErrInvalidQuery]), or a query prop error.
func (c *Config) setURL(resolver types.ConfigQueryResolver, serviceURL *url.URL) error {
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

	host, port, err := util.SplitURLHost(serviceURL)
	if err != nil {
		return fmt.Errorf("parsing host: %w", err)
	}

	c.Host = host
	if c.Host == "" {
		c.Host = defaultHost
	}

	c.Port = defaultPort

	if port != "" {
		portNumber, err := strconv.ParseUint(port, 10, 16)
		if err != nil {
			return fmt.Errorf("failed to parse port: %w", err)
		}

		c.Port = uint16(portNumber)
	}

	query, err := util.ParseURLQuery(serviceURL)
	if err != nil {
		return err
	}

	return format.ApplyQuery(resolver, query)
}
