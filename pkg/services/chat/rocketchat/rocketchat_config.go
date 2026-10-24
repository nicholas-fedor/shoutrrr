package rocketchat

import (
	"fmt"
	"net/url"
	"strings"

	"github.com/nicholas-fedor/shoutrrr/pkg/services/standard"
	"github.com/nicholas-fedor/shoutrrr/pkg/util"
)

// Config holds the configuration for the Rocket.Chat service.
type Config struct {
	// EnumlessConfig provides the empty enum set, since Rocket.Chat has no
	// enum fields.
	standard.EnumlessConfig

	// UserName is the username to display for the message (optional).
	UserName string `optional:"" url:"user"`
	// Host is the Rocket.Chat server hostname (required).
	Host string `url:"host"`
	// Port is the Rocket.Chat server port (optional).
	Port string `url:"port"`
	// TokenA is the first part of the webhook token (required).
	TokenA string `sensitive:"true" url:"path1"`
	// Channel is the target channel or user (optional, can include # for channels or @ for users).
	Channel string `url:"path3"`
	// TokenB is the second part of the webhook token (required).
	TokenB string `sensitive:"true" url:"path2"`
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
// The username is the userinfo, and the channel follows the tokens as a path
// segment: a channel without its # prefix, which [Config.SetURL] restores, or
// a user recipient with its @ prefix. A channel whose name starts with @ goes
// in the URL fragment after a trailing slash instead, so it does not read back
// as a user recipient.
//
// Returns:
//   - A pointer to a url.URL struct representing the current configuration
func (c *Config) GetURL() *url.URL {
	path := fmt.Sprintf("/%s/%s", c.TokenA, c.TokenB)

	var fragment string

	switch channel := strings.TrimPrefix(c.Channel, "#"); {
	case channel == "":
		// No channel, so the path ends after the tokens.
	case strings.HasPrefix(c.Channel, "#@"):
		path += "/"
		fragment = channel
	default:
		path += "/" + channel
	}

	serviceURL := &url.URL{
		User:       util.URLUserPassword(c.UserName, ""),
		Host:       util.JoinURLHost(c.Host, c.Port),
		Path:       path,
		Fragment:   fragment,
		Scheme:     Scheme,
		ForceQuery: false,
	}

	return serviceURL
}

// SetURL updates the Config from a URL representation of its field values.
//
// Parameters:
//   - serviceURL: The URL to parse and extract configuration from
//
// Returns:
//   - error: An error if the URL is invalid or missing required components,
//     such as [ErrHostMissing] for a URL without a host,
//     [ErrNotEnoughArguments] for a missing or empty integration ID or token,
//     [ErrInvalidChannel] for a channel with a slash or path segments after
//     the channel, or
//     [util.ErrInvalidHost] for a host that is not a hostname or a bracketed
//     IPv6 address
func (c *Config) SetURL(serviceURL *url.URL) error {
	userName := serviceURL.User.Username()

	host, port, err := util.SplitURLHost(serviceURL)
	if err != nil {
		return fmt.Errorf("parsing host: %w", err)
	}

	path := strings.Split(serviceURL.Path, "/")
	if serviceURL.String() != "rocketchat://dummy@dummy.com" {
		// A Rocket.Chat server has no default host.
		if host == "" {
			return ErrHostMissing
		}

		// The webhook URL needs both the integration ID and the token.
		if len(path) < MinPathParts || path[1] == "" || path[TokenBIndex] == "" {
			return ErrNotEnoughArguments
		}
	}

	// A URL without a channel segment, or with an empty one, names no channel.
	var channel string

	if len(path) > ChannelIndex {
		switch {
		case serviceURL.Fragment != "":
			channel = "#" + serviceURL.Fragment
		case path[ChannelIndex] == "":
			// A trailing slash after the tokens names no channel.
		case !strings.HasPrefix(path[ChannelIndex], "@"):
			channel = "#" + path[ChannelIndex]
		default:
			channel = path[ChannelIndex]
		}
	}

	// The channel path segment GetURL writes holds neither a slash, which only a
	// fragment can carry, nor further segments, though a trailing slash is fine.
	// The config is left unchanged when the URL is rejected.
	if strings.Contains(channel, "/") ||
		len(path) > ChannelIndex+1 && strings.Join(path[ChannelIndex+1:], "") != "" {
		return ErrInvalidChannel
	}

	c.Port = port
	c.UserName = userName
	c.Host = host
	c.Channel = channel

	if len(path) > 1 {
		c.TokenA = path[1]
	}

	if len(path) > TokenBIndex {
		c.TokenB = path[TokenBIndex]
	}

	return nil
}
