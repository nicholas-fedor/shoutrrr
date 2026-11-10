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

// serviceDocAliases maps a scheme to the service name in its docs URL, for
// each scheme whose docs page has another service's name.
//
// Every other scheme is its own name in the docs URL.
var serviceDocAliases = map[string]string{
	"mqtts":    "mqtt",
	"hangouts": "googlechat",
	"xmpps":    "xmpp",
}

// deriveDocURL returns the docs URL of a service.
//
// The URL names the service's category and its name. The category is the
// parent directory of the config type's package, since each service lives in
// pkg/services/CATEGORY/NAME. An alias scheme, such as mqtts, takes the name of
// the service whose page it shares.
//
// Parameters:
//   - scheme: the service's scheme.
//   - config: the service's config, whose type is in the service's package.
//
// Returns:
//   - string: the docs URL, ending with a slash.
func deriveDocURL(scheme string, config types.ServiceConfig) string {
	pkgPath := reflect.Indirect(reflect.ValueOf(config)).Type().PkgPath()
	// For Discord, the package path ends with pkg/services/chat/discord.
	category := path.Base(path.Dir(pkgPath))

	docName := scheme
	if alias, ok := serviceDocAliases[scheme]; ok {
		docName = alias
	}

	return docBaseURL + strings.Join([]string{"services", category, docName, ""}, "/")
}

// listServicesJSON returns the schemes of every service as a JSON array.
//
// The schemes come from [router.ServiceRouter.ListServices], so the list
// follows the services the library has, in no particular order.
//
// Returns:
//   - string: the schemes as a JSON array of strings, or an errorResult as
//     JSON when encoding fails.
func listServicesJSON() string {
	r := router.ServiceRouter{}
	schemes := r.ListServices()

	data, err := json.Marshal(schemes, jsonopts.Default)
	if err != nil {
		return marshalError(err)
	}

	return string(data)
}

// configSchemaJSON returns the config schema of a service as JSON.
//
// The schema lists each field the config format has, with its type, default,
// URL part, keys, and enum names. For a config with a nil *url.URL field, such
// as generic's, it also starts with a required WebhookURL field, since a URL
// cannot be generated without one.
//
// Parameters:
//   - serviceName: the service's scheme.
//
// Returns:
//   - string: the configSchema as JSON, or an errorResult as JSON when no
//     service has the scheme.
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
