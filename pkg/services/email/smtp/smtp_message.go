package smtp

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"io"
	"maps"
	"mime"
	"net/mail"
	"slices"
	"strings"
	"time"

	"github.com/nicholas-fedor/shoutrrr/pkg/types"
)

const (
	// contentHTML is the Content-Type for HTML message parts.
	contentHTML = "text/html; charset=\"UTF-8\""
	// contentPlain is the Content-Type for plain-text message parts.
	contentPlain = "text/plain; charset=\"UTF-8\""
	// contentMultipart is the Content-Type format string for multipart/alternative messages.
	contentMultipart = "multipart/alternative; boundary=%s"
	// boundaryByteLen is the number of random bytes used to generate a multipart boundary
	// and Message-ID token.
	boundaryByteLen = 8
	// templatePlain is the Templater ID for the text/plain part.
	templatePlain = "plain"
	// templateHTML is the Templater ID for the text/html part.
	templateHTML = "HTML"
	// maxHeaderLineOctets is the RFC 5322 maximum length of a header line,
	// excluding the CRLF. See https://datatracker.ietf.org/doc/html/rfc5322#section-2.1.1.
	maxHeaderLineOctets = 998
	// headerLineOctets is the RFC 5322 recommended length of a header line,
	// excluding the CRLF. See https://datatracker.ietf.org/doc/html/rfc5322#section-2.1.1.
	headerLineOctets = 78
	// encodedWordLineOctets is the RFC 2047 maximum length of a header line
	// that contains an encoded-word. See https://datatracker.ietf.org/doc/html/rfc2047#section-2.
	encodedWordLineOctets = 76
)

// headerUnfolder removes the line breaks of RFC 5322 folding whitespace and
// keeps the whitespace that follows each break.
var headerUnfolder = strings.NewReplacer(
	"\r\n ", " ", "\r\n\t", "\t",
	"\n ", " ", "\n\t", "\t",
	"\r ", " ", "\r\t", "\t",
)

// headers constructs email headers for the SMTP message.
//
// Non-ASCII header values are encoded per RFC 2047
// (https://datatracker.ietf.org/doc/html/rfc2047). Values are left unfolded
// for [writeHeaders] to fold. When [Config.UseHTML] is true, the Content-Type
// is multipart/alternative with the given boundary.
//
// Parameters:
//   - config: SMTP configuration providing sender, subject, and HTML options.
//   - toAddress: The recipient address for the To header.
//   - boundary: Multipart boundary used when HTML mail is enabled.
//
// Returns:
//   - A map of header names to encoded values.
func headers(config *Config, toAddress, boundary string) map[string]string {
	var contentType string
	if config.UseHTML {
		contentType = fmt.Sprintf(contentMultipart, boundary)
	} else {
		contentType = contentPlain
	}

	// Header values must be US-ASCII per RFC 5322 section 2.2
	// (https://datatracker.ietf.org/doc/html/rfc5322#section-2.2), so any
	// non-ASCII content is wrapped in RFC 2047 encoded-words
	// (https://datatracker.ietf.org/doc/html/rfc2047). Pure ASCII values are
	// passed through as-is.
	from := &mail.Address{Name: config.FromName, Address: config.FromAddress}
	recipient := &mail.Address{Name: "", Address: toAddress}

	headerMap := map[string]string{
		"Subject":      mime.QEncoding.Encode("UTF-8", config.Subject),
		"Date":         time.Now().Format(time.RFC1123Z),
		"To":           recipient.String(),
		"From":         from.String(),
		"Message-ID":   generateMessageID(config.FromAddress),
		"MIME-version": "1.0",
		"Content-Type": contentType,
	}
	if !config.UseHTML {
		headerMap["Content-Transfer-Encoding"] = "8bit"
	}

	return headerMap
}

// generateMessageID returns an RFC 5322 Message-ID using the sender domain.
//
// See https://datatracker.ietf.org/doc/html/rfc5322#section-3.6.4.
//
// Parameters:
//   - fromAddress: The sender address whose domain is used in the Message-ID.
//
// Returns:
//   - A Message-ID of the form <id@domain>. The domain falls back to localhost
//     when fromAddress has no host part.
func generateMessageID(fromAddress string) string {
	domain := "localhost"
	if _, host, found := strings.Cut(fromAddress, "@"); found && host != "" {
		domain = host
	}

	b := make([]byte, boundaryByteLen)
	if _, err := rand.Read(b); err != nil {
		return fmt.Sprintf("<%d@%s>", time.Now().UnixNano(), domain)
	}

	return fmt.Sprintf("<%s@%s>", hex.EncodeToString(b), domain)
}

// foldHeader returns a header field folded to RFC 5322 line limits.
//
// Existing folding line breaks in value are removed first, keeping their
// whitespace. Lines longer than [headerLineOctets], or [encodedWordLineOctets]
// when the field contains an RFC 2047 encoded-word, are folded by inserting a
// line break before whitespace, so unfolding restores the field. The space
// after the colon is a fold point too, which moves an encoded-word that does
// not fit after the name onto a continuation line. No line is left with
// whitespace alone. A token that no whitespace splits within
// [maxHeaderLineOctets] is split at that limit, with a space inserted, because
// no line may be longer.
//
// Parameters:
//   - name: The header field name.
//   - value: The header field body, which may already be folded.
//
// Returns:
//   - The `name: value` field with `\n` inserted before folding whitespace.
func foldHeader(name, value string) string {
	field := name + ": " + headerUnfolder.Replace(value)

	limit := headerLineOctets
	if strings.Contains(field, "=?") {
		limit = encodedWordLineOctets
	}

	var builder strings.Builder

	builder.Grow(len(field) + len(field)/limit*2)

	remaining := field
	for len(remaining) > limit {
		breakAt := foldPoint(remaining, limit)
		if breakAt < 0 {
			if len(remaining) <= maxHeaderLineOctets {
				break
			}

			builder.WriteString(remaining[:maxHeaderLineOctets])
			builder.WriteString("\n")

			remaining = " " + remaining[maxHeaderLineOctets:]

			continue
		}

		builder.WriteString(remaining[:breakAt])
		builder.WriteString("\n")

		remaining = remaining[breakAt:]
	}

	builder.WriteString(remaining)

	return builder.String()
}

// foldPoint returns where to break the first line of line, before a
// whitespace character with other text on both sides of it.
//
// It prefers the last such point that keeps the first line within limit, and
// otherwise takes the first point that keeps it within [maxHeaderLineOctets].
//
// Parameters:
//   - line: The rest of the header field, starting at the current line.
//   - limit: The preferred maximum length of the first line.
//
// Returns:
//   - int: the index of the whitespace to break before, or -1 if there is none.
func foldPoint(line string, limit int) int {
	first := strings.IndexFunc(line, isNotFoldingSpace)
	last := strings.LastIndexFunc(line, isNotFoldingSpace)

	for i := min(limit, last-1); i > first; i-- {
		if isFoldingSpace(rune(line[i])) {
			return i
		}
	}

	for i := max(limit+1, first+1); i < last && i <= maxHeaderLineOctets; i++ {
		if isFoldingSpace(rune(line[i])) {
			return i
		}
	}

	return -1
}

// isFoldingSpace reports whether r is RFC 5322 whitespace, a space or a
// horizontal tab.
//
// Parameters:
//   - r: The character to check.
//
// Returns:
//   - bool: true for a space or a horizontal tab.
func isFoldingSpace(r rune) bool {
	return r == ' ' || r == '\t'
}

// isNotFoldingSpace reports whether r is anything other than RFC 5322
// whitespace.
//
// Parameters:
//   - r: The character to check.
//
// Returns:
//   - bool: true for any character but a space or a horizontal tab.
func isNotFoldingSpace(r rune) bool {
	return !isFoldingSpace(r)
}

// generateBoundary returns a random multipart boundary token.
func generateBoundary() (string, error) {
	b := make([]byte, boundaryByteLen)
	if _, err := rand.Read(b); err != nil {
		return "", fmt.Errorf("generating multipart boundary: %w", err)
	}

	return hex.EncodeToString(b), nil
}

// writeHeaders writes email headers to the provided writer.
//
// Keys are written in sorted order, each field folded by [foldHeader]. A blank
// line is written after the headers to separate them from the body.
//
// Parameters:
//   - writer: The DATA stream writer.
//   - headerMap: Header names and values to write.
//
// Returns:
//   - An error if a header line or the trailing blank line cannot be written.
func writeHeaders(writer io.Writer, headerMap map[string]string) error {
	for _, key := range slices.Sorted(maps.Keys(headerMap)) {
		if _, err := fmt.Fprintln(writer, foldHeader(key, headerMap[key])); err != nil {
			return fail(FailWriteHeaders, err)
		}
	}

	_, err := fmt.Fprintln(writer)
	if err != nil {
		return fail(FailWriteHeaders, err)
	}

	return nil
}

// writeAlternative writes a multipart/alternative email body.
//
// It writes a text/plain part followed by a text/html part, then the closing
// boundary.
//
// Parameters:
//   - writer: The DATA stream writer.
//   - message: The notification body used for both parts.
//   - boundary: The multipart boundary string.
//   - templates: Template source for the plain and HTML parts.
//
// Returns:
//   - An error if a part header or body cannot be written.
func writeAlternative(
	writer io.Writer,
	message string,
	boundary string,
	templates types.Templater,
) error {
	if err := writePartHeader(writer, boundary, contentPlain); err != nil {
		return fail(FailPlainHeader, err)
	}

	if err := writePart(writer, message, templatePlain, templates); err != nil {
		return err
	}

	if err := writePartHeader(writer, boundary, contentHTML); err != nil {
		return fail(FailHTMLHeader, err)
	}

	if err := writePart(writer, message, templateHTML, templates); err != nil {
		return err
	}

	if err := writePartHeader(writer, boundary, ""); err != nil {
		return fail(FailMultiEndHeader, err)
	}

	return nil
}

// writePart writes a single message part using the named template when registered.
//
// When no template is registered for the part, the raw message is written.
//
// Parameters:
//   - writer: The DATA stream writer.
//   - message: The notification body.
//   - templateName: The template name to apply, typically [templatePlain] or [templateHTML].
//   - templates: Template source for the part.
//
// Returns:
//   - An error if the template cannot be executed or the raw message cannot be written.
func writePart(writer io.Writer, message, templateName string, templates types.Templater) error {
	if tpl, found := templates.GetTemplate(templateName); found {
		data := map[string]string{
			"message": message,
		}
		if err := tpl.Execute(writer, data); err != nil {
			return fail(FailMessageTemplate, err)
		}

		return nil
	}

	if _, err := fmt.Fprint(writer, message); err != nil {
		return fail(FailMessageRaw, err)
	}

	return nil
}

// writePartHeader writes a multipart boundary header to the provided writer.
//
// An empty contentType writes the closing boundary.
//
// Parameters:
//   - writer: The DATA stream writer.
//   - boundary: The multipart boundary string.
//   - contentType: The Content-Type for the next part, or empty to close the multipart body.
//
// Returns:
//   - An error if the boundary or content-type header cannot be written.
func writePartHeader(writer io.Writer, boundary, contentType string) error {
	suffix := "\n"
	if contentType == "" {
		suffix = "--"
	}

	if _, err := fmt.Fprintf(writer, "\n\n--%s%s", boundary, suffix); err != nil {
		return fmt.Errorf("writing multipart boundary: %w", err)
	}

	if contentType != "" {
		if _, err := fmt.Fprintf(
			writer,
			"Content-Type: %s\nContent-Transfer-Encoding: 8bit\n\n",
			contentType,
		); err != nil {
			return fmt.Errorf("writing content type header: %w", err)
		}
	}

	return nil
}
