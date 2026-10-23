package ifttt

import (
	"errors"
	"fmt"
	"net/url"
	"strings"

	"github.com/nicholas-fedor/shoutrrr/pkg/format"
	"github.com/nicholas-fedor/shoutrrr/pkg/services/standard"
	"github.com/nicholas-fedor/shoutrrr/pkg/types"
	"github.com/nicholas-fedor/shoutrrr/pkg/util"
)

// Config holds settings for the IFTTT notification service.
type Config struct {
	standard.EnumlessConfig

	WebHookID         string   `required:"true" sensitive:"true"                                                      url:"host"`
	Value1            string   `key:"value1"    optional:""`
	Value2            string   `key:"value2"    optional:""`
	Value3            string   `key:"value3"    optional:""`
	Title             string   `default:""      desc:"Notification title, optionally set by the sender"               key:"title"`
	Events            []string `key:"events"    required:"true"`
	UseMessageAsValue uint8    `default:"2"     desc:"Sets the corresponding value field to the notification message" key:"messagevalue"`
	UseTitleAsValue   uint8    `default:"0"     desc:"Sets the corresponding value field to the notification title"   key:"titlevalue"`
}

const (
	Scheme              = "ifttt" // Scheme identifies this service in configuration URLs.
	DefaultMessageValue = 2       // Default value field (1-3) for the notification message
	DisabledValue       = 0       // Value to disable title assignment
	MinValueField       = 1       // Minimum valid value field (Value1)
	MaxValueField       = 3       // Maximum valid value field (Value3)
	MinLength           = 1       // Minimum length for required fields like Events and WebHookID
)

var (
	ErrInvalidMessageValue = errors.New(
		"invalid value for messagevalue: only values 1-3 are supported",
	)
	ErrInvalidTitleValue = errors.New(
		"invalid value for titlevalue: only values 1-3 or 0 (for disabling) are supported",
	)
	ErrTitleMessageConflict = errors.New("titlevalue cannot use the same number as messagevalue")
	ErrMissingEvents        = errors.New("events missing from config URL")
	ErrMissingWebhookID     = errors.New("webhook ID missing from config URL")
)

// GetURL generates a URL from the current configuration values.
func (c *Config) GetURL() *url.URL {
	resolver := format.NewPropKeyResolver(c)

	return c.getURL(&resolver)
}

// SetURL updates the configuration from a URL representation.
func (c *Config) SetURL(serviceURL *url.URL) error {
	resolver := format.NewPropKeyResolver(c)

	return c.setURL(&resolver, serviceURL)
}

func (c *Config) getURL(resolver types.ConfigQueryResolver) *url.URL {
	return &url.URL{
		Host:     c.WebHookID,
		Path:     "/",
		Scheme:   Scheme,
		RawQuery: format.BuildQuery(resolver),
	}
}

// setURL updates the config from a service URL and validates it.
//
// Parameters:
//   - resolver: Resolver used to apply query parameters to config fields.
//   - serviceURL: The service URL to parse.
//
// Returns:
//   - error: [util.ErrInvalidHost] for a webhook ID with a colon or brackets, an
//     error wrapping [util.ErrInvalidQuery] for a malformed query, a query
//     parameter error, a value field error from
//     [Config.validateValueFields], [ErrMissingEvents], or [ErrMissingWebhookID].
func (c *Config) setURL(resolver types.ConfigQueryResolver, serviceURL *url.URL) error {
	if c.UseMessageAsValue == DisabledValue {
		c.UseMessageAsValue = DefaultMessageValue
	}

	// The webhook ID is the host as written, which cannot hold a port or brackets
	// that url.URL.Hostname would strip.
	if strings.ContainsAny(serviceURL.Host, ":[]") {
		return fmt.Errorf("webhook ID: %w", util.ErrInvalidHost)
	}

	c.WebHookID = serviceURL.Host

	query, err := util.ParseURLQuery(serviceURL)
	if err != nil {
		return err
	}

	for key, vals := range query {
		if err := resolver.Set(key, vals[0]); err != nil {
			return fmt.Errorf("setting config property %q from URL query: %w", key, err)
		}
	}

	if err := c.validateValueFields(); err != nil {
		return err
	}

	if serviceURL.String() != "ifttt://dummy@dummy.com" {
		if len(c.Events) < MinLength {
			return ErrMissingEvents
		}

		if len(c.WebHookID) < MinLength {
			return ErrMissingWebhookID
		}
	}

	return nil
}

// validateValueFields checks the value fields that the message and title
// are written to.
//
// Returns:
//   - error: [ErrInvalidMessageValue] when messagevalue is outside 1 to 3,
//     [ErrInvalidTitleValue] when titlevalue is above 3, or
//     [ErrTitleMessageConflict] when both select the same field.
func (c *Config) validateValueFields() error {
	if c.UseMessageAsValue > MaxValueField || c.UseMessageAsValue < MinValueField {
		return ErrInvalidMessageValue
	}

	if c.UseTitleAsValue > MaxValueField {
		return ErrInvalidTitleValue
	}

	if c.UseTitleAsValue != DisabledValue &&
		c.UseTitleAsValue == c.UseMessageAsValue {
		return ErrTitleMessageConflict
	}

	return nil
}
