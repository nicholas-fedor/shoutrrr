package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"net"
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

// errCannotSetDurationField is returned when a duration field cannot be set.
var errCannotSetDurationField = errors.New("cannot set duration field")

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
	configSchema := format.GetConfigFormat(config)
	durationType := reflect.TypeFor[time.Duration]()

	// Apply defaults manually to handle duration fields correctly.
	// PropKeyResolver.Set() can't parse duration strings like "10s" as integers,
	// so we detect duration fields and parse them with time.ParseDuration instead.
	for _, node := range configSchema.Items {
		field := node.Field()
		if field.DefaultValue == "" {
			continue
		}

		if field.Type == durationType {
			// Parse duration defaults directly.
			if dur, err := time.ParseDuration(field.DefaultValue); err == nil {
				configValue.FieldByName(field.Name).SetInt(int64(dur))
			}
		} else {
			// Apply non-duration defaults via PropKeyResolver.
			for _, key := range field.Keys {
				if err := pkr.Set(key, field.DefaultValue); err != nil {
					return marshalError(fmt.Errorf("invalid default for %q: %w", key, err))
				}
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

			// Apply defaults only to zero-valued fields so values parsed by
			// SetURL from the webhook URL (e.g., host, path, query params)
			// are not overwritten.
			configSchema = format.GetConfigFormat(config)

			for _, node := range configSchema.Items {
				field := node.Field()
				if field.DefaultValue == "" {
					continue
				}

				fieldVal := configValue.FieldByName(field.Name)
				if !fieldVal.IsValid() || !fieldVal.IsZero() {
					continue
				}

				if field.Type == durationType {
					if dur, err := time.ParseDuration(field.DefaultValue); err == nil {
						fieldVal.SetInt(int64(dur))
					}
				} else {
					for _, key := range field.Keys {
						if err := pkr.Set(key, field.DefaultValue); err != nil {
							return marshalError(fmt.Errorf("invalid default for %q: %w", key, err))
						}
					}
				}
			}
		}
	}

	// Set remaining config values from the JSON input.
	for key, value := range values {
		if value == "" {
			continue
		}

		// Handle time.Duration fields by parsing them directly.
		// PropKeyResolver.Set() tries to parse duration strings like "10s" as
		// integers, which fails. Parse user-provided durations the same way
		// defaults are handled above.
		if field := configValue.FieldByName(key); field.IsValid() {
			if field.Type() == reflect.TypeFor[time.Duration]() {
				dur, err := time.ParseDuration(value)
				if err != nil {
					return marshalError(fmt.Errorf("invalid duration %q for %q: %w", value, key, err))
				}

				if !field.CanSet() {
					return marshalError(fmt.Errorf("%w %q to %q", errCannotSetDurationField, key, value))
				}

				field.SetInt(int64(dur))

				continue
			}
		}

		if err := pkr.Set(key, value); err != nil {
			if field := configValue.FieldByName(key); field.IsValid() && field.CanSet() {
				if setErr := setFieldFromString(configValue, key, value); setErr != nil {
					return marshalError(fmt.Errorf("invalid config %q=%q: %w", key, value, err))
				}
			} else {
				return marshalError(fmt.Errorf("invalid config %q=%q: %w", key, value, err))
			}
		}
	}

	// Try to generate the URL. If GetURL() panics (e.g., missing required fields),
	// return just the scheme as a clean default instead of an error.
	var generatedURL *url.URL

	func() {
		defer func() {
			if rec := recover(); rec != nil {
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
		// Use http:// for local hosts, https:// otherwise.
		host := webhookURL
		if idx := strings.Index(host, "/"); idx != -1 {
			host = host[:idx]
		}

		if isLocalHost(host) {
			webhookURL = "http://" + webhookURL
		} else {
			webhookURL = "https://" + webhookURL
		}
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

	// Guard against non-nillable kinds before calling IsNil to avoid panic.
	if configRef.IsValid() && isNillableKind(configRef.Kind()) && configRef.IsNil() {
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

// isLocalHost reports whether host refers to a local machine (localhost,
// loopback IP, or link-local IPv6). Used to decide whether to default to
// http:// or https:// when no scheme is provided.
func isLocalHost(host string) bool {
	// Strip optional port suffix.
	if h, _, err := net.SplitHostPort(host); err == nil {
		host = h
	}

	return host == "localhost" ||
		strings.HasPrefix(host, "127.") ||
		host == "::1" ||
		strings.HasPrefix(host, "[::1]")
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
	if !configRef.IsValid() {
		return nil, false
	}

	// Only call IsNil on nillable kinds to avoid panic.
	if isNillableKind(configRef.Kind()) && configRef.IsNil() {
		return nil, false
	}

	svcConfig, ok := configRef.Interface().(types.ServiceConfig)

	return svcConfig, ok
}
