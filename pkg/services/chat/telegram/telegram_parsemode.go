package telegram

import (
	"github.com/nicholas-fedor/shoutrrr/pkg/format"
	"github.com/nicholas-fedor/shoutrrr/pkg/types"
)

// parseMode is how Telegram formats the text of a message.
type parseMode int

// parseModeVals holds the parse modes and their enum formatter.
type parseModeVals struct {
	// Enum formats and parses the parse mode names.
	Enum types.EnumFormatter
	// None sends the text as is.
	None parseMode
	// Markdown formats the text as legacy Markdown.
	Markdown parseMode
	// HTML formats the text as HTML.
	HTML parseMode
	// MarkdownV2 formats the text as MarkdownV2.
	MarkdownV2 parseMode
}

// Parse modes, in the order of their names in the enum formatter.
const (
	// ParseModeNone sends the text as is.
	ParseModeNone parseMode = iota
	// ParseModeMarkdown formats the text as legacy Markdown.
	ParseModeMarkdown
	// ParseModeHTML formats the text as HTML.
	ParseModeHTML
	// ParseModeMarkdownV2 formats the text as MarkdownV2.
	ParseModeMarkdownV2
)

// ParseModes is an enum helper for parseMode.
var ParseModes = &parseModeVals{
	None:       ParseModeNone,
	Markdown:   ParseModeMarkdown,
	HTML:       ParseModeHTML,
	MarkdownV2: ParseModeMarkdownV2,
	Enum: format.CreateEnumFormatter(
		[]string{
			"None",
			"Markdown",
			"HTML",
			"MarkdownV2",
		},
	),
}

// String returns the name of the parse mode, as the parse_mode param takes it.
//
// Returns:
//   - string: the name, such as HTML.
func (pm parseMode) String() string {
	return ParseModes.Enum.Print(int(pm))
}
