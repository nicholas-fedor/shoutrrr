package mattermost

import (
	"errors"
	"fmt"
	"net/url"
	"strings"

	"github.com/nicholas-fedor/shoutrrr/pkg/format"
	"github.com/nicholas-fedor/shoutrrr/pkg/services/standard"
	"github.com/nicholas-fedor/shoutrrr/pkg/types"
)

// Scheme is the identifying part of this service's configuration URL.
const Scheme = "mattermost"

// Static errors for configuration validation.
var (
	ErrNotEnoughArguments = errors.New(
		"the apiURL does not include enough arguments, either provide 1 or 3 arguments (they may be empty)",
	)
)

// ErrorMessage represents error events within the Mattermost service.
type ErrorMessage string

// Config holds all configuration information for the Mattermost service.
type Config struct {
	standard.EnumlessConfig

	UserName   string `desc:"Override webhook user"                                             optional:"" url:"user"`
	Icon       string `desc:"Use emoji or URL as icon (based on presence of http(s):// prefix)" optional:""                 default:""   key:"icon,icon_emoji,icon_url"`
	Title      string `desc:"Notification title, optionally set by the sender (not used)"                                   default:""   key:"title"`
	Channel    string `desc:"Override webhook channel"                                          optional:"" url:"path2"`
	Host       string `desc:"Mattermost server host"                                                        url:"host,port"`
	Token      string `desc:"Webhook token"                                                                 url:"path1"`
	DisableTLS bool   `                                                                                                     default:"No" key:"disabletls"`
}

// CreateConfigFromURL creates a new Config instance from a URL representation.
func CreateConfigFromURL(url *url.URL) (*Config, error) {
	config := &Config{}
	if err := config.SetURL(url); err != nil {
		return nil, err
	}

	return config, nil
}

// GetURL returns a URL representation of the Config's current field values.
func (c *Config) GetURL() *url.URL {
	resolver := format.NewPropKeyResolver(c)

	return c.getURL(&resolver) // Pass pointer to resolver
}

// SetURL updates the Config from a URL representation of its field values.
func (c *Config) SetURL(url *url.URL) error {
	resolver := format.NewPropKeyResolver(c)

	return c.setURL(&resolver, url) // Pass pointer to resolver
}

// getURL constructs a URL from the Config's fields using the provided resolver.
func (c *Config) getURL(resolver types.ConfigQueryResolver) *url.URL {
	paths := []string{"", c.Token, c.Channel}
	if c.Channel == "" {
		paths = paths[:2]
	}

	var user *url.Userinfo
	if c.UserName != "" {
		user = url.User(c.UserName)
	}

	return &url.URL{
		User:       user,
		Host:       c.Host,
		Path:       strings.Join(paths, "/"),
		Scheme:     Scheme,
		ForceQuery: false,
		RawQuery:   format.BuildQuery(resolver),
	}
}

// setURL updates the Config from a URL using the provided resolver.
func (c *Config) setURL(resolver types.ConfigQueryResolver, url *url.URL) error {
	c.Host = url.Host
	c.UserName = url.User.Username()

	if err := c.parsePath(url); err != nil {
		return err
	}

	for key, vals := range url.Query() {
		if err := resolver.Set(key, vals[0]); err != nil {
			return fmt.Errorf("setting query parameter %q to %q: %w", key, vals[0], err)
		}
	}

	return nil
}

// parsePath extracts Token and Channel from the URL path and validates arguments.
func (c *Config) parsePath(url *url.URL) error {
	path := strings.Split(strings.Trim(url.Path, "/"), "/")
	isDummy := url.String() == "mattermost://dummy@dummy.com"

	if !isDummy && (len(path) < 1 || path[0] == "") {
		return ErrNotEnoughArguments
	}

	if len(path) > 0 && path[0] != "" {
		c.Token = path[0]
	}

	if len(path) > 1 && path[1] != "" {
		c.Channel = path[1]
	}

	return nil
}
