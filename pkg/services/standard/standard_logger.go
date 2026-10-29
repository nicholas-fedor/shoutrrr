package standard

import (
	"context"
	"log/slog"

	"github.com/nicholas-fedor/shoutrrr/internal/logbridge"
	"github.com/nicholas-fedor/shoutrrr/pkg/types"
	"github.com/nicholas-fedor/shoutrrr/pkg/util"
)

// Logger provides a service's logging methods. Debug, Info, and Warn log
// leveled records with key/value attributes. When the service was given a
// [types.SlogLogger], they reach its slog logger. Any other logger receives
// each record as one line of text, such as "WARN sending without TLS". Log
// and Logf write text to the given logger as it is.
//
// The zero value discards leveled records. Call SetLogger before Log or Logf.
type Logger struct {
	// logger receives the text that Log and Logf write.
	logger types.StdLogger
	// leveled receives the records that Debug, Info, and Warn log. Nil
	// discards them.
	leveled *slog.Logger
}

// discardLogger discards every record. A Logger without a logger logs to it.
var discardLogger = slog.New(slog.DiscardHandler)

// Debug logs msg with its attributes at [slog.LevelDebug], for detail that
// helps to trace a send, such as a request that succeeded.
//
// Parameters:
//   - ctx: the context of the call, passed to the log handler.
//   - msg: the message, a constant that names the event.
//   - args: attributes as key/value pairs or [slog.Attr] values.
func (sl *Logger) Debug(ctx context.Context, msg string, args ...any) {
	sl.logRecord(ctx, slog.LevelDebug, msg, args...)
}

// Info logs msg with its attributes at [slog.LevelInfo].
//
// Parameters:
//   - ctx: the context of the call, passed to the log handler.
//   - msg: the message, a constant that names the event.
//   - args: attributes as key/value pairs or [slog.Attr] values.
func (sl *Logger) Info(ctx context.Context, msg string, args ...any) {
	sl.logRecord(ctx, slog.LevelInfo, msg, args...)
}

// Log maps to the service loggers Logger.Print function.
func (sl *Logger) Log(v ...any) {
	sl.logger.Print(v...)
}

// Logf maps to the service loggers Logger.Printf function.
func (sl *Logger) Logf(format string, v ...any) {
	sl.logger.Printf(format, v...)
}

// SetLogger maps the specified logger to the logging methods.
//
// Parameters:
//   - logger: the logger to use. A nil logger discards everything.
func (sl *Logger) SetLogger(logger types.StdLogger) {
	if logger == nil {
		sl.logger = util.DiscardLogger
	} else {
		sl.logger = logger
	}

	sl.leveled = logbridge.New(logger)
}

// Slog returns the slog logger that receives the records Debug, Info, and
// Warn log.
//
// Returns:
//   - *slog.Logger: the logger, or a logger that discards every record when
//     SetLogger was not called.
func (sl *Logger) Slog() *slog.Logger {
	if sl.leveled == nil {
		return discardLogger
	}

	return sl.leveled
}

// Warn logs msg with its attributes at [slog.LevelWarn], for a problem that
// the send works around, such as a setting that weakens its security.
//
// Parameters:
//   - ctx: the context of the call, passed to the log handler.
//   - msg: the message, a constant that names the event.
//   - args: attributes as key/value pairs or [slog.Attr] values.
func (sl *Logger) Warn(ctx context.Context, msg string, args ...any) {
	sl.logRecord(ctx, slog.LevelWarn, msg, args...)
}

// logRecord logs a record at level. Its source is the location of the call to
// Debug, Info, or Warn.
//
// Parameters:
//   - ctx: the context of the call. Nil means the background context.
//   - level: the level of the record.
//   - msg: the message.
//   - args: attributes as key/value pairs or [slog.Attr] values.
func (sl *Logger) logRecord(ctx context.Context, level slog.Level, msg string, args ...any) {
	// The call two above this method is the service's call to Debug, Info,
	// or Warn.
	logbridge.Log(ctx, sl.Slog(), logbridge.CallerPC(2), level, msg, args...) //nolint:mnd // Explained above.
}
