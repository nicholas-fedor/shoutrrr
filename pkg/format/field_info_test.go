package format

import (
	"reflect"
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
// embedded and unexported fields are skipped.
func TestGetStructFieldInfo(t *testing.T) {
	t.Parallel()

	modes := CreateEnumFormatter([]string{"Off", "On"})
	config := taggedConfig{hidden: "unused"}

	fields := getStructFieldInfo(reflect.TypeOf(config), map[string]types.EnumFormatter{"Mode": modes})

	want := []FieldInfo{
		{
			Name: "Name", Type: reflect.TypeFor[string](), Description: "The name", Template: "{{.name}}",
			DefaultValue: "shoutrrr", Keys: []string{"name", "n"}, ItemSeparator: ',',
		},
		{Name: "Host", Type: reflect.TypeFor[string](), Required: true, URLParts: []URLPart{URLHost, URLPort}, ItemSeparator: ','},
		{Name: "Mode", Type: reflect.TypeFor[int](), EnumFormatter: modes, Base: DefaultBase, ItemSeparator: ','},
		{Name: "Title", Type: reflect.TypeFor[string](), Title: true, ItemSeparator: ','},
		{Name: "Items", Type: reflect.TypeFor[[]string](), Required: true, ItemSeparator: ';'},
		{Name: "Color", Type: reflect.TypeFor[uint](), Required: true, Base: 16, ItemSeparator: ','},
		{Name: "Count", Type: reflect.TypeFor[int](), Required: true, Base: DefaultBase, ItemSeparator: ','},
	}

	assert.Equal(t, want, fields)
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
