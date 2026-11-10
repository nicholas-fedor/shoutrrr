package format

import (
	"reflect"
	"sort"
	"strings"
	"unicode"
)

// MarkdownTreeRenderer renders a ContainerNode tree into a markdown documentation string.
type MarkdownTreeRenderer struct {
	// HeaderPrefix is the prefix string added to section headers (e.g., "## " for H2 headers).
	HeaderPrefix string
	// PropsDescription is the introductory text displayed before the query/param properties list.
	PropsDescription string
	// PropsEmptyMessage is the message displayed when no query/param properties are available.
	PropsEmptyMessage string
}

// Constants for dynamic path segment offsets.
// These offsets are added to URLPath to represent dynamic path segments beyond the base path.
const (
	// PathOffset1 represents the first dynamic path segment (URLPath + 1).
	PathOffset1 = 1
	// PathOffset2 represents the second dynamic path segment (URLPath + 2).
	PathOffset2 = 2
	// PathOffset3 represents the third dynamic path segment (URLPath + 3).
	PathOffset3 = 3
)

// RenderTree renders a ContainerNode tree into a markdown documentation string.
//
// The fields are split into URL fields and query or param props. URL fields
// are listed in URL order, each with its place in the URL. Props are listed
// required ones first, with their defaults, aliases, and possible values.
//
// Parameters:
//   - root: The container node tree containing all field information to render.
//   - scheme: The URL scheme (e.g., "https", "smtp") used when displaying URL field examples.
//
// Returns:
//   - string: the markdown documentation.
func (r MarkdownTreeRenderer) RenderTree(root *ContainerNode, scheme string) string {
	stringBuilder := strings.Builder{}

	// Initialize slices to categorize fields by their URL part types.
	// queryFields holds fields that are query parameters or have no URL parts.
	queryFields := make([]*FieldInfo, 0, len(root.Items))
	// urlFields holds fields that map to standard URL components (user, password, host, port, path).
	urlFields := make([]*FieldInfo, 0, len(root.Items))
	// dynamicURLFields holds fields that map to dynamic path segments (path offsets 1-3).
	dynamicURLFields := make([]*FieldInfo, 0, len(root.Items))

	// Categorize each field based on its URLParts.
	for _, node := range root.Items {
		field := node.Field()
		for _, urlPart := range field.URLParts {
			switch {
			case urlPart == URLQuery:
				// Query parameter fields are added to queryFields.
				queryFields = append(queryFields, field)
			case urlPart > URLPath:
				// Numbered path segments after the first are added to dynamicURLFields.
				dynamicURLFields = append(dynamicURLFields, field)
			default:
				// Standard URL components are added to urlFields.
				urlFields = append(urlFields, field)
			}
		}

		// Fields with no URL parts default to queryFields.
		if len(field.URLParts) < 1 {
			queryFields = append(queryFields, field)
		}
	}

	// Append dynamic fields to urlFields so they appear in the URL section.
	urlFields = append(urlFields, dynamicURLFields...)

	// Sort urlFields by their primary URLPart to ensure consistent ordering.
	// The order follows the URL structure, from the user to the path.
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

	// Generate the URL Fields section.
	r.writeURLFields(&stringBuilder, urlFields, scheme)

	// Add blank line between URL Fields section and Query/Param Props section.
	if len(urlFields) > 0 {
		stringBuilder.WriteString("\n")
	}

	// Sort queryFields to place required fields first, followed by optional fields.
	sort.SliceStable(queryFields, func(i, j int) bool {
		return queryFields[i].Required && !queryFields[j].Required
	})

	// Filter out fields without key tags, as they cannot be set via query parameters.
	keyedFields := make([]*FieldInfo, 0, len(queryFields))
	for _, field := range queryFields {
		if len(field.Keys) > 0 {
			keyedFields = append(keyedFields, field)
		}
	}

	// Generate the Query/Param Props section header.
	r.writeHeader(&stringBuilder, "Query/Param Props")

	// Add either the description or empty message based on whether query fields exist.
	if len(keyedFields) > 0 {
		stringBuilder.WriteString(r.PropsDescription)
	} else {
		stringBuilder.WriteString(r.PropsEmptyMessage)
	}

	// Add a blank line after the description if both exist.
	if len(keyedFields) > 0 && r.PropsDescription != "" {
		// If PropsDescription doesn't end with a newline, we need to end the line
		// and add a blank line (\n\n). If it already ends with a newline, we only
		// need to add one more for the blank line.
		if strings.HasSuffix(r.PropsDescription, "\n") {
			stringBuilder.WriteString("\n")
		} else {
			stringBuilder.WriteString("\n\n")
		}
	}

	// Generate documentation for each query field.
	for i, field := range keyedFields {
		// Add blank line between fields.
		if i > 0 {
			stringBuilder.WriteString("\n")
		}

		// Write the primary field information (name, description, required/default).
		r.writeFieldPrimary(&stringBuilder, field)

		// Write additional field information (aliases, enum values).
		r.writeFieldExtras(&stringBuilder, field)
	}

	return stringBuilder.String()
}

// writeFieldExtras writes additional field information including aliases, deprecated keys,
// and possible enum values.
//
// Parameters:
//   - stringBuilder: The string builder to append the formatted output to.
//   - field: The field information to extract aliases and enum values from.
func (MarkdownTreeRenderer) writeFieldExtras(stringBuilder *strings.Builder, field *FieldInfo) {
	// Output aliases if the field has more than one key (alternative names).
	if len(field.Keys) > 1 {
		stringBuilder.WriteString("  Aliases: `")

		for i, key := range field.Keys {
			if i == 0 {
				// Skip the primary key since it is shown as the field name above.
				continue
			}

			if i > 1 {
				stringBuilder.WriteString("`, `")
			}

			stringBuilder.WriteString(key)
		}

		stringBuilder.WriteString("`\n")
	}

	// Output deprecated keys, which still work but should be replaced by the field's key.
	if len(field.DeprecatedKeys) > 0 {
		stringBuilder.WriteString("  Deprecated: `")
		stringBuilder.WriteString(strings.Join(field.DeprecatedKeys, "`, `"))
		stringBuilder.WriteString("`\n")
	}

	// Output possible enum values if the field has an enum formatter.
	if field.EnumFormatter != nil {
		stringBuilder.WriteString("  Possible values: `")

		for i, name := range field.EnumFormatter.Names() {
			if i != 0 {
				stringBuilder.WriteString("`, `")
			}

			stringBuilder.WriteString(name)
		}

		stringBuilder.WriteString("`\n")
	}
}

// writeFieldPrimary writes the primary field information including name, description,
// required status, and default value.
//
// Parameters:
//   - stringBuilder: The string builder to append the formatted output to.
//   - field: The field information to render.
func (MarkdownTreeRenderer) writeFieldPrimary(stringBuilder *strings.Builder, field *FieldInfo) {
	fieldKey := field.Name
	if len(field.Keys) > 0 {
		fieldKey = field.Keys[0]
	}

	// Write the field name in bold.
	stringBuilder.WriteString("* __")
	stringBuilder.WriteString(fieldKey)
	stringBuilder.WriteString("__")

	// Append the field description if available.
	if field.Description != "" {
		stringBuilder.WriteString(" - ")
		stringBuilder.WriteString(field.Description)
	}

	// Mark required fields or show the default value for optional fields.
	if field.Required {
		stringBuilder.WriteString(" (__Required__)\n")
	} else {
		stringBuilder.WriteString("\n  Default: ")

		// Display "*empty*" for fields with no default value.
		if field.DefaultValue == "" {
			stringBuilder.WriteString("*empty*")
		} else {
			// For boolean fields, prepend a checkmark or xmark based on the value.
			if field.Type.Kind() == reflect.Bool {
				defaultValue, _ := ParseBool(field.DefaultValue, false)
				if defaultValue {
					stringBuilder.WriteString("✔ ")
				} else {
					stringBuilder.WriteString("❌ ")
				}
			}

			// Wrap the default value in backticks for code formatting.
			stringBuilder.WriteByte('`')
			stringBuilder.WriteString(field.DefaultValue)
			stringBuilder.WriteByte('`')
		}

		stringBuilder.WriteString("\n")
	}
}

// writeHeader writes a section header with the configured prefix.
//
// Parameters:
//   - stringBuilder: The string builder to append the formatted header to.
//   - text: The header text to write.
func (r MarkdownTreeRenderer) writeHeader(stringBuilder *strings.Builder, text string) {
	stringBuilder.WriteString(r.HeaderPrefix)
	stringBuilder.WriteString(text)
	stringBuilder.WriteString("\n\n")
}

// writeURLFields writes the URL Fields section with an example URL for each field.
//
// Each example shows the whole URL with the field's part in bold. Every part is
// an uppercase placeholder named after its field, such as WEBHOOK_ID for a
// WebhookID field. The port of a host field is PORT.
//
// Parameters:
//   - stringBuilder: The string builder to append the formatted output to.
//   - urlFields: The list of fields that are part of the URL.
//   - scheme: The URL scheme to use in the example URLs.
func (r MarkdownTreeRenderer) writeURLFields(
	stringBuilder *strings.Builder,
	urlFields []*FieldInfo,
	scheme string,
) {
	// Track which fields have been printed to avoid duplicates.
	fieldsPrinted := make(map[string]bool)

	// Write the section header.
	r.writeHeader(stringBuilder, "URL Fields")

	// Generate documentation for each unique URL field.
	for _, field := range urlFields {
		// Skip nil fields and fields that have already been processed.
		if field == nil || fieldsPrinted[field.Name] {
			continue
		}

		// Write the primary field information.
		r.writeFieldPrimary(stringBuilder, field)

		// Write the URL example on a new line within the same list item.
		stringBuilder.WriteString("<br>URL part: <code class=\"service-url\">")
		stringBuilder.WriteString(scheme)
		stringBuilder.WriteString("://")

		// Detect the presence of user and password fields for proper URL formatting.
		hasUser := false
		hasPassword := false
		// Track the highest URLPart to determine if trailing slash is needed.
		maxPart := URLUser

		for _, f := range urlFields {
			if f != nil {
				for _, part := range f.URLParts {
					switch part {
					case URLQuery, URLHost, URLPort, URLPath: // No-op for these cases.
					case URLUser:
						hasUser = true
					case URLPassword:
						hasPassword = true
					}

					// Update the maximum URLPart encountered.
					if part > maxPart {
						maxPart = part
					}
				}
			}
		}

		// Build the URL with the current field highlighted.
		// Iterate through the URL parts up to the highest one present to construct the complete URL structure.
		for i := URLUser; i <= maxPart; i++ {
			for _, fieldInfo := range urlFields {
				// Skip fields that don't correspond to the current URL part.
				if fieldInfo == nil || !fieldInfo.IsURLPart(i) {
					continue
				}

				// Add the appropriate separator between URL components.
				if i > URLUser {
					lastPart := i - 1
					if lastPart == URLPassword && (hasUser || hasPassword) {
						// Add ':' separator only if credentials are present.
						stringBuilder.WriteRune(lastPart.Suffix())
					} else if lastPart != URLPassword {
						// Add standard separators like '/' or '@'.
						stringBuilder.WriteRune(lastPart.Suffix())
					}
				}

				slug := placeholderName(fieldInfo.Name)
				// The port of a host field is written as its own placeholder.
				if slug == "HOST" && i == URLPort {
					slug = "PORT"
				}

				// Highlight the current field with bold HTML tags.
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

		// Add a trailing '/' when the URL has no path.
		if maxPart < URLPath {
			stringBuilder.WriteByte('/')
		}

		stringBuilder.WriteString("</code>\n")

		// Mark this field as printed to avoid duplicates.
		fieldsPrinted[field.Name] = true
	}
}

// placeholderName returns the uppercase placeholder for a field name, with an
// underscore between its words.
//
// An uppercase letter after a lowercase letter or a digit starts a word. So
// does a digit after a letter, and the last uppercase letter of an acronym
// when two lowercase letters follow it. WebhookID becomes WEBHOOK_ID, APIKey
// becomes API_KEY, and Path1 becomes PATH_1. A plural acronym such as IDs
// stays one word.
//
// Parameters:
//   - name: the Go name of the field.
//
// Returns:
//   - string: the placeholder.
func placeholderName(name string) string {
	runes := []rune(name)
	placeholder := strings.Builder{}
	placeholder.Grow(len(name) + len(runes)/2)

	for i, current := range runes {
		if i > 0 && startsWord(runes[i-1], current, runes[i+1:]) {
			placeholder.WriteByte('_')
		}

		placeholder.WriteRune(unicode.ToUpper(current))
	}

	return placeholder.String()
}

// startsWord reports whether a rune of a field name starts a new word.
//
// Parameters:
//   - previous: the rune before current.
//   - current: the rune to check.
//   - rest: the runes after current.
//
// Returns:
//   - bool: true when current starts a new word.
func startsWord(previous, current rune, rest []rune) bool {
	switch {
	case unicode.IsUpper(current):
		if unicode.IsLower(previous) || unicode.IsDigit(previous) {
			return true
		}

		// The last letter of an acronym starts the word that follows it, unless
		// a single lowercase letter makes the acronym plural.
		return unicode.IsUpper(previous) && len(rest) > 1 && unicode.IsLower(rest[0]) && unicode.IsLower(rest[1])
	case unicode.IsDigit(current):
		return unicode.IsLetter(previous)
	default:
		return false
	}
}
