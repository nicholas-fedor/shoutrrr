package format

import (
	"fmt"
	"reflect"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/nicholas-fedor/shoutrrr/pkg/types"
	"github.com/nicholas-fedor/shoutrrr/pkg/util"
)

// Node is the generic config tree item.
type Node interface {
	Field() *FieldInfo
	TokenType() NodeTokenType
	Update(tv reflect.Value)
}

// NodeTokenType is used to represent the type of value that a node has for syntax highlighting.
type NodeTokenType int

// ContainerNode is a Node with child items.
type ContainerNode struct {
	*FieldInfo

	Items        []Node
	MaxKeyLength int
}

// ValueNode is a Node without any child items.
type ValueNode struct {
	*FieldInfo

	Value     string
	tokenType NodeTokenType
}

const (
	// UnknownToken represents all unknown/unspecified tokens.
	UnknownToken NodeTokenType = iota
	// NumberToken represents all numbers.
	NumberToken
	// StringToken represents strings and keys.
	StringToken
	// EnumToken represents enum values.
	EnumToken
	// TrueToken represent boolean true.
	TrueToken
	// FalseToken represent boolean false.
	FalseToken
	// PropToken represent a serializable struct prop.
	PropToken
	// ErrorToken represent a value that was not serializable or otherwise invalid.
	ErrorToken
	// ContainerToken is used for Array/Slice and Map tokens.
	ContainerToken
)

// Constants for number bases.
const (
	BaseDecimalLen = 10
	BaseHexLen     = 16
)

// Field returns the FieldInfo associated with this value node.
//
// Returns:
//   - The node's FieldInfo.
func (n *ValueNode) Field() *FieldInfo {
	return n.FieldInfo
}

// TokenType returns the syntax-highlighting token type for this node's value.
//
// Returns:
//   - The NodeTokenType that matches the stored value.
func (n *ValueNode) TokenType() NodeTokenType {
	return n.tokenType
}

// Update refreshes the node's display string and token type from the provided value.
//
// Parameters:
//   - tv: The reflected field value to render.
func (n *ValueNode) Update(tv reflect.Value) {
	value, token := getValueNodeValue(tv, n.FieldInfo)
	n.Value = value
	n.tokenType = token
}

// Field returns the FieldInfo associated with this container node.
//
// Returns:
//   - The node's FieldInfo.
func (n *ContainerNode) Field() *FieldInfo {
	return n.FieldInfo
}

// TokenType returns ContainerToken for every container node.
//
// Returns:
//   - ContainerToken.
func (n *ContainerNode) TokenType() NodeTokenType {
	return ContainerToken
}

// Update rebuilds the container's child items to match the provided value.
//
// Array and slice values replace Items with one node per element.
// Map values replace Items with one node per key, sorted by key name.
// Other kinds are left unchanged.
//
// Parameters:
//   - tv: The reflected container value to expand into child items.
func (n *ContainerNode) Update(tv reflect.Value) {
	switch n.Type.Kind() {
	case reflect.Array, reflect.Slice:
		n.updateArrayNode(tv)
	case reflect.Map:
		n.updateMapNode(tv)
	case reflect.Invalid,
		reflect.Bool,
		reflect.Int,
		reflect.Int8,
		reflect.Int16,
		reflect.Int32,
		reflect.Int64,
		reflect.Uint,
		reflect.Uint8,
		reflect.Uint16,
		reflect.Uint32,
		reflect.Uint64,
		reflect.Uintptr,
		reflect.Float32,
		reflect.Float64,
		reflect.Complex64,
		reflect.Complex128,
		reflect.Chan,
		reflect.Func,
		reflect.Interface,
		reflect.Pointer,
		reflect.String,
		reflect.Struct,
		reflect.UnsafePointer:
		// No-op for unsupported kinds
	default:
		// No-op for any remaining kinds
	}
}

// updateArrayNode rebuilds the container's Items from an array or slice value.
//
// Each element becomes a ValueNode named by its index.
//
// Parameters:
//   - arrayValue: The reflected array or slice to expand.
func (n *ContainerNode) updateArrayNode(arrayValue reflect.Value) {
	itemCount := arrayValue.Len()
	n.Items = make([]Node, 0, itemCount)

	elemType := arrayValue.Type().Elem()

	for i := range itemCount {
		key := strconv.Itoa(i)
		val := arrayValue.Index(i)
		n.Items = append(n.Items, getValueNode(val, &FieldInfo{
			Name:          key,
			Type:          elemType,
			EnumFormatter: nil,
			Description:   "",
			DefaultValue:  "",
			Template:      "",
			Required:      false,
			URLParts:      nil,
			Title:         false,
			Base:          0,
			Keys:          nil,
			ItemSeparator: 0,
		}))
	}
}

// getArrayNode builds a ContainerNode whose children represent the elements of an array or slice.
//
// Parameters:
//   - arrayValue: The reflected array or slice to expand.
//   - fieldInfo: Metadata for the array or slice field.
//
// Returns:
//   - A ContainerNode with one child node per element.
func getArrayNode(arrayValue reflect.Value, fieldInfo *FieldInfo) *ContainerNode {
	node := &ContainerNode{
		FieldInfo:    fieldInfo,
		Items:        nil,
		MaxKeyLength: 0,
	}
	node.updateArrayNode(arrayValue)

	return node
}

// sortNodeItems sorts node items in place by their FieldInfo name.
//
// Parameters:
//   - nodeItems: The slice of nodes to sort.
func sortNodeItems(nodeItems []Node) {
	sort.Slice(nodeItems, func(i, j int) bool {
		return nodeItems[i].Field().Name < nodeItems[j].Field().Name
	})
}

// updateMapNode rebuilds the container's Items from a map value.
//
// Each map entry becomes a ValueNode named by its string key.
// Items are sorted by key, and MaxKeyLength is set to the longest key.
//
// Parameters:
//   - mapValue: The reflected map to expand.
func (n *ContainerNode) updateMapNode(mapValue reflect.Value) {
	base := n.Base
	if base == 0 {
		base = BaseDecimalLen
	}

	elemType := mapValue.Type().Elem()
	mapKeys := mapValue.MapKeys()
	nodeItems := make([]Node, len(mapKeys))
	maxKeyLength := 0

	for i, keyVal := range mapKeys {
		// The keys will always be strings
		key := keyVal.String()
		val := mapValue.MapIndex(keyVal)
		nodeItems[i] = getValueNode(val, &FieldInfo{
			Name:          key,
			Type:          elemType,
			EnumFormatter: nil,
			Description:   "",
			DefaultValue:  "",
			Template:      "",
			Required:      false,
			URLParts:      nil,
			Title:         false,
			Base:          base,
			Keys:          nil,
			ItemSeparator: 0,
		})
		maxKeyLength = util.Max(len(key), maxKeyLength)
	}

	sortNodeItems(nodeItems)

	n.Items = nodeItems
	n.MaxKeyLength = maxKeyLength
}

// getMapNode builds a ContainerNode whose children represent the entries of a map.
//
// Pointer values are dereferenced before the map is expanded.
//
// Parameters:
//   - mapValue: The reflected map, or a pointer to a map.
//   - fieldInfo: Metadata for the map field.
//
// Returns:
//   - A ContainerNode with one child node per map entry.
func getMapNode(mapValue reflect.Value, fieldInfo *FieldInfo) *ContainerNode {
	if mapValue.Kind() == reflect.Pointer {
		mapValue = mapValue.Elem()
	}

	node := &ContainerNode{
		FieldInfo:    fieldInfo,
		Items:        nil,
		MaxKeyLength: 0,
	}
	node.updateMapNode(mapValue)

	return node
}

// getNode builds the tree node that represents a single config field.
//
// Arrays, slices, and maps become ContainerNodes.
// Every other kind becomes a ValueNode.
//
// Parameters:
//   - fieldVal: The reflected field value.
//   - fieldInfo: Metadata for the field.
//
// Returns:
//   - A Node for the field.
func getNode(fieldVal reflect.Value, fieldInfo *FieldInfo) Node {
	switch fieldInfo.Type.Kind() {
	case reflect.Array, reflect.Slice:
		return getArrayNode(fieldVal, fieldInfo)
	case reflect.Map:
		return getMapNode(fieldVal, fieldInfo)
	case reflect.Invalid,
		reflect.Bool,
		reflect.Int,
		reflect.Int8,
		reflect.Int16,
		reflect.Int32,
		reflect.Int64,
		reflect.Uint,
		reflect.Uint8,
		reflect.Uint16,
		reflect.Uint32,
		reflect.Uint64,
		reflect.Uintptr,
		reflect.Float32,
		reflect.Float64,
		reflect.Complex64,
		reflect.Complex128,
		reflect.Chan,
		reflect.Func,
		reflect.Interface,
		reflect.Pointer,
		reflect.String,
		reflect.Struct,
		reflect.UnsafePointer:
		return getValueNode(fieldVal, fieldInfo)
	default:
		return getValueNode(fieldVal, fieldInfo)
	}
}

// getRootNode builds the root ContainerNode for a service config value.
//
// It reflects the struct fields, attaches enum formatters when the value
// implements types.Enummer, and sorts the resulting child nodes by field name.
//
// Parameters:
//   - value: The config value, typically a struct or a pointer to a struct.
//
// Returns:
//   - A ContainerNode whose children represent the config fields.
func getRootNode(value any) *ContainerNode {
	structValue := reflect.ValueOf(value)
	if structValue.Kind() == reflect.Pointer {
		structValue = structValue.Elem()
	}

	structType := structValue.Type()

	enums := map[string]types.EnumFormatter{}
	if enummer, isEnummer := value.(types.Enummer); isEnummer {
		enums = enummer.Enums()
	}

	infoFields := getStructFieldInfo(structType, enums)
	nodeItems := make([]Node, 0, len(infoFields))
	maxKeyLength := 0

	for i := range infoFields {
		fieldValue := structValue.FieldByName(infoFields[i].Name)
		if !fieldValue.IsValid() {
			fieldValue = reflect.Zero(infoFields[i].Type)
		}

		nodeItems = append(nodeItems, getNode(fieldValue, &infoFields[i]))
		maxKeyLength = util.Max(len(infoFields[i].Name), maxKeyLength)
	}

	sortNodeItems(nodeItems)

	return &ContainerNode{
		FieldInfo: &FieldInfo{
			Name:          "",
			Type:          structType,
			EnumFormatter: nil,
			Description:   "",
			DefaultValue:  "",
			Template:      "",
			Required:      false,
			URLParts:      nil,
			Title:         false,
			Base:          0,
			Keys:          nil,
			ItemSeparator: 0,
		},
		Items:        nodeItems,
		MaxKeyLength: maxKeyLength,
	}
}

// getValueNode wraps a field value in a ValueNode.
//
// The node's Value is the rendered display string and tokenType is the token used to color it.
//
// Parameters:
//   - fieldVal: The reflected field value to render.
//   - fieldInfo: Metadata for the field.
//
// Returns:
//   - A ValueNode pairing the display string with its token type.
func getValueNode(fieldVal reflect.Value, fieldInfo *FieldInfo) *ValueNode {
	value, tokenType := getValueNodeValue(fieldVal, fieldInfo)

	return &ValueNode{
		FieldInfo: fieldInfo,
		Value:     value,
		tokenType: tokenType,
	}
}

// getValueNodeValue renders a field value as its display string and token type.
//
// Enums print via their formatter and durations via Go duration syntax.
// All other kinds are formatted from their reflect.Kind.
//
// Parameters:
//   - fieldValue: The reflected field value to render.
//   - fieldInfo: Metadata for the field, including enum and duration type.
//
// Returns:
//   - The display string for the value.
//   - The NodeTokenType used to color that string.
func getValueNodeValue(fieldValue reflect.Value, fieldInfo *FieldInfo) (string, NodeTokenType) {
	kind := fieldValue.Kind()

	base := fieldInfo.Base
	if base == 0 {
		base = BaseDecimalLen
	}

	if fieldInfo.IsEnum() {
		return fieldInfo.EnumFormatter.Print(int(fieldValue.Int())), EnumToken
	}

	if fieldInfo.Type == durationType {
		return time.Duration(fieldValue.Int()).String(), StringToken
	}

	switch kind {
	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
		val := strconv.FormatUint(fieldValue.Uint(), base)
		if base == BaseHexLen {
			val = "0x" + val
		}

		return val, NumberToken
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		return strconv.FormatInt(fieldValue.Int(), base), NumberToken
	case reflect.String:
		return fieldValue.String(), StringToken
	case reflect.Bool:
		val := fieldValue.Bool()
		if val {
			return PrintBool(val), TrueToken
		}

		return PrintBool(val), FalseToken
	case reflect.Array, reflect.Slice, reflect.Map:
		return getContainerValueString(fieldValue, fieldInfo), UnknownToken
	case reflect.Pointer, reflect.Struct:
		if val, err := GetConfigPropString(fieldValue); err == nil {
			return val, PropToken
		}

		return "<ERR>", ErrorToken
	case reflect.Invalid,
		reflect.Uintptr,
		reflect.Float32,
		reflect.Float64,
		reflect.Complex64,
		reflect.Complex128,
		reflect.Chan,
		reflect.Func,
		reflect.Interface,
		reflect.UnsafePointer:
		return fmt.Sprintf("<?%s>", kind.String()), UnknownToken
	default:
		return fmt.Sprintf("<?%s>", kind.String()), UnknownToken
	}
}

// getContainerValueString renders an array, slice, or map as a single display string.
//
// Elements are joined with the field's ItemSeparator.
// Map entries are written as key:value pairs and sorted by key.
//
// Parameters:
//   - fieldValue: The reflected array, slice, or map to render.
//   - fieldInfo: Metadata for the container field, including ItemSeparator and Base.
//
// Returns:
//   - The joined display string for the container's items.
func getContainerValueString(fieldValue reflect.Value, fieldInfo *FieldInfo) string {
	itemSeparator := fieldInfo.ItemSeparator
	sliceLength := fieldValue.Len()

	var mapKeys []reflect.Value
	if fieldInfo.Type.Kind() == reflect.Map {
		mapKeys = fieldValue.MapKeys()
		sort.Slice(mapKeys, func(a, b int) bool {
			return mapKeys[a].String() < mapKeys[b].String()
		})
	}

	stringBuilder := strings.Builder{}

	var itemFieldInfo *FieldInfo

	for i := range sliceLength {
		var itemValue reflect.Value

		if i > 0 {
			stringBuilder.WriteRune(itemSeparator)
		}

		if mapKeys != nil {
			mapKey := mapKeys[i]
			stringBuilder.WriteString(mapKey.String())
			stringBuilder.WriteRune(':')

			itemValue = fieldValue.MapIndex(mapKey)
		} else {
			itemValue = fieldValue.Index(i)
		}

		if i == 0 {
			itemFieldInfo = &FieldInfo{
				Name:          "",
				Type:          itemValue.Type(),
				EnumFormatter: nil,
				Description:   "",
				DefaultValue:  "",
				Template:      "",
				Required:      false,
				URLParts:      nil,
				Title:         false,
				// Inherit the base from the container
				Base:          fieldInfo.Base,
				Keys:          nil,
				ItemSeparator: 0,
			}

			if itemFieldInfo.Base == 0 {
				itemFieldInfo.Base = BaseDecimalLen
			}
		}

		strVal, _ := getValueNodeValue(itemValue, itemFieldInfo)
		stringBuilder.WriteString(strVal)
	}

	return stringBuilder.String()
}
