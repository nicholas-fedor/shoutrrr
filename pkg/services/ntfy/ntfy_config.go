package ntfy

import (
	"errors"
	"fmt" // Add this import
	"net/url"
	"strings"

	"github.com/nicholas-fedor/shoutrrr/pkg/format"
	"github.com/nicholas-fedor/shoutrrr/pkg/types"
)

// Scheme is the identifying part of this service's configuration URL.
const (
	Scheme = "ntfy"
)

// ErrTopicRequired indicates that the topic is missing from the config URL.
var ErrTopicRequired = errors.New("topic is required")

// Config holds the configuration for the Ntfy service.
type Config struct {
	Title    string   `default:""        desc:"Message title"                                                                                    key:"title"`
	Host     string   `default:"ntfy.sh" desc:"Server hostname and port"                                                                                           url:"host"`
	Topic    string   `                  desc:"Target topic name"                                                                                                  url:"path"     required:""`
	Password string   `                  desc:"Auth password"                                                                                                      url:"password"             optional:""`
	Username string   `                  desc:"Auth username"                                                                                                      url:"user"                 optional:""`
	Scheme   string   `default:"https"   desc:"Server protocol, http or https"                                                                   key:"scheme"`
	Tags     []string `                  desc:"List of tags that may or not map to emojis"                                                       key:"tags"                                   optional:""`
	Priority priority `default:"default" desc:"Message priority with 1=min, 3=default and 5=max"                                                 key:"priority"`
	Actions  []string `                  desc:"Custom user action buttons for notifications, see https://docs.ntfy.sh/publish/#action-buttons"   key:"actions"                                optional:"" sep:";"`
	Click    string   `                  desc:"Website opened when notification is clicked"                                                      key:"click"                                  optional:""`
	Attach   string   `                  desc:"URL of an attachment, see attach via URL"                                                         key:"attach"                                 optional:""`
	Filename string   `                  desc:"File name of the attachment"                                                                      key:"filename"                               optional:""`
	Delay    string   `                  desc:"Timestamp or duration for delayed delivery, see https://docs.ntfy.sh/publish/#scheduled-delivery" key:"delay,at,in"                            optional:""`
	Email    string   `                  desc:"E-mail address for e-mail notifications"                                                          key:"email"                                  optional:""`
	Icon     string   `                  desc:"URL to use as notification icon"                                                                  key:"icon"                                   optional:""`
	Cache    bool     `default:"yes"     desc:"Cache messages"                                                                                   key:"cache"`
	Firebase bool     `default:"yes"     desc:"Send to firebase"                                                                                 key:"firebase"`
}

// Enums returns the fields that use an EnumFormatter for their values.
func (*Config) Enums() map[string]types.EnumFormatter {
	return map[string]types.EnumFormatter{
		"Priority": Priority.Enum,
	}
}

// GetURL returns a URL representation of the Config's current field values.
func (config *Config) GetURL() *url.URL {
	resolver := format.NewPropKeyResolver(config)

	return config.getURL(&resolver)
}

// SetURL updates the Config from a URL representation of its field values.
func (config *Config) SetURL(url *url.URL) error {
	resolver := format.NewPropKeyResolver(config)

	return config.setURL(&resolver, url)
}

// GetAPIURL constructs the API URL for the Ntfy service based on the configuration.
func (config *Config) GetAPIURL() string {
	path := config.Topic
	if !strings.HasPrefix(config.Topic, "/") {
		path = "/" + path
	}

	var creds *url.Userinfo
	if config.Password != "" {
		creds = url.UserPassword(config.Username, config.Password)
	}

	apiURL := url.URL{
		Scheme: config.Scheme,
		Host:   config.Host,
		Path:   path,
		User:   creds,
	}

	return apiURL.String()
}

// getURL constructs a URL from the Config's fields using the provided resolver.
func (config *Config) getURL(resolver types.ConfigQueryResolver) *url.URL {
	return &url.URL{
		User:       url.UserPassword(config.Username, config.Password),
		Host:       config.Host,
		Scheme:     Scheme,
		ForceQuery: true,
		Path:       config.Topic,
		RawQuery:   format.BuildQuery(resolver),
	}
}

// setURL updates the Config from a URL using the provided resolver.
func (config *Config) setURL(resolver types.ConfigQueryResolver, url *url.URL) error {
	password, _ := url.User.Password()
	config.Password = password
	config.Username = url.User.Username()
	config.Host = url.Host
	config.Topic = strings.TrimPrefix(url.Path, "/")

	url.RawQuery = strings.ReplaceAll(url.RawQuery, ";", "%3b")
	for key, vals := range url.Query() {
		if err := resolver.Set(key, vals[0]); err != nil {
			return fmt.Errorf("setting query parameter %q to %q: %w", key, vals[0], err)
		}
	}

	if url.String() != "ntfy://dummy@dummy.com" {
		if config.Topic == "" {
			return ErrTopicRequired
		}
	}

	return nil
}
