package standard

import (
	"log"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/nicholas-fedor/shoutrrr/pkg/util"
)

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

// TestLoggerSetLoggerNil verifies that a nil logger discards messages rather
// than leaving Log and Logf without a logger.
func TestLoggerSetLoggerNil(t *testing.T) {
	t.Parallel()

	logger := &Logger{}
	logger.SetLogger(nil)

	assert.Same(t, util.DiscardLogger, logger.logger)
	assert.NotPanics(t, func() {
		logger.Log("discarded")
		logger.Logf("discarded %d", 1)
	})
}
