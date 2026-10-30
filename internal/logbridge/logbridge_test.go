package logbridge

import (
	"bytes"
	"context"
	"log"
	"log/slog"
	"runtime"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/nicholas-fedor/shoutrrr/pkg/types"
)

// TestNew verifies which slog logger each kind of library logger turns into.
func TestNew(t *testing.T) {
	t.Parallel()

	t.Run("nil discards", func(t *testing.T) {
		t.Parallel()

		assert.False(t, New(nil).Enabled(t.Context(), slog.LevelError))
	})

	t.Run("slog logger as given", func(t *testing.T) {
		t.Parallel()

		logger := slog.New(slog.DiscardHandler)
		assert.Same(t, logger, New(types.NewSlogLogger(logger)))
	})

	t.Run("wrapped logger keeps its attributes", func(t *testing.T) {
		t.Parallel()

		var out bytes.Buffer

		New(With(log.New(&out, "", 0), "service", "ntfy")).Warn("sending without TLS")
		assert.Equal(t, "WARN sending without TLS service=ntfy\n", out.String())
	})

	t.Run("plain logger gets lines", func(t *testing.T) {
		t.Parallel()

		var out bytes.Buffer

		New(log.New(&out, "", 0)).Info("sent")
		assert.Equal(t, "INFO sent\n", out.String())
	})
}

// TestTextHandler verifies the line a plain logger receives for each record:
// the level, the message, then the attributes quoted and grouped as slog's
// text handler writes them.
func TestTextHandler(t *testing.T) {
	t.Parallel()

	// tests lists log calls with the line each should write.
	tests := []struct {
		log  func(logger *slog.Logger)
		name string
		want string
	}{
		{
			name: "debug",
			log:  func(logger *slog.Logger) { logger.Debug("sent") },
			want: "DEBUG sent",
		},
		{
			name: "info",
			log:  func(logger *slog.Logger) { logger.Info("sent") },
			want: "INFO sent",
		},
		{
			name: "warn with attributes",
			log: func(logger *slog.Logger) {
				logger.Warn("config key is deprecated", "key", "scheme", "replacement", "disabletls")
			},
			want: "WARN config key is deprecated key=scheme replacement=disabletls",
		},
		{
			name: "error",
			log:  func(logger *slog.Logger) { logger.Error("failed") },
			want: "ERROR failed",
		},
		{
			name: "quoted value",
			log:  func(logger *slog.Logger) { logger.Info("failed", "error", `bad "x" y`) },
			want: `INFO failed error="bad \"x\" y"`,
		},
		{
			name: "with attributes",
			log:  func(logger *slog.Logger) { logger.With("service", "mqtt").Warn("port", "port", 1883) },
			want: "WARN port service=mqtt port=1883",
		},
		{
			name: "with group",
			log:  func(logger *slog.Logger) { logger.WithGroup("tls").Info("connected", "verify", false) },
			want: "INFO connected tls.verify=false",
		},
		{
			name: "group attribute",
			log:  func(logger *slog.Logger) { logger.Info("failed", slog.Group("response", "status", 500)) },
			want: "INFO failed response.status=500",
		},
		{
			name: "empty group name",
			log:  func(logger *slog.Logger) { logger.WithGroup("").Info("sent", "count", 1) },
			want: "INFO sent count=1",
		},
		{
			name: "group without attributes",
			log:  func(logger *slog.Logger) { logger.WithGroup("tls").Info("sent") },
			want: "INFO sent",
		},
		{
			name: "empty attributes",
			log:  func(logger *slog.Logger) { logger.With().Info("sent") },
			want: "INFO sent",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			var out bytes.Buffer

			tt.log(New(log.New(&out, "", 0)))
			assert.Equal(t, tt.want+"\n", out.String())
		})
	}
}

// TestTextHandlerBuiltinKeys verifies that attributes keyed time, level, or
// msg reach a plain logger, while the record's own time, level, and message
// are not repeated among the attributes.
func TestTextHandlerBuiltinKeys(t *testing.T) {
	t.Parallel()

	var out bytes.Buffer

	at := time.Date(2026, 10, 28, 12, 0, 0, 0, time.UTC)

	New(log.New(&out, "", 0)).Warn("retry scheduled",
		"time", at, "level", "high", "msg", "other", "msg", "retry scheduled")

	assert.Equal(t,
		`WARN retry scheduled time=2026-10-28T12:00:00.000Z level=high msg=other msg="retry scheduled"`+"\n",
		out.String())
}

// TestBuiltinAttrDropper verifies that the drop function removes only the
// first top-level level and msg attributes that hold the record's own level
// and message, whichever order the attributes come in.
func TestBuiltinAttrDropper(t *testing.T) {
	t.Parallel()

	drop := builtinAttrDropper(slog.LevelWarn, "sending without TLS")

	// calls lists attributes in the order they reach the function, with
	// whether each should be kept.
	calls := []struct {
		attr   slog.Attr
		groups []string
		keep   bool
	}{
		{attr: slog.String("msg", "other"), keep: true},
		{attr: slog.String("level", "high"), keep: true},
		{attr: slog.Any("level", slog.LevelWarn), groups: []string{"tls"}, keep: true},
		{attr: slog.String("msg", "sending without TLS"), keep: false},
		{attr: slog.Any("level", slog.LevelWarn), keep: false},
		{attr: slog.String("msg", "sending without TLS"), keep: true},
		{attr: slog.Any("level", slog.LevelWarn), keep: true},
	}

	for i, call := range calls {
		got := drop(call.groups, call.attr)
		if call.keep {
			assert.True(t, got.Equal(call.attr), "call %d: want %v kept, got %v", i, call.attr, got)
		} else {
			assert.True(t, got.Equal(slog.Attr{}), "call %d: want %v dropped, got %v", i, call.attr, got)
		}
	}
}

// TestLog verifies that Log gives the record the caller's location as its
// source, passes the caller's context, and replaces a nil context with the
// background context. A disabled level logs nothing.
func TestLog(t *testing.T) {
	t.Parallel()

	var out bytes.Buffer

	logger := slog.New(slog.NewTextHandler(&out, &slog.HandlerOptions{
		AddSource: true,
		Level:     slog.LevelInfo,
		ReplaceAttr: func(groups []string, attr slog.Attr) slog.Attr {
			if len(groups) == 0 && attr.Key == slog.TimeKey {
				return slog.Attr{}
			}

			return attr
		},
	}))

	Log(t.Context(), logger, 0, slog.LevelDebug, "discarded")
	assert.Empty(t, out.String())

	_, file, line, _ := runtime.Caller(0)

	Log(nil, logger, 0, slog.LevelWarn, "sending without TLS", "service", "ntfy") //nolint:staticcheck // Checks that a nil context is replaced.

	assert.Equal(t,
		"level=WARN source="+file+":"+strconv.Itoa(line+2)+` msg="sending without TLS" service=ntfy`+"\n",
		out.String())
}

// TestLogSkip verifies that Log names as its source the call the given number
// of calls above the function that calls Log, as a function that logs for its
// caller needs.
func TestLogSkip(t *testing.T) {
	t.Parallel()

	var out bytes.Buffer

	logger := slog.New(slog.NewTextHandler(&out, &slog.HandlerOptions{
		AddSource: true,
		ReplaceAttr: func(groups []string, attr slog.Attr) slog.Attr {
			if len(groups) == 0 && attr.Key == slog.TimeKey {
				return slog.Attr{}
			}

			return attr
		},
	}))

	_, file, line, _ := runtime.Caller(0)

	logForCaller(logger)

	assert.Equal(t, "level=WARN source="+file+":"+strconv.Itoa(line+2)+" msg=insecure\n", out.String())
}

// TestTextHandlerConcurrent verifies that loggers derived from one handler
// can log at the same time, each record as one whole line. Run it with -race.
func TestTextHandlerConcurrent(t *testing.T) {
	t.Parallel()

	var (
		out bytes.Buffer
		wg  sync.WaitGroup
	)

	logger := New(log.New(&out, "", 0))

	for range 8 {
		wg.Go(func() { logger.With("service", "ntfy").WithGroup("tls").Warn("insecure", "verify", false) })
	}

	wg.Wait()

	lines := strings.Split(strings.TrimSuffix(out.String(), "\n"), "\n")
	require.Len(t, lines, 8)

	for _, line := range lines {
		assert.Equal(t, "WARN insecure service=ntfy tls.verify=false", line)
	}
}

// TestTextHandlerEmptyScopes verifies that adding no attributes, or a group
// without a name, returns the same handler. slog.Logger skips the handler in
// both cases, so the handler is called directly.
func TestTextHandlerEmptyScopes(t *testing.T) {
	t.Parallel()

	handler := New(log.New(&bytes.Buffer{}, "", 0)).Handler()

	assert.Same(t, handler, handler.WithAttrs(nil))
	assert.Same(t, handler, handler.WithGroup(""))
}

// TestTextHandlerSiblings verifies that loggers derived from the same logger
// keep their own attributes, even when the attributes they share leave spare
// room behind them.
func TestTextHandlerSiblings(t *testing.T) {
	t.Parallel()

	var out bytes.Buffer

	parent := New(log.New(&out, "", 0)).With("a", 1).With("b", 2).With("c", 3)
	first := parent.With("d", "first")
	second := parent.With("d", "second")

	first.Info("sent")
	second.Info("sent")

	assert.Equal(t, "INFO sent a=1 b=2 c=3 d=first\nINFO sent a=1 b=2 c=3 d=second\n", out.String())
}

// TestWith verifies that a wrapped logger passes printed text to the logger it
// wraps unchanged, and adds its attributes only to leveled records.
func TestWith(t *testing.T) {
	t.Parallel()

	t.Run("plain logger", func(t *testing.T) {
		t.Parallel()

		var out bytes.Buffer

		logger := With(log.New(&out, "", 0), "service", "logger")
		logger.Print("raw ", "text")
		logger.Printf("raw %d", 1)
		logger.Println("raw", 2)
		New(logger).Warn("sending without TLS")

		assert.Equal(t, "raw text\nraw 1\nraw 2\nWARN sending without TLS service=logger\n", out.String())
	})

	t.Run("nested", func(t *testing.T) {
		t.Parallel()

		var out bytes.Buffer

		logger := With(With(log.New(&out, "", 0), "a", 1), "b", 2)
		logger.Print("raw")
		New(logger).Info("sent")

		assert.Equal(t, "raw\nINFO sent a=1 b=2\n", out.String())
	})

	t.Run("slog logger", func(t *testing.T) {
		t.Parallel()

		var out bytes.Buffer

		logger := With(types.NewSlogLogger(newTextLogger(&out)), "service", "logger")
		logger.Print("raw ", "text")
		logger.Printf("raw %d", 1)
		logger.Println("raw", 2)
		New(logger).Warn("sending without TLS")

		assert.Equal(t,
			`level=INFO msg="raw text" service=logger`+"\n"+
				`level=INFO msg="raw 1" service=logger`+"\n"+
				`level=INFO msg="raw 2" service=logger`+"\n"+
				`level=WARN msg="sending without TLS" service=logger`+"\n",
			out.String())
	})

	t.Run("nil", func(t *testing.T) {
		t.Parallel()

		assert.Nil(t, With(nil, "service", "ntfy"))
	})
}

// logForCaller logs a warning to logger whose source is the call to
// logForCaller, as a function that logs for its caller does.
//
// Parameters:
//   - logger: the logger that receives the warning.
func logForCaller(logger *slog.Logger) {
	Log(context.Background(), logger, 1, slog.LevelWarn, "insecure")
}

// newTextLogger returns a slog logger that writes records to out with slog's
// text handler, leaving out the time so the output can be compared.
//
// Parameters:
//   - out: the buffer that receives the records.
//
// Returns:
//   - *slog.Logger: the logger.
func newTextLogger(out *bytes.Buffer) *slog.Logger {
	return slog.New(slog.NewTextHandler(out, &slog.HandlerOptions{
		AddSource: false,
		Level:     slog.LevelDebug,
		ReplaceAttr: func(groups []string, attr slog.Attr) slog.Attr {
			if len(groups) == 0 && attr.Key == slog.TimeKey {
				return slog.Attr{}
			}

			return attr
		},
	}))
}
