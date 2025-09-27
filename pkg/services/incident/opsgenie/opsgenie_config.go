package opsgenie

import (
	"errors"
	"fmt"
	"net/url"
	"strconv"

	"github.com/nicholas-fedor/shoutrrr/pkg/format"
	"github.com/nicholas-fedor/shoutrrr/pkg/types"
)

const (
	defaultPort = 443        // defaultPort is the default port for OpsGenie API connections.
	Scheme      = "opsgenie" // Scheme is the identifying part of this service's configuration URL.
)

// ErrAPIKeyMissing indicates that the API key is missing from the config URL path.
var ErrAPIKeyMissing = errors.New("API key missing from config URL path")

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

// Enums returns an empty map because the OpsGenie service doesn't use Enums.
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

	result := &url.URL{
		Host:     host,
		Path:     "/" + config.APIKey,
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
	config.Host = url.Hostname()

	if url.String() != "opsgenie://dummy@dummy.com" {
		if len(url.Path) > 0 {
			config.APIKey = url.Path[1:]
		} else {
			return ErrAPIKeyMissing
		}
	}

	if url.Port() != "" {
		port, err := strconv.ParseUint(url.Port(), 10, 16)
		if err != nil {
			return fmt.Errorf("parsing port %q: %w", url.Port(), err)
		}

		config.Port = uint16(port)
	} else {
		config.Port = defaultPort
	}

	for key, vals := range url.Query() {
		if err := resolver.Set(key, vals[0]); err != nil {
			return fmt.Errorf("setting query parameter %q to %q: %w", key, vals[0], err)
		}
	}

	return nil
}
