package util

import (
	"reflect"
)

// IsUnsignedInt reports whether kind is an unsigned integer type.
//
// Parameters:
//   - kind: The reflect.Kind to check.
//
// Returns:
//   - true if kind is Uint, Uint8, Uint16, Uint32, or Uint64; false otherwise.
func IsUnsignedInt(kind reflect.Kind) bool {
	return kind >= reflect.Uint && kind <= reflect.Uint64
}

// IsSignedInt reports whether kind is a signed integer type.
//
// Parameters:
//   - kind: The reflect.Kind to check.
//
// Returns:
//   - true if kind is Int, Int8, Int16, Int32, or Int64; false otherwise.
func IsSignedInt(kind reflect.Kind) bool {
	return kind >= reflect.Int && kind <= reflect.Int64
}

// IsCollection reports whether kind is a slice or array type.
//
// Parameters:
//   - kind: The reflect.Kind to check.
//
// Returns:
//   - true if kind is Slice or Array; false otherwise.
func IsCollection(kind reflect.Kind) bool {
	return kind == reflect.Slice || kind == reflect.Array
}

// IsNumeric reports whether kind is a numeric type.
//
// Parameters:
//   - kind: The reflect.Kind to check.
//
// Returns:
//   - true if kind is any integer (signed or unsigned), float, or complex type; false otherwise.
func IsNumeric(kind reflect.Kind) bool {
	return kind >= reflect.Int && kind <= reflect.Complex128
}
