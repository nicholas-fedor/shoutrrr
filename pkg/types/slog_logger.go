package types

import (
	"context"
	"fmt"
	"log/slog"
	"strings"
	"time"
)

// SlogLogger is a [StdLogger] that carries a [*slog.Logger]. Pass it wherever
// the library takes a StdLogger. Services then log leveled records with
// key/value attributes to the slog logger, instead of lines of text.
//
// Calls to its Print methods log the text as a record at [slog.LevelInfo],
// without a source location, because printed text has no call that names the
// event. The zero value and a nil *SlogLogger discard every record. A SlogLogger is
// safe for concurrent use when its handler is.
type SlogLogger struct {
	// logger receives the records. Nil discards them.
	logger *slog.Logger
}

// discardSlogLogger discards every record. A SlogLogger without a slog logger
// logs to it.
var discardSlogLogger = slog.New(slog.DiscardHandler)

// NewSlogLogger returns a [StdLogger] that logs leveled records to logger.
//
// Parameters:
//   - logger: the slog logger. A nil logger discards every record.
//
// Returns:
//   - *SlogLogger: the logger to pass to the library.
func NewSlogLogger(logger *slog.Logger) *SlogLogger {
	return &SlogLogger{logger: logger}
}

// Print logs args, formatted in the manner of [fmt.Print], at
// [slog.LevelInfo].
//
// Parameters:
//   - args: the values to log.
func (l *SlogLogger) Print(args ...any) {
	l.logText(fmt.Sprint(args...))
}

// Printf logs args, formatted in the manner of [fmt.Printf], at
// [slog.LevelInfo].
//
// Parameters:
//   - format: the format string.
//   - args: the values to format.
func (l *SlogLogger) Printf(format string, args ...any) {
	l.logText(fmt.Sprintf(format, args...))
}

// Println logs args, formatted in the manner of [fmt.Println] without the
// final newline, at [slog.LevelInfo].
//
// Parameters:
//   - args: the values to log.
func (l *SlogLogger) Println(args ...any) {
	l.logText(strings.TrimSuffix(fmt.Sprintln(args...), "\n"))
}

// Slog returns the slog logger that receives the records.
//
// Returns:
//   - *slog.Logger: the logger given to [NewSlogLogger], or a logger that
//     discards every record when there is none.
func (l *SlogLogger) Slog() *slog.Logger {
	if l == nil || l.logger == nil {
		return discardSlogLogger
	}

	return l.logger
}

// logText logs text as a record at [slog.LevelInfo] without a source
// location. Errors from the handler are discarded, as [slog.Logger] discards
// them.
//
// Parameters:
//   - text: the message.
func (l *SlogLogger) logText(text string) {
	logger := l.Slog()
	ctx := context.Background()

	if !logger.Enabled(ctx, slog.LevelInfo) {
		return
	}

	_ = logger.Handler().Handle(ctx, slog.NewRecord(time.Now(), slog.LevelInfo, text, 0)) //nolint:errcheck // Discarded as slog.Logger discards it.
}
