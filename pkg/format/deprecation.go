package format

import (
	"errors"
	"fmt"
	"reflect"
	"slices"
	"strings"

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
	Deprecation

	// deprecated is the index of the deprecated field.
	deprecated []int
	// replacement is the index of the field that replaces it.
	replacement []int
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
var ErrInvalidDeprecatedField = errors.New("invalid deprecated field")

// SyncDeprecatedFields copies each deprecated field of config into the field
// that replaces it, so a service reads only the replacement.
//
// A deprecated field carries a `deprecated` tag that names its replacement.
// The replacement must be a direct, exported field of the same type, and it
// cannot be deprecated itself. The value moves only when
// the deprecated field is set and the replacement holds its zero value, so a
// replacement set by the URL or by the caller wins. The deprecated field keeps
// the value the caller gave it.
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
func SyncDeprecatedFields(config types.ServiceConfig) ([]Deprecation, error) {
	value := reflect.ValueOf(config)
	if value.Kind() != reflect.Pointer || value.IsNil() || value.Elem().Kind() != reflect.Struct {
		return nil, fmt.Errorf("%w: %T is not a pointer to a struct", ErrInvalidDeprecatedField, config)
	}

	pairs, err := deprecatedFieldPairs(value.Elem().Type())
	if err != nil {
		return nil, err
	}

	used := []Deprecation{}

	for i := range pairs {
		if pairs[i].copy(value.Elem()) {
			used = append(used, pairs[i].Deprecation)
		}
	}

	slices.SortFunc(used, func(a, b Deprecation) int { return strings.Compare(a.Name, b.Name) })

	return used, nil
}

// moveDeprecatedFields copies the deprecated fields of a struct into their
// replacements and then clears them. A later sync in GetURL then has nothing
// to copy back into a replacement changed after the move. A struct with
// invalid tags is left unchanged.
//
// Parameters:
//   - structValue: the addressable struct value.
func moveDeprecatedFields(structValue reflect.Value) {
	pairs, err := deprecatedFieldPairs(structValue.Type())
	if err != nil {
		return
	}

	for i := range pairs {
		pairs[i].copy(structValue)
		structValue.FieldByIndex(pairs[i].deprecated).SetZero()
	}
}

// deprecatedFieldPairs finds the deprecated fields of a struct type and their
// replacements.
//
// Parameters:
//   - structType: the config struct type.
//
// Returns:
//   - []deprecatedFieldPair: one pair for each field with a `deprecated` tag.
//   - error: [ErrInvalidDeprecatedField] when a tag names no usable
//     replacement. A promoted field is rejected, because an embedded pointer
//     on its path can be nil. A deprecated field is rejected, because chains
//     would depend on field order.
func deprecatedFieldPairs(structType reflect.Type) ([]deprecatedFieldPair, error) {
	var pairs []deprecatedFieldPair

	for field := range structType.Fields() {
		name, tagged := field.Tag.Lookup(deprecatedFieldTag)
		if !tagged {
			continue
		}

		replacement, found := structType.FieldByName(name)
		_, chained := replacement.Tag.Lookup(deprecatedFieldTag)

		if !found || len(replacement.Index) != 1 || chained || name == field.Name ||
			!field.IsExported() || !replacement.IsExported() || replacement.Type != field.Type {
			return nil, fmt.Errorf(
				"%w: %s.%s names %q as its replacement",
				ErrInvalidDeprecatedField, structType.Name(), field.Name, name,
			)
		}

		pairs = append(pairs, deprecatedFieldPair{
			Name:        field.Name,
			Replacement: name,
			deprecated:  field.Index,
			replacement: replacement.Index,
		})
	}

	return pairs, nil
}

// copy copies the deprecated field into its replacement when the deprecated
// field is set and the replacement holds its zero value.
//
// Parameters:
//   - structValue: the addressable struct value.
//
// Returns:
//   - bool: whether the deprecated field is set.
func (p *deprecatedFieldPair) copy(structValue reflect.Value) bool {
	deprecated := structValue.FieldByIndex(p.deprecated)
	if deprecated.IsZero() {
		return false
	}

	if replacement := structValue.FieldByIndex(p.replacement); replacement.IsZero() {
		replacement.Set(deprecated)
	}

	return true
}
