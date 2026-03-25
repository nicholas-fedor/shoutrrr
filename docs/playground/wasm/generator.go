package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/url"
	"reflect"
	"strings"
	"time"

	"github.com/nicholas-fedor/shoutrrr/pkg/format"
	"github.com/nicholas-fedor/shoutrrr/pkg/router"
	"github.com/nicholas-fedor/shoutrrr/pkg/types"
)

// errNoConfigField is returned when a service struct has no Config field.
var errNoConfigField = errors.New("service has no Config field")

// errNoSetURL is returned when a config does not support SetURL.
var errNoSetURL = errors.New("config does not support SetURL")

// generateURLString builds a Shoutrrr URL from serviceName and configJSON.
// The configJSON is a JSON object mapping field names to string values.
// Returns JSON with a "url" field on success or "error" on failure.
//
// For services with nil *url.URL fields (e.g., generic), it extracts the
// "WebhookURL" value, initializes the service's config via reflection,
// and calls SetURL() before generating the final URL.
//
// It validates that all *url.URL pointer fields are non-nil before calling
// GetURL() to prevent nil pointer panics in service-specific implementations.
func generateURLString(serviceName, configJSON string) string {
	r := router.ServiceRouter{}

	service, err := r.NewService(serviceName)
	if err != nil {
		return marshalError(err)
	}

	config := format.GetServiceConfig(service)
	pkr := format.NewPropKeyResolver(config)
	configValue := reflect.Indirect(reflect.ValueOf(config))

	// Set default props, ignoring errors for time.Duration fields
	// which can't be parsed by PropKeyResolver.Set().
	_ = pkr.SetDefaultProps(config) //nolint:errcheck // Duration parse errors handled below.

	// Manually set time.Duration fields to their defaults.
	configSchema := format.GetConfigFormat(config)
	durationType := reflect.TypeFor[time.Duration]()

	for _, node := range configSchema.Items {
		field := node.Field()
		if field.Type == durationType && field.DefaultValue != "" {
			if dur, err := time.ParseDuration(field.DefaultValue); err == nil {
				configValue.FieldByName(field.Name).SetInt(int64(dur))
			}
		}
	}

	var values map[string]string
	if err := json.Unmarshal([]byte(configJSON), &values); err != nil {
		return marshalError(err)
	}

	// Handle webhook URL for services with nil *url.URL fields (e.g., generic).
	if webhookURL, ok := values["WebhookURL"]; ok && webhookURL != "" {
		delete(values, "WebhookURL")

		if err := initWebhookURL(service, webhookURL); err != nil {
			return marshalError(err)
		}

		// Re-extract config from the service's field now that it's initialized.
		// If extraction fails (e.g., SetURL didn't populate the config field),
		// keep the original config from GetServiceConfig() and continue with it.
		if svcConfig, ok := getServiceConfigFromService(service); ok {
			config = svcConfig
			pkr = format.NewPropKeyResolver(config)
			configValue = reflect.Indirect(reflect.ValueOf(config))
		}
	}

	// Set remaining config values from the JSON input.
	for key, value := range values {
		if value == "" {
			continue
		}

		// Skip time.Duration fields - PropKeyResolver.Set() tries to parse
		// duration strings like "10s" as integers, which fails.
		// These fields keep their defaults set by SetDefaultProps().
		if field := configValue.FieldByName(key); field.IsValid() {
			if field.Type() == reflect.TypeFor[time.Duration]() {
				continue
			}
		}

		if err := pkr.Set(key, value); err != nil {
			if field := configValue.FieldByName(key); field.IsValid() && field.CanSet() {
				setFieldFromString(configValue, key, value)
			}
		}
	}

	// Try to generate the URL. If GetURL() panics (e.g., missing required fields),
	// return just the scheme as a clean default instead of an error.
	var generatedURL *url.URL

	func() {
		defer func() {
			if r := recover(); r != nil {
				generatedURL = nil
			}
		}()

		generatedURL = config.GetURL()
	}()

	if generatedURL == nil {
		return fmt.Sprintf(`{"url":%q}`, serviceName+"://")
	}

	return fmt.Sprintf(`{"url":%q}`, generatedURL.String())
}

// initWebhookURL initializes the webhook URL on a service's config field.
// This is needed for services like generic that have unexported *url.URL
// fields that must be set via SetURL() before GetURL() can be called.
//
// It initializes the config if nil, parses the webhookURL (adding https://
// if no scheme is provided), and calls SetURL() on the config.
func initWebhookURL(service types.Service, webhookURL string) error {
	// Add default scheme if missing (url.Parse can't parse host:port without scheme).
	if !strings.Contains(webhookURL, "://") {
		webhookURL = "https://" + webhookURL
	}

	parsedURL, err := url.Parse(webhookURL)
	if err != nil {
		return fmt.Errorf("invalid webhook URL: %w", err)
	}

	// Get service's config field directly via reflection.
	serviceValue := reflect.Indirect(reflect.ValueOf(service))

	configField, found := serviceValue.Type().FieldByName("Config")
	if !found {
		return errNoConfigField
	}

	configRef := serviceValue.FieldByIndex(configField.Index)

	// Initialize config if nil.
	if configRef.IsNil() {
		configType := configField.Type
		if configType.Kind() == reflect.Pointer {
			configType = configType.Elem()
		}

		newConfig := reflect.New(configType)
		configRef.Set(newConfig)
	}

	// Call SetURL on the actual config.
	setter, ok := configRef.Interface().(interface {
		SetURL(webhookURL *url.URL) error
	})
	if !ok {
		return errNoSetURL
	}

	if err := setter.SetURL(parsedURL); err != nil {
		return fmt.Errorf("setting webhook URL: %w", err)
	}

	return nil
}

// getServiceConfigFromService extracts the ServiceConfig from a service's
// Config field via reflection. Returns the config and true if found and
// non-nil, or nil and false otherwise.
func getServiceConfigFromService(service types.Service) (types.ServiceConfig, bool) {
	serviceValue := reflect.Indirect(reflect.ValueOf(service))

	configField, found := serviceValue.Type().FieldByName("Config")
	if !found {
		return nil, false
	}

	configRef := serviceValue.FieldByIndex(configField.Index)
	if configRef.IsNil() {
		return nil, false
	}

	svcConfig, ok := configRef.Interface().(types.ServiceConfig)

	return svcConfig, ok
}
