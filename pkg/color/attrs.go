package color

// Attribute represents a single SGR (Select Graphic Rendition) code used for
// styling terminal text output. It is an integer type that corresponds to
// specific ANSI escape sequence parameter values for controlling text
// appearance, including colors, formatting, and visual effects.
//
// Attribute values are designed to be combined when constructing ANSI escape
// sequences. The zero value (Reset) clears all formatting and returns text
// to the default terminal appearance.
//
// Attributes are used in conjunction with the Color type to create reusable
// styling configurations that can be applied to strings for colored terminal
// output.
type Attribute int

// Internal constants used for constructing ANSI escape sequences and
// managing buffer allocations.
const (
	// escape is the ANSI escape character (ESC, 0x1b) used to begin
	// escape sequences for terminal control and styling.
	escape = "\x1b"

	// builderGrowthFactor specifies the multiplier used for pre-allocating
	// strings.Builder capacity when constructing escape sequences. This
	// value provides a reasonable buffer for typical SGR sequences
	// containing multiple attributes.
	builderGrowthFactor = 4

	// rgbColorFormatSpecifier is the SGR parameter value (2) that indicates
	// the following parameters specify a 24-bit RGB color using the format
	// "38;2;R;G;B" for foreground or "48;2;R;G;B" for background colors.
	rgbColorFormatSpecifier Attribute = 2
)

// Base text formatting attributes.
// These attributes control fundamental text styling such as boldness,
// italics, underlining, and visibility. Multiple base attributes can be
// combined to create complex text formatting.
//
// Base attributes start at SGR code 0 (Reset) and increment through 9.
const (
	// Reset clears all text formatting and returns to the default terminal
	// appearance. This is typically used at the end of styled text to
	// prevent formatting from bleeding into subsequent output.
	Reset Attribute = iota

	// Bold applies increased intensity to the text, making it appear
	// thicker and more prominent. Not all terminal emulators support this.
	Bold

	// Faint applies decreased intensity to the text, making it appear
	// dimmer or lighter than normal text. Support varies across terminals.
	Faint

	// Italic applies italic styling to the text. This is not widely
	// supported across all terminal emulators.
	Italic

	// Underline adds a line beneath the text. This is commonly supported
	// across most terminal emulators.
	Underline

	// BlinkSlow makes the text blink slowly (less than 150 times per
	// minute). This is rarely supported in modern terminal emulators.
	BlinkSlow

	// BlinkRapid makes the text blink rapidly (150 times per minute or
	// more). This is rarely supported in modern terminal emulators.
	BlinkRapid

	// ReverseVideo swaps the foreground and background colors. This is
	// useful for highlighting text or creating inverted color schemes.
	ReverseVideo

	// Concealed hides the text, making it invisible. This is sometimes
	// used for passwords or hidden content. Not widely supported.
	Concealed

	// CrossedOut applies a line through the text (strikethrough).
	// This is useful for indicating deleted or deprecated content.
	CrossedOut
)

// Reset attributes for clearing specific text formatting.
// These attributes undo the effects of their corresponding base attributes,
// allowing selective removal of formatting without a full reset.
//
// Reset attributes start at SGR code 22 and increment through 29,
// corresponding to the base attributes in the 0-9 range.
const (
	// ResetBold removes bold or faint formatting, returning text to
	// normal intensity.
	ResetBold Attribute = iota + 22

	// ResetItalic removes italic formatting from the text.
	ResetItalic

	// ResetUnderline removes underline formatting from the text.
	ResetUnderline

	// ResetBlinking disables any blinking effects applied to the text.
	ResetBlinking

	// _ is a placeholder for SGR code 25, which is reserved.
	_

	// ResetReversed restores normal color ordering, undoing the effect
	// of ReverseVideo.
	ResetReversed

	// ResetConcealed makes concealed text visible again.
	ResetConcealed

	// ResetCrossedOut removes the strikethrough formatting from the text.
	ResetCrossedOut
)

// Foreground colors for text styling.
// These attributes set the text color using the standard 8-color palette
// defined in the ANSI standard. Each color corresponds to a specific SGR
// code in the 30-37 range.
//
// Foreground colors can be combined with base attributes and background
// colors to create rich text styling.
const (
	// FgBlack sets the foreground color to black.
	FgBlack Attribute = iota + 30

	// FgRed sets the foreground color to red.
	FgRed

	// FgGreen sets the foreground color to green.
	FgGreen

	// FgYellow sets the foreground color to yellow.
	FgYellow

	// FgBlue sets the foreground color to blue.
	FgBlue

	// FgMagenta sets the foreground color to magenta (purple).
	FgMagenta

	// FgCyan sets the foreground color to cyan.
	FgCyan

	// FgWhite sets the foreground color to white.
	FgWhite

	// foreground is an internal marker used to distinguish foreground
	// color attributes from other attribute types. It is used internally
	// for implementing 256-color and 24-bit RGB color support.
	foreground
)

// High-intensity foreground colors.
// These attributes provide bright or bold variants of the standard
// foreground colors, corresponding to SGR codes in the 90-97 range.
// High-intensity colors offer better visibility and contrast in
// terminal output.
const (
	// FgHiBlack sets the foreground color to bright black (gray).
	FgHiBlack Attribute = iota + 90

	// FgHiRed sets the foreground color to bright red.
	FgHiRed

	// FgHiGreen sets the foreground color to bright green.
	FgHiGreen

	// FgHiYellow sets the foreground color to bright yellow.
	FgHiYellow

	// FgHiBlue sets the foreground color to bright blue.
	FgHiBlue

	// FgHiMagenta sets the foreground color to bright magenta.
	FgHiMagenta

	// FgHiCyan sets the foreground color to bright cyan.
	FgHiCyan

	// FgHiWhite sets the foreground color to bright white.
	FgHiWhite
)

// Background colors for text styling.
// These attributes set the background color using the standard 8-color
// palette defined in the ANSI standard. Each color corresponds to a
// specific SGR code in the 40-47 range.
//
// Background colors can be combined with foreground colors and text
// attributes to create visually distinct output.
const (
	// BgBlack sets the background color to black.
	BgBlack Attribute = iota + 40

	// BgRed sets the background color to red.
	BgRed

	// BgGreen sets the background color to green.
	BgGreen

	// BgYellow sets the background color to yellow.
	BgYellow

	// BgBlue sets the background color to blue.
	BgBlue

	// BgMagenta sets the background color to magenta (purple).
	BgMagenta

	// BgCyan sets the background color to cyan.
	BgCyan

	// BgWhite sets the background color to white.
	BgWhite

	// background is an internal marker used to distinguish background
	// color attributes from other attribute types. It is used internally
	// for implementing 256-color and 24-bit RGB color support.
	background
)

// High-intensity background colors.
// These attributes provide bright variants of the standard background
// colors, corresponding to SGR codes in the 100-107 range.
// High-intensity background colors can be used for emphasis or
// highlighting specific text regions.
const (
	// BgHiBlack sets the background color to bright black (gray).
	BgHiBlack Attribute = iota + 100

	// BgHiRed sets the background color to bright red.
	BgHiRed

	// BgHiGreen sets the background color to bright green.
	BgHiGreen

	// BgHiYellow sets the background color to bright yellow.
	BgHiYellow

	// BgHiBlue sets the background color to bright blue.
	BgHiBlue

	// BgHiMagenta sets the background color to bright magenta.
	BgHiMagenta

	// BgHiCyan sets the background color to bright cyan.
	BgHiCyan

	// BgHiWhite sets the background color to bright white.
	BgHiWhite
)
