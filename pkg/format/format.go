package format

import (
	"strconv"
	"strings"
)

// ParseBool reads a boolean from its text.
//
// The text matches in any case. "true", "1", "yes", and "y" read as true,
// and "false", "0", "no", and "n" read as false.
//
// Parameters:
//   - value: the text to read.
//   - defaultValue: the value to return for any other text.
//
// Returns:
//   - bool: the boolean, or defaultValue when the text is not one of the above.
//   - bool: whether the text was one of the above.
func ParseBool(value string, defaultValue bool) (bool, bool) {
	switch strings.ToLower(value) {
	case "true", "1", "yes", "y":
		return true, true
	case "false", "0", "no", "n":
		return false, true
	default:
		return defaultValue, false
	}
}

// PrintBool writes a boolean as "Yes" or "No".
//
// Parameters:
//   - value: the boolean to write.
//
// Returns:
//   - string: "Yes" for true and "No" for false.
func PrintBool(value bool) string {
	if value {
		return "Yes"
	}

	return "No"
}

// IsNumber reports whether a string reads as a floating-point number.
//
// Parameters:
//   - value: the string to check.
//
// Returns:
//   - bool: true when [strconv.ParseFloat] reads the string.
func IsNumber(value string) bool {
	_, err := strconv.ParseFloat(value, 64)

	return err == nil
}
