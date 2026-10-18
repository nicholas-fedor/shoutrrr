package format

import (
	"testing"

	"github.com/onsi/ginkgo/v2"
	"github.com/onsi/gomega"
)

// TestFormat runs the Ginkgo specs of the renderers and URL parts with
// NO_COLOR set, so the colors chosen while rendering stay off.
func TestFormat(t *testing.T) {
	t.Setenv("NO_COLOR", "true")
	gomega.RegisterFailHandler(ginkgo.Fail)
	ginkgo.RunSpecs(t, "Shoutrrr Format Suite")
}
