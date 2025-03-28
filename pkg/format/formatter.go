package format

import (
	"errors"
	"fmt"
	"math"
	"reflect"
	"strconv"
	"strings"

	"github.com/nicholas-fedor/shoutrrr/pkg/types"
	"github.com/nicholas-fedor/shoutrrr/pkg/util"
)

// Constants for map parsing and type sizes.
const (
	KeyValuePairSize = 2  // Number of elements in a key:value pair
	Int32BitSize     = 32 // Bit size for 32-bit integers
	Int64BitSize     = 64 // Bit size for 64-bit integers
)

// Errors defined as static variables for better error handling.
var (
	ErrInvalidEnumValue      = errors.New("not a valid enum value")
	ErrInvalidBoolValue      = errors.New("accepted values are 1, true, yes or 0, false, no")
	ErrUnsupportedFieldKey   = errors.New("field key format is not supported")
	ErrInvalidFieldValue     = errors.New("invalid field value format")
	ErrUnsupportedField      = errors.New("field format is not supported")
	ErrInvalidFieldCount     = errors.New("invalid field value count")
	ErrInvalidFieldKind      = errors.New("invalid field kind")
	ErrUnsupportedMapValue   = errors.New("map value format is not supported")
	ErrInvalidFieldValueData = errors.New("invalid field value")
	ErrFailedToSetEnumValue  = errors.New("failed to set enum value")
	ErrUnexpectedUintKind    = errors.New("unexpected uint kind")
	ErrUnexpectedIntKind     = errors.New("unexpected int kind")
	ErrParseIntFailed        = errors.New("failed to parse integer")
	ErrParseUintFailed       = errors.New("failed to parse unsigned integer")
)

// GetServiceConfig extracts the inner config from a service.
func GetServiceConfig(service types.Service) types.ServiceConfig {
	serviceValue := reflect.Indirect(reflect.ValueOf(service))

	configField, ok := serviceValue.Type().FieldByName("Config")
	if !ok {
		panic("service does not have a Config field")
	}

	configRef := serviceValue.FieldByIndex(configField.Index)
	if configRef.IsNil() {
		configType := configField.Type
		if configType.Kind() == reflect.Ptr {
			configType = configType.Elem()
		}

		newConfig := reflect.New(configType).Interface()
		if config, ok := newConfig.(types.ServiceConfig); ok {
			return config
		}

		panic("failed to create new config instance")
	}

	if config, ok := configRef.Interface().(types.ServiceConfig); ok {
		return config
	}

	panic("config reference is not a ServiceConfig")
}

// ColorFormatTree generates a color-highlighted string representation of a node tree.
func ColorFormatTree(rootNode *ContainerNode, withValues bool) string {
	return ConsoleTreeRenderer{WithValues: withValues}.RenderTree(rootNode, "")
}

// GetServiceConfigFormat retrieves type and field information from a service's config.
func GetServiceConfigFormat(service types.Service) *ContainerNode {
	return GetConfigFormat(GetServiceConfig(service))
}

// GetConfigFormat retrieves type and field information from a ServiceConfig.
func GetConfigFormat(config types.ServiceConfig) *ContainerNode {
	return getRootNode(config)
}

// SetConfigField updates a config field with a deserialized value from a string.
func SetConfigField(config reflect.Value, field FieldInfo, inputValue string) (bool, error) {
	configField := config.FieldByName(field.Name)
	if field.EnumFormatter != nil {
		return setEnumField(configField, field, inputValue)
	}

	switch field.Type.Kind() {
	case reflect.String:
		configField.SetString(inputValue)

		return true, nil
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		return setIntField(configField, field, inputValue)
	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
		return setUintField(configField, field, inputValue)
	case reflect.Bool:
		return setBoolField(configField, inputValue)
	case reflect.Map:
		return setMapField(configField, field, inputValue)
	case reflect.Struct:
		return setStructField(configField, field, inputValue)
	case reflect.Slice, reflect.Array:
		return setSliceOrArrayField(configField, field, inputValue)
	case reflect.Invalid,
		reflect.Uintptr,
		reflect.Float32,
		reflect.Float64,
		reflect.Complex64,
		reflect.Complex128,
		reflect.Chan,
		reflect.Func,
		reflect.Interface,
		reflect.Pointer,
		reflect.UnsafePointer:
		return false, fmt.Errorf("%w: %v", ErrInvalidFieldKind, field.Type.Kind())
	default:
		return false, fmt.Errorf("%w: %v", ErrInvalidFieldKind, field.Type.Kind())
	}
}

// setIntField handles integer field setting.
func setIntField(configField reflect.Value, field FieldInfo, inputValue string) (bool, error) {
	number, base := util.StripNumberPrefix(inputValue)

	value, err := strconv.ParseInt(number, base, field.Type.Bits())
	if err != nil {
		return false, fmt.Errorf("%w: %w", ErrParseIntFailed, err)
	}

	configField.SetInt(value)

	return true, nil
}

// setUintField handles unsigned integer field setting.
func setUintField(configField reflect.Value, field FieldInfo, inputValue string) (bool, error) {
	number, base := util.StripNumberPrefix(inputValue)

	value, err := strconv.ParseUint(number, base, field.Type.Bits())
	if err != nil {
		return false, fmt.Errorf("%w: %w", ErrParseUintFailed, err)
	}

	configField.SetUint(value)

	return true, nil
}

// setBoolField handles boolean field setting.
func setBoolField(configField reflect.Value, inputValue string) (bool, error) {
	value, ok := ParseBool(inputValue, false)
	if !ok {
		return false, ErrInvalidBoolValue
	}

	configField.SetBool(value)

	return true, nil
}

// setMapField handles map field setting.
func setMapField(configField reflect.Value, field FieldInfo, inputValue string) (bool, error) {
	if field.Type.Key().Kind() != reflect.String {
		return false, ErrUnsupportedFieldKey
	}

	mapValue := reflect.MakeMap(field.Type)

	pairs := strings.Split(inputValue, ",")
	for _, pair := range pairs {
		elems := strings.Split(pair, ":")
		if len(elems) != KeyValuePairSize {
			return false, ErrInvalidFieldValue
		}

		key, valueRaw := elems[0], elems[1]

		value, err := getMapValue(field.Type.Elem(), valueRaw)
		if err != nil {
			return false, err
		}

		mapValue.SetMapIndex(reflect.ValueOf(key), value)
	}

	configField.Set(mapValue)

	return true, nil
}

// setStructField handles struct field setting.
func setStructField(configField reflect.Value, field FieldInfo, inputValue string) (bool, error) {
	valuePtr, err := GetConfigPropFromString(field.Type, inputValue)
	if err != nil {
		return false, err
	}

	configField.Set(valuePtr.Elem())

	return true, nil
}

// setSliceOrArrayField handles slice or array field setting.
func setSliceOrArrayField(
	configField reflect.Value,
	field FieldInfo,
	inputValue string,
) (bool, error) {
	elemType := field.Type.Elem()
	elemKind := elemType.Kind()

	if elemKind == reflect.Ptr {
		elemKind = elemType.Elem().Kind()
	}

	if elemKind != reflect.Struct && elemKind != reflect.String {
		return false, ErrUnsupportedField
	}

	values := strings.Split(inputValue, string(field.ItemSeparator))
	if field.Type.Kind() == reflect.Array && len(values) != field.Type.Len() {
		return false, fmt.Errorf("%w: needs to be %d", ErrInvalidFieldCount, field.Type.Len())
	}

	return setSliceOrArrayValues(configField, field, elemType, values)
}

// setSliceOrArrayValues sets the actual values for slice or array fields.
func setSliceOrArrayValues(
	configField reflect.Value,
	field FieldInfo,
	elemType reflect.Type,
	values []string,
) (bool, error) {
	isPtrSlice := elemType.Kind() == reflect.Ptr
	baseType := elemType

	if isPtrSlice {
		baseType = elemType.Elem()
	}

	if baseType.Kind() == reflect.Struct {
		slice := reflect.MakeSlice(reflect.SliceOf(elemType), 0, len(values))

		for _, v := range values {
			propPtr, err := GetConfigPropFromString(baseType, v)
			if err != nil {
				return false, err
			}

			if isPtrSlice {
				slice = reflect.Append(slice, propPtr)
			} else {
				slice = reflect.Append(slice, propPtr.Elem())
			}
		}

		configField.Set(slice)

		return true, nil
	}

	// Handle string slice/array
	value := reflect.ValueOf(values)

	if field.Type.Kind() == reflect.Array {
		arr := reflect.Indirect(reflect.New(field.Type))
		reflect.Copy(arr, value)
		configField.Set(arr)
	} else {
		configField.Set(value)
	}

	return true, nil
}

// setEnumField handles enum field setting.
func setEnumField(configField reflect.Value, field FieldInfo, inputValue string) (bool, error) {
	value := field.EnumFormatter.Parse(inputValue)
	if value == EnumInvalid {
		return false, fmt.Errorf(
			"%w: accepted values are %v",
			ErrInvalidEnumValue,
			field.EnumFormatter.Names(),
		)
	}

	configField.SetInt(int64(value))

	if actual := int(configField.Int()); actual != value {
		return false, fmt.Errorf(
			"%w: expected %d, got %d (canSet: %v)",
			ErrFailedToSetEnumValue,
			value,
			actual,
			configField.CanSet(),
		)
	}

	return true, nil
}

// getMapValue converts a raw string to a map value based on type.
func getMapValue(valueType reflect.Type, valueRaw string) (reflect.Value, error) {
	switch valueType.Kind() {
	case reflect.String:
		return reflect.ValueOf(valueRaw), nil
	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
		return getMapUintValue(valueRaw, valueType)
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		return getMapIntValue(valueRaw, valueType)
	case reflect.Invalid,
		reflect.Bool,
		reflect.Uintptr,
		reflect.Float32,
		reflect.Float64,
		reflect.Complex64,
		reflect.Complex128,
		reflect.Array,
		reflect.Chan,
		reflect.Func,
		reflect.Interface,
		reflect.Map,
		reflect.Pointer,
		reflect.Slice,
		reflect.Struct,
		reflect.UnsafePointer:
		return reflect.Value{}, ErrUnsupportedMapValue
	default:
		return reflect.Value{}, ErrUnsupportedMapValue
	}
}

// getMapUintValue converts a string to an unsigned integer map value.
func getMapUintValue(valueRaw string, valueType reflect.Type) (reflect.Value, error) {
	number, base := util.StripNumberPrefix(valueRaw)

	numValue, err := strconv.ParseUint(number, base, valueType.Bits())
	if err != nil {
		return reflect.Value{}, fmt.Errorf("%w: %w", ErrParseUintFailed, err)
	}

	switch valueType.Kind() {
	case reflect.Uint:
		return reflect.ValueOf(uint(numValue)), nil
	case reflect.Uint8:
		if numValue > math.MaxUint8 {
			return reflect.Value{}, fmt.Errorf(
				"%w: value %d exceeds uint8 range",
				ErrParseUintFailed,
				numValue,
			)
		}

		return reflect.ValueOf(uint8(numValue)), nil
	case reflect.Uint16:
		if numValue > math.MaxUint16 {
			return reflect.Value{}, fmt.Errorf(
				"%w: value %d exceeds uint16 range",
				ErrParseUintFailed,
				numValue,
			)
		}

		return reflect.ValueOf(uint16(numValue)), nil
	case reflect.Uint32:
		if numValue > math.MaxUint32 {
			return reflect.Value{}, fmt.Errorf(
				"%w: value %d exceeds uint32 range",
				ErrParseUintFailed,
				numValue,
			)
		}

		return reflect.ValueOf(uint32(numValue)), nil
	case reflect.Uint64:
		return reflect.ValueOf(numValue), nil
	case reflect.Invalid,
		reflect.Bool,
		reflect.Int,
		reflect.Int8,
		reflect.Int16,
		reflect.Int32,
		reflect.Int64,
		reflect.Uintptr,
		reflect.Float32,
		reflect.Float64,
		reflect.Complex64,
		reflect.Complex128,
		reflect.Array,
		reflect.Chan,
		reflect.Func,
		reflect.Interface,
		reflect.Map,
		reflect.Pointer,
		reflect.Slice,
		reflect.String,
		reflect.Struct,
		reflect.UnsafePointer:
		return reflect.Value{}, ErrUnexpectedUintKind
	default:
		return reflect.Value{}, ErrUnexpectedUintKind
	}
}

// getMapIntValue converts a string to a signed integer map value.
func getMapIntValue(valueRaw string, valueType reflect.Type) (reflect.Value, error) {
	number, base := util.StripNumberPrefix(valueRaw)

	numValue, err := strconv.ParseInt(number, base, valueType.Bits())
	if err != nil {
		return reflect.Value{}, fmt.Errorf("%w: %w", ErrParseIntFailed, err)
	}

	switch valueType.Kind() {
	case reflect.Int:
		bits := valueType.Bits()
		if bits == Int32BitSize {
			if numValue < math.MinInt32 || numValue > math.MaxInt32 {
				return reflect.Value{}, fmt.Errorf(
					"%w: value %d exceeds int%d range",
					ErrParseIntFailed,
					numValue,
					bits,
				)
			}
		}

		return reflect.ValueOf(int(numValue)), nil
	case reflect.Int8:
		if numValue < math.MinInt8 || numValue > math.MaxInt8 {
			return reflect.Value{}, fmt.Errorf(
				"%w: value %d exceeds int8 range",
				ErrParseIntFailed,
				numValue,
			)
		}

		return reflect.ValueOf(int8(numValue)), nil
	case reflect.Int16:
		if numValue < math.MinInt16 || numValue > math.MaxInt16 {
			return reflect.Value{}, fmt.Errorf(
				"%w: value %d exceeds int16 range",
				ErrParseIntFailed,
				numValue,
			)
		}

		return reflect.ValueOf(int16(numValue)), nil
	case reflect.Int32:
		if numValue < math.MinInt32 || numValue > math.MaxInt32 {
			return reflect.Value{}, fmt.Errorf(
				"%w: value %d exceeds int32 range",
				ErrParseIntFailed,
				numValue,
			)
		}

		return reflect.ValueOf(int32(numValue)), nil
	case reflect.Int64:
		return reflect.ValueOf(numValue), nil
	case reflect.Invalid,
		reflect.Bool,
		reflect.Uint,
		reflect.Uint8,
		reflect.Uint16,
		reflect.Uint32,
		reflect.Uint64,
		reflect.Uintptr,
		reflect.Float32,
		reflect.Float64,
		reflect.Complex64,
		reflect.Complex128,
		reflect.Array,
		reflect.Chan,
		reflect.Func,
		reflect.Interface,
		reflect.Map,
		reflect.Pointer,
		reflect.Slice,
		reflect.String,
		reflect.Struct,
		reflect.UnsafePointer:
		return reflect.Value{}, ErrUnexpectedIntKind
	default:
		return reflect.Value{}, ErrUnexpectedIntKind
	}
}

// GetConfigFieldString converts a config field value to its string representation.
func GetConfigFieldString(config reflect.Value, field FieldInfo) (string, error) {
	configField := config.FieldByName(field.Name)
	if field.IsEnum() {
		return field.EnumFormatter.Print(int(configField.Int())), nil
	}

	strVal, token := getValueNodeValue(configField, &field)
	if token == ErrorToken {
		return "", ErrInvalidFieldValueData
	}

	return strVal, nil
}
