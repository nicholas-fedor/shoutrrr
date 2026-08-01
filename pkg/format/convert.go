package format

import (
	"errors"
	"fmt"
	"html"
	"regexp"
	"strconv"
	"strings"
	"unicode"
)

// maxHeadingLevel is the maximum valid HTML heading level.
const maxHeadingLevel = 6

var (
	markdownHeadingRegex         = regexp.MustCompile(`(?m)^#{1,6}\s+`)
	markdownBoldRegex            = regexp.MustCompile(`\*{1,2}(.+?)\*{1,2}`)
	markdownItalicRegex          = regexp.MustCompile(`_{1,2}(.+?)_{1,2}`)
	markdownBoldHTMLRegex        = regexp.MustCompile(`\*\*(.+?)\*\*`)
	markdownItalicStarHTMLRegex  = regexp.MustCompile(`\*([^*].*?)\*`)
	markdownItalicUnderHTMLRegex = regexp.MustCompile(`_([^_].*?)_`)
	markdownStrongUnderHTMLRegex = regexp.MustCompile(`__(.+?)__`)
	markdownCodeRegex            = regexp.MustCompile("`(.+?)`")
	markdownLinkRegex            = regexp.MustCompile(`\[(.+?)\]\(.+?\)`)
	markdownImageRegex           = regexp.MustCompile(`!\[(.+?)\]\(.+?\)`)
	markdownListRegex            = regexp.MustCompile(`(?m)^\s*[-+*]\s+`)
	markdownHRRegex              = regexp.MustCompile(`(?m)^\s*[-*_]{3,}\s*$`)
	htmlTagRegex                 = regexp.MustCompile(`<[^>]+>`)
	mismatchedStrongEmRegex      = regexp.MustCompile(`<strong><em>(.+?)</strong></em>`)
	mismatchedEmStrongRegex      = regexp.MustCompile(`<em><strong>(.+?)</em></strong>`)
	ErrUnsupportedFormat         = errors.New("unsupported format conversion")
)

// ConvertFormat converts a message body from one format to another.
//
// Supported formats: "text", "markdown", "html".
// Conversions:
//   - markdown → text: strips markdown formatting, returns plain text
//   - html → text: strips HTML tags, returns plain text
//   - text → markdown: escapes special characters for safe markdown inclusion
//   - text → html: escapes HTML entities
//   - markdown → html: converts basic markdown to HTML
//   - html → markdown: converts basic HTML to markdown
//   - any → same format: returns body unchanged
//
// Parameters:
//   - body: the message body to convert.
//   - from: the source format.
//   - targetFormat: the desired output format.
//
// Returns:
//   - string: the converted body.
//   - error: an error if the conversion is unsupported.
func ConvertFormat(body, from, targetFormat string) (string, error) {
	if from == targetFormat {
		return body, nil
	}

	switch from {
	case "markdown":
		switch targetFormat {
		case "text":
			return markdownToText(body), nil
		case "html":
			return markdownToHTML(body), nil
		}
	case "html":
		switch targetFormat {
		case "text":
			return htmlToText(body), nil
		case "markdown":
			return htmlToMarkdown(body), nil
		}
	case "text":
		switch targetFormat {
		case "markdown":
			return textToMarkdown(body), nil
		case "html":
			return textToHTML(body), nil
		}
	}

	return "", fmt.Errorf("%w: %s → %s", ErrUnsupportedFormat, from, targetFormat)
}

// markdownToText strips markdown formatting from the body and returns plain text.
//
// Parameters:
//   - body: the markdown text.
//
// Returns:
//   - string: the plain text.
func markdownToText(body string) string {
	const (
		astPlaceholder = "\x00AST\x00"
		undPlaceholder = "\x00UND\x00"
	)

	text := body
	text = markdownHeadingRegex.ReplaceAllString(text, "")
	text = strings.ReplaceAll(text, `\*`, astPlaceholder)
	text = strings.ReplaceAll(text, `\_`, undPlaceholder)
	text = markdownBoldRegex.ReplaceAllString(text, "$1")
	text = markdownItalicRegex.ReplaceAllString(text, "$1")
	text = markdownCodeRegex.ReplaceAllString(text, "$1")
	text = markdownImageRegex.ReplaceAllString(text, "$1")
	text = markdownLinkRegex.ReplaceAllString(text, "$1")
	text = markdownListRegex.ReplaceAllString(text, "")
	text = markdownHRRegex.ReplaceAllString(text, "")
	text = strings.ReplaceAll(text, astPlaceholder, "*")
	text = strings.ReplaceAll(text, undPlaceholder, "_")
	text = strings.TrimSpace(text)

	return text
}

// markdownToHTML converts basic markdown to HTML.
//
// Parameters:
//   - body: the markdown text.
//
// Returns:
//   - string: the HTML text.
func markdownToHTML(body string) string {
	lines := strings.Split(body, "\n")

	var htmlBuilder strings.Builder

	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if trimmed == "" {
			htmlBuilder.WriteString("<br>")

			continue
		}

		if strings.HasPrefix(trimmed, "#") {
			level := 0

			for _, c := range trimmed {
				if c == '#' {
					level++
				} else {
					break
				}
			}

			if level <= maxHeadingLevel && (level < len(trimmed) && unicode.IsSpace(rune(trimmed[level]))) {
				htmlBuilder.WriteString("<h")
				htmlBuilder.WriteString(strconv.Itoa(level))
				htmlBuilder.WriteString(">")

				text := strings.TrimSpace(trimmed[level:])
				text = applyInlineFormatting(text)
				htmlBuilder.WriteString(text)

				htmlBuilder.WriteString("</h")
				htmlBuilder.WriteString(strconv.Itoa(level))
				htmlBuilder.WriteString(">")

				continue
			}
		}

		htmlBuilder.WriteString("<p>")

		text := trimmed
		text = applyInlineFormatting(text)
		htmlBuilder.WriteString(text)
		htmlBuilder.WriteString("</p>")
	}

	return htmlBuilder.String()
}

// htmlToText strips HTML tags from the body and returns plain text.
//
// Parameters:
//   - body: the HTML text.
//
// Returns:
//   - string: the plain text.
func htmlToText(body string) string {
	text := htmlTagRegex.ReplaceAllString(body, "")
	text = html.UnescapeString(text)
	text = strings.TrimSpace(text)

	return text
}

// htmlToMarkdown converts basic HTML to markdown.
//
// Parameters:
//   - body: the HTML text.
//
// Returns:
//   - string: the markdown text.
func htmlToMarkdown(body string) string {
	var lastHref string

	text := htmlTagRegex.ReplaceAllStringFunc(body, func(tag string) string {
		lower := strings.ToLower(tag)
		switch {
		case strings.HasPrefix(lower, "<br"):
			return "\n"
		case lower == "</strong>", lower == "</b>":
			return "**"
		case lower == "</em>", lower == "</i>":
			return "_"
		case strings.HasPrefix(lower, "<h1"):
			return "# "
		case strings.HasPrefix(lower, "<h2"):
			return "## "
		case strings.HasPrefix(lower, "<h3"):
			return "### "
		case strings.HasPrefix(lower, "<h4"):
			return "#### "
		case strings.HasPrefix(lower, "<h5"):
			return "##### "
		case strings.HasPrefix(lower, "<h6"):
			return "###### "
		case strings.HasPrefix(lower, "<a") && (len(lower) > 2 && (lower[2] == ' ' || lower[2] == '>')):
			if idx := strings.Index(lower, `href="`); idx != -1 {
				start := idx + len(`href="`)

				end := strings.Index(lower[start:], `"`)
				if end != -1 {
					lastHref = tag[start : start+end]
				}
			} else if idx := strings.Index(lower, `href='`); idx != -1 {
				start := idx + len(`href='`)

				end := strings.Index(lower[start:], `'`)
				if end != -1 {
					lastHref = tag[start : start+end]
				}
			}

			return "["
		case lower == "</a>":
			href := lastHref
			lastHref = ""

			if href != "" {
				return "](" + href + ")"
			}

			return "]"
		case strings.HasPrefix(lower, "<strong") && (len(lower) > 7 && (lower[7] == '>' || lower[7] == ' ')),
			strings.HasPrefix(lower, "<b") && (len(lower) > 2 && (lower[2] == '>' || lower[2] == ' ')):
			return "**"
		case strings.HasPrefix(lower, "<em") && (len(lower) > 3 && (lower[3] == '>' || lower[3] == ' ')),
			strings.HasPrefix(lower, "<i") && (len(lower) > 2 && (lower[2] == '>' || lower[2] == ' ')):
			return "_"
		case lower == "</p>":
			return "\n\n"
		case strings.HasPrefix(lower, "</h1"), strings.HasPrefix(lower, "</h2"),
			strings.HasPrefix(lower, "</h3"), strings.HasPrefix(lower, "</h4"),
			strings.HasPrefix(lower, "</h5"), strings.HasPrefix(lower, "</h6"):
			return "\n"
		case strings.HasPrefix(lower, "</"), strings.HasPrefix(lower, "<!"):
			return ""
		default:
			return ""
		}
	})

	text = strings.TrimSpace(text)
	text = html.UnescapeString(text)

	return text
}

// textToMarkdown escapes special characters in the body for safe markdown inclusion.
//
// Parameters:
//   - body: the plain text.
//
// Returns:
//   - string: the escaped markdown text.
func textToMarkdown(body string) string {
	text := strings.ReplaceAll(body, "_", "\\_")
	text = strings.ReplaceAll(text, "*", "\\*")
	text = strings.ReplaceAll(text, "`", "\\`")
	text = strings.ReplaceAll(text, "[", "\\[")

	return text
}

// textToHTML escapes HTML entities in the body.
//
// Parameters:
//   - body: the plain text.
//
// Returns:
//   - string: the HTML-escaped text.
func textToHTML(body string) string {
	return html.EscapeString(body)
}

// applyInlineFormatting prepares text for emphasis conversion by protecting
// escaped delimiters, running emphasis conversion, then restoring literals.
//
// Parameters:
//   - text: the markdown text.
//
// Returns:
//   - string: the text with emphasis converted.
func applyInlineFormatting(text string) string {
	text = html.EscapeString(text)
	text = strings.ReplaceAll(text, `\*`, "\x00AST\x00")
	text = strings.ReplaceAll(text, `\_`, "\x00UND\x00")
	text = applyEmphasis(text)
	text = strings.ReplaceAll(text, "\x00AST\x00", "*")
	text = strings.ReplaceAll(text, "\x00UND\x00", "_")

	return text
}

// applyEmphasis converts markdown emphasis markers to HTML tags.
//
// It iteratively applies bold then italic regex passes until stable,
// then fixes the two possible mismatched-nesting patterns that can
// arise when emphasis markers straddle inserted tags.
//
// Parameters:
//   - text: the markdown text.
//
// Returns:
//   - string: the HTML text with emphasis tags.
func applyEmphasis(text string) string {
	text = strings.ReplaceAll(text, `\*`, "\x00AST\x00")
	text = strings.ReplaceAll(text, `\_`, "\x00UND\x00")

	changed := true
	for changed {
		changed = false

		newText := markdownBoldHTMLRegex.ReplaceAllString(text, "<strong>$1</strong>")
		if newText != text {
			changed = true
			text = newText
		}

		newText = markdownItalicStarHTMLRegex.ReplaceAllString(text, "<em>$1</em>")
		if newText != text {
			changed = true
			text = newText
		}

		newText = markdownStrongUnderHTMLRegex.ReplaceAllString(text, "<strong>$1</strong>")
		if newText != text {
			changed = true
			text = newText
		}

		newText = markdownItalicUnderHTMLRegex.ReplaceAllString(text, "<em>$1</em>")
		if newText != text {
			changed = true
			text = newText
		}
	}

	text = strings.ReplaceAll(text, "\x00AST\x00", "*")
	text = strings.ReplaceAll(text, "\x00UND\x00", "_")

	text = mismatchedStrongEmRegex.ReplaceAllString(text, `<strong><em>$1</em></strong>`)
	text = mismatchedEmStrongRegex.ReplaceAllString(text, `<em><strong>$1</strong></em>`)

	return text
}
