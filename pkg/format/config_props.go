package format

import (
	"errors"
	"fmt"
	"reflect"

	"github.com/nicholas-fedor/shoutrrr/pkg/types"
)

var ErrNotConfigProp = errors.New("struct field cannot be used as a prop")

// GetConfigPropFromString deserializes a config property from a string representation using the ConfigProp interface.
func GetConfigPropFromString(structType reflect.Type, value string) (reflect.Value, error) {
	valuePtr := reflect.New(structType)

	configProp, ok := valuePtr.Interface().(types.ConfigProp)
	if !ok {
		return reflect.Value{}, ErrNotConfigProp
	}

	if err := configProp.SetFromProp(value); err != nil {
		return reflect.Value{}, fmt.Errorf("failed to set config prop from string: %w", err)
	}

	return valuePtr, nil
}

// GetConfigPropString serializes a config property to a string representation using the ConfigProp interface.
func GetConfigPropString(propPtr reflect.Value) (string, error) {
	if propPtr.Kind() != reflect.Ptr {
		propVal := propPtr
		propPtr = reflect.New(propVal.Type())
		propPtr.Elem().Set(propVal)
	}

	if propPtr.CanInterface() {
		if configProp, ok := propPtr.Interface().(types.ConfigProp); ok {
			s, err := configProp.GetPropValue()
			if err != nil {
				return "", fmt.Errorf("failed to get config prop string: %w", err)
			}

			return s, nil
		}
	}

	return "", ErrNotConfigProp
}
