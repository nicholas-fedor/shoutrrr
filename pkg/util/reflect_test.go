package util

import (
	"reflect"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestIsUnsignedInt(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		kind reflect.Kind
		want bool
	}{
		{
			name: "returns true for Uint",
			kind: reflect.Uint,
			want: true,
		},
		{
			name: "returns true for Uint8",
			kind: reflect.Uint8,
			want: true,
		},
		{
			name: "returns true for Uint16",
			kind: reflect.Uint16,
			want: true,
		},
		{
			name: "returns true for Uint32",
			kind: reflect.Uint32,
			want: true,
		},
		{
			name: "returns true for Uint64",
			kind: reflect.Uint64,
			want: true,
		},
		{
			name: "returns false for Uintptr",
			kind: reflect.Uintptr,
			want: false,
		},
		{
			name: "returns false for Int",
			kind: reflect.Int,
			want: false,
		},
		{
			name: "returns false for Int8",
			kind: reflect.Int8,
			want: false,
		},
		{
			name: "returns false for String",
			kind: reflect.String,
			want: false,
		},
		{
			name: "returns false for Float64",
			kind: reflect.Float64,
			want: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			got := IsUnsignedInt(tt.kind)
			assert.Equal(t, tt.want, got, "IsUnsignedInt(%v) should return %v", tt.kind, tt.want)
		})
	}
}

func TestIsSignedInt(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		kind reflect.Kind
		want bool
	}{
		{
			name: "returns true for Int",
			kind: reflect.Int,
			want: true,
		},
		{
			name: "returns true for Int8",
			kind: reflect.Int8,
			want: true,
		},
		{
			name: "returns true for Int16",
			kind: reflect.Int16,
			want: true,
		},
		{
			name: "returns true for Int32",
			kind: reflect.Int32,
			want: true,
		},
		{
			name: "returns true for Int64",
			kind: reflect.Int64,
			want: true,
		},
		{
			name: "returns false for Uint",
			kind: reflect.Uint,
			want: false,
		},
		{
			name: "returns false for Uint8",
			kind: reflect.Uint8,
			want: false,
		},
		{
			name: "returns false for String",
			kind: reflect.String,
			want: false,
		},
		{
			name: "returns false for Float64",
			kind: reflect.Float64,
			want: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			got := IsSignedInt(tt.kind)
			assert.Equal(t, tt.want, got, "IsSignedInt(%v) should return %v", tt.kind, tt.want)
		})
	}
}

func TestIsCollection(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		kind reflect.Kind
		want bool
	}{
		{
			name: "returns true for Slice",
			kind: reflect.Slice,
			want: true,
		},
		{
			name: "returns true for Array",
			kind: reflect.Array,
			want: true,
		},
		{
			name: "returns false for Map",
			kind: reflect.Map,
			want: false,
		},
		{
			name: "returns false for String",
			kind: reflect.String,
			want: false,
		},
		{
			name: "returns false for Int",
			kind: reflect.Int,
			want: false,
		},
		{
			name: "returns false for Struct",
			kind: reflect.Struct,
			want: false,
		},
		{
			name: "returns false for Chan",
			kind: reflect.Chan,
			want: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			got := IsCollection(tt.kind)
			assert.Equal(t, tt.want, got, "IsCollection(%v) should return %v", tt.kind, tt.want)
		})
	}
}

func TestIsNumeric(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		kind reflect.Kind
		want bool
	}{
		{
			name: "returns true for Int",
			kind: reflect.Int,
			want: true,
		},
		{
			name: "returns true for Int8",
			kind: reflect.Int8,
			want: true,
		},
		{
			name: "returns true for Int16",
			kind: reflect.Int16,
			want: true,
		},
		{
			name: "returns true for Int32",
			kind: reflect.Int32,
			want: true,
		},
		{
			name: "returns true for Int64",
			kind: reflect.Int64,
			want: true,
		},
		{
			name: "returns true for Uint",
			kind: reflect.Uint,
			want: true,
		},
		{
			name: "returns true for Uint8",
			kind: reflect.Uint8,
			want: true,
		},
		{
			name: "returns true for Float32",
			kind: reflect.Float32,
			want: true,
		},
		{
			name: "returns true for Float64",
			kind: reflect.Float64,
			want: true,
		},
		{
			name: "returns true for Complex64",
			kind: reflect.Complex64,
			want: true,
		},
		{
			name: "returns true for Complex128",
			kind: reflect.Complex128,
			want: true,
		},
		{
			name: "returns false for String",
			kind: reflect.String,
			want: false,
		},
		{
			name: "returns false for Bool",
			kind: reflect.Bool,
			want: false,
		},
		{
			name: "returns false for Slice",
			kind: reflect.Slice,
			want: false,
		},
		{
			name: "returns false for Pointer",
			kind: reflect.Pointer,
			want: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			got := IsNumeric(tt.kind)
			assert.Equal(t, tt.want, got, "IsNumeric(%v) should return %v", tt.kind, tt.want)
		})
	}
}
