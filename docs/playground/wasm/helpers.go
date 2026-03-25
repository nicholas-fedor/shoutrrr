package main

import (
	"encoding/json"
	"fmt"
	"reflect"
	"strings"
	"time"

	"github.com/nicholas-fedor/shoutrrr/pkg/format"
	"github.com/nicholas-fedor/shoutrrr/pkg/types"
)

// urlTypeName is the reflected type name for *url.URL fields,
// used to identify services that require a webhook URL.
const urlTypeName = "*url.URL"

// classifyType returns a human-readable type string for typ.
// If isEnum is true, returns "enum" regardless of the underlying kind.
// time.Duration is classified as "string" to allow duration values like "10s".
//
//nolint:exhaustive // Only handling common config field types.
func classifyType(typ reflect.Type, isEnum bool) string {
	if isEnum {
		return "enum"
	}

	// Handle time.Duration as string to allow "10s", "5m", etc.
	if typ == reflect.TypeFor[time.Duration]() {
		return "string"
	}

	switch typ.Kind() {
	case reflect.String:
		return "string"
	case reflect.Bool:
		return "bool"
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		return "int"
	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
		return "uint"
	case reflect.Slice, reflect.Array:
		return "array"
	case reflect.Map:
		return "map"
	default:
		return typ.Kind().String()
	}
}

// urlPartToString converts a slice of URLPart values to a comma-separated
// string representation (e.g., "user,password" for URLUser and URLPassword).
func urlPartToString(parts []format.URLPart) string {
	if len(parts) == 0 {
		return ""
	}

	names := make([]string, len(parts))
	for i, part := range parts {
		switch part {
		case format.URLUser:
			names[i] = "user"
		case format.URLPassword:
			names[i] = "password"
		case format.URLHost:
			names[i] = "host"
		case format.URLPort:
			names[i] = "port"
		case format.URLPath:
			names[i] = "path"
		case format.URLQuery:
			names[i] = "query"
		default:
			if part > format.URLPath {
				offset := int(part - format.URLPath)
				names[i] = fmt.Sprintf("path%d", offset)
			} else {
				names[i] = fmt.Sprintf("unknown(%d)", part)
			}
		}
	}

	return strings.Join(names, ",")
}

// getEnumNames returns the string names from ef, or nil if ef is nil.
func getEnumNames(ef types.EnumFormatter) []string {
	if ef == nil {
		return nil
	}

	return ef.Names()
}

// setFieldFromString sets configValue[fieldName] to value using reflection.
// Supports string and bool field types. Bool values are parsed via
// format.ParseBool().
func setFieldFromString(configValue reflect.Value, fieldName, value string) {
	field := configValue.FieldByName(fieldName)
	if !field.IsValid() || !field.CanSet() {
		return
	}

	//nolint:exhaustive // Only handling string and bool fields.
	switch field.Kind() {
	case reflect.String:
		field.SetString(value)
	case reflect.Bool:
		bv, ok := format.ParseBool(value, false)
		if ok {
			field.SetBool(bv)
		}
	default:
		// Unsupported field type for direct string assignment.
	}
}

// marshalError serializes err as a JSON errorResult.
//
//nolint:errcheck,errchkjson // errorResult struct always marshals successfully.
func marshalError(err error) string {
	if err == nil {
		return marshalErrorStr("unknown error")
	}

	data, _ := json.Marshal(errorResult{Error: err.Error()})

	return string(data)
}

// marshalErrorStr serializes msg as a JSON errorResult.
//
//nolint:errcheck,errchkjson // errorResult struct always marshals.
func marshalErrorStr(msg string) string {
	data, _ := json.Marshal(errorResult{Error: msg})

	return string(data)
}
