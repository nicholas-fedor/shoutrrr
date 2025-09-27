package rocketchat

import (
	"errors"
	"fmt"
	"net/url"
	"strings"

	"github.com/nicholas-fedor/shoutrrr/pkg/services/standard"
)

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

// GetURL returns a URL representation of the Config's current field values.
func (config *Config) GetURL() *url.URL {
	host := config.Host
	if config.Port != "" {
		host = fmt.Sprintf("%s:%s", config.Host, config.Port)
	}

	url := &url.URL{
		Host:       host,
		Path:       fmt.Sprintf("%s/%s", config.TokenA, config.TokenB),
		Scheme:     Scheme,
		ForceQuery: false,
	}

	return url
}

// SetURL updates the Config from a URL representation of its field values.
func (config *Config) SetURL(serviceURL *url.URL) error {
	userName := serviceURL.User.Username()
	host := serviceURL.Hostname()

	path := strings.Split(serviceURL.Path, "/")
	if serviceURL.String() != "rocketchat://dummy@dummy.com" {
		if len(path) < MinPathParts {
			return ErrNotEnoughArguments
		}
	}

	config.Port = serviceURL.Port()
	config.UserName = userName
	config.Host = host

	if len(path) > 1 {
		config.TokenA = path[1]
	}

	if len(path) > TokenBIndex {
		config.TokenB = path[TokenBIndex]
	}

	if len(path) > ChannelIndex {
		switch {
		case serviceURL.Fragment != "":
			config.Channel = "#" + serviceURL.Fragment
		case !strings.HasPrefix(path[ChannelIndex], "@"):
			config.Channel = "#" + path[ChannelIndex]
		default:
			config.Channel = path[ChannelIndex]
		}
	}

	return nil
}
