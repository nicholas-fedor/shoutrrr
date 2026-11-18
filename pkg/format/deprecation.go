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
	// Deprecation names the deprecated field and its replacement.
	Deprecation

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
	// one replacement, the first one that is set wins.
	for i := range pairs {
		if pairs[i].copy(structValue) {
			used = append(used, pairs[i].Deprecation)
		}
	}

	slices.SortFunc(used, func(a, b Deprecation) int { return strings.Compare(a.Name, b.Name) })

	return used, nil
}

// moveDeprecatedFields copies the deprecated fields of a struct into their
// replacements and then clears them.
//
// A later sync in GetURL then has nothing to copy back into a replacement
// changed after the move. A struct with invalid tags is left unchanged.
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
		structValue.Field(pairs[i].deprecated).SetZero()
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
//   - []deprecatedFieldPair: one pair for each field with a `deprecated` tag,
//     in field order. Callers must not change it.
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
	)

	// The loop body is a closure, so it breaks instead of returning. A return
	// in it would move the results to the heap on every call, cache hits too.
	for field := range structType.Fields() {
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
			Name:        field.Name,
			Replacement: name,
			deprecated:  field.Index[0],
			replacement: replacement.Index[0],
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
// Parameters:
//   - structValue: the addressable struct value.
//
// Returns:
//   - bool: whether the deprecated field is set.
func (p *deprecatedFieldPair) copy(structValue reflect.Value) bool {
	deprecated := structValue.Field(p.deprecated)
	if deprecated.IsZero() {
		return false
	}

	if replacement := structValue.Field(p.replacement); replacement.IsZero() {
		replacement.Set(deprecated)
	}

	return true
}
