// Package generator provides common utilities for interactive CLI generators.
//
// The generator package facilitates question/answer-based user interaction through
// the UserDialog type. It supports capturing various input types including strings,
// integers, booleans, and regex patterns with built-in validation. The package is
// designed to support service configuration generation in CLI applications.
//
// Key features:
//   - Interactive prompts with validation
//   - Support for multiple input types (string, int, bool)
//   - Regex pattern matching for input validation
//   - Property-based input for non-interactive usage
//   - Colored output for better user experience
package generator

import (
	"bufio"
	"errors"
	"fmt"
	"io"
	"regexp"
	"strconv"

	"github.com/nicholas-fedor/shoutrrr/pkg/color"
	"github.com/nicholas-fedor/shoutrrr/pkg/format"
)

// UserDialog facilitates question/answer-based user interaction for CLI generators.
// It provides methods to prompt users for various input types with validation,
// supporting both interactive and property-based (non-interactive) modes.
//
// The UserDialog uses a bufio.Scanner for reading input from the provided reader,
// and writes output to the provided writer. It maintains a map of pre-defined
// properties that can be used instead of prompting the user interactively.
type UserDialog struct {
	reader  io.Reader         // source for reading user input
	writer  io.Writer         // destination for writing prompts and messages
	scanner *bufio.Scanner    // buffered scanner for reading lines of input
	props   map[string]string // pre-defined properties for non-interactive mode
}

// errInvalidFormat indicates that the user input does not match the expected format.
// This error is returned when input fails to match a regex pattern or validation rule.
var errInvalidFormat = errors.New("invalid format")

// errRequired indicates that a required field was left empty.
// This error is returned when validation requires at least one character of input.
var errRequired = errors.New("field is required")

// errNotANumber indicates that the input could not be parsed as a number.
// This error is returned when parsing integer input fails.
var errNotANumber = errors.New("not a number")

// errInvalidBoolFormat indicates that a boolean answer was not "yes" or "no".
// This error is returned when boolean parsing fails to recognize the input.
var errInvalidBoolFormat = errors.New("answer must be yes or no")

// NewUserDialog initializes a new UserDialog instance with safe defaults.
//
// If the provided props map is nil, an empty map is created to prevent nil
// pointer dereferences during property lookups.
//
// Parameters:
//   - reader: the source for reading user input (typically os.Stdin)
//   - writer: the destination for writing prompts and output (typically os.Stdout)
//   - props: a map of pre-defined property values keyed by field name; may be nil
//
// Returns:
//   - *UserDialog: a configured UserDialog instance ready for user interaction
func NewUserDialog(reader io.Reader, writer io.Writer, props map[string]string) *UserDialog {
	if props == nil {
		props = map[string]string{}
	}

	return &UserDialog{
		reader:  reader,
		writer:  writer,
		scanner: bufio.NewScanner(reader),
		props:   props,
	}
}

// Query prompts the user with the given prompt and returns regex capture groups
// if the input matches the validator pattern.
//
// If a property with the given key exists in the UserDialog's props map,
// the property value is validated and used instead of prompting interactively.
//
// Parameters:
//   - prompt: the message displayed to the user
//   - validator: a compiled regular expression for validating and capturing input
//   - key: the property key for looking up pre-defined values
//
// Returns:
//   - []string: the regex capture groups from the matched input; nil if no match
func (ud *UserDialog) Query(prompt string, validator *regexp.Regexp, key string) []string {
	var groups []string

	ud.QueryString(prompt, ValidateFormat(func(answer string) bool {
		groups = validator.FindStringSubmatch(answer)

		return groups != nil
	}), key)

	return groups
}

// QueryAll prompts the user with the given prompt and returns multiple regex matches
// up to the specified maximum number of matches.
//
// If a property with the given key exists in the UserDialog's props map,
// the property value is validated and used instead of prompting interactively.
//
// Parameters:
//   - prompt: the message displayed to the user
//   - validator: a compiled regular expression for validating and capturing input
//   - key: the property key for looking up pre-defined values
//   - maxMatches: the maximum number of matches to return; negative for all matches
//
// Returns:
//   - [][]string: a slice of regex capture groups for each match; nil if no matches
func (ud *UserDialog) QueryAll(
	prompt string,
	validator *regexp.Regexp,
	key string,
	maxMatches int,
) [][]string {
	var matches [][]string

	ud.QueryString(prompt, ValidateFormat(func(answer string) bool {
		matches = validator.FindAllStringSubmatch(answer, maxMatches)

		return matches != nil
	}), key)

	return matches
}

// QueryBool prompts the user with the given prompt and returns the answer as a boolean.
//
// The method accepts various boolean representations through format.ParseBool,
// including "yes"/"no", "true"/"false", "y"/"n", "1"/"0", etc.
//
// If a property with the given key exists in the UserDialog's props map,
// the property value is validated and used instead of prompting interactively.
//
// Parameters:
//   - prompt: the message displayed to the user
//   - key: the property key for looking up pre-defined values
//
// Returns:
//   - bool: the parsed boolean value; defaults to false if input cannot be parsed
func (ud *UserDialog) QueryBool(prompt, key string) bool {
	var value bool

	ud.QueryString(prompt, func(answer string) error {
		parsed, ok := format.ParseBool(answer, false)
		if ok {
			value = parsed

			return nil
		}

		return fmt.Errorf(
			"%w: use %v or %v",
			errInvalidBoolFormat,
			format.ColorizeTrue("yes"),
			format.ColorizeFalse("no"),
		)
	}, key)

	return value
}

// QueryInt prompts the user with the given prompt and returns the answer as an integer.
//
// The method supports decimal, hexadecimal (with 0x or # prefix), and negative numbers.
// Hexadecimal input with # prefix (e.g., #ffa080) is explicitly treated as base 16.
//
// If a property with the given key exists in the UserDialog's props map,
// the property value is validated and used instead of prompting interactively.
//
// Parameters:
//   - prompt: the message displayed to the user
//   - key: the property key for looking up pre-defined values
//   - bitSize: the integer type bit size (0 for int, 8 for int8, 16 for int16, etc.)
//
// Returns:
//   - int64: the parsed integer value; 0 if parsing fails
func (ud *UserDialog) QueryInt(prompt, key string, bitSize int) int64 {
	validator := regexp.MustCompile(`^((0x|#)([0-9a-fA-F]+))|(-?\d+)$`)

	var value int64

	ud.QueryString(prompt, func(answer string) error {
		groups := validator.FindStringSubmatch(answer)
		if len(groups) < 1 {
			return errNotANumber
		}

		number := groups[0]

		base := 0
		if groups[2] == "#" {
			// Explicitly treat #ffa080 as hexadecimal
			base = 16
			number = groups[3]
		}

		var err error

		value, err = strconv.ParseInt(number, base, bitSize)
		if err != nil {
			return fmt.Errorf("parsing integer from %q: %w", answer, err)
		}

		return nil
	}, key)

	return value
}

// QueryString prompts the user with the given prompt and returns the answer
// if it passes the provided validator function.
//
// If a property with the given key exists in the UserDialog's props map and
// passes validation, that value is returned without prompting interactively.
// If the property value fails validation, an error message is displayed and
// interactive prompting begins.
//
// If the validator parameter is nil, a no-op validator is used that accepts
// any input.
//
// The method loops until valid input is received or the input source is closed.
// When input is closed (EOF), an empty string is returned.
//
// Parameters:
//   - prompt: the message displayed to the user
//   - validator: a function that validates the input and returns an error if invalid; may be nil
//   - key: the property key for looking up pre-defined values
//
// Returns:
//   - string: the validated user input; empty string if input source is closed
func (ud *UserDialog) QueryString(prompt string, validator func(string) error, key string) string {
	if validator == nil {
		validator = func(string) error { return nil }
	}

	answer, foundProp := ud.props[key]
	if foundProp {
		err := validator(answer)
		colAnswer := format.ColorizeValue(answer, false)
		colKey := format.ColorizeProp(key)

		if err == nil {
			ud.Writelnf("Using prop value %v for %v", colAnswer, colKey)

			return answer
		}

		ud.Writelnf("Supplied prop value %v is not valid for %v: %v", colAnswer, colKey, err)
	}

	for {
		ud.Write("%v ", prompt)

		cfg := color.DefaultConfig()
		c := color.NewWithConfig(cfg, color.FgHiWhite)
		c.Set()

		if !ud.scanner.Scan() {
			if err := ud.scanner.Err(); err != nil {
				ud.Writelnf(err.Error())
				c.Unset()

				continue
			}
			// Input closed, return an empty string
			c.Unset()

			return ""
		}

		answer = ud.scanner.Text()

		c.Unset()

		if err := validator(answer); err != nil {
			ud.Writelnf("%v", err)
			ud.Writelnf("")

			continue
		}

		return answer
	}
}

// QueryStringPattern prompts the user with the given prompt and returns the answer
// if it matches the provided regex pattern.
//
// This is a convenience method that wraps QueryString with regex pattern validation.
// Unlike Query, this method does not return capture groups, only the matched string.
//
// If a property with the given key exists in the UserDialog's props map,
// the property value is validated and used instead of prompting interactively.
//
// Parameters:
//   - prompt: the message displayed to the user
//   - validator: a compiled regular expression that the input must match; must not be nil
//   - key: the property key for looking up pre-defined values
//
// Returns:
//   - string: the validated user input that matches the pattern
//
// Panics:
//   - If validator is nil, the method panics with an appropriate error message
func (ud *UserDialog) QueryStringPattern(
	prompt string,
	validator *regexp.Regexp,
	key string,
) string {
	if validator == nil {
		panic("validator cannot be nil")
	}

	return ud.QueryString(prompt, func(s string) error {
		if validator.MatchString(s) {
			return nil
		}

		return errInvalidFormat
	}, key)
}

// Write sends a formatted message to the user through the configured writer.
//
// The message is formatted using fmt.Fprintf with the provided variadic arguments.
// If writing fails, an error message is written to the same writer.
//
// Parameters:
//   - message: the format string for the message
//   - v: variadic arguments for formatting the message
func (ud *UserDialog) Write(message string, v ...any) {
	if _, err := fmt.Fprintf(ud.writer, message, v...); err != nil {
		_, _ = fmt.Fprint(ud.writer, "failed to write to output: ", err, "\n")
	}
}

// Writelnf writes a formatted message to the user, appending a newline character.
//
// This is a convenience method that calls Write with the format string and
// appends "\n" to complete the line.
//
// Parameters:
//   - fmtStr: the format string for the message
//   - v: variadic arguments for formatting the message
func (ud *UserDialog) Writelnf(fmtStr string, v ...any) {
	ud.Write(fmtStr+"\n", v...)
}

// ValidateFormat wraps a boolean validator function to return an error on false results.
//
// This utility function adapts simple boolean validators to the error-returning
// validator signature used by QueryString. When the wrapped validator returns false,
// errInvalidFormat is returned.
//
// Parameters:
//   - validator: a function that takes a string and returns true if valid, false otherwise
//
// Returns:
//   - func(string) error: a validator function that returns nil on success or errInvalidFormat on failure
func ValidateFormat(validator func(string) bool) func(string) error {
	return func(answer string) error {
		if validator(answer) {
			return nil
		}

		return errInvalidFormat
	}
}

// Required validates that the input string contains at least one character.
//
// This is a common validator function that ensures non-empty input from the user.
// It is typically used with QueryString to make fields mandatory.
//
// Parameters:
//   - answer: the input string to validate
//
// Returns:
//   - error: nil if the answer is non-empty, errRequired if the answer is empty
func Required(answer string) error {
	if answer == "" {
		return errRequired
	}

	return nil
}
