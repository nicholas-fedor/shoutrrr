package pagerduty

import (
	"fmt"
	"net/url"
	"strconv"
	"strings"

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

// Service identity and URL defaults.
const (
	// Scheme is the identifying part of this service's configuration URL.
	Scheme = "pagerduty"
	// defaultHost is the Events API v2 host used when the URL has no host.
	defaultHost = "events.pagerduty.com"
	// defaultPort is the HTTPS port used when the URL has no port.
	defaultPort = 443
	// integrationKeyLength is the length of an integration key, in hexadecimal
	// characters.
	integrationKeyLength = 32
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

// prepare sets the PagerDuty events host for a config without a host, and
// port 443 for one without a port, then runs the config's checks. Reading a
// service URL and each send run it, so a config built in code reaches the
// endpoint its URL would and fails as its URL would.
//
// Returns:
//   - error: the error from the config's checks, such as an invalid severity.
func (c *Config) prepare() error {
	if c.Host == "" {
		c.Host = defaultHost
	}

	if c.Port == 0 {
		c.Port = defaultPort
	}

	return c.validate()
}

// setURL updates the Config from a URL using the provided resolver.
//
// A URL without a host uses the PagerDuty events host, and a URL without a
// port uses port 443. The query props start from their defaults, so a prop the
// URL leaves out does not keep the value an earlier URL set.
//
// Parameters:
//   - resolver: Resolver used to apply the query props.
//   - serviceURL: The service URL to parse.
//
// Returns:
//   - error: a host that is not a hostname or a bracketed IPv6 address
//     ([util.ErrInvalidHost]), an invalid port, a malformed query
//     ([util.ErrInvalidQuery]), or a query prop error. Otherwise the error from
//     the config's checks, such as a missing integration key.
func (c *Config) setURL(resolver *format.PropKeyResolver, serviceURL *url.URL) error {
	c.IntegrationKey = strings.TrimPrefix(serviceURL.Path, "/")

	host, port, err := util.SplitURLHost(serviceURL)
	if err != nil {
		return fmt.Errorf("parsing host: %w", err)
	}

	c.Host = host
	c.Port = 0

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

	if err := resolver.SetDefaultProps(c); err != nil {
		return fmt.Errorf("setting default props: %w", err)
	}

	if err := format.ApplyQuery(resolver, query); err != nil {
		return err
	}

	return c.prepare()
}

// validate checks the integration key, the severity, and the event action of
// every event. Reading a service URL and each send run it, so a config built in
// code or changed by a param fails before a request.
//
// Returns:
//   - error: an error for a missing integration key or one that is not 32
//     hexadecimal characters. Otherwise an error for a severity or event
//     action that PagerDuty does not accept, or nil.
func (c *Config) validate() error {
	if c.IntegrationKey == "" {
		return errMissingIntegrationKey
	}

	if len(c.IntegrationKey) != integrationKeyLength || !util.IsHex(c.IntegrationKey) {
		return errInvalidIntegrationKey
	}

	if err := validateSeverity(c.Severity); err != nil {
		return err
	}

	return validateEventAction(c.Action)
}
