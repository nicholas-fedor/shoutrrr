package smtp

import (
	"bytes"
	"errors"
	"net/mail"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/nicholas-fedor/shoutrrr/internal/failures"
	"github.com/nicholas-fedor/shoutrrr/pkg/services/standard"
)

// failingWriter accepts a number of writes and fails every later one.
type failingWriter struct {
	// written holds the accepted writes.
	written bytes.Buffer
	// writes is the number of writes to accept.
	writes int
}

// errWriteFailed is the error a failingWriter returns.
var errWriteFailed = errors.New("stream closed")

// newTemplates returns templates for the plain and HTML parts.
//
// Parameters:
//   - t: the test.
//   - plain: the plain template, or "" for none.
//   - html: the HTML template, or "" for none.
//
// Returns:
//   - *standard.Templater: the templates.
func newTemplates(t *testing.T, plain, html string) *standard.Templater {
	t.Helper()

	templates := &standard.Templater{}

	if plain != "" {
		require.NoError(t, templates.SetTemplateString(templatePlain, plain))
	}

	if html != "" {
		require.NoError(t, templates.SetTemplateString(templateHTML, html))
	}

	return templates
}

// TestHeaders verifies the message headers: the subject, sender, and
// recipient encoded for US-ASCII header values, the MIME type of plain or HTML
// mail, a Date, and a Message-ID on the sender's domain.
func TestHeaders(t *testing.T) {
	t.Parallel()

	// tests lists configs and recipients with the headers each should
	// produce, apart from Date and Message-ID.
	tests := []struct {
		want     map[string]string
		name     string
		to       string
		boundary string
		config   Config
	}{
		{
			name:   "plain text",
			config: Config{FromAddress: "sender@example.com", Subject: "Backup done"},
			to:     "rec@example.com",
			want: map[string]string{
				"Subject": "Backup done", "To": "<rec@example.com>", "From": "<sender@example.com>", "MIME-version": "1.0",
				"Content-Type": `text/plain; charset="UTF-8"`, "Content-Transfer-Encoding": "8bit",
			},
		},
		{
			name:     "HTML",
			config:   Config{FromAddress: "sender@example.com", FromName: "Alerts", Subject: "Backup done", UseHTML: true},
			to:       "rec@example.com",
			boundary: "0123456789abcdef",
			want: map[string]string{
				"Subject": "Backup done", "To": "<rec@example.com>", "From": `"Alerts" <sender@example.com>`, "MIME-version": "1.0",
				"Content-Type": "multipart/alternative; boundary=0123456789abcdef",
			},
		},
		{
			name:   "non-ASCII subject and sender name",
			config: Config{FromAddress: "sender@example.com", FromName: "Grüßer", Subject: "Prüfung"},
			to:     "rec@example.com",
			want: map[string]string{
				"Subject": "=?UTF-8?q?Pr=C3=BCfung?=", "To": "<rec@example.com>", "From": "=?utf-8?q?Gr=C3=BC=C3=9Fer?= <sender@example.com>",
				"MIME-version": "1.0", "Content-Type": `text/plain; charset="UTF-8"`, "Content-Transfer-Encoding": "8bit",
			},
		},
		{
			name:   "name and address that need quoting",
			config: Config{FromAddress: "odd name@example.com", FromName: "Doe, John", Subject: "=?UTF-8?q?Hello?="},
			to:     "rec@example.com",
			want: map[string]string{
				"Subject": "=?UTF-8?q?Hello?=", "To": "<rec@example.com>", "From": `"Doe, John" <"odd name"@example.com>`,
				"MIME-version": "1.0", "Content-Type": `text/plain; charset="UTF-8"`, "Content-Transfer-Encoding": "8bit",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			got := headers(&tt.config, tt.to, tt.boundary)

			date, err := mail.ParseDate(got["Date"])
			require.NoError(t, err)
			assert.WithinDuration(t, time.Now(), date, time.Minute)
			assert.Regexp(t, `^<[0-9a-f]{16}@example\.com>$`, got["Message-ID"])

			delete(got, "Date")
			delete(got, "Message-ID")
			assert.Equal(t, tt.want, got)
		})
	}
}

// TestHeadersEncoding verifies that header values keep line breaks out of the
// raw header, so they cannot add headers, encode a non-ASCII subject and
// sender name, and stay parseable.
func TestHeadersEncoding(t *testing.T) {
	t.Parallel()

	got := headers(&Config{
		FromAddress: "sender@example.com",
		FromName:    "Ops\r\nBcc: victim@example.com",
		Subject:     "Hi\r\nBcc: victim@example.com",
	}, "grüß@example.com", "")

	for _, name := range []string{"Subject", "From", "To"} {
		assert.NotContains(t, got[name], "\r", name)
		assert.NotContains(t, got[name], "\n", name)
	}

	for _, name := range []string{"Subject", "From"} {
		assert.Regexp(t, `^[\x20-\x7e]*$`, got[name], name)
	}

	from, err := mail.ParseAddress(got["From"])
	require.NoError(t, err)
	assert.Equal(t, &mail.Address{Name: "Ops\r\nBcc: victim@example.com", Address: "sender@example.com"}, from)

	to, err := mail.ParseAddress(got["To"])
	require.NoError(t, err)
	assert.Equal(t, "grüß@example.com", to.Address)
}

// TestHeadersFoldsLongSubject verifies that a subject longer than a header line
// is folded so that no line exceeds 998 octets and unfolding restores it.
func TestHeadersFoldsLongSubject(t *testing.T) {
	t.Parallel()

	subject := strings.Repeat("A", 1200)
	got := headers(&Config{FromAddress: "sender@example.com", Subject: subject}, "rec@example.com", "")

	assert.Equal(t, subject, strings.ReplaceAll(got["Subject"], "\n ", ""))

	for line := range strings.SplitSeq("Subject: "+got["Subject"], "\n") {
		assert.LessOrEqual(t, len(line), maxHeaderLineOctets)
	}
}

// TestGenerateMessageID verifies that a Message-ID uses the sender's domain,
// or localhost when the address has none, around a random token.
func TestGenerateMessageID(t *testing.T) {
	t.Parallel()

	// tests lists sender addresses with the Message-ID pattern each should
	// produce.
	tests := []struct {
		address string
		want    string
	}{
		{address: "sender@example.com", want: `^<[0-9a-f]{16}@example\.com>$`},
		{address: "sender", want: `^<[0-9a-f]{16}@localhost>$`},
		{address: "sender@", want: `^<[0-9a-f]{16}@localhost>$`},
	}

	for _, tt := range tests {
		t.Run(tt.address, func(t *testing.T) {
			t.Parallel()

			assert.Regexp(t, tt.want, generateMessageID(tt.address))
		})
	}

	first, second := generateMessageID("sender@example.com"), generateMessageID("sender@example.com")
	assert.NotEqual(t, first, second)
}

// TestGenerateBoundary verifies that a boundary is a random 16 hex digit
// token.
func TestGenerateBoundary(t *testing.T) {
	t.Parallel()

	first, err := generateBoundary()
	require.NoError(t, err)

	second, err := generateBoundary()
	require.NoError(t, err)

	assert.Regexp(t, `^[0-9a-f]{16}$`, first)
	assert.NotEqual(t, first, second)
}

// TestFoldHeaderValue verifies that a header value longer than a header line
// is split into lines of at most 998 octets, the first counting the name,
// and that folding an already folded value changes nothing.
func TestFoldHeaderValue(t *testing.T) {
	t.Parallel()

	assert.Equal(t, "short", foldHeaderValue("Subject", "short"))

	value := strings.Repeat("A", 2500)
	folded := foldHeaderValue("Subject", value)
	lines := strings.Split(folded, "\n ")

	require.Len(t, lines, 3)
	assert.Len(t, lines[0], maxHeaderLineOctets-len("Subject: "))
	assert.Len(t, lines[1], maxHeaderLineOctets-1)
	assert.Equal(t, value, strings.Join(lines, ""))
	assert.Equal(t, folded, foldHeaderValue("Subject", folded))
	assert.Equal(t, "ab", foldHeaderValue("X", "a\r\n b"))
	assert.Equal(t, "ab", foldHeaderValue("X", "a\r b"))
}

// TestWriteHeaders verifies that headers are written in name order with a
// blank line after them.
func TestWriteHeaders(t *testing.T) {
	t.Parallel()

	var out bytes.Buffer

	require.NoError(t, writeHeaders(&out, map[string]string{"Subject": "Hi", "From": "<a@example.com>", "To": "<b@example.com>"}))
	assert.Equal(t, "From: <a@example.com>\nSubject: Hi\nTo: <b@example.com>\n\n", out.String())
}

// TestWriteHeadersErrors verifies that a failed write of a header or of the
// blank line after the headers fails with [FailWriteHeaders].
func TestWriteHeadersErrors(t *testing.T) {
	t.Parallel()

	for writes := range 2 {
		writer := &failingWriter{writes: writes}

		err := writeHeaders(writer, map[string]string{"Subject": "Hi"})

		require.ErrorIs(t, err, fail(FailWriteHeaders, nil))
		require.ErrorIs(t, err, errWriteFailed)
	}
}

// TestWriteAlternative verifies the multipart body of an HTML message: a plain
// and an HTML part, each through its template or as the raw message, and the
// closing boundary.
func TestWriteAlternative(t *testing.T) {
	t.Parallel()

	var out bytes.Buffer

	require.NoError(t, writeAlternative(&out, "Hello", "b0und", newTemplates(t, "", "<p>{{.message}}</p>")))
	assert.Equal(t,
		"\n\n--b0und\nContent-Type: text/plain; charset=\"UTF-8\"\nContent-Transfer-Encoding: 8bit\n\nHello"+
			"\n\n--b0und\nContent-Type: text/html; charset=\"UTF-8\"\nContent-Transfer-Encoding: 8bit\n\n<p>Hello</p>"+
			"\n\n--b0und--",
		out.String())
}

// TestWriteAlternativeErrors verifies the failure of each write of the
// multipart body.
func TestWriteAlternativeErrors(t *testing.T) {
	t.Parallel()

	// tests lists the writes accepted before the failure with the failure
	// each should produce.
	tests := []struct {
		name   string
		writes int
		want   failures.FailureID
	}{
		{name: "plain part header", writes: 0, want: FailPlainHeader},
		{name: "plain part", writes: 2, want: FailMessageRaw},
		{name: "HTML part header", writes: 3, want: FailHTMLHeader},
		{name: "HTML part", writes: 5, want: FailMessageRaw},
		{name: "closing boundary", writes: 6, want: FailMultiEndHeader},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			err := writeAlternative(&failingWriter{writes: tt.writes}, "Hello", "b0und", &standard.Templater{})

			require.ErrorIs(t, err, fail(tt.want, nil))
			require.ErrorIs(t, err, errWriteFailed)
		})
	}
}

// TestWritePart verifies that a part is written through its template when one
// is registered, or as the raw message otherwise.
func TestWritePart(t *testing.T) {
	t.Parallel()

	var raw, templated bytes.Buffer

	require.NoError(t, writePart(&raw, "Hello", templatePlain, &standard.Templater{}))
	require.NoError(t, writePart(&templated, "Hello", templatePlain, newTemplates(t, "Note: {{.message}}", "")))

	assert.Equal(t, "Hello", raw.String())
	assert.Equal(t, "Note: Hello", templated.String())
}

// TestWritePartErrors verifies that a template that fails, or a failed write
// of a templated or raw part, fails with the matching failure.
func TestWritePartErrors(t *testing.T) {
	t.Parallel()

	err := writePart(&bytes.Buffer{}, "Hello", templatePlain, newTemplates(t, "{{.message.Missing}}", ""))
	require.ErrorIs(t, err, fail(FailMessageTemplate, nil))

	err = writePart(&failingWriter{}, "Hello", templatePlain, newTemplates(t, "Note: {{.message}}", ""))
	require.ErrorIs(t, err, fail(FailMessageTemplate, nil))
	require.ErrorIs(t, err, errWriteFailed)

	err = writePart(&failingWriter{}, "Hello", templatePlain, &standard.Templater{})
	require.ErrorIs(t, err, fail(FailMessageRaw, nil))
	require.ErrorIs(t, err, errWriteFailed)
}

// TestWritePartHeader verifies a part header and the closing boundary, and
// that a failed write of either line is reported.
func TestWritePartHeader(t *testing.T) {
	t.Parallel()

	var part, closing bytes.Buffer

	require.NoError(t, writePartHeader(&part, "b0und", contentPlain))
	require.NoError(t, writePartHeader(&closing, "b0und", ""))

	assert.Equal(t, "\n\n--b0und\nContent-Type: text/plain; charset=\"UTF-8\"\nContent-Transfer-Encoding: 8bit\n\n", part.String())
	assert.Equal(t, "\n\n--b0und--", closing.String())

	require.ErrorIs(t, writePartHeader(&failingWriter{}, "b0und", contentPlain), errWriteFailed)
	require.ErrorIs(t, writePartHeader(&failingWriter{writes: 1}, "b0und", contentPlain), errWriteFailed)
}

// Write accepts p while writes remain and fails afterwards.
//
// Parameters:
//   - p: the bytes to write.
//
// Returns:
//   - int: the number of bytes written.
//   - error: [errWriteFailed] once no writes remain.
func (w *failingWriter) Write(p []byte) (int, error) {
	if w.writes == 0 {
		return 0, errWriteFailed
	}

	w.writes--

	return w.written.Write(p)
}
