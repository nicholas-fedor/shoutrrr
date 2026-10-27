package format

import (
	"errors"
	"net/url"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"

	"github.com/nicholas-fedor/shoutrrr/pkg/services/standard"
	"github.com/nicholas-fedor/shoutrrr/pkg/types"
)

// subStruct is a struct field type that does not implement
// [types.ConfigProp].
type subStruct struct {
	// Value is the struct's only field.
	Value string
}

// subPropStruct is a struct field type that implements [types.ConfigProp]
// with values written as "@value".
type subPropStruct struct {
	// Value is the value without its "@" prefix.
	Value string
}

// testStruct is a service config with a field of each kind the formatter
// supports.
type testStruct struct {
	// Signed is a keyed int with a default.
	Signed int `default:"0" key:"signed"`
	// Unsigned is a uint without a key.
	Unsigned uint
	// Duration is a keyed duration with a default.
	Duration time.Duration `default:"10s" key:"duration"`
	// Str is a keyed string with a default.
	Str string `default:"notempty" key:"str"`
	// StrSlice is a string slice.
	StrSlice []string
	// StrArray is a string array of three items.
	StrArray [3]string
	// Sub is a struct that is not a config prop.
	Sub subStruct
	// TestEnum is a keyed enum with a default.
	TestEnum int `default:"None" key:"testenum"`
	// SubProp is a config prop struct.
	SubProp subPropStruct
	// SubSlice is a slice of structs that are not config props.
	SubSlice []subStruct
	// SubPropSlice is a slice of config prop structs.
	SubPropSlice []subPropStruct
	// SubPropPtrSlice is a slice of pointers to config prop structs.
	SubPropPtrSlice []*subPropStruct
	// StrMap is a map of strings.
	StrMap map[string]string
	// IntMap is a map of ints.
	IntMap map[string]int
	// Int8Map is a map of int8s.
	Int8Map map[string]int8
	// Int16Map is a map of int16s.
	Int16Map map[string]int16
	// Int32Map is a map of int32s.
	Int32Map map[string]int32
	// Int64Map is a map of int64s.
	Int64Map map[string]int64
	// UintMap is a map of uints.
	UintMap map[string]uint
	// Uint8Map is a map of uint8s.
	Uint8Map map[string]uint8
	// Uint16Map is a map of uint16s.
	Uint16Map map[string]uint16
	// Uint32Map is a map of uint32s.
	Uint32Map map[string]uint32
	// Uint64Map is a map of uint64s.
	Uint64Map map[string]uint64
}

// testStructBadDefault is a service config whose default does not parse.
type testStructBadDefault struct {
	standard.EnumlessConfig

	// Value is an int whose default is not a number.
	Value int `default:"NaN" key:"value"`
}

// testStructListDefaults is a service config with keyed slices and a keyed map
// that have no default, so their default is the empty value.
type testStructListDefaults struct {
	standard.EnumlessConfig

	// Strs is a keyed string slice.
	Strs []string `key:"strs"`
	// Props is a keyed slice of config prop structs.
	Props []subPropStruct `key:"props"`
	// Pairs is a keyed map of strings.
	Pairs map[string]string `key:"pairs"`
}

// testStructNoDefaults is a service config with keyed fields of the kinds an
// empty value does not parse as, none of them with a default.
type testStructNoDefaults struct {
	// Duration is a keyed duration.
	Duration time.Duration `key:"duration"`
	// Signed is a keyed int.
	Signed int `key:"signed"`
	// Unsigned is a keyed uint.
	Unsigned uint `key:"unsigned"`
	// Level is a keyed enum.
	Level int `key:"level"`
	// Flag is a keyed bool.
	Flag bool `key:"flag"`
}

// errInvalidProp is the error subPropStruct returns for a value without its
// "@" prefix.
var errInvalidProp = errors.New("invalid value")

// testEnum formats the TestEnum field of testStruct.
var testEnum = CreateEnumFormatter([]string{"None", "Foo", "Bar"})

// TestParseBool verifies that ParseBool accepts the truthy and falsy
// spellings in any case and returns the default for anything else.
func TestParseBool(t *testing.T) {
	t.Parallel()

	// tests lists raw values with the bool each should parse to.
	tests := []struct {
		raw    string
		want   bool
		wantOK bool
	}{
		{raw: "true", want: true, wantOK: true},
		{raw: "1", want: true, wantOK: true},
		{raw: "yes", want: true, wantOK: true},
		{raw: "y", want: true, wantOK: true},
		{raw: "trUE", want: true, wantOK: true},
		{raw: "false", want: false, wantOK: true},
		{raw: "0", want: false, wantOK: true},
		{raw: "no", want: false, wantOK: true},
		{raw: "N", want: false, wantOK: true},
	}

	for _, tt := range tests {
		t.Run(tt.raw, func(t *testing.T) {
			t.Parallel()

			parsed, ok := ParseBool(tt.raw, !tt.want)
			assert.Equal(t, tt.want, parsed)
			assert.Equal(t, tt.wantOK, ok)
		})
	}

	parsed, ok := ParseBool("bad", true)
	assert.True(t, parsed)
	assert.False(t, ok)

	parsed, ok = ParseBool("", false)
	assert.False(t, parsed)
	assert.False(t, ok)
}

// TestPrintBool verifies that a bool prints as Yes or No.
func TestPrintBool(t *testing.T) {
	t.Parallel()

	assert.Equal(t, "Yes", PrintBool(true))
	assert.Equal(t, "No", PrintBool(false))
}

// TestIsNumber verifies that IsNumber accepts what parses as a float.
func TestIsNumber(t *testing.T) {
	t.Parallel()

	assert.True(t, IsNumber("1.5"))
	assert.True(t, IsNumber("0"))
	assert.True(t, IsNumber("-3"))
	assert.True(t, IsNumber("NaN"))
	assert.False(t, IsNumber("baNaNa"))
	assert.False(t, IsNumber(""))
}

// GetPropValue writes the value with its "@" prefix.
//
// Returns:
//   - string: the value with an "@" prefix.
//   - error: always nil.
func (s *subPropStruct) GetPropValue() (string, error) {
	return "@" + s.Value, nil
}

// SetFromProp reads a value written with an "@" prefix.
//
// Parameters:
//   - propValue: the value with its "@" prefix.
//
// Returns:
//   - error: errInvalidProp when the prefix is missing.
func (s *subPropStruct) SetFromProp(propValue string) error {
	if len(propValue) < 1 || propValue[0] != '@' {
		return errInvalidProp
	}

	s.Value = propValue[1:]

	return nil
}

// Enums returns the enum formatter of the TestEnum field.
//
// Returns:
//   - map[string]types.EnumFormatter: the enum formatters by field name.
func (t *testStruct) Enums() map[string]types.EnumFormatter {
	return map[string]types.EnumFormatter{"TestEnum": testEnum}
}

// GetURL returns an empty URL. The format tests do not use it.
//
// Returns:
//   - *url.URL: an empty URL.
func (t *testStruct) GetURL() *url.URL {
	return &url.URL{}
}

// SetURL ignores the URL. The format tests do not use it.
//
// Parameters:
//   - *url.URL: unused.
//
// Returns:
//   - error: always nil.
func (t *testStruct) SetURL(*url.URL) error {
	return nil
}

// GetURL returns an empty URL. The format tests do not use it.
//
// Returns:
//   - *url.URL: an empty URL.
func (t *testStructBadDefault) GetURL() *url.URL {
	return &url.URL{}
}

// SetURL ignores the URL. The format tests do not use it.
//
// Parameters:
//   - *url.URL: unused.
//
// Returns:
//   - error: always nil.
func (t *testStructBadDefault) SetURL(*url.URL) error {
	return nil
}

// GetURL returns an empty URL. The format tests do not use it.
//
// Returns:
//   - *url.URL: an empty URL.
func (t *testStructListDefaults) GetURL() *url.URL {
	return &url.URL{}
}

// SetURL ignores the URL. The format tests do not use it.
//
// Parameters:
//   - *url.URL: unused.
//
// Returns:
//   - error: always nil.
func (t *testStructListDefaults) SetURL(*url.URL) error {
	return nil
}

// Enums returns the enum formatter of the Level field.
//
// Returns:
//   - map[string]types.EnumFormatter: the enum formatters by field name.
func (t *testStructNoDefaults) Enums() map[string]types.EnumFormatter {
	return map[string]types.EnumFormatter{"Level": testEnum}
}

// GetURL returns an empty URL. The format tests do not use it.
//
// Returns:
//   - *url.URL: an empty URL.
func (t *testStructNoDefaults) GetURL() *url.URL {
	return &url.URL{}
}

// SetURL ignores the URL. The format tests do not use it.
//
// Parameters:
//   - *url.URL: unused.
//
// Returns:
//   - error: always nil.
func (t *testStructNoDefaults) SetURL(*url.URL) error {
	return nil
}
