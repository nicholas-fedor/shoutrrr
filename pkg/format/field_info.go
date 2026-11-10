package format

import (
	"reflect"
	"slices"
	"strconv"
	"strings"

	"github.com/nicholas-fedor/shoutrrr/pkg/types"
	"github.com/nicholas-fedor/shoutrrr/pkg/util"
)

// FieldInfo is the meta data about a config field.
type FieldInfo struct {
	// Type is the Go type of the field.
	Type reflect.Type
	// EnumFormatter reads and writes the field's values when it is an enum, or
	// is nil.
	EnumFormatter types.EnumFormatter
	// Name is the Go name of the field.
	Name string
	// Description is the field's `desc` tag.
	Description string
	// DefaultValue is the field's `default` tag, or empty.
	DefaultValue string
	// Template is the field's `tpl` tag, or empty.
	Template string
	// URLParts lists the URL parts from the field's `url` tag.
	URLParts []URLPart
	// Keys are the query keys from the field's `key` tag, in lower case. The
	// first is the primary key and the rest are aliases.
	Keys []string
	// DeprecatedKeys are the keys from the field's `deprecatedkeys` tag. They
	// set the field like Keys. A URL never contains them, and the resolver
	// reports each one used so the service can warn about it.
	DeprecatedKeys []string
	// Base is the number base of a numeric field, from its `base` tag, or 10.
	Base int
	// ItemSeparator separates the items of a list field, from its `sep` tag, or
	// a comma.
	ItemSeparator rune
	// Required reports that the field has neither a `default` nor an
	// `optional` tag.
	Required bool
	// Title reports that the field has a `title` tag.
	Title bool
	// Sensitive reports that the field holds a secret, such as a token or a
	// password, from its `sensitive:"true"` tag.
	Sensitive bool
}

// DefaultBase represents the default numeric base (decimal) for fields.
const DefaultBase = 10

// IsEnum reports whether the field is an enum.
//
// Returns:
//   - bool: true when the field has an enum formatter and an int type.
func (fi *FieldInfo) IsEnum() bool {
	return fi.EnumFormatter != nil && fi.Type.Kind() == reflect.Int
}

// IsURLPart reports whether the field is serialized as the specified part of a URL.
//
// Parameters:
//   - part: the URL part to check.
//
// Returns:
//   - bool: true when the field's `url` tag names the part.
func (fi *FieldInfo) IsURLPart(part URLPart) bool {
	return slices.Contains(fi.URLParts, part)
}

// getStructFieldInfo reads the field information of a config struct type from
// its struct tags.
//
// Embedded fields, unexported fields, and deprecated fields are left out.
//
// Parameters:
//   - structType: the config struct type.
//   - enums: the enum formatters of the config, by field name.
//
// Returns:
//   - []FieldInfo: the information of each listed field, in field order.
func getStructFieldInfo(structType reflect.Type, enums map[string]types.EnumFormatter) []FieldInfo {
	numFields := structType.NumField()
	fields := make([]FieldInfo, 0, numFields)
	maxKeyLen := 0

	for i := range numFields {
		fieldDef := structType.Field(i)

		if isHiddenField(&fieldDef) {
			// This is an embedded or private field, which should not be part of the Config output
			continue
		}

		if _, deprecated := fieldDef.Tag.Lookup(deprecatedFieldTag); deprecated {
			// A deprecated field is only read by SyncDeprecatedFields, so it is
			// left out of the docs, the generators, and the masked URL.
			continue
		}

		//nolint:exhaustruct_v5 // Partial initialization is intentional, other fields are set below
		info := FieldInfo{
			Name:          fieldDef.Name,
			Type:          fieldDef.Type,
			Required:      true,
			Title:         false,
			ItemSeparator: ',',
		}

		if util.IsNumeric(fieldDef.Type.Kind()) {
			info.Base = getFieldBase(&fieldDef)
		}

		if tag, ok := fieldDef.Tag.Lookup("desc"); ok {
			info.Description = tag
		}

		if tag, ok := fieldDef.Tag.Lookup("tpl"); ok {
			info.Template = tag
		}

		if tag, ok := fieldDef.Tag.Lookup("default"); ok {
			info.Required = false
			info.DefaultValue = tag
		}

		if _, ok := fieldDef.Tag.Lookup("optional"); ok {
			info.Required = false
		}

		if _, ok := fieldDef.Tag.Lookup("title"); ok {
			info.Title = true
		}

		if tag, ok := fieldDef.Tag.Lookup("sensitive"); ok {
			info.Sensitive = tag == "true"
		}

		if tag, ok := fieldDef.Tag.Lookup("url"); ok {
			info.URLParts = ParseURLParts(tag)
		}

		if tag, ok := fieldDef.Tag.Lookup("key"); ok {
			tag := strings.ToLower(tag)
			info.Keys = strings.Split(tag, ",")
		}

		if tag, ok := fieldDef.Tag.Lookup(deprecatedKeysTag); ok {
			info.DeprecatedKeys = strings.Split(strings.ToLower(tag), ",")
		}

		if tag, ok := fieldDef.Tag.Lookup("sep"); ok {
			info.ItemSeparator = rune(tag[0])
		}

		if ef, isEnum := enums[fieldDef.Name]; isEnum {
			info.EnumFormatter = ef
		}

		fields = append(fields, info)

		keyLen := len(fieldDef.Name)
		if keyLen > maxKeyLen {
			maxKeyLen = keyLen
		}
	}

	return fields
}

// isHiddenField reports whether a struct field stays out of the config
// information.
//
// Parameters:
//   - field: the struct field.
//
// Returns:
//   - bool: true for an embedded or unexported field.
func isHiddenField(field *reflect.StructField) bool {
	return field.Anonymous || strings.ToUpper(field.Name[0:1]) != field.Name[0:1]
}

// getFieldBase returns the number base of a numeric struct field.
//
// Parameters:
//   - field: the struct field.
//
// Returns:
//   - int: the base from the field's `base` tag, or [DefaultBase] when the tag
//     is missing or not a number.
func getFieldBase(field *reflect.StructField) int {
	if tag, ok := field.Tag.Lookup("base"); ok {
		if base, err := strconv.ParseUint(tag, 10, 8); err == nil {
			return int(base)
		}
	}

	// Default to base 10 if not tagged
	return DefaultBase
}
