package main

import (
	"encoding/json/v2"
	"fmt"
	"net"
	"net/url"
	"reflect"
	"strings"

	"github.com/nicholas-fedor/shoutrrr/internal/jsonopts"
	"github.com/nicholas-fedor/shoutrrr/pkg/format"
	"github.com/nicholas-fedor/shoutrrr/pkg/router"
	"github.com/nicholas-fedor/shoutrrr/pkg/services/push/pushover"
	"github.com/nicholas-fedor/shoutrrr/pkg/types"
)

// generateURLString builds a Shoutrrr URL from serviceName and configJSON.
//
// The config starts from its defaults. For services with nil *url.URL fields,
// such as generic, a "WebhookURL" value is then read into it, and the other
// values set the config by key or by field name. A config whose GetURL
// panics, as one missing a required field can, gives just the scheme.
//
// Parameters:
//   - serviceName: the service scheme.
//   - configJSON: a JSON object mapping keys or field names to string values.
//
// Returns:
//   - string: JSON with a "url" field on success or an "error" field on
//     failure.
func generateURLString(serviceName, configJSON string) string {
	r := router.ServiceRouter{}

	service, err := r.NewService(serviceName)
	if err != nil {
		return marshalError(err)
	}

	var values map[string]string
	if err := json.Unmarshal([]byte(configJSON), &values, jsonopts.Default); err != nil {
		return marshalError(err)
	}

	config := format.GetServiceConfig(service)
	pkr := format.NewPropKeyResolver(config)
	configValue := reflect.Indirect(reflect.ValueOf(config))

	if err := pkr.SetDefaultProps(config); err != nil {
		return marshalError(fmt.Errorf("invalid default: %w", err))
	}

	// Services with nil *url.URL fields, such as generic, read their webhook
	// URL over the defaults, as their Initialize does.
	if webhookURL, ok := values["WebhookURL"]; ok && webhookURL != "" {
		delete(values, "WebhookURL")

		if err := setWebhookURL(config, webhookURL); err != nil {
			return marshalError(err)
		}
	}

	// Set remaining config values from the JSON input.
	for key, value := range values {
		if value == "" {
			continue
		}

		if err := pkr.Set(key, value); err != nil {
			// A field without a query key, such as a URL part, is set by its
			// Go name.
			field := configField(config, key)
			if field == nil {
				return marshalError(fmt.Errorf("invalid config %q=%q: %w", key, value, err))
			}

			if valid, setErr := format.SetConfigField(configValue, field, value); !valid {
				if setErr == nil {
					setErr = format.ErrInvalidValueForType
				}

				return marshalError(fmt.Errorf("invalid config %q=%q: %w", key, value, setErr))
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

	if serviceName == pushover.Scheme {
		pushover.MaskURL(generatedURL)
	}

	return fmt.Sprintf(`{"url":%q}`, generatedURL.String())
}

// setWebhookURL reads a webhook URL into a config.
//
// This is needed for services like generic that have unexported *url.URL
// fields that must be set via SetURL() before GetURL() can be called. Query
// keys in the URL replace the config's values for those keys. A webhook URL
// without a scheme gets http:// for a local host and https:// otherwise.
//
// Parameters:
//   - config: the config to read the URL into.
//   - webhookURL: the webhook URL, with or without a scheme.
//
// Returns:
//   - error: the parse error or the error from SetURL, or nil.
func setWebhookURL(config types.ServiceConfig, webhookURL string) error {
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

	if err := config.SetURL(parsedURL); err != nil {
		return fmt.Errorf("setting webhook URL: %w", err)
	}

	return nil
}

// isLocalHost reports whether host refers to the local machine.
//
// It decides whether a webhook URL without a scheme gets http:// or https://.
// A port after the host is ignored.
//
// Parameters:
//   - host: the host, with an optional port.
//
// Returns:
//   - bool: true for localhost, an IPv4 loopback address in 127.0.0.0/8, or
//     the IPv6 loopback address.
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
