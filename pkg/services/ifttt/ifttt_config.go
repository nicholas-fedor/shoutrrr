package ifttt

import (
	"errors"
	"fmt"
	"net/url"

	"github.com/nicholas-fedor/shoutrrr/pkg/format"
	"github.com/nicholas-fedor/shoutrrr/pkg/services/standard"
	"github.com/nicholas-fedor/shoutrrr/pkg/types"
)

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

// Config holds settings for the IFTTT notification service.
type Config struct {
	standard.EnumlessConfig
	WebHookID         string   `required:"true" url:"host"`
	Events            []string `required:"true"            key:"events"`
	Value1            string   `                           key:"value1"       optional:""`
	Value2            string   `                           key:"value2"       optional:""`
	Value3            string   `                           key:"value3"       optional:""`
	UseMessageAsValue uint8    `                           key:"messagevalue"             default:"2" desc:"Sets the corresponding value field to the notification message"`
	UseTitleAsValue   uint8    `                           key:"titlevalue"               default:"0" desc:"Sets the corresponding value field to the notification title"`
	Title             string   `                           key:"title"                    default:""  desc:"Notification title, optionally set by the sender"`
}

// GetURL generates a URL from the current configuration values.
func (config *Config) GetURL() *url.URL {
	resolver := format.NewPropKeyResolver(config)

	return config.getURL(&resolver)
}

// SetURL updates the configuration from a URL representation.
func (config *Config) SetURL(url *url.URL) error {
	resolver := format.NewPropKeyResolver(config)

	return config.setURL(&resolver, url)
}

func (config *Config) getURL(resolver types.ConfigQueryResolver) *url.URL {
	return &url.URL{
		Host:     config.WebHookID,
		Path:     "/",
		Scheme:   Scheme,
		RawQuery: format.BuildQuery(resolver),
	}
}

func (config *Config) setURL(resolver types.ConfigQueryResolver, url *url.URL) error {
	if config.UseMessageAsValue == DisabledValue {
		config.UseMessageAsValue = DefaultMessageValue
	}

	config.WebHookID = url.Hostname()

	for key, vals := range url.Query() {
		if err := resolver.Set(key, vals[0]); err != nil {
			return fmt.Errorf("setting config property %q from URL query: %w", key, err)
		}
	}

	if config.UseMessageAsValue > MaxValueField || config.UseMessageAsValue < MinValueField {
		return ErrInvalidMessageValue
	}

	if config.UseTitleAsValue > MaxValueField {
		return ErrInvalidTitleValue
	}

	if config.UseTitleAsValue != DisabledValue &&
		config.UseTitleAsValue == config.UseMessageAsValue {
		return ErrTitleMessageConflict
	}

	if url.String() != "ifttt://dummy@dummy.com" {
		if len(config.Events) < MinLength {
			return ErrMissingEvents
		}

		if len(config.WebHookID) < MinLength {
			return ErrMissingWebhookID
		}
	}

	return nil
}
