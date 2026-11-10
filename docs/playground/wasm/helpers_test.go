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
	"github.com/nicholas-fedor/shoutrrr/pkg/router"
)

// TestClassifyType verifies the type name the form gets for each kind of Go
// type, for enums, and for durations.
func TestClassifyType(t *testing.T) {
	t.Parallel()

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
			t.Parallel()

			actual := classifyType(tc.typ, tc.isEnum)
			assert.Equal(t, tc.expected, actual)
		})
	}
}

// TestURLPartToString verifies the names of URL parts, and that each name
// reads back as its part through format.ParseURLPart.
func TestURLPartToString(t *testing.T) {
	t.Parallel()

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
			t.Parallel()

			actual := urlPartToString(tc.parts)
			assert.Equal(t, tc.expected, actual)
		})
	}

	t.Run("names later path segments as format reads them", func(t *testing.T) {
		t.Parallel()

		for _, part := range []format.URLPart{format.URLPath, format.URLPath + 1, format.URLPath + 2} {
			name := urlPartToString([]format.URLPart{part})
			assert.Equal(t, part, format.ParseURLPart(name), name)
		}

		assert.Equal(t, "path2", urlPartToString([]format.URLPart{format.URLPath + 1}))
	})
}

// TestConfigField verifies that a field is found by its Go name, and that a
// name the config does not have finds nothing.
func TestConfigField(t *testing.T) {
	t.Parallel()

	r := router.ServiceRouter{}
	service, err := r.NewService("mqtt")
	require.NoError(t, err)

	config := format.GetServiceConfig(service)

	field := configField(config, "Port")
	require.NotNil(t, field)
	assert.Equal(t, "Port", field.Name)
	assert.Nil(t, configField(config, "NoSuchField"))
}

// TestHelpersMarshalError verifies that an error, an empty error, and a nil
// error are each encoded as an errorResult.
func TestHelpersMarshalError(t *testing.T) {
	t.Parallel()

	t.Run("serializes error to JSON", func(t *testing.T) {
		t.Parallel()

		result := marshalError(errors.New("test error"))
		assert.JSONEq(t, `{"error":"test error"}`, result)
	})

	t.Run("handles empty error message", func(t *testing.T) {
		t.Parallel()

		result := marshalError(errors.New(""))
		assert.JSONEq(t, `{"error":""}`, result)
	})

	t.Run("handles nil error", func(t *testing.T) {
		t.Parallel()

		result := marshalError(nil)
		assert.JSONEq(t, `{"error":"unknown error"}`, result)
	})
}

// TestHelpersMarshalErrorStr verifies that a message, including an empty one
// and one with characters JSON escapes, is encoded as an errorResult.
func TestHelpersMarshalErrorStr(t *testing.T) {
	t.Parallel()

	t.Run("serializes string to error JSON", func(t *testing.T) {
		t.Parallel()

		result := marshalErrorStr("something failed")
		assert.JSONEq(t, `{"error":"something failed"}`, result)
	})

	t.Run("handles empty string", func(t *testing.T) {
		t.Parallel()

		result := marshalErrorStr("")
		assert.JSONEq(t, `{"error":""}`, result)
	})

	t.Run("handles string with special characters", func(t *testing.T) {
		t.Parallel()

		result := marshalErrorStr(`error with "quotes"`)
		assert.JSONEq(t, `{"error":"error with \"quotes\""}`, result)
	})
}

// TestHelpersExtractScheme verifies the scheme read from service URLs and
// custom URLs, and the empty scheme of text that is not a URL.
func TestHelpersExtractScheme(t *testing.T) {
	t.Parallel()

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
			t.Parallel()

			actual := extractScheme(tc.rawURL)
			assert.Equal(t, tc.expected, actual)
		})
	}

	t.Run("handles scheme with plus separator", func(t *testing.T) {
		t.Parallel()

		assert.Equal(t, "log", extractScheme("log+http://localhost"))
	})
}
