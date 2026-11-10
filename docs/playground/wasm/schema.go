package main

import (
	"encoding/json/v2"
	"path"
	"reflect"
	"strings"

	"github.com/nicholas-fedor/shoutrrr/internal/jsonopts"
	"github.com/nicholas-fedor/shoutrrr/pkg/format"
	"github.com/nicholas-fedor/shoutrrr/pkg/router"
	"github.com/nicholas-fedor/shoutrrr/pkg/types"
)

// docBaseURL is the base URL for service documentation links.
const docBaseURL = "https://shoutrrr.nickfedor.com/"

// serviceDocAliases maps scheme names to their documentation service name
// when the docs path differs from the scheme. Most services use the scheme
// name directly in the docs URL; only aliases need entries here.
var serviceDocAliases = map[string]string{
	"mqtts":    "mqtt",
	"hangouts": "googlechat",
	"xmpps":    "xmpp",
}

// deriveDocURL builds the documentation URL for a service by extracting the
// category from the config struct's Go package path. The service packages
// follow the convention pkg/services/{category}/{serviceName}, so the
// category is derived from the penultimate path segment.
//
// The config parameter is used rather than the Service interface because
// the concrete config type lives in the same package as the service, and
// reflection on interface values may not return complete package paths.
//
// Aliases (e.g., mqtts → mqtt) are resolved via serviceDocAliases.
func deriveDocURL(scheme string, config types.ServiceConfig) string {
	pkgPath := reflect.Indirect(reflect.ValueOf(config)).Type().PkgPath()
	// pkgPath example: github.com/nicholas-fedor/shoutrrr/pkg/services/chat/discord
	category := path.Base(path.Dir(pkgPath))

	docName := scheme
	if alias, ok := serviceDocAliases[scheme]; ok {
		docName = alias
	}

	return docBaseURL + strings.Join([]string{"services", category, docName, ""}, "/")
}

// listServicesJSON returns all registered service schemes as a JSON-encoded
// array of strings. It uses router.ListServices() which reads directly from
// the service map, ensuring automatic parity with the Shoutrrr library.
func listServicesJSON() string {
	r := router.ServiceRouter{}
	schemes := r.ListServices()

	data, err := json.Marshal(schemes, jsonopts.Default)
	if err != nil {
		return marshalError(err)
	}

	return string(data)
}

// configSchemaJSON returns the configuration schema for serviceName as a
// JSON-encoded configSchema. It introspects the service's config struct via
// reflection to extract field metadata (name, type, required, description,
// default value, URL part, enum values).
//
// For services with nil *url.URL fields (e.g., generic), it prepends a
// synthetic "WebhookURL" field that must be provided before URL generation.
func configSchemaJSON(serviceName string) string {
	r := router.ServiceRouter{}

	service, err := r.NewService(serviceName)
	if err != nil {
		return marshalError(err)
	}

	config := format.GetServiceConfig(service)
	if config == nil {
		return marshalErrorStr("service has no config")
	}

	configNode := format.GetConfigFormat(config)
	fields := convertFields(configNode.Items)

	// Add synthetic WebhookURL field for services with nil *url.URL fields.
	// These services (e.g., generic) need a webhook URL set via SetURL().
	configValue := reflect.Indirect(reflect.ValueOf(config))

	for i := range configValue.NumField() {
		field := configValue.Type().Field(i)
		fieldValue := configValue.Field(i)

		if field.Type.Kind() == reflect.Pointer &&
			field.Type.String() == urlTypeName &&
			fieldValue.IsNil() {
			fields = append([]fieldSchema{{
				Name:         "WebhookURL",
				Type:         "string",
				Required:     true,
				Description:  "The webhook URL to send notifications to",
				DefaultValue: "",
				URLPart:      "host",
				Keys:         nil,
				EnumValues:   nil,
			}}, fields...)

			break
		}
	}

	schema := configSchema{
		Service: serviceName,
		Scheme:  serviceName,
		DocURL:  deriveDocURL(serviceName, config),
		Fields:  fields,
	}

	data, err := json.Marshal(schema, jsonopts.Default)
	if err != nil {
		return marshalError(err)
	}

	return string(data)
}

// convertFields converts the fields of a config format to the schemas the
// frontend renders.
//
// A field that [format.FieldInfo.IsEnum] reports as an enum also lists the
// names of its values.
//
// Parameters:
//   - nodes: the items of the config format.
//
// Returns:
//   - []fieldSchema: one schema for each field, in the order of the items.
func convertFields(nodes []format.Node) []fieldSchema {
	fields := make([]fieldSchema, 0, len(nodes))

	for _, node := range nodes {
		field := node.Field()
		isEnum := field.IsEnum()

		var enumValues []string
		if isEnum {
			enumValues = field.EnumFormatter.Names()
		}

		fields = append(fields, fieldSchema{
			Name:         field.Name,
			Type:         classifyType(field.Type, isEnum),
			Description:  field.Description,
			DefaultValue: field.DefaultValue,
			URLPart:      urlPartToString(field.URLParts),
			Keys:         field.Keys,
			EnumValues:   enumValues,
			Required:     field.Required,
		})
	}

	return fields
}
