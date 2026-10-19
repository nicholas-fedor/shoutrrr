package logger

import (
	"log"

	"github.com/onsi/ginkgo/v2"
)

// New returns a logger that writes to [ginkgo.GinkgoWriter] with a "[Test] "
// prefix, so service logs appear with the output of the spec that made them.
//
// Returns:
//   - *log.Logger: the logger.
func New() *log.Logger {
	return log.New(ginkgo.GinkgoWriter, "[Test] ", 0)
}
