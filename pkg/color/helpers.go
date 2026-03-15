package color

import "strings"

// Blackf is a convenient helper function to print with black foreground. A
// newline is appended to format by default.
func Blackf(format string, a ...any) {
	colorPrint(format, FgBlack, a...)
}

// BlackString is a convenient helper function to return a string with black
// foreground.
func BlackString(format string, a ...any) string {
	return colorString(format, FgBlack, a...)
}

// Bluef is a convenient helper function to print with blue foreground. A
// newline is appended to format by default.
func Bluef(format string, a ...any) {
	colorPrint(format, FgBlue, a...)
}

// BlueString is a convenient helper function to return a string with blue
// foreground.
func BlueString(format string, a ...any) string {
	return colorString(format, FgBlue, a...)
}

// Cyanf is a convenient helper function to print with cyan foreground. A
// newline is appended to format by default.
func Cyanf(format string, a ...any) {
	colorPrint(format, FgCyan, a...)
}

// CyanString is a convenient helper function to return a string with cyan
// foreground.
func CyanString(format string, a ...any) string {
	return colorString(format, FgCyan, a...)
}

// Greenf is a convenient helper function to print with green foreground. A
// newline is appended to format by default.
func Greenf(format string, a ...any) {
	colorPrint(format, FgGreen, a...)
}

// GreenString is a convenient helper function to return a string with green
// foreground.
func GreenString(format string, a ...any) string {
	return colorString(format, FgGreen, a...)
}

// HiBlackf is a convenient helper function to print with hi-intensity black foreground. A
// newline is appended to format by default.
func HiBlackf(format string, a ...any) {
	colorPrint(format, FgHiBlack, a...)
}

// HiBlackString is a convenient helper function to return a string with hi-intensity black
// foreground.
func HiBlackString(format string, a ...any) string {
	return colorString(format, FgHiBlack, a...)
}

// HiBluef is a convenient helper function to print with hi-intensity blue foreground. A
// newline is appended to format by default.
func HiBluef(format string, a ...any) {
	colorPrint(format, FgHiBlue, a...)
}

// HiBlueString is a convenient helper function to return a string with hi-intensity blue
// foreground.
func HiBlueString(format string, a ...any) string {
	return colorString(format, FgHiBlue, a...)
}

// HiCyanf is a convenient helper function to print with hi-intensity cyan foreground. A
// newline is appended to format by default.
func HiCyanf(format string, a ...any) {
	colorPrint(format, FgHiCyan, a...)
}

// HiCyanString is a convenient helper function to return a string with hi-intensity cyan
// foreground.
func HiCyanString(format string, a ...any) string {
	return colorString(format, FgHiCyan, a...)
}

// HiGreenf is a convenient helper function to print with hi-intensity green foreground. A
// newline is appended to format by default.
func HiGreenf(format string, a ...any) {
	colorPrint(format, FgHiGreen, a...)
}

// HiGreenString is a convenient helper function to return a string with hi-intensity green
// foreground.
func HiGreenString(format string, a ...any) string {
	return colorString(format, FgHiGreen, a...)
}

// HiMagentaf is a convenient helper function to print with hi-intensity magenta foreground.
// A newline is appended to format by default.
func HiMagentaf(format string, a ...any) {
	colorPrint(format, FgHiMagenta, a...)
}

// HiMagentaString is a convenient helper function to return a string with hi-intensity magenta
// foreground.
func HiMagentaString(format string, a ...any) string {
	return colorString(format, FgHiMagenta, a...)
}

// HiRedf is a convenient helper function to print with hi-intensity red foreground. A
// newline is appended to format by default.
func HiRedf(format string, a ...any) {
	colorPrint(format, FgHiRed, a...)
}

// HiRedString is a convenient helper function to return a string with hi-intensity red
// foreground.
func HiRedString(format string, a ...any) string {
	return colorString(format, FgHiRed, a...)
}

// HiYellowf is a convenient helper function to print with hi-intensity yellow foreground.
// A newline is appended to format by default.
func HiYellowf(format string, a ...any) {
	colorPrint(format, FgHiYellow, a...)
}

// HiYellowString is a convenient helper function to return a string with hi-intensity yellow
// foreground.
func HiYellowString(format string, a ...any) string {
	return colorString(format, FgHiYellow, a...)
}

// HiWhitef is a convenient helper function to print with hi-intensity white foreground. A
// newline is appended to format by default.
func HiWhitef(format string, a ...any) {
	colorPrint(format, FgHiWhite, a...)
}

// HiWhiteString is a convenient helper function to return a string with hi-intensity white
// foreground.
func HiWhiteString(format string, a ...any) string {
	return colorString(format, FgHiWhite, a...)
}

// Magentaf is a convenient helper function to print with magenta foreground.
// A newline is appended to format by default.
func Magentaf(format string, a ...any) {
	colorPrint(format, FgMagenta, a...)
}

// MagentaString is a convenient helper function to return a string with magenta
// foreground.
func MagentaString(format string, a ...any) string {
	return colorString(format, FgMagenta, a...)
}

// Redf is a convenient helper function to print with red foreground. A
// newline is appended to format by default.
func Redf(format string, a ...any) {
	colorPrint(format, FgRed, a...)
}

// RedString is a convenient helper function to return a string with red
// foreground.
func RedString(format string, a ...any) string {
	return colorString(format, FgRed, a...)
}

// Whitef is a convenient helper function to print with white foreground. A
// newline is appended to format by default.
func Whitef(format string, a ...any) {
	colorPrint(format, FgWhite, a...)
}

// WhiteString is a convenient helper function to return a string with white
// foreground.
func WhiteString(format string, a ...any) string {
	return colorString(format, FgWhite, a...)
}

// Yellowf is a convenient helper function to print with yellow foreground.
// A newline is appended to format by default.
func Yellowf(format string, a ...any) {
	colorPrint(format, FgYellow, a...)
}

// YellowString is a convenient helper function to return a string with yellow
// foreground.
func YellowString(format string, a ...any) string {
	return colorString(format, FgYellow, a...)
}

// colorPrint prints formatted text with the specified color attribute.
// Uses the default configuration for backward compatibility.
func colorPrint(format string, attribute Attribute, arguments ...any) {
	c := New(attribute)

	if !strings.HasSuffix(format, "\n") {
		format += "\n"
	}

	if len(arguments) == 0 {
		_, _ = c.Print(format)
	} else {
		_, _ = c.Printf(format, arguments...)
	}
}

// colorString returns a formatted string with the specified color attribute.
// Uses the default configuration for backward compatibility.
func colorString(format string, attribute Attribute, arguments ...any) string {
	c := New(attribute)

	if len(arguments) == 0 {
		return c.SprintFunc()(format)
	}

	return c.SprintfFunc()(format, arguments...)
}
