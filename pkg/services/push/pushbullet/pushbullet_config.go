package pushbullet

import (
	"errors"
	"fmt"
	"net/url"
	"strings"

	"github.com/nicholas-fedor/shoutrrr/pkg/format"
	"github.com/nicholas-fedor/shoutrrr/pkg/services/standard"
	"github.com/nicholas-fedor/shoutrrr/pkg/types"
)

// Scheme is the scheme part of the service configuration URL.
const Scheme = "pushbullet"

// ExpectedTokenLength is the required length for a valid Pushbullet token.
const ExpectedTokenLength = 34

// ErrTokenIncorrectSize indicates that the token has an incorrect size.
var ErrTokenIncorrectSize = errors.New("token has incorrect size")

// Config holds the configuration for the Pushbullet service.
type Config struct {
	standard.EnumlessConfig

	Targets []string `url:"path"`
	Token   string   `url:"host"`
	Title   string   `           default:"Shoutrrr notification" key:"title"`
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

// getURL constructs a URL from the Config's fields using the provided resolver.
func (config *Config) getURL(resolver types.ConfigQueryResolver) *url.URL {
	return &url.URL{
		Host:       config.Token,
		Path:       "/" + strings.Join(config.Targets, "/"),
		Scheme:     Scheme,
		ForceQuery: false,
		RawQuery:   format.BuildQuery(resolver),
	}
}

// setURL updates the Config from a URL using the provided resolver.
func (config *Config) setURL(resolver types.ConfigQueryResolver, url *url.URL) error {
	path := url.Path
	if len(path) > 0 && path[0] == '/' {
		path = path[1:]
	}

	if url.Fragment != "" {
		path += "/#" + url.Fragment
	}

	targets := strings.Split(path, "/")

	token := url.Hostname()
	if url.String() != "pushbullet://dummy@dummy.com" {
		if err := validateToken(token); err != nil {
			return err
		}
	}

	config.Token = token
	config.Targets = targets

	for key, vals := range url.Query() {
		if err := resolver.Set(key, vals[0]); err != nil {
			return fmt.Errorf("setting query parameter %q to %q: %w", key, vals[0], err)
		}
	}

	return nil
}

// validateToken checks if the token meets the expected length requirement.
func validateToken(token string) error {
	if len(token) != ExpectedTokenLength {
		return ErrTokenIncorrectSize
	}

	return nil
}
