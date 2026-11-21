package format

import (
	"fmt"
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
	// owner is the config struct type the field was read from, or nil for a
	// FieldInfo built by hand.
	owner reflect.Type
	// Name is the Go name of the field.
	Name string
	// Description is the field's `desc` tag.
	Description string
	// DefaultValue is the field's `default` tag, or empty.
	DefaultValue string
	// Template is the field's `tpl` tag, or empty.
	Template string
	// replacedBy is the key that replaces the field's keys, in lower case,
	// from its `replacedby` tag, or empty. The field's keys then set the
	// replacement's field instead of their own.
	replacedBy string
	// URLParts lists the URL parts from the field's `url` tag.
	URLParts []URLPart
	// Keys are the query keys from the field's `key` tag, in lower case. The
	// first is the primary key and the rest are aliases.
	Keys []string
	// DeprecatedKeys are the keys from the field's `deprecatedkeys` tag. They
	// set the field like Keys. A URL never contains them, and the resolver
	// reports each one used so the service can warn about it.
	DeprecatedKeys []string
	// replacedValues maps each value of the field's keys onto the value the
	// replacement key takes, from its `replacedvalues` tag. Nil passes values
	// as written. A malformed tag gives an empty mapping, which accepts no
	// value.
	replacedValues []valueReplacement
	// Base is the number base of a numeric field, from its `base` tag, or 10.
	Base int
	// index is the field's index in owner.
	index int
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
	// levelMapped reports that a service maps the level param onto the field
	// while the URL leaves its key out, from its `levelmapped:"true"` tag. The
	// URL that its config writes keeps the key whenever a URL set it, as
	// [URLKeys] describes.
	levelMapped bool
}

// valueReplacement maps a value of a replaced key onto the value its
// replacement key takes.
type valueReplacement struct {
	// from is the value of the replaced key, in lower case.
	from string
	// to is the value the replacement key takes.
	to string
}

// DefaultBase represents the default numeric base (decimal) for fields.
const DefaultBase = 10

// Struct tags that mark config fields.
const (
	// levelMappedTag marks a field that a service maps the level param onto,
	// so the URL that its config writes keeps the key whenever a URL set it to
	// its default.
	levelMappedTag = "levelmapped"
	// replacedByTag names the key that replaces a field's keys.
	replacedByTag = "replacedby"
	// replacedValuesTag maps the values of a field's replaced keys onto the
	// values of the key that replaces them, as from=to pairs separated by
	// commas.
	replacedValuesTag = "replacedvalues"
)

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

// replacedValue returns the value that the replacement key takes for a value
// of the field's keys.
//
// The value matches its mapping in any case, but only at the mapped value's
// byte length, so a look-alike such as the long s in httpſ does not match
// https. A field without a mapping passes the value as written.
//
// Parameters:
//   - value: the value of a key that the replacement key replaces.
//
// Returns:
//   - string: the value for the replacement key.
//   - error: an error wrapping [ErrInvalidValueForType] that lists the
//     accepted values, for a value the mapping does not list.
func (fi *FieldInfo) replacedValue(value string) (string, error) {
	if fi.replacedValues == nil {
		return value, nil
	}

	for _, replacement := range fi.replacedValues {
		if equalFoldSameLength(replacement.from, value) {
			return replacement.to, nil
		}
	}

	accepted := make([]string, 0, len(fi.replacedValues))
	for _, replacement := range fi.replacedValues {
		accepted = append(accepted, replacement.from)
	}

	return "", fmt.Errorf("%w: %q (accepted: %s)", ErrInvalidValueForType, value, strings.Join(accepted, ", "))
}

// replaces reports whether the field's key replaces the keys of another field.
//
// That holds when the field's primary key is the one that the other field's
// `replacedby` tag names, and the field's own keys are not replaced.
//
// Parameters:
//   - replaced: the field whose keys another key replaces.
//
// Returns:
//   - bool: true when the field is the replacement of replaced.
func (fi *FieldInfo) replaces(replaced *FieldInfo) bool {
	return fi.replacedBy == "" && len(fi.Keys) > 0 && fi.Keys[0] == replaced.replacedBy
}

// value returns the field in config. A field read from config's own struct
// type is reached by its index, and any other by its name, which searches the
// struct's fields.
//
// Parameters:
//   - config: the config struct value.
//
// Returns:
//   - reflect.Value: the field's value in config.
func (fi *FieldInfo) value(config reflect.Value) reflect.Value {
	if fi.owner != nil && config.Type() == fi.owner {
		return config.Field(fi.index)
	}

	return config.FieldByName(fi.Name)
}

// newValueFieldInfo describes a value that is not a config field, such as an
// item of a list or the config struct itself. It has no tags and no struct to
// be looked up in.
//
// Parameters:
//   - name: the value's name, such as the item's key, or empty.
//   - valueType: the value's type.
//   - base: the number base of a numeric value, or 0.
//
// Returns:
//   - *FieldInfo: the description of the value.
func newValueFieldInfo(name string, valueType reflect.Type, base int) *FieldInfo {
	return &FieldInfo{
		Name:           name,
		Type:           valueType,
		EnumFormatter:  nil,
		owner:          nil,
		Description:    "",
		DefaultValue:   "",
		Template:       "",
		URLParts:       nil,
		Keys:           nil,
		DeprecatedKeys: nil,
		Base:           base,
		index:          0,
		ItemSeparator:  0,
		Required:       false,
		Title:          false,
		Sensitive:      false,
		levelMapped:    false,
		replacedBy:     "",
		replacedValues: nil,
	}
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
			owner:         structType,
			index:         i,
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

		info.levelMapped = fieldDef.Tag.Get(levelMappedTag) == "true"

		if tag, ok := fieldDef.Tag.Lookup(replacedByTag); ok {
			info.replacedBy = strings.ToLower(tag)
		}

		if tag, ok := fieldDef.Tag.Lookup(replacedValuesTag); ok {
			info.replacedValues = parseReplacedValues(tag)
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

// equalFoldSameLength reports whether two strings match in any case and have
// the same byte length.
//
// The length check rejects a look-alike that Unicode case folding matches,
// such as the long s in httpſ against https.
//
// Parameters:
//   - a: the first string.
//   - b: the second string.
//
// Returns:
//   - bool: true when the strings match.
func equalFoldSameLength(a, b string) bool {
	return len(a) == len(b) && strings.EqualFold(a, b)
}

// parseReplacedValues reads a `replacedvalues` tag.
//
// Parameters:
//   - tag: from=to pairs separated by commas, such as "http=yes,https=no".
//
// Returns:
//   - []valueReplacement: the pairs in tag order, each from value in lower
//     case. A tag with a pair that has no equals sign or an empty from value
//     gives an empty, non-nil slice, which accepts no value and which
//     [SyncDeprecatedFields] reports.
func parseReplacedValues(tag string) []valueReplacement {
	pairs := strings.Split(tag, ",")
	values := make([]valueReplacement, 0, len(pairs))

	for _, pair := range pairs {
		from, to, found := strings.Cut(pair, "=")
		if !found || from == "" {
			return []valueReplacement{}
		}

		values = append(values, valueReplacement{from: strings.ToLower(from), to: to})
	}

	return values
}
