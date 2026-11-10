package format

import (
	"errors"
	"fmt"
	"reflect"

	"github.com/nicholas-fedor/shoutrrr/pkg/types"
)

// ErrNotConfigProp indicates a type that does not implement [types.ConfigProp].
var ErrNotConfigProp = errors.New("struct field cannot be used as a prop")

// GetConfigPropFromString reads a config property from its string form.
//
// The property is a new value of structType, set through its
// [types.ConfigProp] SetFromProp method.
//
// Parameters:
//   - structType: the type of the property, whose pointer implements
//     [types.ConfigProp].
//   - value: the string form of the property.
//
// Returns:
//   - reflect.Value: a pointer to the new property.
//   - error: [ErrNotConfigProp] when the type is not a config prop, or the
//     error from SetFromProp, or nil on success.
func GetConfigPropFromString(structType reflect.Type, value string) (reflect.Value, error) {
	valuePtr := reflect.New(structType)

	configProp, ok := reflect.TypeAssert[types.ConfigProp](valuePtr)
	if !ok {
		return reflect.Value{}, ErrNotConfigProp
	}

	if err := configProp.SetFromProp(value); err != nil {
		return reflect.Value{}, fmt.Errorf("failed to set config prop from string: %w", err)
	}

	return valuePtr, nil
}

// GetConfigPropString writes a config property in its string form.
//
// A property passed by value is copied to a new pointer, so its
// [types.ConfigProp] GetPropValue method can be called.
//
// Parameters:
//   - propPtr: the property, or a pointer to it.
//
// Returns:
//   - string: the string form of the property.
//   - error: [ErrNotConfigProp] when the property is not a config prop, or the
//     error from GetPropValue, or nil on success.
func GetConfigPropString(propPtr reflect.Value) (string, error) {
	if propPtr.Kind() != reflect.Pointer {
		propVal := propPtr
		propPtr = reflect.New(propVal.Type())
		propPtr.Elem().Set(propVal)
	}

	if propPtr.CanInterface() {
		if configProp, ok := reflect.TypeAssert[types.ConfigProp](propPtr); ok {
			s, err := configProp.GetPropValue()
			if err != nil {
				return "", fmt.Errorf("failed to get config prop string: %w", err)
			}

			return s, nil
		}
	}

	return "", ErrNotConfigProp
}
