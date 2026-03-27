//go:build js && wasm

package main

import (
	"errors"
	"reflect"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/nicholas-fedor/shoutrrr/pkg/format"
)

// testSetFieldStruct is used by setFieldFromString tests.
type testSetFieldStruct struct {
	Name    string
	Enabled bool
}

func TestClassifyType(t *testing.T) {
	tests := []struct {
		typ      reflect.Type
		isEnum   bool
		expected string
	}{
		{reflect.TypeFor[string](), false, "string"},
		{reflect.TypeFor[bool](), false, "bool"},
		{reflect.TypeFor[int](), false, "int"},
		{reflect.TypeFor[int8](), false, "int"},
		{reflect.TypeFor[int16](), false, "int"},
		{reflect.TypeFor[int32](), false, "int"},
		{reflect.TypeFor[int64](), false, "int"},
		{reflect.TypeFor[uint](), false, "uint"},
		{reflect.TypeFor[uint8](), false, "uint"},
		{reflect.TypeFor[uint16](), false, "uint"},
		{reflect.TypeFor[uint32](), false, "uint"},
		{reflect.TypeFor[uint64](), false, "uint"},
		{reflect.TypeFor[float32](), false, "float32"},
		{reflect.TypeFor[float64](), false, "float64"},
		{reflect.TypeFor[[]string](), false, "array"},
		{reflect.TypeFor[[3]int](), false, "array"},
		{reflect.TypeFor[map[string]string](), false, "map"},
		{reflect.TypeFor[int](), true, "enum"},
		{reflect.TypeFor[time.Duration](), false, "string"},
	}

	for _, tc := range tests {
		name := tc.typ.String()
		if tc.isEnum {
			name = "enum_override"
		}
		t.Run(name, func(t *testing.T) {
			actual := classifyType(tc.typ, tc.isEnum)
			assert.Equal(t, tc.expected, actual)
		})
	}
}

func TestURLPartToString(t *testing.T) {
	tests := []struct {
		name     string
		parts    []format.URLPart
		expected string
	}{
		{"empty parts", []format.URLPart(nil), ""},
		{"single user part", []format.URLPart{format.URLUser}, "user"},
		{"single password part", []format.URLPart{format.URLPassword}, "password"},
		{"single host part", []format.URLPart{format.URLHost}, "host"},
		{"single port part", []format.URLPart{format.URLPort}, "port"},
		{"single path part", []format.URLPart{format.URLPath}, "path"},
		{"single query part", []format.URLPart{format.URLQuery}, "query"},
		{"user and password", []format.URLPart{format.URLUser, format.URLPassword}, "user,password"},
		{"host and port", []format.URLPart{format.URLHost, format.URLPort}, "host,port"},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			actual := urlPartToString(tc.parts)
			assert.Equal(t, tc.expected, actual)
		})
	}

	t.Run("handles path offsets greater than URLPath", func(t *testing.T) {
		parts := []format.URLPart{format.URLPath + 1}
		result := urlPartToString(parts)
		assert.Equal(t, "path1", result)
	})
}

func TestGetEnumNames(t *testing.T) {
	t.Run("returns nil for nil formatter", func(t *testing.T) {
		assert.Nil(t, getEnumNames(nil))
	})

	t.Run("returns names from formatter", func(t *testing.T) {
		ef := format.CreateEnumFormatter([]string{"Option1", "Option2", "Option3"})
		assert.Equal(t, []string{"Option1", "Option2", "Option3"}, getEnumNames(ef))
	})
}

func TestSetFieldFromString(t *testing.T) {
	t.Run("sets string field", func(t *testing.T) {
		s := testSetFieldStruct{}
		val := reflect.ValueOf(&s).Elem()
		err := setFieldFromString(val, "Name", "hello")
		require.NoError(t, err)
		assert.Equal(t, "hello", s.Name)
	})

	t.Run("sets bool field from Yes", func(t *testing.T) {
		s := testSetFieldStruct{}
		val := reflect.ValueOf(&s).Elem()
		err := setFieldFromString(val, "Enabled", "Yes")
		require.NoError(t, err)
		assert.True(t, s.Enabled)
	})

	t.Run("sets bool field from No", func(t *testing.T) {
		s := testSetFieldStruct{Enabled: true}
		val := reflect.ValueOf(&s).Elem()
		err := setFieldFromString(val, "Enabled", "No")
		require.NoError(t, err)
		assert.False(t, s.Enabled)
	})

	t.Run("sets bool field from true", func(t *testing.T) {
		s := testSetFieldStruct{}
		val := reflect.ValueOf(&s).Elem()
		err := setFieldFromString(val, "Enabled", "true")
		require.NoError(t, err)
		assert.True(t, s.Enabled)
	})

	t.Run("sets bool field from false", func(t *testing.T) {
		s := testSetFieldStruct{Enabled: true}
		val := reflect.ValueOf(&s).Elem()
		err := setFieldFromString(val, "Enabled", "false")
		require.NoError(t, err)
		assert.False(t, s.Enabled)
	})

	t.Run("sets bool field from 1", func(t *testing.T) {
		s := testSetFieldStruct{}
		val := reflect.ValueOf(&s).Elem()
		err := setFieldFromString(val, "Enabled", "1")
		require.NoError(t, err)
		assert.True(t, s.Enabled)
	})

	t.Run("sets bool field from 0", func(t *testing.T) {
		s := testSetFieldStruct{Enabled: true}
		val := reflect.ValueOf(&s).Elem()
		err := setFieldFromString(val, "Enabled", "0")
		require.NoError(t, err)
		assert.False(t, s.Enabled)
	})

	t.Run("returns error for invalid field name", func(t *testing.T) {
		s := testSetFieldStruct{}
		val := reflect.ValueOf(&s).Elem()
		err := setFieldFromString(val, "NonExistent", "value")
		require.Error(t, err)
		assert.Empty(t, s.Name)
	})

	t.Run("sets bool field from lowercase yes", func(t *testing.T) {
		s := testSetFieldStruct{}
		val := reflect.ValueOf(&s).Elem()
		err := setFieldFromString(val, "Enabled", "yes")
		require.NoError(t, err)
		assert.True(t, s.Enabled)
	})

	t.Run("sets bool field from uppercase TRUE", func(t *testing.T) {
		s := testSetFieldStruct{}
		val := reflect.ValueOf(&s).Elem()
		err := setFieldFromString(val, "Enabled", "TRUE")
		require.NoError(t, err)
		assert.True(t, s.Enabled)
	})

	t.Run("sets bool field from uppercase FALSE", func(t *testing.T) {
		s := testSetFieldStruct{Enabled: true}
		val := reflect.ValueOf(&s).Elem()
		err := setFieldFromString(val, "Enabled", "FALSE")
		require.NoError(t, err)
		assert.False(t, s.Enabled)
	})

	t.Run("returns error for invalid bool input", func(t *testing.T) {
		s := testSetFieldStruct{Enabled: true}
		val := reflect.ValueOf(&s).Elem()
		err := setFieldFromString(val, "Enabled", "invalid")
		require.Error(t, err)
		assert.True(t, s.Enabled)
	})

	t.Run("returns error for numeric invalid bool input", func(t *testing.T) {
		s := testSetFieldStruct{Enabled: true}
		val := reflect.ValueOf(&s).Elem()
		err := setFieldFromString(val, "Enabled", "2")
		require.Error(t, err)
		assert.True(t, s.Enabled)
	})
}

func TestHelpersMarshalError(t *testing.T) {
	t.Run("serializes error to JSON", func(t *testing.T) {
		result := marshalError(errors.New("test error"))
		assert.Equal(t, `{"error":"test error"}`, result)
	})

	t.Run("handles empty error message", func(t *testing.T) {
		result := marshalError(errors.New(""))
		assert.Equal(t, `{"error":""}`, result)
	})

	t.Run("handles nil error", func(t *testing.T) {
		result := marshalError(nil)
		assert.Equal(t, `{"error":"unknown error"}`, result)
	})
}

func TestHelpersMarshalErrorStr(t *testing.T) {
	t.Run("serializes string to error JSON", func(t *testing.T) {
		result := marshalErrorStr("something failed")
		assert.Equal(t, `{"error":"something failed"}`, result)
	})

	t.Run("handles empty string", func(t *testing.T) {
		result := marshalErrorStr("")
		assert.Equal(t, `{"error":""}`, result)
	})

	t.Run("handles string with special characters", func(t *testing.T) {
		result := marshalErrorStr(`error with "quotes"`)
		assert.Equal(t, `{"error":"error with \"quotes\""}`, result)
	})
}

func TestHelpersExtractScheme(t *testing.T) {
	tests := []struct {
		name     string
		rawURL   string
		expected string
	}{
		{"discord URL", "discord://token@webhook", "discord"},
		{"teams compound scheme", "teams+https://example.com/path", "teams"},
		{"smtp URL", "smtp://user:pass@host:587", "smtp"},
		{"ntfy URL", "ntfy://ntfy.sh/topic", "ntfy"},
		{"generic URL", "generic://192.168.1.100:8123/path", "generic"},
		{"invalid URL no scheme", "invalid", ""},
		{"empty string", "", ""},
		{"only colon", "://path", ""},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			actual := extractScheme(tc.rawURL)
			assert.Equal(t, tc.expected, actual)
		})
	}

	t.Run("handles scheme with plus separator", func(t *testing.T) {
		assert.Equal(t, "log", extractScheme("log+http://localhost"))
	})
}

func TestIsNillableKind(t *testing.T) {
	nillableKinds := []struct {
		name string
		kind reflect.Kind
	}{
		{"reflect.Slice", reflect.Slice},
		{"reflect.Ptr", reflect.Pointer},
		{"reflect.Map", reflect.Map},
		{"reflect.Interface", reflect.Interface},
		{"reflect.Func", reflect.Func},
		{"reflect.Chan", reflect.Chan},
	}

	for _, tc := range nillableKinds {
		t.Run(tc.name+" returns true", func(t *testing.T) {
			assert.True(t, isNillableKind(tc.kind))
		})
	}

	nonNillableKinds := []struct {
		name string
		kind reflect.Kind
	}{
		{"reflect.Struct", reflect.Struct},
		{"reflect.Int", reflect.Int},
		{"reflect.String", reflect.String},
		{"reflect.Bool", reflect.Bool},
		{"reflect.Float64", reflect.Float64},
		{"reflect.Array", reflect.Array},
	}

	for _, tc := range nonNillableKinds {
		t.Run(tc.name+" returns false", func(t *testing.T) {
			assert.False(t, isNillableKind(tc.kind))
		})
	}
}
