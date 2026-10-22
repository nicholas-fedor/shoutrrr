package format

import (
	"net/url"
	"reflect"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/nicholas-fedor/shoutrrr/pkg/types"
)

// unsupportedConfig has fields of kinds the formatter cannot set.
type unsupportedConfig struct {
	// Ratio is a float.
	Ratio float64
	// Flags is a map with non-string keys.
	Flags map[int]string
	// Switches is a map of bools.
	Switches map[string]bool
	// Numbers is a slice of ints.
	Numbers []int
	// Hex is a uint in base 16.
	Hex uint `base:"16"`
	// Signed is an int in base 16.
	SignedHex int `base:"16"`
}

// configService is a service with a pointer config.
type configService struct {
	types.Service

	// Config is the service config.
	Config *testStruct
}

// configlessService is a service without a Config field.
type configlessService struct {
	types.Service
}

// newFieldTarget returns a settable zero value of T and its field infos by
// name.
//
// Parameters:
//   - t: the test.
//
// Returns:
//   - reflect.Value: the settable value.
//   - map[string]*FieldInfo: the field infos by field name.
func newFieldTarget[T any](t *testing.T) (reflect.Value, map[string]*FieldInfo) {
	t.Helper()

	value := reflect.New(reflect.TypeFor[T]()).Elem()

	var enums map[string]types.EnumFormatter
	if enummer, ok := reflect.TypeAssert[types.Enummer](value.Addr()); ok {
		enums = enummer.Enums()
	}

	infos := getStructFieldInfo(value.Type(), enums)
	fields := make(map[string]*FieldInfo, len(infos))

	for i := range infos {
		fields[infos[i].Name] = &infos[i]
	}

	return value, fields
}

// TestSetConfigField verifies that each supported field kind is set from its
// string and reads back as the same string, that a 0x or # prefix reads a
// number in base 16, that a map value may contain colons, and that an empty
// value sets a slice or map to empty.
func TestSetConfigField(t *testing.T) {
	t.Parallel()

	// tests lists fields and values with the value each field should hold.
	tests := []struct {
		want  any
		field string
		value string
		read  string
	}{
		{field: "Signed", value: "-45", want: -45},
		{field: "Signed", value: "0x10", want: 16, read: "16"},
		{field: "Signed", value: "#ff", want: 255, read: "255"},
		{field: "Unsigned", value: "5", want: uint(5)},
		{field: "Unsigned", value: "0X1F", want: uint(31), read: "31"},
		{field: "Duration", value: "1h2m3s", want: time.Hour + 2*time.Minute + 3*time.Second},
		{field: "Duration", value: "0", want: time.Duration(0), read: "0s"},
		{field: "Str", value: "text", want: "text"},
		{field: "TestEnum", value: "foo", want: 1, read: "Foo"},
		{field: "SubProp", value: "@whoa", want: subPropStruct{Value: "whoa"}},
		{field: "StrSlice", value: "one,two", want: []string{"one", "two"}},
		{field: "StrArray", value: "one,two,three", want: [3]string{"one", "two", "three"}},
		{field: "SubPropSlice", value: "@be,@the", want: []subPropStruct{{Value: "be"}, {Value: "the"}}},
		{field: "SubPropPtrSlice", value: "@diet,@glue", want: []*subPropStruct{{Value: "diet"}, {Value: "glue"}}},
		{field: "StrMap", value: "a:1,b:2", want: map[string]string{"a": "1", "b": "2"}},
		{
			field: "StrMap", value: "link:https://example.com:8443/x,owner:ops",
			want: map[string]string{"link": "https://example.com:8443/x", "owner": "ops"},
		},
		{field: "IntMap", value: "a:-1,b:0x10", want: map[string]int{"a": -1, "b": 16}, read: "a:-1,b:16"},
		{field: "Int8Map", value: "a:-128,b:127", want: map[string]int8{"a": -128, "b": 127}},
		{field: "Int16Map", value: "a:1", want: map[string]int16{"a": 1}},
		{field: "Int32Map", value: "a:1", want: map[string]int32{"a": 1}},
		{field: "Int64Map", value: "a:1", want: map[string]int64{"a": 1}},
		{field: "UintMap", value: "a:#ff", want: map[string]uint{"a": 255}, read: "a:255"},
		{field: "Uint8Map", value: "a:255", want: map[string]uint8{"a": 255}},
		{field: "Uint16Map", value: "a:1", want: map[string]uint16{"a": 1}},
		{field: "Uint32Map", value: "a:1", want: map[string]uint32{"a": 1}},
		{field: "Uint64Map", value: "a:1", want: map[string]uint64{"a": 1}},
		{field: "StrSlice", value: "", want: []string(nil)},
		{field: "SubPropSlice", value: "", want: []subPropStruct(nil)},
		{field: "SubPropPtrSlice", value: "", want: []*subPropStruct(nil)},
		{field: "StrMap", value: "", want: map[string]string(nil)},
		{field: "IntMap", value: "", want: map[string]int(nil)},
	}

	for _, tt := range tests {
		t.Run(tt.field+"="+tt.value, func(t *testing.T) {
			t.Parallel()

			value, fields := newFieldTarget[testStruct](t)

			valid, err := SetConfigField(value, fields[tt.field], tt.value)
			require.NoError(t, err)
			assert.True(t, valid)
			assert.Equal(t, tt.want, value.FieldByName(tt.field).Interface())

			read := tt.read
			if read == "" {
				read = tt.value
			}

			formatted, err := GetConfigFieldString(value, fields[tt.field])
			require.NoError(t, err)
			assert.Equal(t, read, formatted)
		})
	}
}

// TestSetConfigFieldBase verifies that a field with a number base reads
// digits in that base, with or without a hex prefix, and writes them back in
// it.
func TestSetConfigFieldBase(t *testing.T) {
	t.Parallel()

	// tests lists fields and values with the number and string each should
	// produce.
	tests := []struct {
		field string
		value string
		want  any
		read  string
	}{
		{field: "Hex", value: "ff", want: uint(255), read: "0xff"},
		{field: "Hex", value: "0xFF", want: uint(255), read: "0xff"},
		{field: "Hex", value: "#50d9ff", want: uint(0x50d9ff), read: "0x50d9ff"},
		{field: "SignedHex", value: "-1f", want: -31, read: "-1f"},
		{field: "SignedHex", value: "0x10", want: 16, read: "10"},
	}

	for _, tt := range tests {
		t.Run(tt.field+"="+tt.value, func(t *testing.T) {
			t.Parallel()

			value, fields := newFieldTarget[unsupportedConfig](t)

			valid, err := SetConfigField(value, fields[tt.field], tt.value)
			require.NoError(t, err)
			assert.True(t, valid)
			assert.Equal(t, tt.want, value.FieldByName(tt.field).Interface())

			formatted, err := GetConfigFieldString(value, fields[tt.field])
			require.NoError(t, err)
			assert.Equal(t, tt.read, formatted)
		})
	}
}

// TestSetConfigFieldErrors verifies that a value that does not parse for its
// field fails with the matching error and leaves the field unchanged.
func TestSetConfigFieldErrors(t *testing.T) {
	t.Parallel()

	// tests lists fields and values with the error each should produce.
	tests := []struct {
		wantErr error
		field   string
		value   string
	}{
		{field: "Signed", value: "z7", wantErr: ErrParseIntFailed},
		{field: "Unsigned", value: "-3", wantErr: ErrParseUintFailed},
		{field: "Duration", value: "10", wantErr: ErrParseDurationFailed},
		{field: "Duration", value: "z7", wantErr: ErrParseDurationFailed},
		{field: "TestEnum", value: "Baz", wantErr: ErrInvalidEnumValue},
		{field: "Sub", value: "@awol", wantErr: ErrNotConfigProp},
		{field: "SubProp", value: "missing initial at symbol", wantErr: errInvalidProp},
		{field: "SubSlice", value: "@a,@b", wantErr: ErrNotConfigProp},
		{field: "SubPropSlice", value: "@a,b", wantErr: errInvalidProp},
		{field: "StrArray", value: "one,two,three,four", wantErr: ErrInvalidFieldCount},
		{field: "StrArray", value: "one,two", wantErr: ErrInvalidFieldCount},
		{field: "StrArray", value: "", wantErr: ErrInvalidFieldCount},
		{field: "StrMap", value: "a", wantErr: ErrInvalidFieldValue},
		{field: "IntMap", value: "a:one", wantErr: ErrParseIntFailed},
		{field: "IntMap", value: "a:1:2", wantErr: ErrParseIntFailed},
		{field: "Signed", value: "0xzz", wantErr: ErrParseIntFailed},
		{field: "Int8Map", value: "a:128", wantErr: ErrParseIntFailed},
		{field: "Int16Map", value: "a:32768", wantErr: ErrParseIntFailed},
		{field: "Int32Map", value: "a:2147483648", wantErr: ErrParseIntFailed},
		{field: "UintMap", value: "a:-1", wantErr: ErrParseUintFailed},
		{field: "Uint8Map", value: "a:256", wantErr: ErrParseUintFailed},
		{field: "Uint16Map", value: "a:65536", wantErr: ErrParseUintFailed},
		{field: "Uint32Map", value: "a:4294967296", wantErr: ErrParseUintFailed},
	}

	for _, tt := range tests {
		t.Run(tt.field+"="+tt.value, func(t *testing.T) {
			t.Parallel()

			value, fields := newFieldTarget[testStruct](t)
			before := value.FieldByName(tt.field).Interface()

			valid, err := SetConfigField(value, fields[tt.field], tt.value)

			require.ErrorIs(t, err, tt.wantErr)
			assert.False(t, valid)
			assert.Equal(t, before, value.FieldByName(tt.field).Interface())
		})
	}
}

// TestSetConfigFieldUnsupported verifies that fields of kinds the formatter
// does not support fail.
func TestSetConfigFieldUnsupported(t *testing.T) {
	t.Parallel()

	// tests lists fields with the error setting each should produce.
	tests := []struct {
		wantErr error
		field   string
		value   string
	}{
		{field: "Ratio", value: "1.5", wantErr: ErrInvalidFieldKind},
		{field: "Flags", value: "1:a", wantErr: ErrUnsupportedFieldKey},
		{field: "Switches", value: "a:yes", wantErr: ErrUnsupportedMapValue},
		{field: "Numbers", value: "1,2", wantErr: ErrUnsupportedField},
	}

	for _, tt := range tests {
		t.Run(tt.field, func(t *testing.T) {
			t.Parallel()

			value, fields := newFieldTarget[unsupportedConfig](t)

			valid, err := SetConfigField(value, fields[tt.field], tt.value)

			require.ErrorIs(t, err, tt.wantErr)
			assert.False(t, valid)
		})
	}

	value, fields := newFieldTarget[spelledDefaultsConfig](t)

	valid, err := SetConfigField(value, fields["Plain"], "maybe")
	require.ErrorIs(t, err, ErrInvalidBoolValue)
	assert.False(t, valid)
}

// TestGetConfigFieldString verifies that a field that cannot be written as a
// string fails with [ErrInvalidFieldValueData].
func TestGetConfigFieldString(t *testing.T) {
	t.Parallel()

	value, fields := newFieldTarget[testStruct](t)

	_, err := GetConfigFieldString(value, fields["Sub"])
	require.ErrorIs(t, err, ErrInvalidFieldValueData)

	formatted, err := GetConfigFieldString(value, fields["TestEnum"])
	require.NoError(t, err)
	assert.Equal(t, "None", formatted)
}

// TestGetServiceConfig verifies that a service's config is returned, that a
// nil config pointer gives a new config, and that a service without a Config
// field panics.
func TestGetServiceConfig(t *testing.T) {
	t.Parallel()

	config := &testStruct{Str: "set"}
	assert.Same(t, config, GetServiceConfig(&configService{Config: config}))

	created, ok := GetServiceConfig(&configService{}).(*testStruct)
	require.True(t, ok)
	assert.Equal(t, &testStruct{}, created)

	assert.PanicsWithValue(t, "service does not have a Config field", func() {
		GetServiceConfig(&configlessService{})
	})
}

// TestGetServiceConfigFormat verifies that a service's config format lists
// its config's fields.
func TestGetServiceConfigFormat(t *testing.T) {
	t.Parallel()

	node := GetServiceConfigFormat(&configService{Config: &testStruct{Str: "set"}})

	require.Len(t, node.Items, reflect.TypeFor[testStruct]().NumField())

	found := false

	for _, item := range node.Items {
		if item.Field().Name == "Str" {
			found = true

			valueNode, ok := item.(*ValueNode)
			require.True(t, ok)
			assert.Equal(t, "set", valueNode.Value)
		}
	}

	assert.True(t, found, "no Str item")
}

// TestColorFormatTree verifies that a format tree renders as the console
// renderer renders it, with or without values.
func TestColorFormatTree(t *testing.T) {
	t.Parallel()

	node := GetConfigFormat(&testStruct{})

	assert.Equal(t, ConsoleTreeRenderer{WithValues: true}.RenderTree(node, ""), ColorFormatTree(node, true))
	assert.Equal(t, ConsoleTreeRenderer{WithValues: false}.RenderTree(node, ""), ColorFormatTree(node, false))
}

// GetURL returns an empty URL. The formatter tests do not use it.
//
// Returns:
//   - *url.URL: an empty URL.
func (c *unsupportedConfig) GetURL() *url.URL { return &url.URL{} }

// SetURL ignores the URL. The formatter tests do not use it.
//
// Parameters:
//   - *url.URL: unused.
//
// Returns:
//   - error: always nil.
func (c *unsupportedConfig) SetURL(*url.URL) error { return nil }
