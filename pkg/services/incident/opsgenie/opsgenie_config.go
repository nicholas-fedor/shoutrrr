package opsgenie

import (
	"errors"
	"fmt"
	"net/url"
	"strconv"

	"github.com/nicholas-fedor/shoutrrr/pkg/format"
	"github.com/nicholas-fedor/shoutrrr/pkg/types"
)

// Config holds the configuration for the OpsGenie service.
type Config struct {
	APIKey      string            `desc:"The OpsGenie API key"                                                                                   url:"path"`
	Host        string            `desc:"The OpsGenie API host. Use 'api.eu.opsgenie.com' for EU instances"                                      url:"host" default:"api.opsgenie.com"`
	Port        uint16            `desc:"The OpsGenie API port."                                                                                 url:"port" default:"443"`
	Alias       string            `desc:"Client-defined identifier of the alert"                                                                                                       key:"alias"       optional:"true"`
	Description string            `desc:"Description field of the alert"                                                                                                               key:"description" optional:"true"`
	Responders  []Entity          `desc:"Teams, users, escalations and schedules that the alert will be routed to send notifications"                                                  key:"responders"  optional:"true"`
	VisibleTo   []Entity          `desc:"Teams and users that the alert will become visible to without sending any notification"                                                       key:"visibleTo"   optional:"true"`
	Actions     []string          `desc:"Custom actions that will be available for the alert"                                                                                          key:"actions"     optional:"true"`
	Tags        []string          `desc:"Tags of the alert"                                                                                                                            key:"tags"        optional:"true"`
	Details     map[string]string `desc:"Map of key-value pairs to use as custom properties of the alert"                                                                              key:"details"     optional:"true"`
	Entity      string            `desc:"Entity field of the alert that is generally used to specify which domain the Source field of the alert"                                       key:"entity"      optional:"true"`
	Source      string            `desc:"Source field of the alert"                                                                                                                    key:"source"      optional:"true"`
	Priority    string            `desc:"Priority level of the alert. Possible values are P1, P2, P3, P4 and P5"                                                                       key:"priority"    optional:"true"`
	Note        string            `desc:"Additional note that will be added while creating the alert"                                                                                  key:"note"        optional:"true"`
	User        string            `desc:"Display name of the request owner"                                                                                                            key:"user"        optional:"true"`
	Title       string            `desc:"notification title, optionally set by the sender"                                                                  default:""                 key:"title"`
}

const (
	defaultPort = 443        // defaultPort is the default port for OpsGenie API connections.
	Scheme      = "opsgenie" // Scheme is the identifying part of this service's configuration URL.
)

// ErrAPIKeyMissing indicates that the API key is missing from the config URL path.
var ErrAPIKeyMissing = errors.New("API key missing from config URL path")

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
	var host string
	if c.Port > 0 {
		host = fmt.Sprintf("%s:%d", c.Host, c.Port)
	} else {
		host = c.Host
	}

	result := &url.URL{
		Host:     host,
		Path:     "/" + c.APIKey,
		Scheme:   Scheme,
		RawQuery: format.BuildQuery(resolver),
	}

	return result
}

// setURL updates the Config from a URL using the provided resolver.
func (c *Config) setURL(resolver types.ConfigQueryResolver, serviceURL *url.URL) error {
	c.Host = serviceURL.Hostname()

	if serviceURL.String() != "opsgenie://dummy@dummy.com" {
		if serviceURL.Path != "" {
			c.APIKey = serviceURL.Path[1:]
		} else {
			return ErrAPIKeyMissing
		}
	}

	if serviceURL.Port() != "" {
		port, err := strconv.ParseUint(serviceURL.Port(), 10, 16)
		if err != nil {
			return fmt.Errorf("parsing port %q: %w", serviceURL.Port(), err)
		}

		c.Port = uint16(port)
	} else {
		c.Port = defaultPort
	}

	for key, vals := range serviceURL.Query() {
		if err := resolver.Set(key, vals[0]); err != nil {
			return fmt.Errorf("setting query parameter %q to %q: %w", key, vals[0], err)
		}
	}

	return nil
}
