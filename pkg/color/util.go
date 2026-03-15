package color

import (
	"fmt"
	"os"
	"strings"
)

func boolPtr(v bool) *bool {
	return &v
}

// noColorIsSet returns true if the NO_COLOR environment variable is set (regardless of value).
// This follows the NO_COLOR standard (https://no-color.org/) where the presence
// of the environment variable, with any value including empty, should disable colors.
func noColorIsSet() bool {
	_, exists := os.LookupEnv("NO_COLOR")

	return exists
}

// sprintln is a helper function to format a string with fmt.Sprintln and trim the trailing newline.
func sprintln(a ...any) string {
	return strings.TrimSuffix(fmt.Sprintln(a...), "\n")
}
