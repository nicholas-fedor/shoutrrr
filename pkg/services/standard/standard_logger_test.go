package standard

import (
	"context"
	"log"
	"log/slog"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/nicholas-fedor/shoutrrr/pkg/types"
	"github.com/nicholas-fedor/shoutrrr/pkg/util"
)

// recordingHandler records the records it handles, with the context of each.
// It is safe for concurrent use.
type recordingHandler struct {
	// contexts holds the context each record was handled with.
	contexts []context.Context
	// records holds the handled records.
	records []slog.Record
	// level is the lowest level the handler handles.
	level slog.Level
	// mu guards contexts and records.
	mu sync.Mutex
}

// contextKey identifies the test value in a context.
type contextKey struct{}

// Enabled reports whether level is at least the handler's level.
//
// Parameters:
//   - level: the level of a record.
//
// Returns:
//   - bool: true when the handler records the level.
func (h *recordingHandler) Enabled(_ context.Context, level slog.Level) bool {
	return level >= h.level
}

// Handle records the record and its context.
//
// Parameters:
//   - ctx: the context of the call.
//   - record: the record.
//
// Returns:
//   - error: always nil.
//
//nolint:gocritic // slog.Handler defines Handle with a Record value.
func (h *recordingHandler) Handle(ctx context.Context, record slog.Record) error {
	h.mu.Lock()
	defer h.mu.Unlock()

	h.contexts = append(h.contexts, ctx)
	h.records = append(h.records, record.Clone())

	return nil
}

// WithAttrs returns h. The tests do not add attributes to the handler.
//
// Returns:
//   - slog.Handler: h.
func (h *recordingHandler) WithAttrs([]slog.Attr) slog.Handler { return h }

// WithGroup returns h. The tests do not add groups to the handler.
//
// Returns:
//   - slog.Handler: h.
func (h *recordingHandler) WithGroup(string) slog.Handler { return h }

// TestLoggerLog verifies that Log writes each message as a line to the logger
// set with SetLogger.
func TestLoggerLog(t *testing.T) {
	t.Parallel()

	var out strings.Builder

	logger := &Logger{}
	logger.SetLogger(log.New(&out, "", 0))

	logger.Log("foo")
	logger.Log("bar", 7)

	assert.Equal(t, "foo\nbar7\n", out.String())
}

// TestLoggerLogf verifies that Logf formats the message before writing it.
func TestLoggerLogf(t *testing.T) {
	t.Parallel()

	var out strings.Builder

	logger := &Logger{}
	logger.SetLogger(log.New(&out, "", 0))

	logger.Logf("foo %d", 7)

	assert.Equal(t, "foo 7\n", out.String())
}

// TestLoggerOutput verifies that Output returns the logger given to
// SetLogger, and the discard logger for a zero-value Logger or after
// SetLogger(nil), so Log and Logf never lack a logger.
func TestLoggerOutput(t *testing.T) {
	t.Parallel()

	plain := log.New(&strings.Builder{}, "", 0)

	given := &Logger{}
	given.SetLogger(plain)
	assert.Same(t, plain, given.Output())

	discarding := &Logger{}
	discarding.SetLogger(nil)

	for _, logger := range []*Logger{{}, discarding} {
		assert.Same(t, util.DiscardLogger, logger.Output())
		assert.NotPanics(t, func() {
			logger.Log("discarded")
			logger.Logf("discarded %d", 1)
		})
	}
}

// TestLoggerLeveledPlain verifies that Debug, Info, and Warn write each record
// as one line to a plain logger, with every level included.
func TestLoggerLeveledPlain(t *testing.T) {
	t.Parallel()

	var out strings.Builder

	logger := &Logger{}
	logger.SetLogger(log.New(&out, "", 0))

	logger.Debug(t.Context(), "request sent", "status", 200)
	logger.Info(t.Context(), "message sent")
	logger.Warn(t.Context(), "response has no timestamp")

	assert.Equal(t,
		"DEBUG request sent status=200\nINFO message sent\nWARN response has no timestamp\n",
		out.String())
}

// TestLoggerLeveledSlog verifies that Debug, Info, and Warn reach the slog
// logger of a [types.SlogLogger] at their levels, filtered by its handler.
// Each record carries the caller's context. Its source is the caller's
// location.
func TestLoggerLeveledSlog(t *testing.T) {
	t.Parallel()

	handler := &recordingHandler{level: slog.LevelInfo}
	ctx := context.WithValue(t.Context(), contextKey{}, "send")

	logger := &Logger{}
	logger.SetLogger(types.NewSlogLogger(slog.New(handler)))

	logger.Debug(ctx, "request sent")
	logger.Info(ctx, "message sent", "count", 2)

	_, file, line, _ := runtime.Caller(0)

	logger.Warn(ctx, "response has no timestamp")

	require.Len(t, handler.records, 2)
	assert.Equal(t, slog.LevelInfo, handler.records[0].Level)
	assert.Equal(t, "message sent", handler.records[0].Message)
	assert.Equal(t, slog.LevelWarn, handler.records[1].Level)
	assert.Equal(t, "response has no timestamp", handler.records[1].Message)
	assert.Equal(t, []context.Context{ctx, ctx}, handler.contexts)

	var attrs []slog.Attr

	handler.records[0].Attrs(func(attr slog.Attr) bool {
		attrs = append(attrs, attr)

		return true
	})
	assert.Equal(t, []slog.Attr{slog.Int("count", 2)}, attrs)

	source := handler.records[1].Source()
	require.NotNil(t, source)
	assert.Equal(t, filepath.Base(file), filepath.Base(source.File))
	assert.Equal(t, line+2, source.Line)
}

// TestLoggerLeveledWithoutLogger verifies that Debug, Info, and Warn discard
// their records on a zero-value Logger and after SetLogger(nil). A nil context
// must not panic either.
func TestLoggerLeveledWithoutLogger(t *testing.T) {
	t.Parallel()

	unset := &Logger{}
	discarding := &Logger{}
	discarding.SetLogger(nil)

	for _, logger := range []*Logger{unset, discarding} {
		assert.NotPanics(t, func() {
			logger.Debug(t.Context(), "discarded")
			logger.Info(t.Context(), "discarded")
			logger.Warn(nil, "discarded") //nolint:staticcheck // A nil context must not panic.
		})
	}
}

// TestLoggerNilContext verifies that a record logged with a nil context
// reaches the handler with the background context.
func TestLoggerNilContext(t *testing.T) {
	t.Parallel()

	handler := &recordingHandler{level: slog.LevelDebug}

	logger := &Logger{}
	logger.SetLogger(types.NewSlogLogger(slog.New(handler)))
	logger.Warn(nil, "sending without TLS") //nolint:staticcheck // Checks that a nil context is replaced.

	assert.Equal(t, []context.Context{context.Background()}, handler.contexts)
}

// TestLoggerSlog verifies that Slog returns the slog logger that receives the
// leveled records, and a logger that discards every record before SetLogger.
func TestLoggerSlog(t *testing.T) {
	t.Parallel()

	assert.False(t, (&Logger{}).Slog().Enabled(t.Context(), slog.LevelError))

	slogLogger := slog.New(&recordingHandler{level: slog.LevelDebug})

	logger := &Logger{}
	logger.SetLogger(types.NewSlogLogger(slogLogger))

	assert.Same(t, slogLogger, logger.Slog())
}
