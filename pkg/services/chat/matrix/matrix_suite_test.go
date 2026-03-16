package matrix

import (
	"testing"

	"github.com/onsi/gomega"

	ginkgo "github.com/onsi/ginkgo/v2"
)

// TestMatrix runs the Matrix test suite.
func TestMatrix(t *testing.T) {
	t.Parallel()
	gomega.RegisterFailHandler(ginkgo.Fail)
	ginkgo.RunSpecs(t, "Matrix Test Suite")
}
