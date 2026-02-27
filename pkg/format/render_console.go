package format

import (
	"fmt"
	"strings"

	"github.com/nicholas-fedor/shoutrrr/pkg/color"
	"github.com/nicholas-fedor/shoutrrr/pkg/util"
)

// ConsoleTreeRenderer renders a ContainerNode tree into an ansi-colored console string.
type ConsoleTreeRenderer struct {
	WithValues bool
}

// Constants for console rendering.
const (
	DescriptionColumnWidth = 60 // Width of the description column in console output
	ItemSeparatorLength    = 2  // Length of the ", " separator between container items
	DefaultValueOffset     = 16 // Minimum offset before description when no values are shown
	ValueOffset            = 30 // Offset before description when values are shown
	ContainerBracketLength = 4  // Length of container delimiters (e.g., "{ }" or "[ ]")
	KeySeparatorLength     = 2  // Length of the ": " separator after a key in containers
)

// RenderTree renders a ContainerNode tree into an ansi-colored console string.
func (r ConsoleTreeRenderer) RenderTree(root *ContainerNode, _ string) string {
	stringBuilder := strings.Builder{}

	for _, node := range root.Items {
		fieldKey := node.Field().Name
		stringBuilder.WriteString(fieldKey)

		for i := len(fieldKey); i <= root.MaxKeyLength; i++ {
			stringBuilder.WriteRune(' ')
		}

		var valueLen int // Initialize without assignment; set later

		preLen := DefaultValueOffset // Default spacing before the description when no values are rendered

		field := node.Field()

		if r.WithValues {
			preLen = ValueOffset // Adjusts the spacing when values are included
			valueLen = r.writeNodeValue(&stringBuilder, node)
		} else {
			// Since no values were supplied, substitute the value with the type
			typeName := field.Type.String()

			// If the value is an enum type, providing the name is a bit pointless
			// Instead, use a common string "option" to signify the type
			if field.EnumFormatter != nil {
				typeName = "option"
			}

			valueLen = len(typeName)
			stringBuilder.WriteString(color.CyanString(typeName))
		}

		stringBuilder.WriteString(strings.Repeat(" ", util.Max(preLen-valueLen, 1)))
		stringBuilder.WriteString(ColorizeDesc(field.Description))
		stringBuilder.WriteString(
			strings.Repeat(" ", util.Max(DescriptionColumnWidth-len(field.Description), 1)),
		)

		if len(field.URLParts) > 0 && field.URLParts[0] != URLQuery {
			stringBuilder.WriteString(" <URL: ")

			for i, part := range field.URLParts {
				if i > 0 {
					stringBuilder.WriteString(", ")
				}

				if part > URLPath {
					part = URLPath
				}

				stringBuilder.WriteString(ColorizeEnum(part))
			}

			stringBuilder.WriteString(">")
		}

		if field.Template != "" {
			fmt.Fprintf(&stringBuilder, " <Template: %s>", ColorizeString(field.Template))
		}

		if field.DefaultValue != "" {
			fmt.Fprintf(&stringBuilder, " <Default: %s>",
				ColorizeValue(field.DefaultValue, field.EnumFormatter != nil))
		}

		if field.Required {
			fmt.Fprintf(&stringBuilder, " <%s>", ColorizeFalse("Required"))
		}

		if len(field.Keys) > 1 {
			stringBuilder.WriteString(" <Aliases: ")

			for i, key := range field.Keys {
				if i == 0 {
					// Skip primary alias (as it's the same as the field name)
					continue
				}

				if i > 1 {
					stringBuilder.WriteString(", ")
				}

				stringBuilder.WriteString(ColorizeString(key))
			}

			stringBuilder.WriteString(">")
		}

		if field.EnumFormatter != nil {
			stringBuilder.WriteString(ColorizeContainer(" ["))

			for i, name := range field.EnumFormatter.Names() {
				if i != 0 {
					stringBuilder.WriteString(", ")
				}

				stringBuilder.WriteString(ColorizeEnum(name))
			}

			stringBuilder.WriteString(ColorizeContainer("]"))
		}

		stringBuilder.WriteRune('\n')
	}

	return stringBuilder.String()
}

func (r ConsoleTreeRenderer) writeContainer(
	stringBuilder *strings.Builder,
	node *ContainerNode,
) int {
	kind := node.Type.Kind()
	hasKeys := !util.IsCollection(kind)

	totalLen := ContainerBracketLength // Length of the opening and closing brackets ({ } or [ ])

	if hasKeys {
		stringBuilder.WriteString("{ ")
	} else {
		stringBuilder.WriteString("[ ")
	}

	for i, itemNode := range node.Items {
		if i != 0 {
			stringBuilder.WriteString(", ")

			totalLen += KeySeparatorLength // Accounts for the : separator between keys and values in containers
		}

		if hasKeys {
			itemKey := itemNode.Field().Name
			stringBuilder.WriteString(itemKey)
			stringBuilder.WriteString(": ")

			totalLen += len(itemKey) + ItemSeparatorLength
		}

		valLen := r.writeNodeValue(stringBuilder, itemNode)
		totalLen += valLen
	}

	if hasKeys {
		stringBuilder.WriteString(" }")
	} else {
		stringBuilder.WriteString(" ]")
	}

	return totalLen
}

func (r ConsoleTreeRenderer) writeNodeValue(stringBuilder *strings.Builder, node Node) int {
	if contNode, isContainer := node.(*ContainerNode); isContainer {
		return r.writeContainer(stringBuilder, contNode)
	}

	if valNode, isValue := node.(*ValueNode); isValue {
		stringBuilder.WriteString(ColorizeToken(valNode.Value, valNode.tokenType))

		return len(valNode.Value)
	}

	stringBuilder.WriteByte('?')

	return 1
}
