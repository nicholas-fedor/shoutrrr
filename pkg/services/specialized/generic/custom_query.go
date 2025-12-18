package generic

import (
	"net/url"
	"strings"
)

// Constants for character values and offsets used in header normalization and query parsing.
const (
	ExtraPrefixChar      = '$'       // Prefix for extra data in query parameters
	HeaderPrefixChar     = '@'       // Prefix for header values in query parameters
	CaseOffset           = 'a' - 'A' // Offset between lowercase and uppercase letters
	UppercaseA           = 'A'       // ASCII value for uppercase A
	UppercaseZ           = 'Z'       // ASCII value for uppercase Z
	DashChar             = '-'       // Dash character for header formatting
	HeaderCapacityFactor = 2         // Estimated capacity multiplier for header string builder
)

// normalizedHeaderKey converts a header key to HTTP header format (e.g., "ContentType" -> "content-type").
func normalizedHeaderKey(key string) string {
	stringBuilder := strings.Builder{}
	// Pre-allocate capacity for efficiency
	stringBuilder.Grow(len(key) * HeaderCapacityFactor)

	for i, c := range key {
		if UppercaseA <= c && c <= UppercaseZ {
			// If character is uppercase
			if i > 0 && key[i-1] != DashChar {
				// Add dash before uppercase if not after dash
				stringBuilder.WriteRune(DashChar)
			}
		} else if i == 0 || key[i-1] == DashChar {
			// First char or after dash: convert to lowercase
			c -= CaseOffset
		}

		stringBuilder.WriteRune(c)
	}

	return stringBuilder.String()
}

// appendCustomQueryValues adds headers and extra data to URL query parameters with prefixes.
func appendCustomQueryValues(
	query url.Values,
	headers map[string]string,
	extraData map[string]string,
) {
	for key, value := range headers {
		// Add headers with @ prefix
		query.Set(string(HeaderPrefixChar)+key, value)
	}

	for key, value := range extraData {
		// Add extra data with $ prefix
		query.Set(string(ExtraPrefixChar)+key, value)
	}
}

// stripCustomQueryValues extracts headers and extra data from query parameters and removes them from the query.
func stripCustomQueryValues(query url.Values) (map[string]string, map[string]string) {
	// Map for extracted headers
	headers := make(map[string]string)
	// Map for extracted extra data
	extraData := make(map[string]string)

	for key, values := range query {
		// Skip keys with length <= 1 to prevent processing empty or invalid keys that could result in malformed headers or JSON fields
		if len(key) <= 1 {
			continue
		}

		switch key[0] {
		case HeaderPrefixChar: // Header prefixed with @
			// Normalize header key
			headerKey := normalizedHeaderKey(key[1:])
			headers[headerKey] = values[0]
		case ExtraPrefixChar: // Extra data prefixed with $
			extraData[key[1:]] = values[0]
		default: // Skip non-prefixed keys
			continue
		}

		// Remove the custom key from query
		delete(query, key)
	}

	return headers, extraData
}
