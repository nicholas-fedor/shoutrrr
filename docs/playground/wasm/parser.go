package main

import (
	"encoding/json/v2"
	"fmt"
	"net/url"
	"reflect"
	"strings"

	"github.com/nicholas-fedor/shoutrrr/internal/jsonopts"
	"github.com/nicholas-fedor/shoutrrr/pkg/format"
	"github.com/nicholas-fedor/shoutrrr/pkg/router"
	"github.com/nicholas-fedor/shoutrrr/pkg/types"
)

// parseURLString reads a Shoutrrr URL into its service name and config values.
//
// The URL is read by [router.ServiceRouter.Locate], which initializes the
// service as the CLI's verify command does. For a config with a WebhookURL
// method, such as generic's, the values also hold the webhook URL without its
// scheme.
//
// Parameters:
//   - rawURL: the Shoutrrr URL.
//
// Returns:
//   - string: the parseResult as JSON, or an errorResult as JSON when the URL
//     is rejected.
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

	data, err := json.Marshal(result, jsonopts.Default)
	if err != nil {
		return marshalError(err)
	}

	return string(data)
}

// validateURLString checks that a Shoutrrr URL reads into a service.
//
// The service is located and initialized from the URL, which reads and checks
// its config.
//
// Parameters:
//   - rawURL: the Shoutrrr URL.
//
// Returns:
//   - string: {"valid":true}, or an errorResult as JSON when the URL is
//     rejected.
func validateURLString(rawURL string) string {
	r := router.ServiceRouter{}

	_, err := r.Locate(rawURL)
	if err != nil {
		return marshalError(err)
	}

	return `{"valid":true}`
}

// extractConfigValues reads the value of each config field as text.
//
// Each value is written by [format.GetConfigFieldString], as a URL would hold
// it. A value it cannot write is formatted with %v instead.
//
// Parameters:
//   - config: the config to read.
//   - configNode: the config's format, which lists its fields.
//
// Returns:
//   - map[string]string: the values by Go field name.
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
			if fieldValue.CanInterface() {
				strVal = fmt.Sprintf("%v", fieldValue.Interface())
			} else {
				strVal = ""
			}
		}

		values[field.Name] = strVal
	}

	return values
}

// extractScheme returns the service scheme of a Shoutrrr URL.
//
// For a custom URL such as "teams+https://...", it returns the service part,
// "teams".
//
// Parameters:
//   - rawURL: the Shoutrrr URL.
//
// Returns:
//   - string: the scheme, or empty when the URL has no "://".
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

// extractWebhookDisplay returns a config's webhook URL without its scheme.
//
// The URL comes from the config's WebhookURL method, which must take no
// arguments and return a *url.URL. It is written as its host, path, and query.
//
// Parameters:
//   - configValue: the config, as a reflect value.
//
// Returns:
//   - string: the webhook URL without its scheme, or empty when the config has
//     no such method or the method returns nil.
func extractWebhookDisplay(configValue reflect.Value) string {
	method := configValue.MethodByName("WebhookURL")
	if !method.IsValid() {
		return ""
	}

	// Validate method signature before calling.
	methodType := method.Type()
	if methodType.NumIn() != 0 || methodType.NumOut() != 1 {
		return ""
	}

	// Validate output type is nillable (pointer or interface) before calling
	// IsNil to avoid panic on non-nillable return types.
	outKind := methodType.Out(0).Kind()
	if outKind != reflect.Pointer && outKind != reflect.Interface {
		return ""
	}

	results := method.Call(nil)
	if len(results) != 1 || results[0].IsNil() {
		return ""
	}

	webhookURL, ok := reflect.TypeAssert[*url.URL](results[0])
	if !ok {
		return ""
	}

	display := webhookURL.Host + webhookURL.Path
	if webhookURL.RawQuery != "" {
		display += "?" + webhookURL.RawQuery
	}

	return display
}
