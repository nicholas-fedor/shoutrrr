package zulip

import (
	"testing"

	"github.com/onsi/gomega"

	ginkgo "github.com/onsi/ginkgo/v2"
)

// TestZulip runs the Zulip test suite.
func TestZulip(t *testing.T) {
	t.Parallel()
	gomega.RegisterFailHandler(ginkgo.Fail)
	ginkgo.RunSpecs(t, "Zulip Suite")
}
