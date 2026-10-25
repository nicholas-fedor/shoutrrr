package logger

import (
	"net/url"

	"github.com/nicholas-fedor/shoutrrr/pkg/services/standard"
)

// Config is the configuration object for the Logger Service.
type Config struct {
	// EnumlessConfig provides the empty enum set, since the logger service has
	// no enum fields.
	standard.EnumlessConfig
}

// Identifiers for the logger service.
const (
	// Scheme is the identifying part of this service's configuration URL.
	Scheme = "logger"
)

// GetURL returns a URL representation of its current field values.
//
// Returns:
//   - *url.URL: the URL logger://, since the config has no fields.
func (c *Config) GetURL() *url.URL {
	return &url.URL{
		Scheme: Scheme,
		Opaque: "//", // Ensures "logger://" output
	}
}

// SetURL accepts any URL and leaves the config unchanged, since the config has
// no fields.
//
// Returns:
//   - error: always nil.
func (c *Config) SetURL(_ *url.URL) error {
	return nil
}
