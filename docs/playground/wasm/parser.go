package main

import (
	"encoding/json"
	"net/url"
	"reflect"
	"strings"

	"github.com/nicholas-fedor/shoutrrr/pkg/format"
	"github.com/nicholas-fedor/shoutrrr/pkg/router"
	"github.com/nicholas-fedor/shoutrrr/pkg/types"
)

// parseURLString parses a Shoutrrr URL and returns the service name and
// configuration values as a JSON-encoded parseResult. It uses router.Locate()
// which calls service.Initialize() internally — the same flow as the CLI's
// verify command.
//
// For services with a WebhookURL() method (e.g., generic), the webhook URL
// is extracted and included in the config values without the scheme prefix.
func parseURLString(rawURL string) string {
	r := router.ServiceRouter{}

	service, err := r.Locate(rawURL)
	if err != nil {
		return marshalError(err)
	}

	scheme := extractScheme(rawURL)
	config := format.GetServiceConfig(service)
	configNode := format.GetConfigFormat(config)
	values := extractConfigValues(config, configNode)

	// Extract webhook URL for services that have it (e.g., generic).
	// Display without scheme since the generic service uses DisableTLS
	// to determine http vs https.
	webhookDisplay := extractWebhookDisplay(reflect.ValueOf(config))
	if webhookDisplay != "" {
		values["WebhookURL"] = webhookDisplay
	}

	result := parseResult{
		Service: scheme,
		Config:  values,
	}

	data, err := json.Marshal(result)
	if err != nil {
		return marshalError(err)
	}

	return string(data)
}

// validateURLString validates a Shoutrrr URL by attempting to locate and
// initialize the corresponding service. Returns {"valid":true} on success
// or a JSON error on failure.
func validateURLString(rawURL string) string {
	r := router.ServiceRouter{}

	_, err := r.Locate(rawURL)
	if err != nil {
		return marshalError(err)
	}

	return `{"valid":true}`
}

// extractConfigValues reads all field values from a config struct and
// returns them as a string map. Uses format.GetConfigFieldString() for
// proper serialization of each field type.
func extractConfigValues(
	config types.ServiceConfig,
	configNode *format.ContainerNode,
) map[string]string {
	values := make(map[string]string, len(configNode.Items))
	configValue := reflect.Indirect(reflect.ValueOf(config))

	for _, node := range configNode.Items {
		field := node.Field()
		fieldValue := configValue.FieldByName(field.Name)

		if !fieldValue.IsValid() {
			continue
		}

		strVal, err := format.GetConfigFieldString(configValue, field)
		if err != nil {
			strVal = fieldValue.String()
		}

		values[field.Name] = strVal
	}

	return values
}

// extractScheme extracts the service scheme from a Shoutrrr URL string.
// For compound schemes like "teams+https", it returns the base scheme "teams".
func extractScheme(rawURL string) string {
	before, _, ok := strings.Cut(rawURL, "://")
	if !ok {
		return ""
	}

	scheme := before

	// Handle compound schemes (e.g., "teams+https://").
	if plusIdx := strings.IndexRune(scheme, '+'); plusIdx >= 0 {
		scheme = scheme[:plusIdx]
	}

	return scheme
}

// extractWebhookDisplay extracts the webhook URL from a config via its
// WebhookURL() method and formats it as host+path?query (without scheme).
// Returns empty string if the method doesn't exist or returns nil.
func extractWebhookDisplay(configValue reflect.Value) string {
	method := configValue.MethodByName("WebhookURL")
	if !method.IsValid() {
		return ""
	}

	results := method.Call(nil)
	if len(results) != 1 || results[0].IsNil() {
		return ""
	}

	webhookURL, ok := results[0].Interface().(*url.URL)
	if !ok {
		return ""
	}

	display := webhookURL.Host + webhookURL.Path
	if webhookURL.RawQuery != "" {
		display += "?" + webhookURL.RawQuery
	}

	return display
}
