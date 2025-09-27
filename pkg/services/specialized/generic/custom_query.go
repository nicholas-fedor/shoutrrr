package generic

import (
	"net/url"
	"strings"
)

// Constants for character values and offsets.
const (
	ExtraPrefixChar      = '$'       // Prefix for extra data in query parameters
	HeaderPrefixChar     = '@'       // Prefix for header values in query parameters
	CaseOffset           = 'a' - 'A' // Offset between lowercase and uppercase letters
	UppercaseA           = 'A'       // ASCII value for uppercase A
	UppercaseZ           = 'Z'       // ASCII value for uppercase Z
	DashChar             = '-'       // Dash character for header formatting
	HeaderCapacityFactor = 2         // Estimated capacity multiplier for header string builder
)

func normalizedHeaderKey(key string) string {
	stringBuilder := strings.Builder{}
	stringBuilder.Grow(len(key) * HeaderCapacityFactor)

	for i, c := range key {
		if UppercaseA <= c && c <= UppercaseZ {
			// Char is uppercase
			if i > 0 && key[i-1] != DashChar {
				// Add missing dash
				stringBuilder.WriteRune(DashChar)
			}
		} else if i == 0 || key[i-1] == DashChar {
			// First char, or previous was dash
			c -= CaseOffset
		}

		stringBuilder.WriteRune(c)
	}

	return stringBuilder.String()
}

func appendCustomQueryValues(
	query url.Values,
	headers map[string]string,
	extraData map[string]string,
) {
	for key, value := range headers {
		query.Set(string(HeaderPrefixChar)+key, value)
	}

	for key, value := range extraData {
		query.Set(string(ExtraPrefixChar)+key, value)
	}
}

func stripCustomQueryValues(query url.Values) (map[string]string, map[string]string) {
	headers := make(map[string]string)
	extraData := make(map[string]string)

	for key, values := range query {
		switch key[0] {
		case HeaderPrefixChar:
			headerKey := normalizedHeaderKey(key[1:])
			headers[headerKey] = values[0]
		case ExtraPrefixChar:
			extraData[key[1:]] = values[0]
		default:
			continue
		}

		delete(query, key)
	}

	return headers, extraData
}
