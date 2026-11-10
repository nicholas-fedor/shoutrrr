package main

import (
	"encoding/json/v2"
	"fmt"
	"reflect"
	"strings"
	"time"

	"github.com/nicholas-fedor/shoutrrr/internal/jsonopts"
	"github.com/nicholas-fedor/shoutrrr/pkg/format"
	"github.com/nicholas-fedor/shoutrrr/pkg/types"
	"github.com/nicholas-fedor/shoutrrr/pkg/util"
)

// urlTypeName is the reflected type name for *url.URL fields,
// used to identify services that require a webhook URL.
const urlTypeName = "*url.URL"

// classifyType returns a human-readable type string for typ.
//
// An enum is "enum" whatever its kind. A time.Duration is "string", so the
// form takes values such as "10s".
//
// Parameters:
//   - typ: the field's Go type.
//   - isEnum: whether the field is an enum.
//
// Returns:
//   - string: "enum", "string", "bool", "int", "uint", "array", "map", or the
//     kind's name for any other type.
func classifyType(typ reflect.Type, isEnum bool) string {
	if isEnum {
		return "enum"
	}

	// Handle time.Duration as string to allow "10s", "5m", etc.
	if typ == reflect.TypeFor[time.Duration]() {
		return "string"
	}

	kind := typ.Kind()

	switch {
	case kind == reflect.String:
		return "string"
	case kind == reflect.Bool:
		return "bool"
	case util.IsSignedInt(kind):
		return "int"
	case util.IsUnsignedInt(kind):
		return "uint"
	case util.IsCollection(kind):
		return "array"
	case kind == reflect.Map:
		return "map"
	default:
		return kind.String()
	}
}

// urlPartToString names URL parts as a comma-separated list, such as
// "user,password" for URLUser and URLPassword.
//
// Each name is the one a `url` struct tag uses, so [format.ParseURLPart] reads
// it back as the same part. The second path segment is "path2".
//
// Parameters:
//   - parts: the URL parts of a field.
//
// Returns:
//   - string: the part names, or empty for no parts.
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
				// The first path segment is "path", so a later one is named by
				// its position, as format.ParseURLPart reads it.
				names[i] = fmt.Sprintf("path%d", int(part-format.URLPath)+1)
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

// configField finds the field of a config by its Go name.
//
// Parameters:
//   - config: the config.
//   - name: the Go name of the field.
//
// Returns:
//   - *format.FieldInfo: the field, or nil when the config has no field of that
//     name.
func configField(config types.ServiceConfig, name string) *format.FieldInfo {
	for _, node := range format.GetConfigFormat(config).Items {
		if field := node.Field(); field.Name == name {
			return field
		}
	}

	return nil
}

// marshalError serializes err as a JSON errorResult.
//
//nolint:errcheck // errorResult struct always marshals successfully.
func marshalError(err error) string {
	if err == nil {
		return marshalErrorStr("unknown error")
	}

	data, _ := json.Marshal(errorResult{Error: err.Error()}, jsonopts.Default)

	return string(data)
}

// marshalErrorStr serializes msg as a JSON errorResult.
//
//nolint:errcheck // errorResult struct always marshals.
func marshalErrorStr(msg string) string {
	data, _ := json.Marshal(errorResult{Error: msg}, jsonopts.Default)

	return string(data)
}
