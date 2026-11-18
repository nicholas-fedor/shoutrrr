package format

import (
	"errors"
	"net/url"
	"sync/atomic"
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

// onceProp is a [types.ConfigProp] that accepts its first parse and rejects
// every later one, as a config prop from outside the package may.
type onceProp struct {
	// Value is the parsed value.
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

// testStructTwoBadDefaults is a service config with two defaults that do not
// parse, ahead of a valid one. Their struct order is the reverse of their
// name order.
type testStructTwoBadDefaults struct {
	standard.EnumlessConfig

	// Later is an int whose default is not a number. It comes first in the
	// struct and second by name.
	Later int `default:"NaN" key:"a"`
	// Early is a duration whose default is not a duration. It comes first by
	// name.
	Early time.Duration `default:"soon" key:"b"`
	// Name is a string with a valid default.
	Name string `default:"kept" key:"name"`
}

// testStructEmptyKey is a service config with a field whose key tag is empty.
type testStructEmptyKey struct {
	standard.EnumlessConfig

	// Unkeyed has a default but an empty key tag, so no key sets it.
	Unkeyed string `default:"unused" key:""`
	// Name is a keyed string with a default.
	Name string `default:"kept" key:"name"`
}

// testStructFilledListDefaults is a service config with a keyed slice and a
// keyed map that have defaults.
type testStructFilledListDefaults struct {
	standard.EnumlessConfig

	// Strs is a keyed string slice with two default items.
	Strs []string `default:"a,b" key:"strs"`
	// Pairs is a keyed map with one default pair.
	Pairs map[string]string `default:"x:1" key:"pairs"`
}

// testStructOncePropDefault is a service config with a keyed config prop
// whose default parses only once.
type testStructOncePropDefault struct {
	standard.EnumlessConfig

	// Prop is a keyed config prop with a default.
	Prop onceProp `default:"set" key:"prop"`
	// Name is a keyed string with a default.
	Name string `default:"kept" key:"name"`
}

// label is a string type of its own, which a string is not assignable to.
type label string

// testStructLabelSource is a service config with a keyed string default and a
// keyed int without one.
type testStructLabelSource struct {
	standard.EnumlessConfig

	// Name is a keyed string with a default.
	Name string `default:"kept" key:"name"`
	// Count is a keyed int without a default.
	Count int `key:"count"`
}

// testStructLabelTarget is a service config with the fields of
// testStructLabelSource, its Name of another type.
type testStructLabelTarget struct {
	standard.EnumlessConfig

	// Name is a keyed label.
	Name label `key:"name"`
	// Count is a keyed int.
	Count int `key:"count"`
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

// errOncePropParsed is the error onceProp returns after its first parse.
var errOncePropParsed = errors.New("already parsed")

// oncePropParses counts the parses of every onceProp. Only
// TestPropKeyResolverSetDefaultPropsReparseError uses the type.
var oncePropParses atomic.Int32

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

// GetPropValue writes the value as it is.
//
// Returns:
//   - string: the value.
//   - error: always nil.
func (p *onceProp) GetPropValue() (string, error) {
	return p.Value, nil
}

// SetFromProp reads the value on the first parse of any onceProp.
//
// Parameters:
//   - propValue: the value.
//
// Returns:
//   - error: errOncePropParsed on every parse after the first.
func (p *onceProp) SetFromProp(propValue string) error {
	if oncePropParses.Add(1) > 1 {
		return errOncePropParsed
	}

	p.Value = propValue

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
func (t *testStructTwoBadDefaults) GetURL() *url.URL {
	return &url.URL{}
}

// SetURL ignores the URL. The format tests do not use it.
//
// Parameters:
//   - *url.URL: unused.
//
// Returns:
//   - error: always nil.
func (t *testStructTwoBadDefaults) SetURL(*url.URL) error {
	return nil
}

// GetURL returns an empty URL. The format tests do not use it.
//
// Returns:
//   - *url.URL: an empty URL.
func (t *testStructEmptyKey) GetURL() *url.URL {
	return &url.URL{}
}

// SetURL ignores the URL. The format tests do not use it.
//
// Parameters:
//   - *url.URL: unused.
//
// Returns:
//   - error: always nil.
func (t *testStructEmptyKey) SetURL(*url.URL) error {
	return nil
}

// GetURL returns an empty URL. The format tests do not use it.
//
// Returns:
//   - *url.URL: an empty URL.
func (t *testStructFilledListDefaults) GetURL() *url.URL {
	return &url.URL{}
}

// SetURL ignores the URL. The format tests do not use it.
//
// Parameters:
//   - *url.URL: unused.
//
// Returns:
//   - error: always nil.
func (t *testStructFilledListDefaults) SetURL(*url.URL) error {
	return nil
}

// GetURL returns an empty URL. The format tests do not use it.
//
// Returns:
//   - *url.URL: an empty URL.
func (t *testStructOncePropDefault) GetURL() *url.URL {
	return &url.URL{}
}

// SetURL ignores the URL. The format tests do not use it.
//
// Parameters:
//   - *url.URL: unused.
//
// Returns:
//   - error: always nil.
func (t *testStructOncePropDefault) SetURL(*url.URL) error {
	return nil
}

// GetURL returns an empty URL. The format tests do not use it.
//
// Returns:
//   - *url.URL: an empty URL.
func (t *testStructLabelSource) GetURL() *url.URL {
	return &url.URL{}
}

// SetURL ignores the URL. The format tests do not use it.
//
// Parameters:
//   - *url.URL: unused.
//
// Returns:
//   - error: always nil.
func (t *testStructLabelSource) SetURL(*url.URL) error {
	return nil
}

// GetURL returns an empty URL. The format tests do not use it.
//
// Returns:
//   - *url.URL: an empty URL.
func (t *testStructLabelTarget) GetURL() *url.URL {
	return &url.URL{}
}

// SetURL ignores the URL. The format tests do not use it.
//
// Parameters:
//   - *url.URL: unused.
//
// Returns:
//   - error: always nil.
func (t *testStructLabelTarget) SetURL(*url.URL) error {
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
