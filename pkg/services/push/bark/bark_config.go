package bark

import (
	"errors"
	"fmt"
	"net/url"
	"strings"

	"github.com/nicholas-fedor/shoutrrr/pkg/format"
	"github.com/nicholas-fedor/shoutrrr/pkg/services/standard"
	"github.com/nicholas-fedor/shoutrrr/pkg/types"
)

// Config holds configuration settings for the Bark service.
type Config struct {
	standard.EnumlessConfig

	Title     string `default:""      desc:"Notification title, optionally set by the sender"           key:"title"`
	Host      string `                desc:"Server hostname and port"                                                  url:"host"`
	Path      string `default:"/"     desc:"Server path"                                                               url:"path"`
	DeviceKey string `                desc:"The key for each device"                                                   url:"password"`
	Scheme    string `default:"https" desc:"Server protocol, http or https"                             key:"scheme"`
	Sound     string `default:""      desc:"Value from https://github.com/Finb/Bark/tree/master/Sounds" key:"sound"`
	Badge     int64  `default:"0"     desc:"The number displayed next to App icon"                      key:"badge"`
	Icon      string `default:""      desc:"An url to the icon, available only on iOS 15 or later"      key:"icon"`
	Group     string `default:""      desc:"The group of the notification"                              key:"group"`
	URL       string `default:""      desc:"Url that will jump when click notification"                 key:"url"`
	Category  string `default:""      desc:"Reserved field, no use yet"                                 key:"category"`
	Copy      string `default:""      desc:"The value to be copied"                                     key:"copy"`
}

// Scheme is the identifying part of this service's configuration URL.
const (
	Scheme = "bark"
)

// ErrSetQueryFailed indicates a failure to set a configuration value from a query parameter.
var ErrSetQueryFailed = errors.New("failed to set query parameter")

// GetAPIURL constructs the API URL for the specified endpoint using the current configuration.
func (c *Config) GetAPIURL(endpoint string) string {
	path := strings.Builder{}
	if !strings.HasPrefix(c.Path, "/") {
		path.WriteByte('/')
	}

	path.WriteString(c.Path)

	if !strings.HasSuffix(path.String(), "/") {
		path.WriteByte('/')
	}

	path.WriteString(endpoint)

	apiURL := url.URL{
		Scheme: c.Scheme,
		Host:   c.Host,
		Path:   path.String(),
	}

	return apiURL.String()
}

// GetURL returns a URL representation of the current configuration values.
func (c *Config) GetURL() *url.URL {
	resolver := format.NewPropKeyResolver(c)

	return c.getURL(&resolver)
}

// SetURL updates the configuration from a URL representation.
func (c *Config) SetURL(url *url.URL) error {
	resolver := format.NewPropKeyResolver(c)

	return c.setURL(&resolver, url)
}

func (c *Config) getURL(resolver types.ConfigQueryResolver) *url.URL {
	return &url.URL{
		User:       url.UserPassword("", c.DeviceKey),
		Host:       c.Host,
		Scheme:     Scheme,
		ForceQuery: true,
		Path:       c.Path,
		RawQuery:   format.BuildQuery(resolver),
	}
}

func (c *Config) setURL(resolver types.ConfigQueryResolver, url *url.URL) error {
	password, _ := url.User.Password()
	c.DeviceKey = password
	c.Host = url.Host
	c.Path = url.Path

	for key, vals := range url.Query() {
		if err := resolver.Set(key, vals[0]); err != nil {
			return fmt.Errorf("%w '%s': %w", ErrSetQueryFailed, key, err)
		}
	}

	return nil
}
