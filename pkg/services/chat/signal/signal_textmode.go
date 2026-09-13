package signal

import (
	"github.com/nicholas-fedor/shoutrrr/pkg/format"
	"github.com/nicholas-fedor/shoutrrr/pkg/types"
)

// textMode is the signal-cli-rest-api text_mode value.
type textMode int

// textModeVals holds named text mode values and their enum formatter.
type textModeVals struct {
	// None omits text_mode so the server default applies.
	None textMode
	// Normal sends text_mode=normal.
	Normal textMode
	// Styled sends text_mode=styled.
	Styled textMode
	// Enum formats and parses text mode values.
	Enum types.EnumFormatter
}

const (
	// TextModeNone omits the JSON text_mode field.
	TextModeNone textMode = iota
	// TextModeNormal sends plaintext to the API.
	TextModeNormal
	// TextModeStyled enables styled markup in the message body.
	TextModeStyled
)

// TextModes is the enum helper for textMode.
var TextModes = &textModeVals{
	None:   TextModeNone,
	Normal: TextModeNormal,
	Styled: TextModeStyled,
	Enum: format.CreateEnumFormatter(
		[]string{
			"None",
			"Normal",
			"Styled",
		},
	),
}

// String returns the config enum name for this text mode.
//
// Returns:
//   - None, Normal, Styled, or Invalid.
func (tm textMode) String() string {
	return TextModes.Enum.Print(int(tm))
}

// payloadValue returns the JSON text_mode value, or empty to omit the field.
//
// Returns:
//   - "normal", "styled", or "".
func (tm textMode) payloadValue() string {
	switch tm {
	case TextModeNone:
		return ""
	case TextModeNormal:
		return "normal"
	case TextModeStyled:
		return "styled"
	default:
		return ""
	}
}
