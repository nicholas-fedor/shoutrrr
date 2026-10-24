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
	Type          reflect.Type
	EnumFormatter types.EnumFormatter
	Name          string
	Description   string
	DefaultValue  string
	Template      string
	URLParts      []URLPart
	Keys          []string
	// DeprecatedKeys are the keys from the field's `deprecatedkeys` tag. They
	// set the field like Keys. A URL never contains them, and the resolver
	// reports each one used so the service can warn about it.
	DeprecatedKeys []string
	Base           int
	ItemSeparator  rune
	Required       bool
	Title          bool
	// Sensitive reports that the field holds a secret, such as a token or a
	// password, from its `sensitive:"true"` tag.
	Sensitive bool
}

// DefaultBase represents the default numeric base (decimal) for fields.
const DefaultBase = 10

// IsEnum returns whether a EnumFormatter has been assigned to the field and that it is of a suitable type.
func (fi *FieldInfo) IsEnum() bool {
	return fi.EnumFormatter != nil && fi.Type.Kind() == reflect.Int
}

// IsURLPart returns whether the field is serialized as the specified part of an URL.
func (fi *FieldInfo) IsURLPart(part URLPart) bool {
	return slices.Contains(fi.URLParts, part)
}

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

func isHiddenField(field *reflect.StructField) bool {
	return field.Anonymous || strings.ToUpper(field.Name[0:1]) != field.Name[0:1]
}

func getFieldBase(field *reflect.StructField) int {
	if tag, ok := field.Tag.Lookup("base"); ok {
		if base, err := strconv.ParseUint(tag, 10, 8); err == nil {
			return int(base)
		}
	}

	// Default to base 10 if not tagged
	return DefaultBase
}
