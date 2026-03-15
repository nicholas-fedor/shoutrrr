package util

import "strings"

// hex is the base used for hexadecimal number parsing.
const hex int = 16

// StripNumberPrefix removes hexadecimal prefixes from number strings.
//
// It recognizes the following prefixes:
//   - "#" (e.g., "#FF" becomes "FF")
//   - "0x" or "0X" (e.g., "0xFF" becomes "FF")
//
// Parameters:
//   - input: The number string that may contain a base prefix.
//
// Returns:
//   - The input string with any recognized prefix removed.
//   - The base (16 for hex prefixes, 0 if no prefix was found).
//     A return value of 0 indicates that strconv should attempt to identify the base.
func StripNumberPrefix(input string) (string, int) {
	if strings.HasPrefix(input, "#") {
		return input[1:], hex
	}

	if strings.HasPrefix(input, "0x") || strings.HasPrefix(input, "0X") {
		return input[2:], hex
	}

	return input, 0
}
