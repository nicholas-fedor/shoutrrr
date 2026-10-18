package format

import (
	"reflect"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/nicholas-fedor/shoutrrr/pkg/types"
)

// nodeConfig has a field for each node token type.
type nodeConfig struct {
	// Count is a number.
	Count int
	// Color is a number in base 16.
	Color uint `base:"16"`
	// Name is a string.
	Name string
	// Mode is an enum.
	Mode int
	// On is a true bool.
	On bool
	// Off is a false bool.
	Off bool
	// Prop is a config prop.
	Prop subPropStruct
	// Broken is a struct that is not a config prop.
	Broken subStruct
	// Ratio is a float, which the formatter does not support.
	Ratio float64
	// Wait is a duration.
	Wait time.Duration
	// Tags is a slice.
	Tags []string
	// Codes is a map of uints.
	Codes map[string]uint
	// Limits is a map.
	Limits map[string]int
	// Pairs is a slice with a custom separator.
	Pairs []string `sep:"|"`
	// hidden is unexported.
	hidden string
}

// nodeModes formats the Mode field of nodeConfig.
var nodeModes = CreateEnumFormatter([]string{"Off", "On"})

// newNodeConfig returns a node config with every field set.
//
// Returns:
//   - *nodeConfig: the config.
func newNodeConfig() *nodeConfig {
	return &nodeConfig{
		Count: -3, Color: 0xff, Name: "shoutrrr", Mode: 1, On: true, Prop: subPropStruct{Value: "a"},
		Ratio: 1.5, Wait: 90 * time.Second, Tags: []string{"x", "y"}, Codes: map[string]uint{"b": 0x10, "a": 0xff},
		Limits: map[string]int{"long-key": 1, "k": -2}, Pairs: []string{"p", "q"}, hidden: "unused",
	}
}

// itemsByName returns a container's items by field name.
//
// Parameters:
//   - node: the container.
//
// Returns:
//   - map[string]Node: the items by field name.
func itemsByName(node *ContainerNode) map[string]Node {
	items := make(map[string]Node, len(node.Items))
	for _, item := range node.Items {
		items[item.Field().Name] = item
	}

	return items
}

// TestGetRootNode verifies that the root node lists the exported fields in
// name order with the longest name length, reads enums from the config, and
// uses a zero value for a field the value does not have.
func TestGetRootNode(t *testing.T) {
	t.Parallel()

	root := getRootNode(newNodeConfig())

	names := make([]string, 0, len(root.Items))
	for _, item := range root.Items {
		names = append(names, item.Field().Name)
	}

	assert.Equal(t, []string{
		"Broken", "Codes", "Color", "Count", "Limits", "Mode", "Name", "Off", "On", "Pairs", "Prop", "Ratio", "Tags", "Wait",
	}, names)
	assert.Equal(t, len("Broken"), root.MaxKeyLength)
	assert.Equal(t, reflect.TypeFor[nodeConfig](), root.Type)
	assert.Equal(t, ContainerToken, root.TokenType())
	assert.Same(t, root.FieldInfo, root.Field())

	enums := getRootNode(&testStruct{TestEnum: 2})
	assert.Equal(t, "Bar", itemsByName(enums)["TestEnum"].(*ValueNode).Value)

	value := getRootNode(testStruct{Str: "by value"})
	assert.Equal(t, "by value", itemsByName(value)["Str"].(*ValueNode).Value)
}

// TestValueNodes verifies the value and token type of each scalar field.
func TestValueNodes(t *testing.T) {
	t.Parallel()

	config := newNodeConfig()
	value := reflect.ValueOf(config).Elem()
	fields := getStructFieldInfo(value.Type(), map[string]types.EnumFormatter{"Mode": nodeModes})

	// tests lists fields with the value and token type each node should have.
	tests := []struct {
		field string
		value string
		token NodeTokenType
	}{
		{field: "Count", value: "-3", token: NumberToken},
		{field: "Color", value: "0xff", token: NumberToken},
		{field: "Name", value: "shoutrrr", token: StringToken},
		{field: "Mode", value: "On", token: EnumToken},
		{field: "On", value: "Yes", token: TrueToken},
		{field: "Off", value: "No", token: FalseToken},
		{field: "Prop", value: "@a", token: PropToken},
		{field: "Broken", value: "<ERR>", token: ErrorToken},
		{field: "Ratio", value: "<?float64>", token: UnknownToken},
		{field: "Wait", value: "1m30s", token: StringToken},
	}

	for _, tt := range tests {
		t.Run(tt.field, func(t *testing.T) {
			t.Parallel()

			var field *FieldInfo

			for i := range fields {
				if fields[i].Name == tt.field {
					field = &fields[i]
				}
			}

			require.NotNil(t, field)

			node, ok := getNode(value.FieldByName(tt.field), field).(*ValueNode)
			require.True(t, ok)

			assert.Equal(t, tt.value, node.Value)
			assert.Equal(t, tt.token, node.TokenType())
			assert.Same(t, field, node.Field())
		})
	}
}

// TestValueNodeUpdate verifies that updating a value node reads the new value
// and its token type.
func TestValueNodeUpdate(t *testing.T) {
	t.Parallel()

	config := &nodeConfig{}
	items := itemsByName(getRootNode(config))

	on, ok := items["On"].(*ValueNode)
	require.True(t, ok)
	assert.Equal(t, FalseToken, on.TokenType())

	on.Update(reflect.ValueOf(true))

	assert.Equal(t, "Yes", on.Value)
	assert.Equal(t, TrueToken, on.TokenType())
}

// TestContainerNodes verifies that slices list their items by index and that
// maps list their items in key order with the longest key length, numbers in
// base 10 unless the map's field info sets a base.
func TestContainerNodes(t *testing.T) {
	t.Parallel()

	items := itemsByName(getRootNode(newNodeConfig()))

	tags, ok := items["Tags"].(*ContainerNode)
	require.True(t, ok)
	assert.Equal(t, []Node{
		&ValueNode{FieldInfo: &FieldInfo{Name: "0", Type: reflect.TypeFor[string]()}, Value: "x", tokenType: StringToken},
		&ValueNode{FieldInfo: &FieldInfo{Name: "1", Type: reflect.TypeFor[string]()}, Value: "y", tokenType: StringToken},
	}, tags.Items)

	codes, ok := items["Codes"].(*ContainerNode)
	require.True(t, ok)
	assert.Equal(t, []Node{
		&ValueNode{FieldInfo: &FieldInfo{Name: "a", Type: reflect.TypeFor[uint](), Base: DefaultBase}, Value: "255", tokenType: NumberToken},
		&ValueNode{FieldInfo: &FieldInfo{Name: "b", Type: reflect.TypeFor[uint](), Base: DefaultBase}, Value: "16", tokenType: NumberToken},
	}, codes.Items)
	assert.Equal(t, 1, codes.MaxKeyLength)

	hex := getMapNode(reflect.ValueOf(map[string]uint{"a": 0xff}), &FieldInfo{Type: reflect.TypeFor[map[string]uint](), Base: 16})
	assert.Equal(t, "0xff", hex.Items[0].(*ValueNode).Value)

	limits, ok := items["Limits"].(*ContainerNode)
	require.True(t, ok)
	assert.Equal(t, len("long-key"), limits.MaxKeyLength)
	assert.Equal(t, "k", limits.Items[0].Field().Name)
	assert.Equal(t, DefaultBase, limits.Items[0].Field().Base)
}

// TestContainerNodeUpdate verifies that updating a container reads the new
// items of a slice or map, and that updating a container of another kind
// changes nothing.
func TestContainerNodeUpdate(t *testing.T) {
	t.Parallel()

	items := itemsByName(getRootNode(&nodeConfig{}))

	tags, ok := items["Tags"].(*ContainerNode)
	require.True(t, ok)
	assert.Empty(t, tags.Items)

	tags.Update(reflect.ValueOf([]string{"z"}))
	require.Len(t, tags.Items, 1)
	assert.Equal(t, "z", tags.Items[0].(*ValueNode).Value)

	limits, ok := items["Limits"].(*ContainerNode)
	require.True(t, ok)

	limits.Update(reflect.ValueOf(map[string]int{"abc": 1}))
	require.Len(t, limits.Items, 1)
	assert.Equal(t, 3, limits.MaxKeyLength)

	other := &ContainerNode{FieldInfo: &FieldInfo{Type: reflect.TypeFor[string]()}, Items: []Node{tags}}
	other.Update(reflect.ValueOf("text"))
	assert.Equal(t, []Node{tags}, other.Items)
}

// TestGetMapNodePointer verifies that a map node reads through a pointer to
// the map.
func TestGetMapNodePointer(t *testing.T) {
	t.Parallel()

	values := map[string]string{"a": "1"}
	node := getMapNode(reflect.ValueOf(&values), &FieldInfo{Type: reflect.TypeFor[map[string]string]()})

	require.Len(t, node.Items, 1)
	assert.Equal(t, "1", node.Items[0].(*ValueNode).Value)
}

// TestGetContainerValueString verifies that slices and maps are written as
// their items joined by the field's separator, with map items in key order as
// key:value and numbers in the base of the field info.
func TestGetContainerValueString(t *testing.T) {
	t.Parallel()

	config := newNodeConfig()
	value := reflect.ValueOf(config).Elem()
	fields := getStructFieldInfo(value.Type(), nil)

	written := map[string]string{}

	for i := range fields {
		switch fields[i].Name {
		case "Tags", "Codes", "Limits", "Pairs":
			written[fields[i].Name], _ = getValueNodeValue(value.FieldByName(fields[i].Name), &fields[i])
		}
	}

	assert.Equal(t, map[string]string{
		"Tags": "x,y", "Codes": "a:255,b:16", "Limits": "k:-2,long-key:1", "Pairs": "p|q",
	}, written)

	hex, _ := getValueNodeValue(reflect.ValueOf(map[string]uint{"a": 0xff}),
		&FieldInfo{Type: reflect.TypeFor[map[string]uint](), Base: 16, ItemSeparator: ','})
	assert.Equal(t, "a:0xff", hex)

	empty, token := getValueNodeValue(reflect.ValueOf([]string{}), &FieldInfo{Type: reflect.TypeFor[[]string](), ItemSeparator: ','})
	assert.Empty(t, empty)
	assert.Equal(t, UnknownToken, token)
}
