package opsgenie

import (
	"errors"
	"fmt"
	"net/url"
	"strconv"
	"strings"

	"github.com/nicholas-fedor/shoutrrr/pkg/format"
	"github.com/nicholas-fedor/shoutrrr/pkg/types"
	"github.com/nicholas-fedor/shoutrrr/pkg/util"
)

// Config holds the configuration for the OpsGenie service.
type Config struct {
	// Details holds custom properties of the alert, set as key:value pairs.
	Details map[string]string `desc:"Map of key-value pairs to use as custom properties of the alert" key:"details" optional:"true"`
	// APIKey is the Opsgenie API key, written as the path of the service URL.
	APIKey string `desc:"The OpsGenie API key" sensitive:"true" url:"path"`
	// Host is the Opsgenie API host, api.eu.opsgenie.com for EU instances.
	Host string `default:"api.opsgenie.com" desc:"The OpsGenie API host. Use 'api.eu.opsgenie.com' for EU instances" url:"host"`
	// Alias is the client-defined identifier of the alert, which Opsgenie uses
	// to deduplicate alerts.
	Alias string `desc:"Client-defined identifier of the alert" key:"alias" optional:"true"`
	// Description is added to the alert's description, after the message when
	// the message goes there.
	Description string `desc:"Description field of the alert" key:"description" optional:"true"`
	// Entity names the domain of the alert's source, such as a server or an
	// application.
	Entity string `desc:"Entity field of the alert that is generally used to specify which domain the Source field of the alert" key:"entity" optional:"true"`
	// Source is the source of the alert.
	Source string `desc:"Source field of the alert" key:"source" optional:"true"`
	// Priority is the alert priority, P1 to P5. When neither the URL nor the
	// send sets it, the level param chooses it.
	Priority string `desc:"Priority level of the alert. Possible values are P1, P2, P3, P4 and P5" key:"priority" optional:"true"`
	// Note is added to the alert when it is created.
	Note string `desc:"Additional note that will be added while creating the alert" key:"note" optional:"true"`
	// User is the display name of the request owner.
	User string `desc:"Display name of the request owner" key:"user" optional:"true"`
	// Title becomes the alert message, and the notification message then goes
	// to the description.
	Title string `default:"" desc:"notification title, optionally set by the sender" key:"title"`
	// Responders lists the teams, users, escalations, and schedules the alert
	// is routed to, as type:identifier entries.
	Responders []Entity `desc:"Teams, users, escalations and schedules that the alert will be routed to send notifications" key:"responders" optional:"true"`
	// VisibleTo lists the teams and users that can see the alert without being
	// notified, as type:identifier entries.
	VisibleTo []Entity `desc:"Teams and users that the alert will become visible to without sending any notification" key:"visibleTo" optional:"true"`
	// Actions lists the custom actions available on the alert.
	Actions []string `desc:"Custom actions that will be available for the alert" key:"actions" optional:"true"`
	// Tags lists the alert's tags.
	Tags []string `desc:"Tags of the alert" key:"tags" optional:"true"`
	// Port is the Opsgenie API port.
	Port uint16 `default:"443" desc:"The OpsGenie API port." url:"port"`
}

// Service identity and URL defaults.
const (
	// defaultPort is the default port for OpsGenie API connections.
	defaultPort = 443
	// Scheme is the identifying part of this service's configuration URL.
	Scheme = "opsgenie"
	// DefaultHost is the OpsGenie API host a URL without a host uses.
	DefaultHost = "api.opsgenie.com"
)

// Errors for an invalid config URL.
var (
	// ErrAPIKeyMissing indicates that the API key is missing from the config URL path.
	ErrAPIKeyMissing = errors.New("API key missing from config URL path")
	// ErrInvalidPort indicates a port that is not a number from 1 to 65535.
	ErrInvalidPort = errors.New("invalid port")
)

// Enums returns an empty map because the OpsGenie service doesn't use Enums.
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
//   - error: the error from reading the URL, such as [ErrAPIKeyMissing] or
//     [ErrInvalidPort].
func (c *Config) SetURL(serviceURL *url.URL) error {
	resolver := format.NewPropKeyResolver(c)

	return c.setURL(&resolver, serviceURL)
}

// applyDefaults sets [DefaultHost] and port 443 when the config has no host or
// port, as reading a URL without them does. A config built in code then
// reaches the same API.
func (c *Config) applyDefaults() {
	if c.Host == "" {
		c.Host = DefaultHost
	}

	if c.Port == 0 {
		c.Port = defaultPort
	}
}

// getURL constructs a URL from the Config's fields using the provided resolver.
//
// Parameters:
//   - resolver: the resolver that builds the query from the config props.
//
// Returns:
//   - *url.URL: the service URL, with the API key as its path.
func (c *Config) getURL(resolver types.ConfigQueryResolver) *url.URL {
	port := ""
	if c.Port > 0 {
		port = strconv.FormatUint(uint64(c.Port), 10)
	}

	result := &url.URL{
		Host:     util.JoinURLHost(c.Host, port),
		Path:     "/" + c.APIKey,
		Scheme:   Scheme,
		RawQuery: format.BuildQuery(resolver),
	}

	return result
}

// setURL updates the Config from a URL using the provided resolver.
//
// A URL without a host uses [DefaultHost], and one without a port uses 443.
//
// Parameters:
//   - resolver: the resolver that sets the alert props from the query.
//   - serviceURL: the service URL.
//
// Returns:
//   - error: [util.ErrInvalidHost] for a host that is not a hostname or a
//     bracketed IPv6 address, [util.ErrInvalidPort] for a port that is not a
//     number, [ErrInvalidPort] for a port outside 1 to 65535, an error
//     wrapping [util.ErrInvalidQuery] for a malformed query, a query prop
//     error, or [ErrAPIKeyMissing] for a missing or empty API key.
func (c *Config) setURL(resolver types.ConfigQueryResolver, serviceURL *url.URL) error {
	host, port, err := util.SplitURLHost(serviceURL)
	if err != nil {
		return fmt.Errorf("parsing host: %w", err)
	}

	c.Host = host
	c.APIKey = strings.TrimPrefix(serviceURL.Path, "/")
	c.Port = 0

	if port != "" {
		portNumber, err := strconv.ParseUint(port, 10, 16)
		if err != nil {
			return fmt.Errorf("%w %q: %w", ErrInvalidPort, port, err)
		}

		if portNumber == 0 {
			return fmt.Errorf("%w %q", ErrInvalidPort, port)
		}

		c.Port = uint16(portNumber)
	}

	c.applyDefaults()

	query, err := util.ParseURLQuery(serviceURL)
	if err != nil {
		return err
	}

	if err := format.ApplyQuery(resolver, query); err != nil {
		return err
	}

	return c.validate()
}

// validate checks that the config has the API key every alert request needs.
// Reading a service URL and each send run it, so a config built in code fails
// before a request.
//
// Returns:
//   - error: [ErrAPIKeyMissing] for a config without an API key, or nil.
func (c *Config) validate() error {
	if c.APIKey == "" {
		return ErrAPIKeyMissing
	}

	return nil
}
