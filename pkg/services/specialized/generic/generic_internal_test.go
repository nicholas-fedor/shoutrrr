package generic

import (
	"github.com/onsi/ginkgo/v2"
	"github.com/onsi/gomega"
)

var _ = ginkgo.Describe("the default HTTP client", func() {
	ginkgo.It("should bound requests with the default timeout", func() {
		gomega.Expect(newDefaultHTTPClient().Timeout).To(gomega.Equal(defaultHTTPTimeout))
	})
})
