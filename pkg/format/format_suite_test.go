package format

import (
	"os"
	"testing"

	"github.com/onsi/ginkgo/v2"
	"github.com/onsi/gomega"
)

// The renderer and URL part specs disable color so their expected output
// carries no escape codes.
var _ = ginkgo.BeforeSuite(func() {
	_ = os.Setenv("NO_COLOR", "true")
})

// TestFormat runs the Ginkgo specs of the renderers and URL parts.
func TestFormat(t *testing.T) {
	t.Parallel()
	gomega.RegisterFailHandler(ginkgo.Fail)
	ginkgo.RunSpecs(t, "Shoutrrr Format Suite")
}
