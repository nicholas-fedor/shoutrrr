package rocketchat

import (
	"errors"
	"fmt"
	"net/url"
	"strings"

	"github.com/nicholas-fedor/shoutrrr/pkg/services/standard"
)

// Config for the Rocket.Chat service.
type Config struct {
	standard.EnumlessConfig

	UserName string `optional:"" url:"user"`
	Host     string `            url:"host"`
	Port     string `            url:"port"`
	TokenA   string `            url:"path1"`
	Channel  string `            url:"path3"`
	TokenB   string `            url:"path2"`
}

// Scheme is the identifying part of this service's configuration URL.
const Scheme = "rocketchat"

// Constants for URL path length checks.
const (
	MinPathParts = 3 // Minimum number of path parts required (including empty first slash)
	TokenBIndex  = 2 // Index for TokenB in path
	ChannelIndex = 3 // Index for Channel in path
)

// Static errors for configuration validation.
var (
	ErrNotEnoughArguments = errors.New("the apiURL does not include enough arguments")
)

// GetURL returns a URL representation of the Config's current field values.
func (c *Config) GetURL() *url.URL {
	host := c.Host
	if c.Port != "" {
		host = fmt.Sprintf("%s:%s", c.Host, c.Port)
	}

	url := &url.URL{
		Host:       host,
		Path:       fmt.Sprintf("%s/%s", c.TokenA, c.TokenB),
		Scheme:     Scheme,
		ForceQuery: false,
	}

	return url
}

// SetURL updates the Config from a URL representation of its field values.
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
