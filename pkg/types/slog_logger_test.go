package types

import (
	"bytes"
	"log/slog"
	"testing"

	"github.com/stretchr/testify/assert"
)

// TestSlogLoggerPrint verifies that each Print method logs its text as one
// record at Info, formatted as the fmt function of the same name formats it,
// without Println's final newline.
func TestSlogLoggerPrint(t *testing.T) {
	t.Parallel()

	var out bytes.Buffer

	logger := NewSlogLogger(slog.New(slog.NewTextHandler(&out, &slog.HandlerOptions{
		ReplaceAttr: func(groups []string, attr slog.Attr) slog.Attr {
			if len(groups) == 0 && attr.Key == slog.TimeKey {
				return slog.Attr{}
			}

			return attr
		},
	})))

	logger.Print("foo", 7)
	logger.Printf("bar %d", 8)
	logger.Println("baz", 9)

	assert.Equal(t,
		"level=INFO msg=foo7\n"+
			`level=INFO msg="bar 8"`+"\n"+
			`level=INFO msg="baz 9"`+"\n",
		out.String())
}

// TestSlogLoggerPrintSource verifies that printed text carries no source
// location, even for a handler that adds the source.
func TestSlogLoggerPrintSource(t *testing.T) {
	t.Parallel()

	var out bytes.Buffer

	logger := NewSlogLogger(slog.New(slog.NewTextHandler(&out, &slog.HandlerOptions{AddSource: true})))
	logger.Print("message")

	assert.Contains(t, out.String(), "msg=message")
	assert.NotContains(t, out.String(), "source=")
}

// TestSlogLoggerSlog verifies that Slog returns the slog logger the
// SlogLogger was made with, and a logger that discards every record when there
// is none, so the zero value and a nil pointer are safe to use.
func TestSlogLoggerSlog(t *testing.T) {
	t.Parallel()

	logger := slog.New(slog.DiscardHandler)
	assert.Same(t, logger, NewSlogLogger(logger).Slog())

	// tests lists SlogLoggers without a slog logger.
	tests := map[string]*SlogLogger{
		"nil logger":  NewSlogLogger(nil),
		"zero value":  {},
		"nil pointer": nil,
	}

	for name, logger := range tests {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			assert.False(t, logger.Slog().Enabled(t.Context(), slog.LevelError))
			assert.NotPanics(t, func() {
				logger.Print("discarded")
				logger.Printf("discarded %d", 1)
				logger.Println("discarded")
			})
		})
	}
}
