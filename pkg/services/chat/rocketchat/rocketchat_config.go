package rocketchat

import (
	"fmt"
	"net/url"
	"strings"

	"github.com/nicholas-fedor/shoutrrr/pkg/format"
	"github.com/nicholas-fedor/shoutrrr/pkg/services/standard"
	"github.com/nicholas-fedor/shoutrrr/pkg/util"
)

// Config holds the configuration for the Rocket.Chat service.
type Config struct {
	// EnumlessConfig provides the empty enum set, since Rocket.Chat has no
	// enum fields.
	standard.EnumlessConfig

	// UserName is the username to display for the message (optional).
	UserName string `desc:"Override webhook user" optional:"" url:"user"`
	// Host is the Rocket.Chat server hostname (required).
	Host string `desc:"Rocket.Chat server host" url:"host"`
	// Port is the Rocket.Chat server port (optional).
	Port string `desc:"Rocket.Chat server port" optional:"" url:"port"`
	// TokenA is the first part of the webhook token (required).
	TokenA string `desc:"Webhook integration ID" sensitive:"true" url:"path1"`
	// Channel is the target channel or user (optional, can include # for channels or @ for users).
	Channel string `desc:"Override webhook channel, or @user for a direct message" optional:"" url:"path3"`
	// TokenB is the second part of the webhook token (required).
	TokenB string `desc:"Webhook token" sensitive:"true" url:"path2"`
	// DisableTLS sends the webhook request over HTTP instead of HTTPS.
	DisableTLS bool `default:"no" desc:"Send over HTTP instead of HTTPS" key:"disabletls"`
	// SkipTLSVerify skips verification of the server's TLS certificate.
	SkipTLSVerify bool `default:"no" desc:"Skip TLS certificate verification" key:"skiptlsverify"`
}

// Scheme is the identifying part of this service's configuration URL.
const Scheme = "rocketchat"

// Constants for URL path parsing.
const (
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
// as a user recipient. The query holds the config keys whose values differ
// from their defaults.
//
// Returns:
//   - A pointer to a url.URL struct representing the current configuration
func (c *Config) GetURL() *url.URL {
	resolver := format.NewPropKeyResolver(c)

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
		RawQuery:   format.BuildQuery(&resolver),
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
//     the channel,
//     [util.ErrInvalidHost] for a host that is not a hostname or a bracketed
//     IPv6 address, an error wrapping [util.ErrInvalidQuery] for a malformed
//     query, or [format.ErrInvalidConfigKey] for an unknown query key. A
//     rejected URL leaves the config unchanged.
func (c *Config) SetURL(serviceURL *url.URL) error {
	userName := serviceURL.User.Username()

	host, port, err := util.SplitURLHost(serviceURL)
	if err != nil {
		return fmt.Errorf("parsing host: %w", err)
	}

	path := strings.Split(serviceURL.Path, "/")

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

	// The channel path segment GetURL writes holds no further segments, though
	// a trailing slash is fine.
	if len(path) > ChannelIndex+1 && strings.Join(path[ChannelIndex+1:], "") != "" {
		return ErrInvalidChannel
	}

	next := *c
	next.Port = port
	next.UserName = userName
	next.Host = host
	next.Channel = channel
	// A token the path leaves out is empty, not the one an earlier URL set.
	next.TokenA = ""
	next.TokenB = ""

	if len(path) > 1 {
		next.TokenA = path[1]
	}

	if len(path) > TokenBIndex {
		next.TokenB = path[TokenBIndex]
	}

	// The URL is read into a copy, so a rejected URL leaves the config
	// unchanged.
	if err := next.validate(); err != nil {
		return err
	}

	// The query keys start from their defaults, so a key the URL leaves out
	// does not keep the value an earlier URL set.
	resolver := format.NewPropKeyResolver(&next)
	if err := resolver.SetDefaultProps(&next); err != nil {
		return fmt.Errorf("setting default props: %w", err)
	}

	query, err := util.ParseURLQuery(serviceURL)
	if err != nil {
		return err
	}

	if err := format.ApplyQuery(&resolver, query); err != nil {
		return err
	}

	*c = next

	return nil
}

// validate checks that the config names a webhook and a channel it can post
// to. Reading a service URL and each send run it, so a config built in code
// fails before a request.
//
// Returns:
//   - error: [ErrHostMissing] for a config without a host,
//     [ErrNotEnoughArguments] without the integration ID or the token,
//     [ErrInvalidChannel] for a channel with a slash, or nil.
func (c *Config) validate() error {
	// A Rocket.Chat server has no default host.
	if c.Host == "" {
		return ErrHostMissing
	}

	// The webhook URL needs both the integration ID and the token.
	if c.TokenA == "" || c.TokenB == "" {
		return ErrNotEnoughArguments
	}

	// A channel name cannot hold a slash, which only a URL fragment can carry.
	if strings.Contains(c.Channel, "/") {
		return ErrInvalidChannel
	}

	return nil
}
