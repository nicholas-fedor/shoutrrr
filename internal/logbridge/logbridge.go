package logbridge

import (
	"bytes"
	"context"
	"fmt"
	"log/slog"
	"runtime"
	"slices"
	"strings"
	"time"

	"github.com/nicholas-fedor/shoutrrr/pkg/types"
)

// attrLogger is a [types.StdLogger] whose leveled records carry attributes
// that the plain logger it wraps does not. Text written through its Print
// methods reaches the wrapped logger unchanged.
type attrLogger struct {
	// StdLogger is the wrapped logger. It receives the Print text.
	types.StdLogger

	// logger receives the leveled records, with the attributes added.
	logger *slog.Logger
}

// textHandler renders each record as one line of text for a plain
// [types.StdLogger], in the format of slog's default handler. The line starts
// with the level and the message, and the attributes follow as key=value
// pairs, such as "WARN config key is deprecated key=scheme
// replacement=disabletls". It handles every level, because a plain logger
// cannot filter by level. A textHandler is safe for concurrent use when its
// logger is.
type textHandler struct {
	// logger receives the lines.
	logger types.StdLogger
	// scopes holds the attributes and groups added with WithAttrs and
	// WithGroup, in order. Each one applies its addition to a handler.
	scopes []func(slog.Handler) slog.Handler
}

// New returns the slog logger that receives a service's leveled records.
//
// Parameters:
//   - logger: the logger the library was given.
//
// Returns:
//   - *slog.Logger: the slog logger that logger carries when it carries one,
//     a logger that discards every record when logger is nil, and otherwise a
//     logger that writes each record to logger as one line of text.
func New(logger types.StdLogger) *slog.Logger {
	switch logger := logger.(type) {
	case nil:
		return slog.New(slog.DiscardHandler)
	case *types.SlogLogger:
		return logger.Slog()
	case *attrLogger:
		return logger.logger
	default:
		return slog.New(&textHandler{logger: logger, scopes: nil})
	}
}

// With returns a logger whose leveled records carry the given attributes.
// Text written through its Print methods reaches logger unchanged, so a
// plain logger still gets exactly the text a service prints.
//
// Parameters:
//   - logger: the logger to wrap.
//   - args: attributes as key/value pairs or [slog.Attr] values, as for
//     [slog.Logger.With].
//
// Returns:
//   - types.StdLogger: nil when logger is nil, a [types.SlogLogger] over the
//     attributed slog logger when logger carries one, and otherwise the
//     wrapping logger.
func With(logger types.StdLogger, args ...any) types.StdLogger {
	switch logger := logger.(type) {
	case nil:
		return nil
	case *types.SlogLogger:
		return types.NewSlogLogger(logger.Slog().With(args...))
	case *attrLogger:
		return &attrLogger{StdLogger: logger.StdLogger, logger: logger.logger.With(args...)}
	default:
		return &attrLogger{StdLogger: logger, logger: New(logger).With(args...)}
	}
}

// CallerPC returns the location of a call further up the stack, for a
// record's source.
//
// Parameters:
//   - skip: how many calls above the function that calls CallerPC. With 1, it
//     is the call to that function.
//
// Returns:
//   - uintptr: the program counter of the call, for [slog.NewRecord].
func CallerPC(skip int) uintptr {
	// Skip runtime.Callers and CallerPC itself, as slog's documentation on
	// wrapping output methods describes.
	var pcs [1]uintptr

	runtime.Callers(skip+2, pcs[:]) //nolint:mnd // The two frames explained above.

	return pcs[0]
}

// Log logs a record to logger with sourcePC as its source, as slog's documentation
// on wrapping output methods describes. A handler that adds the source then
// shows the service's call instead of the library's logging code. Errors from
// the handler are discarded, as [slog.Logger] discards them.
//
// Parameters:
//   - ctx: the context of the call, passed to the handler. Nil means the
//     background context.
//   - logger: the logger that receives the record.
//   - sourcePC: the source of the record, from [CallerPC], or 0 for none.
//   - level: the level of the record.
//   - msg: the message.
//   - args: attributes as key/value pairs or [slog.Attr] values.
//
//nolint:contextcheck // A nil context is replaced with the background context, as slog.Logger does.
func Log(ctx context.Context, logger *slog.Logger, sourcePC uintptr, level slog.Level, msg string, args ...any) {
	if ctx == nil {
		ctx = context.Background()
	}

	if !logger.Enabled(ctx, level) {
		return
	}

	record := slog.NewRecord(time.Now(), level, msg, sourcePC)
	record.Add(args...)

	_ = logger.Handler().Handle(ctx, record) //nolint:errcheck // Discarded as slog.Logger discards it.
}

// Enabled reports true for every level, because a plain logger cannot filter
// by level.
//
// Returns:
//   - bool: always true.
func (*textHandler) Enabled(context.Context, slog.Level) bool {
	return true
}

// Handle writes the record to the logger as one line.
//
// Parameters:
//   - ctx: passed to the handler that formats the attributes.
//   - record: the record to write.
//
// Returns:
//   - error: an error from formatting the attributes.
//
//nolint:gocritic // slog.Handler defines Handle with a Record value.
func (h *textHandler) Handle(ctx context.Context, record slog.Record) error {
	// slog's text handler formats the attributes, so they are quoted and
	// grouped as slog quotes and groups them. The line writes the level and
	// message in front of them, so the formatter leaves those out. A zero time
	// leaves out the time.
	var attrs bytes.Buffer

	var formatter slog.Handler = slog.NewTextHandler(&attrs, &slog.HandlerOptions{
		AddSource:   false,
		Level:       slog.LevelDebug,
		ReplaceAttr: builtinAttrDropper(record.Level, record.Message),
	})

	for _, scope := range h.scopes {
		formatter = scope(formatter)
	}

	bare := record.Clone()
	bare.Time = time.Time{}

	err := formatter.Handle(ctx, bare)
	if err != nil {
		return fmt.Errorf("formatting log attributes: %w", err)
	}

	line := record.Level.String() + " " + record.Message
	if formatted := strings.TrimSuffix(attrs.String(), "\n"); formatted != "" {
		line += " " + formatted
	}

	h.logger.Print(line)

	return nil
}

// WithAttrs returns a handler whose lines also carry attrs.
//
// Parameters:
//   - attrs: the attributes to add.
//
// Returns:
//   - slog.Handler: the new handler.
func (h *textHandler) WithAttrs(attrs []slog.Attr) slog.Handler {
	if len(attrs) == 0 {
		return h
	}

	return h.withScope(func(handler slog.Handler) slog.Handler { return handler.WithAttrs(attrs) })
}

// WithGroup returns a handler that qualifies later attributes with name.
//
// Parameters:
//   - name: the group name. An empty name returns h.
//
// Returns:
//   - slog.Handler: the new handler.
func (h *textHandler) WithGroup(name string) slog.Handler {
	if name == "" {
		return h
	}

	return h.withScope(func(handler slog.Handler) slog.Handler { return handler.WithGroup(name) })
}

// withScope returns a copy of h with scope added after its other scopes.
//
// Parameters:
//   - scope: applies an addition to a handler.
//
// Returns:
//   - *textHandler: the copy.
func (h *textHandler) withScope(scope func(slog.Handler) slog.Handler) *textHandler {
	return &textHandler{logger: h.logger, scopes: append(slices.Clip(h.scopes), scope)}
}

// builtinAttrDropper returns a function for [slog.HandlerOptions.ReplaceAttr]
// that leaves the record's level and message out of the formatted attributes.
// It drops the first top-level "level" attribute that holds level, and the
// first top-level "msg" attribute that holds msg. An attribute of a service
// with the same key and value would print the same text, so the result does
// not depend on which of the two comes first. Other attributes keyed "level"
// or "msg" are kept.
//
// Parameters:
//   - level: the record's level.
//   - msg: the record's message.
//
// Returns:
//   - func([]string, slog.Attr) slog.Attr: the replacement function, for one
//     record.
func builtinAttrDropper(level slog.Level, msg string) func([]string, slog.Attr) slog.Attr {
	var levelDropped, msgDropped bool

	return func(groups []string, attr slog.Attr) slog.Attr {
		if len(groups) > 0 {
			return attr
		}

		switch {
		case !levelDropped && attr.Key == slog.LevelKey && attr.Value.Equal(slog.AnyValue(level)):
			levelDropped = true

			return slog.Attr{}
		case !msgDropped && attr.Key == slog.MessageKey && attr.Value.Equal(slog.StringValue(msg)):
			msgDropped = true

			return slog.Attr{}
		default:
			return attr
		}
	}
}
