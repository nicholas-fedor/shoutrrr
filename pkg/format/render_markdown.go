package format

import (
	"reflect"
	"sort"
	"strings"
)

// MarkdownTreeRenderer renders a ContainerNode tree into a markdown documentation string.
type MarkdownTreeRenderer struct {
	HeaderPrefix      string
	PropsDescription  string
	PropsEmptyMessage string
}

// Constants for dynamic path segment offsets.
const (
	PathOffset1 = 1
	PathOffset2 = 2
	PathOffset3 = 3
)

// RenderTree renders a ContainerNode tree into a markdown documentation string.
func (r MarkdownTreeRenderer) RenderTree(root *ContainerNode, scheme string) string {
	stringBuilder := strings.Builder{}

	queryFields := make([]*FieldInfo, 0, len(root.Items))
	urlFields := make([]*FieldInfo, 0, len(root.Items)) // Zero length, capacity for all fields
	dynamicURLFields := make([]*FieldInfo, 0, len(root.Items))

	for _, node := range root.Items {
		field := node.Field()
		for _, urlPart := range field.URLParts {
			switch urlPart {
			case URLQuery:
				queryFields = append(queryFields, field)
			case URLPath + PathOffset1,
				URLPath + PathOffset2,
				URLPath + PathOffset3:
				dynamicURLFields = append(dynamicURLFields, field)
			case URLUser, URLPassword, URLHost, URLPort, URLPath:
				urlFields = append(urlFields, field)
			}
		}

		if len(field.URLParts) < 1 {
			queryFields = append(queryFields, field)
		}
	}

	// Append dynamic fields to urlFields
	urlFields = append(urlFields, dynamicURLFields...)

	// Sort by primary URLPart
	sort.SliceStable(urlFields, func(i, j int) bool {
		urlPartA := URLQuery
		if len(urlFields[i].URLParts) > 0 {
			urlPartA = urlFields[i].URLParts[0]
		}

		urlPartB := URLQuery
		if len(urlFields[j].URLParts) > 0 {
			urlPartB = urlFields[j].URLParts[0]
		}

		return urlPartA < urlPartB
	})

	r.writeURLFields(&stringBuilder, urlFields, scheme)

	sort.SliceStable(queryFields, func(i, j int) bool {
		return queryFields[i].Required && !queryFields[j].Required
	})

	r.writeHeader(&stringBuilder, "Query/Param Props")

	if len(queryFields) > 0 {
		stringBuilder.WriteString(r.PropsDescription)
	} else {
		stringBuilder.WriteString(r.PropsEmptyMessage)
	}

	stringBuilder.WriteRune('\n')

	for _, field := range queryFields {
		r.writeFieldPrimary(&stringBuilder, field)
		r.writeFieldExtras(&stringBuilder, field)
		stringBuilder.WriteRune('\n')
	}

	return stringBuilder.String()
}

func (r MarkdownTreeRenderer) writeURLFields(
	stringBuilder *strings.Builder,
	urlFields []*FieldInfo,
	scheme string,
) {
	fieldsPrinted := make(map[string]bool)

	r.writeHeader(stringBuilder, "URL Fields")

	for _, field := range urlFields {
		if field == nil || fieldsPrinted[field.Name] {
			continue
		}

		r.writeFieldPrimary(stringBuilder, field)

		stringBuilder.WriteString("  URL part: <code class=\"service-url\">")
		stringBuilder.WriteString(scheme)
		stringBuilder.WriteString("://")

		// Check for presence of URLUser or URLPassword
		hasUser := false
		hasPassword := false
		maxPart := URLUser // Track the highest URLPart used

		for _, f := range urlFields {
			if f != nil {
				for _, part := range f.URLParts {
					switch part {
					case URLQuery, URLHost, URLPort, URLPath: // No-op for these cases
					case URLUser:
						hasUser = true
					case URLPassword:
						hasPassword = true
					}

					if part > maxPart {
						maxPart = part
					}
				}
			}
		}

		// Build URL with this field highlighted
		for i := URLUser; i <= URLPath+PathOffset3; i++ {
			urlPart := i
			for _, fieldInfo := range urlFields {
				if fieldInfo != nil && fieldInfo.IsURLPart(urlPart) {
					if i > URLUser {
						lastPart := i - 1
						if lastPart == URLPassword && (hasUser || hasPassword) {
							stringBuilder.WriteRune(
								lastPart.Suffix(),
							) // ':' only if credentials present
						} else if lastPart != URLPassword {
							stringBuilder.WriteRune(lastPart.Suffix()) // '/' or '@'
						}
					}

					slug := strings.ToLower(fieldInfo.Name)
					if slug == "host" && urlPart == URLPort {
						slug = "port"
					}

					if fieldInfo == field {
						stringBuilder.WriteString("<strong>")
						stringBuilder.WriteString(slug)
						stringBuilder.WriteString("</strong>")
					} else {
						stringBuilder.WriteString(slug)
					}

					break
				}
			}
		}

		// Add trailing '/' if no dynamic path segments follow
		if maxPart < URLPath+PathOffset1 {
			stringBuilder.WriteRune('/')
		}

		stringBuilder.WriteString("</code>  \n")

		fieldsPrinted[field.Name] = true
	}
}

func (MarkdownTreeRenderer) writeFieldExtras(stringBuilder *strings.Builder, field *FieldInfo) {
	if len(field.Keys) > 1 {
		stringBuilder.WriteString("  Aliases: `")

		for i, key := range field.Keys {
			if i == 0 {
				// Skip primary alias (as it's the same as the field name)
				continue
			}

			if i > 1 {
				stringBuilder.WriteString("`, `")
			}

			stringBuilder.WriteString(key)
		}

		stringBuilder.WriteString("`  \n")
	}

	if field.EnumFormatter != nil {
		stringBuilder.WriteString("  Possible values: `")

		for i, name := range field.EnumFormatter.Names() {
			if i != 0 {
				stringBuilder.WriteString("`, `")
			}

			stringBuilder.WriteString(name)
		}

		stringBuilder.WriteString("`  \n")
	}
}

func (MarkdownTreeRenderer) writeFieldPrimary(stringBuilder *strings.Builder, field *FieldInfo) {
	fieldKey := field.Name

	stringBuilder.WriteString("*  __")
	stringBuilder.WriteString(fieldKey)
	stringBuilder.WriteString("__")

	if field.Description != "" {
		stringBuilder.WriteString(" - ")
		stringBuilder.WriteString(field.Description)
	}

	if field.Required {
		stringBuilder.WriteString(" (**Required**)  \n")
	} else {
		stringBuilder.WriteString("  \n  Default: ")

		if field.DefaultValue == "" {
			stringBuilder.WriteString("*empty*")
		} else {
			if field.Type.Kind() == reflect.Bool {
				defaultValue, _ := ParseBool(field.DefaultValue, false)
				if defaultValue {
					stringBuilder.WriteString("✔ ")
				} else {
					stringBuilder.WriteString("❌ ")
				}
			}

			stringBuilder.WriteRune('`')
			stringBuilder.WriteString(field.DefaultValue)
			stringBuilder.WriteRune('`')
		}

		stringBuilder.WriteString("  \n")
	}
}

func (r MarkdownTreeRenderer) writeHeader(stringBuilder *strings.Builder, text string) {
	stringBuilder.WriteString(r.HeaderPrefix)
	stringBuilder.WriteString(text)
	stringBuilder.WriteString("\n\n")
}
