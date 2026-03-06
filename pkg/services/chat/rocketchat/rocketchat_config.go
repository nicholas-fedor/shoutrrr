package rocketchat

import (
	"fmt"
	"net/url"
	"strings"

	"github.com/nicholas-fedor/shoutrrr/pkg/services/standard"
)

// Config holds the configuration for the Rocket.Chat service.
type Config struct {
	standard.EnumlessConfig

	// UserName is the username to display for the message (optional).
	UserName string `optional:"" url:"user"`
	// Host is the Rocket.Chat server hostname (required).
	Host string `url:"host"`
	// Port is the Rocket.Chat server port (optional).
	Port string `url:"port"`
	// TokenA is the first part of the webhook token (required).
	TokenA string `url:"path1"`
	// Channel is the target channel or user (optional, can include # for channels or @ for users).
	Channel string `url:"path3"`
	// TokenB is the second part of the webhook token (required).
	TokenB string `url:"path2"`
}

// Scheme is the identifying part of this service's configuration URL.
const Scheme = "rocketchat"

// Constants for URL path parsing and validation.
const (
	// MinPathParts is the minimum number of path parts required (including empty first slash).
	MinPathParts = 3
	// TokenBIndex is the index for TokenB in the URL path.
	TokenBIndex = 2
	// ChannelIndex is the index for Channel in the URL path.
	ChannelIndex = 3
)

// GetURL returns a URL representation of the Config's current field values.
//
// Returns:
//   - A pointer to a url.URL struct representing the current configuration
func (c *Config) GetURL() *url.URL {
	host := c.Host
	if c.Port != "" {
		host = fmt.Sprintf("%s:%s", c.Host, c.Port)
	}

	configURL := &url.URL{
		Host:       host,
		Path:       fmt.Sprintf("%s/%s", c.TokenA, c.TokenB),
		Scheme:     Scheme,
		ForceQuery: false,
	}

	return configURL
}

// SetURL updates the Config from a URL representation of its field values.
//
// Params:
//   - serviceURL: The URL to parse and extract configuration from
//
// Returns:
//   - error: An error if the URL is invalid or missing required components
func (c *Config) SetURL(serviceURL *url.URL) error {
	userName := serviceURL.User.Username()
	host := serviceURL.Hostname()

	path := strings.Split(serviceURL.Path, "/")
	if serviceURL.String() != "rocketchat://dummy@dummy.com" {
		if len(path) < MinPathParts {
			return ErrNotEnoughArguments
		}
	}

	c.Port = serviceURL.Port()
	c.UserName = userName
	c.Host = host

	if len(path) > 1 {
		c.TokenA = path[1]
	}

	if len(path) > TokenBIndex {
		c.TokenB = path[TokenBIndex]
	}

	if len(path) > ChannelIndex {
		switch {
		case serviceURL.Fragment != "":
			c.Channel = "#" + serviceURL.Fragment
		case !strings.HasPrefix(path[ChannelIndex], "@"):
			c.Channel = "#" + path[ChannelIndex]
		default:
			c.Channel = path[ChannelIndex]
		}
	}

	return nil
}
