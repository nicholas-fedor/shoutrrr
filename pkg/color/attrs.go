package color

// Attribute defines a single SGR Code.
type Attribute int

const (
	escape = "\x1b"

	// rgbColorFormatSpecifier represents the SGR parameter for 24-bit RGB color format.
	rgbColorFormatSpecifier Attribute = 2

	// builderGrowthFactor is used for pre-allocating strings.Builder capacity.
	// This provides a reasonable buffer for typical SGR sequences.
	builderGrowthFactor = 4
)

// Base attributes.
const (
	Reset Attribute = iota
	Bold
	Faint
	Italic
	Underline
	BlinkSlow
	BlinkRapid
	ReverseVideo
	Concealed
	CrossedOut
)

const (
	ResetBold Attribute = iota + 22
	ResetItalic
	ResetUnderline
	ResetBlinking
	_
	ResetReversed
	ResetConcealed
	ResetCrossedOut
)

// Foreground text colors.
const (
	FgBlack Attribute = iota + 30
	FgRed
	FgGreen
	FgYellow
	FgBlue
	FgMagenta
	FgCyan
	FgWhite

	// used internally for 256 and 24-bit coloring.
	foreground
)

// Foreground Hi-Intensity text colors.
const (
	FgHiBlack Attribute = iota + 90
	FgHiRed
	FgHiGreen
	FgHiYellow
	FgHiBlue
	FgHiMagenta
	FgHiCyan
	FgHiWhite
)

// Background text colors.
const (
	BgBlack Attribute = iota + 40
	BgRed
	BgGreen
	BgYellow
	BgBlue
	BgMagenta
	BgCyan
	BgWhite

	// used internally for 256 and 24-bit coloring.
	background
)

// Background Hi-Intensity text colors.
const (
	BgHiBlack Attribute = iota + 100
	BgHiRed
	BgHiGreen
	BgHiYellow
	BgHiBlue
	BgHiMagenta
	BgHiCyan
	BgHiWhite
)
