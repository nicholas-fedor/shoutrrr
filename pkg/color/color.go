package color

import (
	"fmt"
	"io"
	"os"
	"slices"
	"strconv"
	"strings"

	"github.com/mattn/go-colorable"
	"golang.org/x/term"
)

// Config holds configuration settings for color output.
type Config struct {
	// NoColor controls whether color output is disabled.
	// When true, all color formatting is stripped from output.
	NoColor bool

	// Output is the default io.Writer used by print functions for colorized output.
	Output io.Writer

	// Error is the default io.Writer used for error output with color support.
	Error io.Writer
}

// Color represents a color with ANSI SGR parameters.
type Color struct {
	params  []Attribute
	noColor *bool   // Instance-level override
	config  *Config // Reference to config (can be shared)
}

// stdoutFd is the file descriptor for stdout, safely converted.
var stdoutFd = int(os.Stdout.Fd())

// mapResetAttributes maps color attributes to their corresponding reset
// attributes for proper cleanup of terminal formatting.
var mapResetAttributes = map[Attribute]Attribute{
	Bold:         ResetBold,
	Faint:        ResetBold,
	Italic:       ResetItalic,
	Underline:    ResetUnderline,
	BlinkSlow:    ResetBlinking,
	BlinkRapid:   ResetBlinking,
	ReverseVideo: ResetReversed,
	Concealed:    ResetConcealed,
	CrossedOut:   ResetCrossedOut,
}

// noColorFromEnv returns whether colors should be disabled based on environment.
func noColorFromEnv() bool {
	return os.Getenv("NO_COLOR") != "" ||
		os.Getenv("TERM") == "dumb" ||
		!term.IsTerminal(stdoutFd)
}

// DefaultConfig returns a Config with sensible defaults.
// The NoColor setting is detected from the environment:
//   - If NO_COLOR environment variable is set, colors are disabled
//   - If TERM environment variable is "dumb", colors are disabled
//   - If stdout is not a terminal, colors are disabled
//
// Output and Error are set to colorable writers that support ANSI escape codes
// on Windows.
func DefaultConfig() *Config {
	return &Config{
		NoColor: noColorFromEnv(),
		Output:  colorable.NewColorableStdout(),
		Error:   colorable.NewColorableStderr(),
	}
}

// New returns a newly created color object using the default configuration.
func New(value ...Attribute) *Color {
	return NewWithConfig(DefaultConfig(), value...)
}

// NewWithConfig returns a newly created color object with the specified configuration.
//
// Example:
//
//	cfg := &color.Config{
//	    NoColor: true,
//	    Output:  os.Stdout,
//	    Error:   os.Stderr,
//	}
//	c := color.NewWithConfig(cfg, color.FgRed)
//	c.Println("This is red text")
func NewWithConfig(cfg *Config, value ...Attribute) *Color {
	c := &Color{
		params:  make([]Attribute, 0, len(value)),
		noColor: nil, // Will use config.NoColor by default
		config:  cfg,
	}

	if cfg.NoColor {
		c.noColor = boolPtr(true)
	}

	c.Add(value...)

	return c
}

// Add is used to chain SGR parameters.
// Use as many as parameters to combine and create custom color objects.
// Example: Add(color.FgRed, color.Underline).
func (c *Color) Add(value ...Attribute) *Color {
	c.params = append(c.params, value...)

	return c
}

// AddBgRGB is used to chain background RGB SGR parameters.
// Use as many as parameters to combine and create custom color objects.
// Example: .Add(34, 0, 12).Add(255, 128, 0).
func (c *Color) AddBgRGB(r, green, blue int) *Color {
	c.params = append(
		c.params,
		background,
		rgbColorFormatSpecifier,
		Attribute(r),
		Attribute(green),
		Attribute(blue),
	)

	return c
}

// AddRGB is used to chain foreground RGB SGR parameters.
// Use as many as parameters to combine and create custom color objects.
// Example: .Add(34, 0, 12).Add(255, 128, 0).
func (c *Color) AddRGB(r, green, blue int) *Color {
	c.params = append(
		c.params,
		foreground,
		rgbColorFormatSpecifier,
		Attribute(r),
		Attribute(green),
		Attribute(blue),
	)

	return c
}

// DisableColor disables the color output.
// Useful to not change any existing code and still being able to output.
// Can be used for flags like "--no-color".
// To enable back use EnableColor() method.
func (c *Color) DisableColor() {
	c.noColor = boolPtr(true)
}

// EnableColor enables the color output.
// Use it in conjunction with DisableColor().
// Otherwise, this method has no side effects.
func (c *Color) EnableColor() {
	c.noColor = boolPtr(false)
}

// Equals returns a boolean value indicating whether two colors are equal.
func (c *Color) Equals(colorToCompare *Color) bool {
	if c == nil && colorToCompare == nil {
		return true
	}

	if c == nil || colorToCompare == nil {
		return false
	}

	if len(c.params) != len(colorToCompare.params) {
		return false
	}

	for _, attr := range c.params {
		if !colorToCompare.attrExists(attr) {
			return false
		}
	}

	return true
}

// Fprint formats using the default formats for its operands and writes to w.
// Spaces are added between operands when neither is a string.
// It returns the number of bytes written and any write error encountered.
// On Windows, users should wrap w with colorable.NewColorable() if w is of
// type *os.File.
//
//nolint:wrapcheck // fmt.Fprintln errors are passed through as-is for API compatibility
func (c *Color) Fprint(writer io.Writer, args ...any) (int, error) {
	if c.isNoColorSet() {
		return fmt.Fprint(writer, args...)
	}

	c.SetWriter(writer)
	defer c.UnsetWriter(writer)

	return fmt.Fprint(writer, args...)
}

// FprintFunc returns a new function that prints the passed arguments as
// colorized with color.Fprint().
func (c *Color) FprintFunc() func(w io.Writer, a ...any) {
	return func(w io.Writer, a ...any) {
		_, _ = c.Fprint(w, a...)
	}
}

// Fprintf formats according to a format specifier and writes to w.
// It returns the number of bytes written and any write error encountered.
// On Windows, users should wrap w with colorable.NewColorable() if w is of
// type *os.File.
//
//nolint:wrapcheck // fmt.Fprintln errors are passed through as-is for API compatibility
func (c *Color) Fprintf(writer io.Writer, format string, args ...any) (int, error) {
	if c.isNoColorSet() {
		return fmt.Fprintf(writer, format, args...)
	}

	c.SetWriter(writer)
	defer c.UnsetWriter(writer)

	return fmt.Fprintf(writer, format, args...)
}

// FprintfFunc returns a new function that prints the passed arguments as
// colorized with color.Fprintf().
func (c *Color) FprintfFunc() func(w io.Writer, format string, a ...any) {
	return func(w io.Writer, format string, a ...any) {
		_, _ = c.Fprintf(w, format, a...)
	}
}

// Fprintln formats using the default formats for its operands and writes to w.
// Spaces are always added between operands and a newline is appended.
// On Windows, users should wrap w with colorable.NewColorable() if w is of
// type *os.File.
//
//nolint:wrapcheck // fmt.Fprintln errors are passed through as-is for API compatibility
func (c *Color) Fprintln(writer io.Writer, args ...any) (int, error) {
	if c.isNoColorSet() {
		return fmt.Fprintln(writer, args...)
	}

	return fmt.Fprintln(writer, c.format()+sprintln(args...)+c.unformat())
}

// FprintlnFunc returns a new function that prints the passed arguments as
// colorized with color.Fprintln().
func (c *Color) FprintlnFunc() func(w io.Writer, a ...any) {
	return func(w io.Writer, a ...any) {
		_, _ = c.Fprintln(w, a...)
	}
}

// Print formats using the default formats for its operands and writes to
// standard output. Spaces are added between operands when neither is a
// string. It returns the number of bytes written and any write error
// encountered. This is the standard fmt.Print() method wrapped with the given
// color.
//
//nolint:wrapcheck // fmt.Fprintln errors are passed through as-is for API compatibility
func (c *Color) Print(args ...any) (int, error) {
	if c.isNoColorSet() {
		return fmt.Fprint(c.config.Output, args...)
	}

	c.Set()
	defer c.Unset()

	return fmt.Fprint(c.config.Output, args...)
}

// PrintFunc returns a new function that prints the passed arguments as
// colorized with color.Print().
func (c *Color) PrintFunc() func(a ...any) {
	return func(a ...any) {
		_, _ = c.Print(a...)
	}
}

// Printf formats according to a format specifier and writes to standard output.
// It returns the number of bytes written and any write error encountered.
// This is the standard fmt.Printf() method wrapped with the given color.
//
//nolint:wrapcheck // fmt.Fprintln errors are passed through as-is for API compatibility
func (c *Color) Printf(format string, args ...any) (int, error) {
	if c.isNoColorSet() {
		return fmt.Fprintf(c.config.Output, format, args...)
	}

	c.Set()
	defer c.Unset()

	return fmt.Fprintf(c.config.Output, format, args...)
}

// PrintfFunc returns a new function that prints the passed arguments as
// colorized with color.Printf().
func (c *Color) PrintfFunc() func(format string, a ...any) {
	return func(format string, a ...any) {
		_, _ = c.Printf(format, a...)
	}
}

// Println formats using the default formats for its operands and writes to
// standard output. Spaces are always added between operands and a newline is
// appended. It returns the number of bytes written and any write error
// encountered. This is the standard fmt.Print() method wrapped with the given
// color.
//
//nolint:wrapcheck // fmt.Fprintln errors are passed through as-is for API compatibility
func (c *Color) Println(args ...any) (int, error) {
	if c.isNoColorSet() {
		return fmt.Fprintln(c.config.Output, args...)
	}

	return fmt.Fprintln(c.config.Output, c.format()+sprintln(args...)+c.unformat())
}

// PrintlnFunc returns a new function that prints the passed arguments as
// colorized with color.Println().
func (c *Color) PrintlnFunc() func(a ...any) {
	return func(a ...any) {
		_, _ = c.Println(a...)
	}
}

// Set sets the SGR sequence.
func (c *Color) Set() *Color {
	if c.isNoColorSet() || c.config.NoColor {
		return c
	}

	_, _ = fmt.Fprint(c.config.Output, c.format())

	return c
}

// SetWriter is used to set the SGR sequence with the given io.Writer.
// This is a low-level function, and users should use the higher-level
// functions, such as color.Fprint, color.Print, etc.
func (c *Color) SetWriter(w io.Writer) *Color {
	if c.isNoColorSet() || c.config.NoColor {
		return c
	}

	_, _ = fmt.Fprint(w, c.format())

	return c
}

// Sprint is just like Print, but returns a string instead of printing it.
func (c *Color) Sprint(a ...any) string {
	return c.wrap(fmt.Sprint(a...))
}

// SprintFunc returns a new function that returns colorized strings for the
// given arguments with fmt.Sprint().
// Useful to put into or mix into other string.
// Windows users should use this in conjunction with color.Output
// Example:
//
//	put := New(FgYellow).SprintFunc()
//	fmt.Fprintf(color.Output, "This is a %s", put("warning"))
func (c *Color) SprintFunc() func(a ...any) string {
	return func(a ...any) string {
		return c.wrap(fmt.Sprint(a...))
	}
}

// Sprintf is just like Printf, but returns a string instead of printing it.
func (c *Color) Sprintf(format string, a ...any) string {
	return c.wrap(fmt.Sprintf(format, a...))
}

// SprintfFunc returns a new function that returns colorized strings for the
// given arguments with fmt.Sprintf().
// Useful to put into or mix into other string.
// Windows users should use this in conjunction with color.Output.
//
// Note: The returned function is memoized for performance.
func (c *Color) SprintfFunc() func(format string, args ...any) string {
	if c.isNoColorSet() {
		return fmt.Sprintf
	}

	return func(format string, args ...any) string {
		return c.wrap(fmt.Sprintf(format, args...))
	}
}

// Sprintln is just like Println, but returns a string instead of printing it.
func (c *Color) Sprintln(a ...any) string {
	return c.wrap(sprintln(a...)) + "\n"
}

// SprintlnFunc returns a new function that returns colorized strings for the
// given arguments with fmt.Sprintln().
// Useful to put into or mix into other string.
// Windows users should use this in conjunction with color.Output.
//
// Note: The returned function is memoized for performance.
func (c *Color) SprintlnFunc() func(a ...any) string {
	if c.isNoColorSet() {
		return func(a ...any) string { return sprintln(a...) + "\n" }
	}

	return func(a ...any) string {
		return c.wrap(sprintln(a...)) + "\n"
	}
}

// Unset resets the color output to default.
// It clears the escape attributes and restores normal terminal formatting.
// This should be called after Set() to clean up the terminal state.
func (c *Color) Unset() {
	if c.isNoColorSet() || c.config.NoColor {
		return
	}

	c.config.unset()
}

// UnsetWriter resets all escape attributes and clears the output with the give
// io.Writer. Usually should be called after SetWriter().
func (c *Color) UnsetWriter(w io.Writer) {
	if c.isNoColorSet() || c.config.NoColor {
		return
	}

	_, _ = fmt.Fprintf(w, "%s[%dm", escape, Reset)
}

func (c *Color) attrExists(a Attribute) bool {
	return slices.Contains(c.params, a)
}

func (c *Color) format() string {
	return fmt.Sprintf("%s[%sm", escape, c.sequence())
}

func (c *Color) isNoColorSet() bool {
	// check first if we have user set action
	if c.noColor != nil {
		return *c.noColor
	}

	// if not return the config option
	return c.config.NoColor
}

// sequence returns a formatted SGR sequence to be plugged into a "\x1b[...m".
// An example output might be: "1;36" -> bold cyan.
func (c *Color) sequence() string {
	if len(c.params) == 0 {
		return ""
	}

	var b strings.Builder
	b.Grow(len(c.params) * builderGrowthFactor) // Pre-allocate for typical cases

	for i, v := range c.params {
		if i > 0 {
			b.WriteByte(';')
		}

		b.WriteString(strconv.Itoa(int(v)))
	}

	return b.String()
}

// unformat returns the ANSI escape sequence to reset the color formatting.
// For each parameter in the color, it uses the specific reset attribute
// if available, or the generic Reset attribute otherwise.
func (c *Color) unformat() string {
	if len(c.params) == 0 {
		return ""
	}

	var b strings.Builder
	b.Grow(len(c.params) * builderGrowthFactor)
	b.WriteString(escape)
	b.WriteByte('[')

	for i, attr := range c.params {
		if i > 0 {
			b.WriteByte(';')
		}

		// Default to generic reset
		resetAttr := Reset

		// Use specific reset if available
		if specificReset, ok := mapResetAttributes[attr]; ok {
			resetAttr = specificReset
		}

		b.WriteString(strconv.Itoa(int(resetAttr)))
	}

	b.WriteByte('m')

	return b.String()
}

// wrap wraps the s string with the colors attributes.
// The string is ready to be printed.
func (c *Color) wrap(s string) string {
	if c.isNoColorSet() || c.config.NoColor {
		return s
	}

	return c.format() + s + c.unformat()
}

// unset resets all escape attributes and clears the output.
func (c *Config) unset() {
	if c.NoColor {
		return
	}

	_, _ = fmt.Fprintf(c.Output, "%s[%dm", escape, Reset)
}

// BgRGB returns a new background color in 24-bit RGB.
func BgRGB(r, g, b int) *Color {
	return New(
		background,
		rgbColorFormatSpecifier,
		Attribute(r),
		Attribute(g),
		Attribute(b),
	)
}

// RGB returns a new foreground color in 24-bit RGB.
func RGB(r, g, b int) *Color {
	return New(
		foreground,
		rgbColorFormatSpecifier,
		Attribute(r),
		Attribute(g),
		Attribute(b),
	)
}
