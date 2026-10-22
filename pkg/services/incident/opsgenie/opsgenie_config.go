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
	Details     map[string]string `desc:"Map of key-value pairs to use as custom properties of the alert"                                        key:"details"                                                            optional:"true"`
	APIKey      string            `desc:"The OpsGenie API key"                                                                                   url:"path"`
	Host        string            `default:"api.opsgenie.com"                                                                                    desc:"The OpsGenie API host. Use 'api.eu.opsgenie.com' for EU instances" url:"host"`
	Alias       string            `desc:"Client-defined identifier of the alert"                                                                 key:"alias"                                                              optional:"true"`
	Description string            `desc:"Description field of the alert"                                                                         key:"description"                                                        optional:"true"`
	Entity      string            `desc:"Entity field of the alert that is generally used to specify which domain the Source field of the alert" key:"entity"                                                             optional:"true"`
	Source      string            `desc:"Source field of the alert"                                                                              key:"source"                                                             optional:"true"`
	Priority    string            `desc:"Priority level of the alert. Possible values are P1, P2, P3, P4 and P5"                                 key:"priority"                                                           optional:"true"`
	Note        string            `desc:"Additional note that will be added while creating the alert"                                            key:"note"                                                               optional:"true"`
	User        string            `desc:"Display name of the request owner"                                                                      key:"user"                                                               optional:"true"`
	Title       string            `default:""                                                                                                    desc:"notification title, optionally set by the sender"                  key:"title"`
	Responders  []Entity          `desc:"Teams, users, escalations and schedules that the alert will be routed to send notifications"            key:"responders"                                                         optional:"true"`
	VisibleTo   []Entity          `desc:"Teams and users that the alert will become visible to without sending any notification"                 key:"visibleTo"                                                          optional:"true"`
	Actions     []string          `desc:"Custom actions that will be available for the alert"                                                    key:"actions"                                                            optional:"true"`
	Tags        []string          `desc:"Tags of the alert"                                                                                      key:"tags"                                                               optional:"true"`
	Port        uint16            `default:"443"                                                                                                 desc:"The OpsGenie API port."                                            url:"port"`
}

const (
	defaultPort = 443        // defaultPort is the default port for OpsGenie API connections.
	Scheme      = "opsgenie" // Scheme is the identifying part of this service's configuration URL.
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
// The documentation dummy URL is accepted without an API key.
//
// Parameters:
//   - resolver: the resolver that sets the alert props from the query.
//   - serviceURL: the service URL.
//
// Returns:
//   - error: [util.ErrInvalidHost] for a host that is not a hostname or a
//     bracketed IPv6 address, [util.ErrInvalidPort] for a port that is not a
//     number, [ErrAPIKeyMissing] for a missing or empty API key,
//     [ErrInvalidPort] for a port outside 1 to 65535, an error wrapping
//     [util.ErrInvalidQuery] for a malformed query, or a query prop error.
func (c *Config) setURL(resolver types.ConfigQueryResolver, serviceURL *url.URL) error {
	host, port, err := util.SplitURLHost(serviceURL)
	if err != nil {
		return fmt.Errorf("parsing host: %w", err)
	}

	c.Host = host

	if serviceURL.String() != "opsgenie://dummy@dummy.com" {
		c.APIKey = strings.TrimPrefix(serviceURL.Path, "/")
		if c.APIKey == "" {
			return ErrAPIKeyMissing
		}

		if c.Host == "" {
			c.Host = DefaultHost
		}
	}

	if port != "" {
		portNumber, err := strconv.ParseUint(port, 10, 16)
		if err != nil {
			return fmt.Errorf("%w %q: %w", ErrInvalidPort, port, err)
		}

		if portNumber == 0 {
			return fmt.Errorf("%w %q", ErrInvalidPort, port)
		}

		c.Port = uint16(portNumber)
	} else {
		c.Port = defaultPort
	}

	query, err := util.ParseURLQuery(serviceURL)
	if err != nil {
		return err
	}

	for key, vals := range query {
		if err := resolver.Set(key, vals[0]); err != nil {
			return fmt.Errorf(
				"setting query parameter %q to %q: %w",
				key,
				vals[0],
				err,
			)
		}
	}

	return nil
}
