package format

import "github.com/nicholas-fedor/shoutrrr/pkg/color"

// Colorizers color text for the console renderer and the generators. They
// share one color config, so they all color or all write plain text. Color
// is off when standard output is not a terminal. Setting NO_COLOR, or TERM
// to "dumb", also turns it off. The check runs once, when the package loads.
var (
	// colorConfig is the color config every colorizer shares. The colorizers
	// read its NoColor setting at each call.
	colorConfig = color.DefaultConfig()

	// ColorizeDesc colorizes the input string as "Description".
	ColorizeDesc = color.NewWithConfig(colorConfig, color.FgHiBlack).SprintFunc()

	// ColorizeTrue colorizes the input string as "True".
	ColorizeTrue = color.NewWithConfig(colorConfig, color.FgHiGreen).SprintFunc()

	// ColorizeFalse colorizes the input string as "False".
	ColorizeFalse = color.NewWithConfig(colorConfig, color.FgHiRed).SprintFunc()

	// ColorizeNumber colorizes the input string as "Number".
	ColorizeNumber = color.NewWithConfig(colorConfig, color.FgHiBlue).SprintFunc()

	// ColorizeString colorizes the input string as "String".
	ColorizeString = color.NewWithConfig(colorConfig, color.FgHiYellow).SprintFunc()

	// ColorizeEnum colorizes the input string as "Enum".
	ColorizeEnum = color.NewWithConfig(colorConfig, color.FgHiCyan).SprintFunc()

	// ColorizeProp colorizes the input string as "Prop".
	ColorizeProp = color.NewWithConfig(colorConfig, color.FgHiMagenta).SprintFunc()

	// ColorizeError colorizes the input string as "Error".
	ColorizeError = ColorizeFalse

	// ColorizeContainer colorizes the input string as "Container".
	ColorizeContainer = ColorizeDesc

	// ColorizeLink colorizes the input string as "Link".
	ColorizeLink = color.NewWithConfig(colorConfig, color.FgHiBlue).SprintFunc()
)

// ColorizeValue colorizes the input string according to what type appears to be.
func ColorizeValue(value string, isEnum bool) string {
	if isEnum {
		return ColorizeEnum(value)
	}

	if isTrue, isType := ParseBool(value, false); isType {
		if isTrue {
			return ColorizeTrue(value)
		}

		return ColorizeFalse(value)
	}

	if IsNumber(value) {
		return ColorizeNumber(value)
	}

	return ColorizeString(value)
}

// ColorizeToken colorizes the value according to the tokenType.
func ColorizeToken(value string, tokenType NodeTokenType) string {
	switch tokenType {
	case NumberToken:
		return ColorizeNumber(value)
	case EnumToken:
		return ColorizeEnum(value)
	case TrueToken:
		return ColorizeTrue(value)
	case FalseToken:
		return ColorizeFalse(value)
	case PropToken:
		return ColorizeProp(value)
	case ErrorToken:
		return ColorizeError(value)
	case ContainerToken:
		return ColorizeContainer(value)
	case StringToken:
		return ColorizeString(value)
	case UnknownToken:
	default:
	}

	return value
}
