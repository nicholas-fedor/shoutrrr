package smtp

import (
	"testing"

	ginkgo "github.com/onsi/ginkgo/v2"
	gomega "github.com/onsi/gomega"
)

func TestSMTP(t *testing.T) {
	t.Parallel()
	gomega.RegisterFailHandler(ginkgo.Fail)
	ginkgo.RunSpecs(t, "SMTP Suite")
}
