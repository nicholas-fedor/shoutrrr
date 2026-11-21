package format

import (
	"errors"
	"fmt"
	"reflect"
	"slices"
	"strings"
	"sync"

	"github.com/nicholas-fedor/shoutrrr/pkg/types"
)

// Deprecation names a deprecated config key or field and its replacement.
type Deprecation struct {
	// Name is the deprecated field name, or the deprecated key in lower case.
	Name string
	// Replacement is the primary key or the field name to use instead.
	Replacement string
}

// deprecatedFieldPair holds the field indexes of a deprecated field and its
// replacement.
type deprecatedFieldPair struct {
	// replaced describes the deprecated field when another key replaces its
	// keys, or is nil for a field with a `deprecated` tag.
	replaced *FieldInfo
	// replacementField describes the replacement of a replaced field, or is
	// nil for a field with a `deprecated` tag.
	replacementField *FieldInfo
	// mappedValues holds the replacement's value for each mapped value of a
	// replaced field, parsed once, when the replacement holds a scalar. It is
	// nil otherwise, and the mapped value is parsed on each copy instead.
	mappedValues map[string]reflect.Value
	// deprecation names the deprecated field and its replacement.
	deprecation Deprecation
	// deprecated is the index of the deprecated field in its struct.
	deprecated int
	// replacement is the index of the field that replaces it.
	replacement int
}

// Struct tags that mark deprecated config keys and fields.
const (
	// deprecatedKeysTag lists the deprecated keys of a keyed field, separated
	// by commas.
	deprecatedKeysTag = "deprecatedkeys"
	// deprecatedFieldTag marks a deprecated field and names the field that
	// replaces it.
	deprecatedFieldTag = "deprecated"
)

// ErrInvalidDeprecatedField reports a `deprecated` tag that names no usable
// replacement, or a config that is not a pointer to a struct. A usable
// replacement is a direct, exported field of the same type that is not
// deprecated itself.
//
// It also reports these tags:
//   - A `replacedby` tag on a field that is not a keyed string field.
//   - A `replacedby` tag that names no primary key of a field whose own keys
//     are not replaced.
//   - A `replacedvalues` tag that is malformed.
//   - A `replacedvalues` tag that maps a value onto one the replacement cannot
//     hold.
var ErrInvalidDeprecatedField = errors.New("invalid deprecated field")

// deprecatedFieldPairsByType maps each struct type with valid deprecated tags
// that has been synced or masked to its []deprecatedFieldPair, so its tags are
// read once.
var deprecatedFieldPairsByType sync.Map

// SyncDeprecatedFields copies each deprecated field of config into the field
// that replaces it, so a service reads only the replacement.
//
// A deprecated field carries a `deprecated` tag that names its replacement.
// The replacement must be a direct, exported field of the same type, and it
// cannot be deprecated itself. The value moves only when the deprecated field
// is set and the replacement holds its zero value. A replacement with a
// non-zero value wins. A replacement set to its zero value, such as a bool set
// to false by the URL or the caller, looks unset, so a set deprecated field
// replaces it. The deprecated field keeps the value the caller gave it.
//
// A string field whose keys another key replaces, through a `replacedby` tag,
// is deprecated too. Its keys never set it, so it is set only when it holds
// neither its zero value nor its default in any case. Its value then moves
// into the field of the replacement key, mapped as its `replacedvalues` tag
// says, by the same rule.
//
// A service calls it on the config it sends with, before applying params.
// GetURL calls it on a copy of the config, so the URL carries the replacement
// key.
//
// Parameters:
//   - config: a pointer to the config struct.
//
// Returns:
//   - []Deprecation: the deprecated fields that are set, sorted by name.
//   - error: [ErrInvalidDeprecatedField] when config is not a pointer to a
//     struct or a tag names no usable replacement. No field is changed then.
//     An error wrapping [ErrInvalidValueForType] for the first replaced
//     field whose value its mapping does not list, or whose value as written
//     its replacement cannot hold. That field is not copied, and every other
//     field still is.
func SyncDeprecatedFields(config types.ServiceConfig) ([]Deprecation, error) {
	// A nil pointer has no element, so its struct value stays invalid.
	var structValue reflect.Value
	if value := reflect.ValueOf(config); value.Kind() == reflect.Pointer {
		structValue = value.Elem()
	}

	if structValue.Kind() != reflect.Struct {
		return nil, fmt.Errorf("%w: %T is not a pointer to a struct", ErrInvalidDeprecatedField, config)
	}

	pairs, err := deprecatedFieldPairs(structValue.Type())
	if err != nil {
		return nil, err
	}

	used := []Deprecation{}

	// The pairs are copied in field order, so when two deprecated fields name
	// one replacement, the first one that is set wins. A value without a
	// mapping stops no other pair, so a GetURL that ignores the error still
	// writes the other replacements.
	var valueErr error

	for i := range pairs {
		set, err := pairs[i].copy(structValue)
		if err != nil {
			if valueErr == nil {
				valueErr = err
			}

			continue
		}

		if set {
			used = append(used, pairs[i].deprecation)
		}
	}

	if valueErr != nil {
		return nil, valueErr
	}

	slices.SortFunc(used, func(a, b Deprecation) int { return strings.Compare(a.Name, b.Name) })

	return used, nil
}

// moveDeprecatedFields copies the deprecated fields of a struct into their
// replacements and then clears them.
//
// A later sync in GetURL then has nothing to copy back into a replacement
// changed after the move. A struct with invalid tags is left unchanged, and so
// is a replaced field whose value its mapping does not list.
//
// Parameters:
//   - structValue: the addressable struct value.
func moveDeprecatedFields(structValue reflect.Value) {
	pairs, err := deprecatedFieldPairs(structValue.Type())
	if err != nil {
		return
	}

	for i := range pairs {
		if _, err := pairs[i].copy(structValue); err == nil {
			structValue.Field(pairs[i].deprecated).SetZero()
		}
	}
}

// deprecatedFieldPairs finds the deprecated fields of a struct type and their
// replacements.
//
// The tags of a type are read on its first call, and its pairs are kept for
// later calls. A type whose tags are invalid is read again on each call.
// Concurrent calls are safe.
//
// Parameters:
//   - structType: the config struct type.
//
// Returns:
//   - []deprecatedFieldPair: one pair for each field with a `deprecated` or
//     `replacedby` tag, in field order. Callers must not change it.
//   - error: [ErrInvalidDeprecatedField] when a tag names no usable
//     replacement. A promoted field is rejected, because an embedded pointer
//     on its path can be nil. A deprecated field is rejected, because chains
//     would depend on field order.
func deprecatedFieldPairs(structType reflect.Type) ([]deprecatedFieldPair, error) {
	cached, _ := deprecatedFieldPairsByType.Load(structType)
	if pairs, found := cached.([]deprecatedFieldPair); found {
		return pairs, nil
	}

	var (
		pairs   []deprecatedFieldPair
		invalid error
		fields  []FieldInfo
	)

	// The loop body is a closure, so it breaks instead of returning. A return
	// in it would move the results to the heap on every call, cache hits too.
	for field := range structType.Fields() {
		if _, replaced := field.Tag.Lookup(replacedByTag); replaced {
			if fields == nil {
				fields = getStructFieldInfo(structType, enumsOf(structType))
			}

			var pair deprecatedFieldPair

			pair, invalid = replacedFieldPair(structType, fields, field.Index[0])
			if invalid != nil {
				break
			}

			pairs = append(pairs, pair)

			continue
		}

		name, tagged := field.Tag.Lookup(deprecatedFieldTag)
		if !tagged {
			continue
		}

		replacement, found := structType.FieldByName(name)
		_, chained := replacement.Tag.Lookup(deprecatedFieldTag)

		if !found || len(replacement.Index) != 1 || chained || name == field.Name ||
			!field.IsExported() || !replacement.IsExported() || replacement.Type != field.Type {
			invalid = fmt.Errorf(
				"%w: %s.%s names %q as its replacement",
				ErrInvalidDeprecatedField, structType.Name(), field.Name, name,
			)

			break
		}

		pairs = append(pairs, deprecatedFieldPair{
			deprecation:      Deprecation{Name: field.Name, Replacement: name},
			replaced:         nil,
			replacementField: nil,
			mappedValues:     nil,
			deprecated:       field.Index[0],
			replacement:      replacement.Index[0],
		})
	}

	if invalid != nil {
		return nil, invalid
	}

	// Concurrent first calls store equal pairs, so either one may stay.
	deprecatedFieldPairsByType.Store(structType, pairs)

	return pairs, nil
}

// copy copies the deprecated field into its replacement when the deprecated
// field is set and the replacement holds its zero value.
//
// A replaced field's value is mapped first, and it is set only when it holds
// neither its zero value nor its default in any case.
//
// Parameters:
//   - structValue: the addressable struct value.
//
// Returns:
//   - bool: whether the deprecated field is set.
//   - error: an error wrapping [ErrInvalidValueForType] for a replaced field's
//     value that its mapping does not list, or that the replacement cannot
//     hold. Nothing is copied then.
func (p *deprecatedFieldPair) copy(structValue reflect.Value) (bool, error) {
	deprecated := structValue.Field(p.deprecated)
	if deprecated.IsZero() {
		return false, nil
	}

	replacement := structValue.Field(p.replacement)

	if p.replaced == nil {
		if replacement.IsZero() {
			replacement.Set(deprecated)
		}

		return true, nil
	}

	value := deprecated.String()
	if equalFoldSameLength(value, p.replaced.DefaultValue) {
		return false, nil
	}

	mapped, err := p.replaced.replacedValue(value)
	if err != nil {
		return false, fmt.Errorf("%s: %w", p.deprecation.Name, err)
	}

	if !replacement.IsZero() {
		return true, nil
	}

	if parsed, found := p.mappedValues[mapped]; found {
		replacement.Set(parsed)

		return true, nil
	}

	// A field without a mapping passes its value as written, which the tag
	// check cannot know, so the value is checked here.
	if valid, err := SetConfigField(structValue, p.replacementField, mapped); !valid || err != nil {
		return false, fmt.Errorf(
			"%s: %w: %q for %s", p.deprecation.Name, ErrInvalidValueForType, mapped, p.replacementField.Name,
		)
	}

	return true, nil
}

// replacedFieldPair pairs a field whose keys another key replaces with the
// field of that key.
//
// Parameters:
//   - structType: the config struct type.
//   - fields: the field information of the struct type.
//   - index: the index of the replaced field in the struct.
//
// Returns:
//   - deprecatedFieldPair: the pair.
//   - error: an error wrapping [ErrInvalidDeprecatedField] for a tag that it
//     reports, or nil.
func replacedFieldPair(structType reflect.Type, fields []FieldInfo, index int) (deprecatedFieldPair, error) {
	var replaced, replacement *FieldInfo

	for i := range fields {
		if fields[i].index == index {
			replaced = &fields[i]
		}
	}

	if replaced != nil {
		for i := range fields {
			if fields[i].replaces(replaced) {
				replacement = &fields[i]
			}
		}
	}

	name := structType.Field(index).Name

	// The field information leaves out unexported and deprecated fields.
	switch {
	case replaced == nil || len(replaced.Keys) == 0:
		return deprecatedFieldPair{}, fmt.Errorf("%w: %s.%s is not a keyed config field", ErrInvalidDeprecatedField, structType.Name(), name)
	case replaced.Type.Kind() != reflect.String:
		return deprecatedFieldPair{}, fmt.Errorf("%w: %s.%s is not a string field", ErrInvalidDeprecatedField, structType.Name(), name)
	case replacement == nil:
		return deprecatedFieldPair{}, fmt.Errorf(
			"%w: %s.%s names %q as its replacement key",
			ErrInvalidDeprecatedField, structType.Name(), name, replaced.replacedBy,
		)
	default:
	}

	mappedValues, err := parseReplacedValuesInto(structType, replaced, replacement)
	if err != nil {
		return deprecatedFieldPair{}, err
	}

	return deprecatedFieldPair{
		deprecation:      Deprecation{Name: name, Replacement: replacement.Name},
		replaced:         replaced,
		replacementField: replacement,
		mappedValues:     mappedValues,
		deprecated:       index,
		replacement:      replacement.index,
	}, nil
}

// parseReplacedValuesInto checks a replaced field's value mapping and parses
// each mapped value as the replacement's type.
//
// Parameters:
//   - structType: the config struct type.
//   - replaced: the replaced field.
//   - replacement: the field of the replacement key.
//
// Returns:
//   - map[string]reflect.Value: the parsed value of each mapped value, or nil
//     for a field without a mapping or a replacement that holds a list, map,
//     or other reference, whose values no two configs may share.
//   - error: an error wrapping [ErrInvalidDeprecatedField] for a malformed
//     `replacedvalues` tag or a mapped value that does not parse into the
//     replacement, or nil.
func parseReplacedValuesInto(structType reflect.Type, replaced, replacement *FieldInfo) (map[string]reflect.Value, error) {
	// A malformed tag leaves an empty mapping, and a missing tag none.
	if replaced.replacedValues != nil && len(replaced.replacedValues) == 0 {
		return nil, fmt.Errorf(
			"%w: %s.%s has a replacedvalues tag that is not from=to pairs",
			ErrInvalidDeprecatedField, structType.Name(), replaced.Name,
		)
	}

	var parsed map[string]reflect.Value

	for _, value := range replaced.replacedValues {
		scratch := reflect.New(structType).Elem()
		if valid, err := SetConfigField(scratch, replacement, value.to); !valid || err != nil {
			return nil, fmt.Errorf(
				"%w: %s.%s maps %q onto %q, which %s cannot hold",
				ErrInvalidDeprecatedField, structType.Name(), replaced.Name, value.from, value.to, replacement.Name,
			)
		}

		if isScalarKind(replacement.Type.Kind()) {
			if parsed == nil {
				parsed = make(map[string]reflect.Value, len(replaced.replacedValues))
			}

			parsed[value.to] = scratch.Field(replacement.index)
		}
	}

	return parsed, nil
}

// enumsOf returns the enum formatters of a config struct type.
//
// Parameters:
//   - structType: the config struct type.
//
// Returns:
//   - map[string]types.EnumFormatter: the formatters of a new config of the
//     type, or nil when a pointer to it is not a [types.Enummer].
func enumsOf(structType reflect.Type) map[string]types.EnumFormatter {
	if enummer, isEnummer := reflect.TypeAssert[types.Enummer](reflect.New(structType)); isEnummer {
		return enummer.Enums()
	}

	return nil
}
