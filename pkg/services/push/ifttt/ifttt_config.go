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
	// EnumlessConfig provides the empty enum set, since IFTTT has no enum
	// fields.
	standard.EnumlessConfig

	// WebHookID is the IFTTT webhook key, written as the host of the service
	// URL.
	WebHookID string `required:"true" sensitive:"true" url:"host"`
	// Value1 is the payload's value1 field.
	Value1 string `desc:"The value1 field of the webhook payload" key:"value1" optional:""`
	// Value2 is the payload's value2 field.
	Value2 string `desc:"The value2 field of the webhook payload" key:"value2" optional:""`
	// Value3 is the payload's value3 field.
	Value3 string `desc:"The value3 field of the webhook payload" key:"value3" optional:""`
	// Title is the notification title, sent in the value field that
	// UseTitleAsValue selects.
	Title string `default:"" desc:"Notification title, optionally set by the sender" key:"title"`
	// Events lists the webhook event names. A send triggers each one.
	Events []string `desc:"The webhook events to trigger" key:"events" required:"true"`
	// UseMessageAsValue selects the value field, 1 to 3, that carries the
	// message.
	UseMessageAsValue uint8 `default:"2" desc:"Sets the corresponding value field to the notification message" key:"messagevalue"`
	// UseTitleAsValue selects the value field, 1 to 3, that carries the title,
	// or 0 to send no title.
	UseTitleAsValue uint8 `default:"0" desc:"Sets the corresponding value field to the notification title" key:"titlevalue"`
}

// Service identity and value field limits.
const (
	// Scheme identifies this service in configuration URLs.
	Scheme = "ifttt"
	// DefaultMessageValue is the default value field (1-3) for the
	// notification message.
	DefaultMessageValue = 2
	// DisabledValue is the titlevalue that sends no title.
	DisabledValue = 0
	// MinValueField is the lowest value field number, for Value1.
	MinValueField = 1
	// MaxValueField is the highest value field number, for Value3.
	MaxValueField = 3
	// MinLength is the minimum length of required fields such as Events and
	// WebHookID.
	MinLength = 1
)

// Errors returned when reading the configuration or the send params.
var (
	// ErrInvalidMessageValue indicates a messagevalue outside 1 to 3.
	ErrInvalidMessageValue = errors.New(
		"invalid value for messagevalue: only values 1-3 are supported",
	)
	// ErrInvalidTitleValue indicates a titlevalue outside 0 to 3.
	ErrInvalidTitleValue = errors.New(
		"invalid value for titlevalue: only values 1-3 or 0 (for disabling) are supported",
	)
	// ErrTitleMessageConflict indicates a titlevalue that selects the same
	// value field as messagevalue.
	ErrTitleMessageConflict = errors.New("titlevalue cannot use the same number as messagevalue")
	// ErrMissingEvents indicates a service URL without events.
	ErrMissingEvents = errors.New("events missing from config URL")
	// ErrMissingWebhookID indicates a service URL without the webhook key.
	ErrMissingWebhookID = errors.New("webhook ID missing from config URL")
)

// GetURL generates a URL from the current configuration values.
//
// Returns:
//   - *url.URL: the service URL.
func (c *Config) GetURL() *url.URL {
	resolver := format.NewPropKeyResolver(c)

	return c.getURL(&resolver)
}

// SetURL updates the configuration from a URL representation.
//
// Parameters:
//   - serviceURL: the service URL.
//
// Returns:
//   - error: the error from reading the URL, such as [ErrMissingEvents].
func (c *Config) SetURL(serviceURL *url.URL) error {
	resolver := format.NewPropKeyResolver(c)

	return c.setURL(&resolver, serviceURL)
}

// getURL constructs a URL from the Config's fields using the provided resolver.
//
// Parameters:
//   - resolver: the resolver that builds the query from the config props.
//
// Returns:
//   - *url.URL: the service URL, with the webhook key as the host.
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

	if err := format.ApplyQuery(resolver, query); err != nil {
		return err
	}

	if err := c.validateValueFields(); err != nil {
		return err
	}

	if len(c.Events) < MinLength {
		return ErrMissingEvents
	}

	if len(c.WebHookID) < MinLength {
		return ErrMissingWebhookID
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
