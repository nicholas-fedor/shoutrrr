package telegram

import (
	"github.com/nicholas-fedor/shoutrrr/pkg/format"
	"github.com/nicholas-fedor/shoutrrr/pkg/types"
)

const (
	ParseModeNone       parseMode = iota // 0
	ParseModeMarkdown                    // 1
	ParseModeHTML                        // 2
	ParseModeMarkdownV2                  // 3
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
		}),
}

type parseMode int

type parseModeVals struct {
	None       parseMode
	Markdown   parseMode
	HTML       parseMode
	MarkdownV2 parseMode
	Enum       types.EnumFormatter
}

func (pm parseMode) String() string {
	return ParseModes.Enum.Print(int(pm))
}
