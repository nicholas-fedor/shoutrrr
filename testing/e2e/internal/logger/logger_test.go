package logger

import (
	"testing"

	"github.com/onsi/ginkgo/v2"
	"github.com/stretchr/testify/assert"
)

// TestNew verifies that the logger writes to Ginkgo's writer with the
// "[Test] " prefix and no timestamps.
func TestNew(t *testing.T) {
	t.Parallel()

	logger := New()

	assert.Same(t, ginkgo.GinkgoWriter, logger.Writer())
	assert.Equal(t, "[Test] ", logger.Prefix())
	assert.Zero(t, logger.Flags())
}
