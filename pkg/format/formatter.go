package format

import (
	"errors"
	"fmt"
	"math"
	"reflect"
	"strconv"
	"strings"
	"time"

	"github.com/nicholas-fedor/shoutrrr/pkg/types"
	"github.com/nicholas-fedor/shoutrrr/pkg/util"
)

// Constants for map parsing and type sizes.
const (
	KeyValuePairSize = 2  // Number of elements in a key:value pair
	Int32BitSize     = 32 // Bit size for 32-bit integers
	Int64BitSize     = 64 // Bit size for 64-bit integers
)

// durationType is the type of a time.Duration field, which is read and written
// as a duration rather than as a number.
var durationType = reflect.TypeFor[time.Duration]()

// Errors returned when a config field is set from its text.
var (
	// ErrInvalidEnumValue indicates a value that names no value of its enum.
	ErrInvalidEnumValue = errors.New("not a valid enum value")
	// ErrInvalidBoolValue indicates a value that [ParseBool] does not read.
	ErrInvalidBoolValue = errors.New("accepted values are 1, true, yes or 0, false, no")
	// ErrUnsupportedFieldKey indicates a map field whose keys are not strings.
	ErrUnsupportedFieldKey = errors.New("field key format is not supported")
	// ErrInvalidFieldValue indicates a map item without the colon between its
	// key and value.
	ErrInvalidFieldValue = errors.New("invalid field value format")
	// ErrUnsupportedField indicates a slice or array field whose items are
	// neither strings nor structs.
	ErrUnsupportedField = errors.New("field format is not supported")
	// ErrInvalidFieldCount indicates an array field given the wrong number of
	// items.
	ErrInvalidFieldCount = errors.New("invalid field value count")
	// ErrInvalidFieldKind indicates a field of a kind that cannot be set from
	// text, such as a float or a pointer.
	ErrInvalidFieldKind = errors.New("invalid field kind")
	// ErrUnsupportedMapValue indicates a map field whose values are neither
	// strings nor integers.
	ErrUnsupportedMapValue = errors.New("map value format is not supported")
	// ErrInvalidFieldValueData indicates a field value that cannot be written
	// as text.
	ErrInvalidFieldValueData = errors.New("invalid field value")
	// ErrFailedToSetEnumValue indicates an enum field that did not hold the
	// value it was set to.
	ErrFailedToSetEnumValue = errors.New("failed to set enum value")
	// ErrUnexpectedUintKind indicates an unsigned integer map value of a kind
	// the reader does not handle.
	ErrUnexpectedUintKind = errors.New("unexpected uint kind")
	// ErrUnexpectedIntKind indicates a signed integer map value of a kind the
	// reader does not handle.
	ErrUnexpectedIntKind = errors.New("unexpected int kind")
	// ErrParseIntFailed indicates a signed integer that does not parse or does
	// not fit its type.
	ErrParseIntFailed = errors.New("failed to parse integer")
	// ErrParseUintFailed indicates an unsigned integer that does not parse or
	// does not fit its type.
	ErrParseUintFailed = errors.New("failed to parse unsigned integer")
	// ErrParseDurationFailed indicates a duration that [time.ParseDuration]
	// does not read.
	ErrParseDurationFailed = errors.New("failed to parse duration")
)

// GetServiceConfig extracts the inner config from a service.
//
// The service must have a Config field. When it is nil, a new zero config of
// its type is returned. The function panics for a service without a Config
// field or with one that is not a [types.ServiceConfig].
//
// Parameters:
//   - service: the service, usually a pointer to its struct.
//
// Returns:
//   - types.ServiceConfig: the service's config, or a zero config.
func GetServiceConfig(service types.Service) types.ServiceConfig {
	serviceValue := reflect.Indirect(reflect.ValueOf(service))

	configField, ok := serviceValue.Type().FieldByName("Config")
	if !ok {
		panic("service does not have a Config field")
	}

	configRef := serviceValue.FieldByIndex(configField.Index)
	if configRef.IsNil() {
		configType := configField.Type
		if configType.Kind() == reflect.Pointer {
			configType = configType.Elem()
		}

		newConfig := reflect.New(configType).Interface()
		if config, ok := newConfig.(types.ServiceConfig); ok {
			return config
		}

		panic("failed to create new config instance")
	}

	if config, ok := reflect.TypeAssert[types.ServiceConfig](configRef); ok {
		return config
	}

	panic("config reference is not a ServiceConfig")
}

// ColorFormatTree generates a color-highlighted string representation of a node tree.
//
// Parameters:
//   - rootNode: the tree to render.
//   - withValues: whether to render each field's value instead of its type.
//
// Returns:
//   - string: the console text, with sensitive values shown.
func ColorFormatTree(rootNode *ContainerNode, withValues bool) string {
	return ConsoleTreeRenderer{WithValues: withValues, MaskSensitive: false}.RenderTree(rootNode, "")
}

// GetServiceConfigFormat retrieves type and field information from a service's config.
//
// Parameters:
//   - service: the service, as [GetServiceConfig] takes it.
//
// Returns:
//   - *ContainerNode: the config's node tree.
func GetServiceConfigFormat(service types.Service) *ContainerNode {
	return GetConfigFormat(GetServiceConfig(service))
}

// GetConfigFormat retrieves type and field information from a ServiceConfig.
//
// Parameters:
//   - config: the config.
//
// Returns:
//   - *ContainerNode: the config's node tree, with a node for each listed
//     field.
func GetConfigFormat(config types.ServiceConfig) *ContainerNode {
	return getRootNode(config)
}

// SetConfigField updates a config field with a deserialized value from a string.
//
// Enum, duration, string, integer, bool, map, struct, slice, and array fields
// can be set. Fields of any other kind fail.
//
// Parameters:
//   - config: the config struct value, which must be settable.
//   - field: the field to set.
//   - inputValue: the field's value as text.
//
// Returns:
//   - bool: true when the field was set.
//   - error: an error wrapping [ErrInvalidFieldKind] for a field of a kind
//     that cannot be set, or the value's parse error, or nil.
func SetConfigField(config reflect.Value, field *FieldInfo, inputValue string) (bool, error) {
	configField := config.FieldByName(field.Name)
	if field.EnumFormatter != nil {
		return setEnumField(configField, field, inputValue)
	}

	if field.Type == durationType {
		return setDurationField(configField, inputValue)
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

// setDurationField sets a time.Duration field from its text.
//
// Parameters:
//   - configField: the field to set.
//   - inputValue: the duration, as [time.ParseDuration] reads it.
//
// Returns:
//   - bool: true when the field was set.
//   - error: an error wrapping [ErrParseDurationFailed] when the duration does
//     not parse.
func setDurationField(configField reflect.Value, inputValue string) (bool, error) {
	duration, err := time.ParseDuration(inputValue)
	if err != nil {
		return false, fmt.Errorf("%w: %w", ErrParseDurationFailed, err)
	}

	configField.SetInt(int64(duration))

	return true, nil
}

// setIntField sets an integer field from its string.
//
// A 0x or # prefix reads the digits in base 16. Without a prefix, the digits
// are read in the field's base.
//
// Parameters:
//   - configField: the field to set.
//   - field: the field's info, with its type and base.
//   - inputValue: the number to set.
//
// Returns:
//   - bool: true when the field was set.
//   - error: an error wrapping [ErrParseIntFailed] when the number does not parse.
func setIntField(configField reflect.Value, field *FieldInfo, inputValue string) (bool, error) {
	number, base := util.StripNumberPrefix(inputValue)
	if base == 0 {
		base = field.Base
	}

	value, err := strconv.ParseInt(number, base, field.Type.Bits())
	if err != nil {
		return false, fmt.Errorf("%w: %w", ErrParseIntFailed, err)
	}

	configField.SetInt(value)

	return true, nil
}

// setUintField sets an unsigned integer field from its string.
//
// A 0x or # prefix reads the digits in base 16. Without a prefix, the digits
// are read in the field's base.
//
// Parameters:
//   - configField: the field to set.
//   - field: the field's info, with its type and base.
//   - inputValue: the number to set.
//
// Returns:
//   - bool: true when the field was set.
//   - error: an error wrapping [ErrParseUintFailed] when the number does not parse.
func setUintField(configField reflect.Value, field *FieldInfo, inputValue string) (bool, error) {
	number, base := util.StripNumberPrefix(inputValue)
	if base == 0 {
		base = field.Base
	}

	value, err := strconv.ParseUint(number, base, field.Type.Bits())
	if err != nil {
		return false, fmt.Errorf("%w: %w", ErrParseUintFailed, err)
	}

	configField.SetUint(value)

	return true, nil
}

// setBoolField sets a bool field from its text.
//
// Parameters:
//   - configField: the field to set.
//   - inputValue: the boolean, as [ParseBool] reads it.
//
// Returns:
//   - bool: true when the field was set.
//   - error: [ErrInvalidBoolValue] when the text is not a boolean.
func setBoolField(configField reflect.Value, inputValue string) (bool, error) {
	value, ok := ParseBool(inputValue, false)
	if !ok {
		return false, ErrInvalidBoolValue
	}

	configField.SetBool(value)

	return true, nil
}

// setMapField sets a map field from comma-separated key:value pairs.
//
// Each pair splits at its first colon, so keys cannot contain a colon while
// values can. An empty value clears the map.
//
// Parameters:
//   - configField: the field to set.
//   - field: the field's info, with its map type.
//   - inputValue: the comma-separated key:value pairs.
//
// Returns:
//   - bool: true when the field was set.
//   - error: [ErrUnsupportedFieldKey] for a map without string keys,
//     [ErrInvalidFieldValue] for a pair without a colon, or the value's parse
//     error.
func setMapField(configField reflect.Value, field *FieldInfo, inputValue string) (bool, error) {
	if field.Type.Key().Kind() != reflect.String {
		return false, ErrUnsupportedFieldKey
	}

	if inputValue == "" {
		configField.SetZero()

		return true, nil
	}

	mapValue := reflect.MakeMap(field.Type)

	pairs := strings.SplitSeq(inputValue, ",")
	for pair := range pairs {
		key, valueRaw, found := strings.Cut(pair, ":")
		if !found {
			return false, ErrInvalidFieldValue
		}

		value, err := getMapValue(field.Type.Elem(), valueRaw)
		if err != nil {
			return false, err
		}

		mapValue.SetMapIndex(reflect.ValueOf(key), value)
	}

	configField.Set(mapValue)

	return true, nil
}

// setStructField sets a struct field from its text.
//
// Parameters:
//   - configField: the field to set.
//   - field: the field's info, with its type.
//   - inputValue: the field's value, as its [types.ConfigProp] reads it.
//
// Returns:
//   - bool: true when the field was set.
//   - error: the error from [GetConfigPropFromString], or nil.
func setStructField(configField reflect.Value, field *FieldInfo, inputValue string) (bool, error) {
	valuePtr, err := GetConfigPropFromString(field.Type, inputValue)
	if err != nil {
		return false, err
	}

	configField.Set(valuePtr.Elem())

	return true, nil
}

// setSliceOrArrayField sets a slice or array field from its separated items.
//
// A slice leaves out empty items, such as the one a trailing separator leaves,
// so a value of only empty items clears it. An array still needs one item for
// each element.
//
// Parameters:
//   - configField: the field to set.
//   - field: the field's info, with its type and item separator.
//   - inputValue: the separated items.
//
// Returns:
//   - bool: true when the field was set.
//   - error: [ErrUnsupportedField] for items that are neither strings nor
//     structs, [ErrInvalidFieldCount] for an array given the wrong number of
//     items, or an item's parse error.
func setSliceOrArrayField(
	configField reflect.Value,
	field *FieldInfo,
	inputValue string,
) (bool, error) {
	elemType := field.Type.Elem()
	elemKind := elemType.Kind()

	if elemKind == reflect.Pointer {
		elemKind = elemType.Elem().Kind()
	}

	if elemKind != reflect.Struct && elemKind != reflect.String {
		return false, ErrUnsupportedField
	}

	if inputValue == "" && field.Type.Kind() == reflect.Slice {
		configField.SetZero()

		return true, nil
	}

	values := strings.Split(inputValue, string(field.ItemSeparator))

	if field.Type.Kind() == reflect.Slice {
		values = util.NonEmpty(values)
		if len(values) == 0 {
			configField.SetZero()

			return true, nil
		}
	} else if len(values) != field.Type.Len() {
		return false, fmt.Errorf("%w: needs to be %d", ErrInvalidFieldCount, field.Type.Len())
	}

	return setSliceOrArrayValues(configField, field, elemType, values)
}

// setSliceOrArrayValues sets a slice or array field to its items.
//
// Struct items are read through their [types.ConfigProp]. String items are
// set as they are.
//
// Parameters:
//   - configField: the field to set.
//   - field: the field's info, with its type.
//   - elemType: the type of each item.
//   - values: the items as text.
//
// Returns:
//   - bool: true when the field was set.
//   - error: the error from reading a struct item, or nil.
func setSliceOrArrayValues(
	configField reflect.Value,
	field *FieldInfo,
	elemType reflect.Type,
	values []string,
) (bool, error) {
	isPtrSlice := elemType.Kind() == reflect.Pointer
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

// setEnumField sets an enum field from the name of its value.
//
// Parameters:
//   - configField: the field to set.
//   - field: the field's info, with its enum formatter.
//   - inputValue: the name or alias of the value.
//
// Returns:
//   - bool: true when the field was set.
//   - error: an error wrapping [ErrInvalidEnumValue] that lists the accepted
//     names, or [ErrFailedToSetEnumValue] when the field does not hold the
//     value, or nil.
func setEnumField(configField reflect.Value, field *FieldInfo, inputValue string) (bool, error) {
	value := field.EnumFormatter.Parse(inputValue)
	if value == EnumInvalid {
		return false, invalidEnumValue(field.EnumFormatter)
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
//
// Parameters:
//   - valueType: the map's value type.
//   - valueRaw: the value as text.
//
// Returns:
//   - reflect.Value: the value.
//   - error: [ErrUnsupportedMapValue] for a value type that is neither a
//     string nor an integer, or the number's parse error, or nil.
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
//
// A 0x or # prefix reads the digits in base 16, and digits without a prefix
// are read in base 10.
//
// Parameters:
//   - valueRaw: the number as text.
//   - valueType: the map's unsigned integer value type.
//
// Returns:
//   - reflect.Value: the number in the value type.
//   - error: an error wrapping [ErrParseUintFailed] for a number that does not
//     parse or fit the type, [ErrUnexpectedUintKind] for a type that is not an
//     unsigned integer, or nil.
func getMapUintValue(valueRaw string, valueType reflect.Type) (reflect.Value, error) {
	number, base := util.StripNumberPrefix(valueRaw)

	if base == 0 {
		base = 10 // Default to decimal for maps if no prefix
	}

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
//
// A 0x or # prefix reads the digits in base 16, and digits without a prefix
// are read in base 10.
//
// Parameters:
//   - valueRaw: the number as text.
//   - valueType: the map's signed integer value type.
//
// Returns:
//   - reflect.Value: the number in the value type.
//   - error: an error wrapping [ErrParseIntFailed] for a number that does not
//     parse or fit the type, [ErrUnexpectedIntKind] for a type that is not a
//     signed integer, or nil.
func getMapIntValue(valueRaw string, valueType reflect.Type) (reflect.Value, error) {
	number, base := util.StripNumberPrefix(valueRaw)

	if base == 0 {
		base = 10 // Default to decimal for maps if no prefix
	}

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
//
// Parameters:
//   - config: the config struct value.
//   - field: the field to write.
//
// Returns:
//   - string: the value as text, as the URL and the docs write it.
//   - error: [ErrInvalidFieldValueData] for a value that cannot be written as
//     text, or nil.
func GetConfigFieldString(config reflect.Value, field *FieldInfo) (string, error) {
	configField := config.FieldByName(field.Name)
	if field.IsEnum() {
		return field.EnumFormatter.Print(int(configField.Int())), nil
	}

	strVal, token := getValueNodeValue(configField, field)
	if token == ErrorToken {
		return "", ErrInvalidFieldValueData
	}

	return strVal, nil
}
