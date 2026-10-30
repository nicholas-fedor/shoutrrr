package logging

import (
	"context"
	"log/slog"
	"sync"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/nicholas-fedor/shoutrrr/pkg/format"
)

// recordingLogger records the warnings logged through it. It is its own slog
// handler, and is safe for concurrent use.
type recordingLogger struct {
	// warnings holds each logged warning.
	warnings []warning
	// mu guards warnings.
	mu sync.Mutex
}

// warning is one warning a [recordingLogger] recorded.
type warning struct {
	// ctx is the context the warning was logged with.
	ctx context.Context //nolint:containedctx // Recorded to check that warnings pass it on.
	// msg is the message.
	msg string
	// source is the function the record names as its source.
	source string
	// attrs holds the attributes as key=value text.
	attrs []string
	// level is the level of the record.
	level slog.Level
}

// contextKey identifies the test value in a context.
type contextKey struct{}

// Enabled reports true for every level.
//
// Returns:
//   - bool: always true.
func (*recordingLogger) Enabled(context.Context, slog.Level) bool { return true }

// Handle records the warning.
//
// Parameters:
//   - ctx: the context of the call.
//   - record: the record.
//
// Returns:
//   - error: always nil.
//
//nolint:gocritic // slog.Handler defines Handle with a Record value.
func (l *recordingLogger) Handle(ctx context.Context, record slog.Record) error {
	var attrs []string

	record.Attrs(func(attr slog.Attr) bool {
		attrs = append(attrs, attr.String())

		return true
	})

	var source string
	if frame := record.Source(); frame != nil {
		source = frame.Function
	}

	l.mu.Lock()
	defer l.mu.Unlock()

	l.warnings = append(l.warnings, warning{
		ctx: ctx, msg: record.Message, source: source, attrs: attrs, level: record.Level,
	})

	return nil
}

// Slog returns a slog logger with l as its handler.
//
// Returns:
//   - *slog.Logger: the logger.
func (l *recordingLogger) Slog() *slog.Logger { return slog.New(l) }

// WithAttrs returns l. Once adds no attributes to the handler.
//
// Returns:
//   - slog.Handler: l.
func (l *recordingLogger) WithAttrs([]slog.Attr) slog.Handler { return l }

// WithGroup returns l. Once adds no groups to the handler.
//
// Returns:
//   - slog.Handler: l.
func (l *recordingLogger) WithGroup(string) slog.Handler { return l }

// messages returns the message of each recorded warning.
//
// Returns:
//   - []string: the messages, in the order they were logged.
func (l *recordingLogger) messages() []string {
	l.mu.Lock()
	defer l.mu.Unlock()

	messages := make([]string, 0, len(l.warnings))
	for _, warning := range l.warnings {
		messages = append(messages, warning.msg)
	}

	return messages
}

// TestOnceDo verifies that each key's warning runs once, that different keys
// run separately, and that Reset lets a warning run again.
func TestOnceDo(t *testing.T) {
	t.Parallel()

	var (
		once  Once
		calls []string
	)

	warn := func(key string) func() { return func() { calls = append(calls, key) } }

	once.Do("a", warn("a"))
	once.Do("a", warn("a"))
	once.Do("b", warn("b"))
	assert.Equal(t, []string{"a", "b"}, calls)

	once.Reset()
	once.Do("a", warn("a"))
	assert.Equal(t, []string{"a", "b", "a"}, calls)
}

// TestOnceReset verifies that Reset lets every kind of warning be logged
// again, including the connection and deprecation warnings.
func TestOnceReset(t *testing.T) {
	t.Parallel()

	var once Once

	logger := &recordingLogger{}
	deprecations := []format.Deprecation{{Name: "insecureskipverify", Replacement: "skiptlsverify"}}

	logAll := func() {
		once.Insecure(t.Context(), logger)
		once.Plaintext(t.Context(), logger)
		once.DeprecatedKeys(t.Context(), logger, deprecations)
		once.DeprecatedFields(t.Context(), logger, deprecations)
	}

	logAll()
	once.Reset()
	logAll()

	want := []string{
		"skipping TLS certificate verification", "sending without TLS",
		"config key is deprecated", "config field is deprecated",
	}
	assert.Equal(t, append(want, want...), logger.messages())
}

// TestOnceDeprecations verifies the message and attributes of the warnings for
// deprecated keys and fields. Each warning is logged at Warn with the caller's
// context. Its source is the caller's call to the Once method, also when an
// earlier deprecation in the list was logged before. Each is logged once, even
// when a key and a field share a name.
func TestOnceDeprecations(t *testing.T) {
	t.Parallel()

	var once Once

	ctx := context.WithValue(t.Context(), contextKey{}, "send")
	logger := &recordingLogger{}
	keys := []format.Deprecation{{Name: "insecureskipverify", Replacement: "skiptlsverify"}}
	fields := []format.Deprecation{{Name: "insecureskipverify", Replacement: "SkipTLSVerify"}}

	once.DeprecatedKeys(ctx, logger, keys)
	once.DeprecatedKeys(ctx, logger, keys)
	once.DeprecatedFields(ctx, logger, fields)
	once.DeprecatedFields(ctx, logger, fields)
	once.DeprecatedKeys(ctx, logger, []format.Deprecation{keys[0], {Name: "disabletlsverification", Replacement: "skiptlsverify"}})

	const source = "github.com/nicholas-fedor/shoutrrr/internal/logging.TestOnceDeprecations"

	assert.Equal(t, []warning{
		{
			ctx:    ctx,
			msg:    "config key is deprecated",
			source: source,
			attrs:  []string{"key=insecureskipverify", "replacement=skiptlsverify"},
			level:  slog.LevelWarn,
		},
		{
			ctx:    ctx,
			msg:    "config field is deprecated",
			source: source,
			attrs:  []string{"field=insecureskipverify", "replacement=SkipTLSVerify"},
			level:  slog.LevelWarn,
		},
		{
			ctx:    ctx,
			msg:    "config key is deprecated",
			source: source,
			attrs:  []string{"key=disabletlsverification", "replacement=skiptlsverify"},
			level:  slog.LevelWarn,
		},
	}, logger.warnings)
}

// TestOnceConcurrent verifies that concurrent sends log a warning once. Run it
// with -race.
func TestOnceConcurrent(t *testing.T) {
	t.Parallel()

	var (
		once Once
		wg   sync.WaitGroup
	)

	logger := &recordingLogger{}

	for range 8 {
		wg.Go(func() { once.Insecure(t.Context(), logger) })
	}

	wg.Wait()

	assert.Equal(t, []string{"skipping TLS certificate verification"}, logger.messages())
}

// TestOnceConnectionWarnings verifies the messages of the insecure and
// plaintext warnings. Each warning is logged at Warn with the caller's context
// and the caller's call as its source. Each is logged once.
func TestOnceConnectionWarnings(t *testing.T) {
	t.Parallel()

	var once Once

	ctx := context.WithValue(t.Context(), contextKey{}, "send")
	logger := &recordingLogger{}

	once.Insecure(ctx, logger)
	once.Insecure(ctx, logger)
	once.Plaintext(ctx, logger)
	once.Plaintext(ctx, logger)

	const source = "github.com/nicholas-fedor/shoutrrr/internal/logging.TestOnceConnectionWarnings"

	assert.Equal(t, []warning{
		{ctx: ctx, msg: "skipping TLS certificate verification", source: source, attrs: nil, level: slog.LevelWarn},
		{ctx: ctx, msg: "sending without TLS", source: source, attrs: nil, level: slog.LevelWarn},
	}, logger.warnings)
}

// TestOnceLoggedSkipsWork verifies that a warning already logged returns
// without allocating, for the connection and deprecation warnings alike, so
// the sends that repeat it stay cheap.
//
//nolint:paralleltest // testing.AllocsPerRun panics in a parallel test.
func TestOnceLoggedSkipsWork(t *testing.T) {
	var once Once

	logger := &recordingLogger{}
	deprecations := []format.Deprecation{{Name: "insecureskipverify", Replacement: "skiptlsverify"}}

	once.Insecure(t.Context(), logger)
	once.Plaintext(t.Context(), logger)
	once.DeprecatedKeys(t.Context(), logger, deprecations)
	once.DeprecatedFields(t.Context(), logger, deprecations)

	assert.Zero(t, testing.AllocsPerRun(100, func() { once.Insecure(t.Context(), logger) }))
	assert.Zero(t, testing.AllocsPerRun(100, func() { once.Plaintext(t.Context(), logger) }))
	assert.Zero(t, testing.AllocsPerRun(100, func() { once.DeprecatedKeys(t.Context(), logger, deprecations) }))
	assert.Zero(t, testing.AllocsPerRun(100, func() { once.DeprecatedFields(t.Context(), logger, deprecations) }))
}
