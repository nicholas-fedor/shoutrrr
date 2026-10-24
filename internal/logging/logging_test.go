package logging

import (
	"fmt"
	"sync"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/nicholas-fedor/shoutrrr/pkg/format"
)

// recordingLogger records what is logged through it. It is safe for
// concurrent use.
type recordingLogger struct {
	// mu guards lines.
	mu sync.Mutex
	// lines holds each logged message.
	lines []string
}

// Log records v as one message.
//
// Parameters:
//   - v: the values to log.
func (l *recordingLogger) Log(v ...any) {
	l.mu.Lock()
	defer l.mu.Unlock()

	l.lines = append(l.lines, fmt.Sprint(v...))
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

// TestOnceDeprecations verifies the warning text for deprecated keys and
// fields, and that each is logged once even when a key and a field share a
// name.
func TestOnceDeprecations(t *testing.T) {
	t.Parallel()

	var once Once

	logger := &recordingLogger{}
	keys := []format.Deprecation{{Name: "insecureskipverify", Replacement: "skiptlsverify"}}
	fields := []format.Deprecation{{Name: "insecureskipverify", Replacement: "SkipTLSVerify"}}

	once.DeprecatedKeys(logger, keys)
	once.DeprecatedKeys(logger, keys)
	once.DeprecatedFields(logger, fields)
	once.DeprecatedFields(logger, fields)

	assert.Equal(t, []string{
		`Warning: key "insecureskipverify" is deprecated, use "skiptlsverify" instead`,
		"Warning: config field insecureskipverify is deprecated, use SkipTLSVerify instead",
	}, logger.lines)
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
		wg.Go(func() { once.Do("insecure", func() { logger.Log("warning") }) })
	}

	wg.Wait()

	assert.Equal(t, []string{"warning"}, logger.lines)
}

// TestOnceConnectionWarnings verifies the text of the insecure and plaintext
// warnings, and that each is logged once.
func TestOnceConnectionWarnings(t *testing.T) {
	t.Parallel()

	var once Once

	logger := &recordingLogger{}

	once.Insecure(logger)
	once.Insecure(logger)
	once.Plaintext(logger)
	once.Plaintext(logger)

	assert.Equal(t, []string{insecureWarning, plaintextWarning}, logger.lines)
	assert.Equal(t, "Warning: skipping TLS certificate verification", insecureWarning)
	assert.Equal(t, "Warning: sending without TLS", plaintextWarning)
}
