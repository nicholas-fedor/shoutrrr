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

// errInvalidFormat indicates an invalid user input format.
var (
	errInvalidFormat     = errors.New("invalid format")
	errRequired          = errors.New("field is required")
	errNotANumber        = errors.New("not a number")
	errInvalidBoolFormat = errors.New("answer must be yes or no")
)

// ValidateFormat wraps a boolean validator to return an error on false results.
func ValidateFormat(validator func(string) bool) func(string) error {
	return func(answer string) error {
		if validator(answer) {
			return nil
		}

		return errInvalidFormat
	}
}

// Required validates that the input contains at least one character.
func Required(answer string) error {
	if answer == "" {
		return errRequired
	}

	return nil
}

// UserDialog facilitates question/answer-based user interaction.
type UserDialog struct {
	reader  io.Reader
	writer  io.Writer
	scanner *bufio.Scanner
	props   map[string]string
}

// NewUserDialog initializes a UserDialog with safe defaults.
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

// Write sends a message to the user.
func (ud *UserDialog) Write(message string, v ...any) {
	if _, err := fmt.Fprintf(ud.writer, message, v...); err != nil {
		_, _ = fmt.Fprint(ud.writer, "failed to write to output: ", err, "\n")
	}
}

// Writelnf writes a formatted message to the user, completing a line.
func (ud *UserDialog) Writelnf(format string, v ...any) {
	ud.Write(format+"\n", v...)
}

// Query prompts the user and returns regex groups if the input matches the validator pattern.
func (ud *UserDialog) Query(prompt string, validator *regexp.Regexp, key string) []string {
	var groups []string

	ud.QueryString(prompt, ValidateFormat(func(answer string) bool {
		groups = validator.FindStringSubmatch(answer)

		return groups != nil
	}), key)

	return groups
}

// QueryAll prompts the user and returns multiple regex matches up to maxMatches.
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

// QueryString prompts the user and returns the answer if it passes the validator.
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
		color.Set(color.FgHiWhite)

		if !ud.scanner.Scan() {
			if err := ud.scanner.Err(); err != nil {
				ud.Writelnf(err.Error())

				continue
			}
			// Input closed, return an empty string
			return ""
		}

		answer = ud.scanner.Text()

		color.Unset()

		if err := validator(answer); err != nil {
			ud.Writelnf("%v", err)
			ud.Writelnf("")

			continue
		}

		return answer
	}
}

// QueryStringPattern prompts the user and returns the answer if it matches the regex pattern.
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

// QueryInt prompts the user and returns the answer as an integer if parseable.
func (ud *UserDialog) QueryInt(prompt, key string, bitSize int) int64 {
	validator := regexp.MustCompile(`^((0x|#)([0-9a-fA-F]+))|(-?[0-9]+)$`)

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

// QueryBool prompts the user and returns the answer as a boolean if parseable.
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
