package main

import (
	"encoding/json"
	"reflect"

	"github.com/nicholas-fedor/shoutrrr/pkg/format"
	"github.com/nicholas-fedor/shoutrrr/pkg/router"
	"github.com/nicholas-fedor/shoutrrr/pkg/types"
)

// listServicesJSON returns all registered service schemes as a JSON-encoded
// array of strings. It uses router.ListServices() which reads directly from
// the service map, ensuring automatic parity with the Shoutrrr library.
func listServicesJSON() string {
	r := router.ServiceRouter{}
	schemes := r.ListServices()

	data, err := json.Marshal(schemes)
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
	configNode := format.GetConfigFormat(config)
	fields := convertFields(configNode.Items, config)

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
		Fields:  fields,
	}

	data, err := json.Marshal(schema)
	if err != nil {
		return marshalError(err)
	}

	return string(data)
}

// convertFields converts a slice of format.Node items to fieldSchema structs.
// It maps each field's metadata (type, required, description, enums) to the
// JSON-serializable format used by the frontend.
func convertFields(nodes []format.Node, config types.ServiceConfig) []fieldSchema {
	fields := make([]fieldSchema, 0, len(nodes))
	enums := config.Enums()

	for _, node := range nodes {
		field := node.Field()

		//nolint:exhaustruct // EnumValues set conditionally below.
		schema := fieldSchema{
			Name:         field.Name,
			Type:         classifyType(field.Type, field.IsEnum()),
			Required:     field.Required,
			Description:  field.Description,
			DefaultValue: field.DefaultValue,
			URLPart:      urlPartToString(field.URLParts),
			Keys:         field.Keys,
		}

		if field.IsEnum() {
			schema.EnumValues = getEnumNames(field.EnumFormatter)
		}

		// Check for enums defined via config.Enums() map (e.g., ntfy Priority).
		if schema.EnumValues == nil && isIntegerKind(field.Type.Kind()) {
			if ef, ok := enums[field.Name]; ok {
				schema.EnumValues = getEnumNames(ef)
				schema.Type = "enum"
			}
		}

		fields = append(fields, schema)
	}

	return fields
}

// isIntegerKind returns true for all signed and unsigned integer kinds.
//
//nolint:exhaustive // Using default case for non-integer kinds.
func isIntegerKind(k reflect.Kind) bool {
	switch k {
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64,
		reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
		return true
	default:
		return false
	}
}
