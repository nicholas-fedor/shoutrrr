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

// normalizedHeaderKey converts a header key from a URL to HTTP header format.
//
// A capital that follows a lowercase letter or a digit starts a new word, so
// "acceptLanguage" becomes "Accept-Language". The first letter of each word is
// capitalized when it is a lowercase ASCII letter. Acronyms, digits, and other
// characters stay as written, so "X-API-Key" and "x-1st" keep their words.
//
// Parameters:
//   - key: the header key, without its @ prefix.
//
// Returns:
//   - string: the header key in HTTP header format.
func normalizedHeaderKey(key string) string {
	stringBuilder := strings.Builder{}
	// Pre-allocate capacity for efficiency
	stringBuilder.Grow(len(key) * HeaderCapacityFactor)

	// previous is the input rune before c, as written rather than capitalized,
	// or 0 at the start of the key.
	var previous rune

	for _, c := range key {
		isUpper := UppercaseA <= c && c <= UppercaseZ
		followsLowerOrDigit := 'a' <= previous && previous <= 'z' || '0' <= previous && previous <= '9'

		switch {
		case isUpper && followsLowerOrDigit:
			// A capital after a lowercase letter or digit starts a camelCase word.
			stringBuilder.WriteRune(DashChar)
			stringBuilder.WriteRune(c)
		case 'a' <= c && c <= 'z' && (previous == 0 || previous == DashChar):
			// The first letter of a word is capitalized.
			stringBuilder.WriteRune(c - CaseOffset)
		default:
			// Any other character is kept as written.
			stringBuilder.WriteRune(c)
		}

		previous = c
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
