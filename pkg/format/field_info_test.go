package format

import (
	"reflect"
	"slices"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/nicholas-fedor/shoutrrr/pkg/services/standard"
	"github.com/nicholas-fedor/shoutrrr/pkg/types"
)

// taggedConfig has fields that use each tag getStructFieldInfo reads.
type taggedConfig struct {
	standard.EnumlessConfig

	// Name has a description, template, default, and keys.
	Name string `default:"shoutrrr" desc:"The name" key:"Name,N" tpl:"{{.name}}"`
	// Host is a required URL part.
	Host string `url:"host,port"`
	// Mode is an enum.
	Mode int `optional:""`
	// Title is shown as a title.
	Title string `optional:"" title:""`
	// Items uses a custom item separator.
	Items []string `sep:";"`
	// Color is a number in base 16.
	Color uint `base:"16"`
	// Count is a number with a base that does not parse.
	Count int `base:"many"`
	// hidden is unexported.
	hidden string
}

// TestGetStructFieldInfo verifies the field info read from each tag, that
// numbers default to base 10, that enums come from the enum map, and that
// embedded and unexported fields are skipped. Each field records its struct
// type and index.
func TestGetStructFieldInfo(t *testing.T) {
	t.Parallel()

	modes := CreateEnumFormatter([]string{"Off", "On"})
	config := taggedConfig{hidden: "unused"}

	configType := reflect.TypeOf(config)
	fields := getStructFieldInfo(configType, map[string]types.EnumFormatter{"Mode": modes})

	// The indexes count the embedded config, which is field 0.
	want := []FieldInfo{
		{
			Name: "Name", Type: reflect.TypeFor[string](), Description: "The name", Template: "{{.name}}",
			DefaultValue: "shoutrrr", Keys: []string{"name", "n"}, ItemSeparator: ',', owner: configType, index: 1,
		},
		{
			Name: "Host", Type: reflect.TypeFor[string](), Required: true, URLParts: []URLPart{URLHost, URLPort},
			ItemSeparator: ',', owner: configType, index: 2,
		},
		{Name: "Mode", Type: reflect.TypeFor[int](), EnumFormatter: modes, Base: DefaultBase, ItemSeparator: ',', owner: configType, index: 3},
		{Name: "Title", Type: reflect.TypeFor[string](), Title: true, ItemSeparator: ',', owner: configType, index: 4},
		{Name: "Items", Type: reflect.TypeFor[[]string](), Required: true, ItemSeparator: ';', owner: configType, index: 5},
		{Name: "Color", Type: reflect.TypeFor[uint](), Required: true, Base: 16, ItemSeparator: ',', owner: configType, index: 6},
		{Name: "Count", Type: reflect.TypeFor[int](), Required: true, Base: DefaultBase, ItemSeparator: ',', owner: configType, index: 7},
	}

	assert.Equal(t, want, fields)
}

// TestFieldInfoValue verifies that a field is found by its index in the struct
// it was read from, and by its name in another struct type or when the field
// info was built by hand.
func TestFieldInfoValue(t *testing.T) {
	t.Parallel()

	fields := getStructFieldInfo(reflect.TypeFor[testStruct](), nil)
	strIndex := slices.IndexFunc(fields, func(field FieldInfo) bool { return field.Name == "Str" })
	require.GreaterOrEqual(t, strIndex, 0)

	str := &fields[strIndex]

	config := testStruct{Str: "own"}
	assert.Equal(t, "own", str.value(reflect.ValueOf(config)).String())

	// other has a Str field at another index than testStruct's.
	other := struct {
		Count int
		Str   string
	}{Str: "other"}
	assert.Equal(t, "other", str.value(reflect.ValueOf(other)).String())

	byHand := &FieldInfo{Name: "Str"}
	assert.Equal(t, "own", byHand.value(reflect.ValueOf(config)).String())
}

// TestGetStructFieldInfoDeprecated verifies that deprecated keys are read in
// lower case. Fields marked deprecated are left out, so no docs, generator, or
// masked URL lists them.
func TestGetStructFieldInfoDeprecated(t *testing.T) {
	t.Parallel()

	fields := getStructFieldInfo(reflect.TypeFor[deprecatedConfig](), nil)

	names := make([]string, 0, len(fields))
	for _, field := range fields {
		names = append(names, field.Name)
	}

	assert.Equal(t, []string{"SkipVerify", "Token", "Pin"}, names)
	assert.Equal(t, []string{"insecure", "noverify"}, fields[0].DeprecatedKeys)
	assert.Nil(t, fields[1].DeprecatedKeys)
}

// TestGetStructFieldInfoReplaced verifies that a field's replacement key and
// value mapping are read from its tags. A malformed mapping accepts no value,
// so a key with one never sets its replacement.
func TestGetStructFieldInfoReplaced(t *testing.T) {
	t.Parallel()

	fields := getStructFieldInfo(reflect.TypeFor[replacedConfig](), nil)
	require.Len(t, fields, 4)

	scheme, label := fields[0], fields[2]
	assert.Equal(t, "disabletls", scheme.replacedBy)
	assert.Equal(t, []valueReplacement{{from: "http", to: "yes"}, {from: "https", to: "no"}}, scheme.replacedValues)
	assert.Equal(t, "name", label.replacedBy)
	assert.Nil(t, label.replacedValues)
	assert.Empty(t, fields[1].replacedBy)

	malformed := getStructFieldInfo(reflect.TypeFor[malformedReplacedValuesConfig](), nil)
	_, err := malformed[0].replacedValue("on")
	require.ErrorIs(t, err, ErrInvalidValueForType)
}

// TestFieldInfoReplacedValue verifies that a value maps in any case onto its
// replacement, that a value the mapping does not list fails naming the
// accepted values, and that a field without a mapping passes values as
// written.
func TestFieldInfoReplacedValue(t *testing.T) {
	t.Parallel()

	fields := getStructFieldInfo(reflect.TypeFor[replacedConfig](), nil)
	scheme, label := fields[0], fields[2]

	mapped, err := scheme.replacedValue("HTTP")
	require.NoError(t, err)
	assert.Equal(t, "yes", mapped)

	_, err = scheme.replacedValue("httpſ")
	require.ErrorIs(t, err, ErrInvalidValueForType)
	require.ErrorContains(t, err, "(accepted: http, https)")

	mapped, err = label.replacedValue("As Written")
	require.NoError(t, err)
	assert.Equal(t, "As Written", mapped)
}

// TestGetStructFieldInfoSensitive verifies that only a `sensitive:"true"` tag
// marks a field as sensitive.
func TestGetStructFieldInfoSensitive(t *testing.T) {
	t.Parallel()

	config := struct {
		Secret    string `sensitive:"true"`
		NotSecret string `sensitive:"false"`
		Plain     string
	}{}

	sensitive := map[string]bool{}
	for _, field := range getStructFieldInfo(reflect.TypeOf(config), nil) {
		sensitive[field.Name] = field.Sensitive
	}

	assert.Equal(t, map[string]bool{"Secret": true, "NotSecret": false, "Plain": false}, sensitive)
}

// TestFieldInfoIsEnum verifies that a field is an enum when it has an enum
// formatter and an int type.
func TestFieldInfoIsEnum(t *testing.T) {
	t.Parallel()

	assert.True(t, (&FieldInfo{Type: reflect.TypeFor[int](), EnumFormatter: testEnum}).IsEnum())
	assert.False(t, (&FieldInfo{Type: reflect.TypeFor[int]()}).IsEnum())
	assert.False(t, (&FieldInfo{Type: reflect.TypeFor[string](), EnumFormatter: testEnum}).IsEnum())
}

// TestFieldInfoIsURLPart verifies that a field reports the URL parts it maps
// to.
func TestFieldInfoIsURLPart(t *testing.T) {
	t.Parallel()

	field := &FieldInfo{URLParts: []URLPart{URLHost, URLPort}}

	assert.True(t, field.IsURLPart(URLHost))
	assert.True(t, field.IsURLPart(URLPort))
	assert.False(t, field.IsURLPart(URLPath))
	assert.False(t, (&FieldInfo{}).IsURLPart(URLHost))
}

// TestGetStructFieldInfoTestStruct verifies the shared test config's keyed
// fields, which the other format tests rely on.
func TestGetStructFieldInfoTestStruct(t *testing.T) {
	t.Parallel()

	fields := getStructFieldInfo(reflect.TypeFor[testStruct](), (&testStruct{}).Enums())
	require.Len(t, fields, reflect.TypeFor[testStruct]().NumField())

	keyed := map[string][]string{}

	for _, field := range fields {
		if len(field.Keys) > 0 {
			keyed[field.Name] = field.Keys
		}
	}

	assert.Equal(t, map[string][]string{
		"Signed": {"signed"}, "Duration": {"duration"}, "Str": {"str"}, "TestEnum": {"testenum"},
	}, keyed)
}
