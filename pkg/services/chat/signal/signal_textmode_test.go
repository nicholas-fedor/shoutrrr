package signal

import (
	"github.com/onsi/ginkgo/v2"
	"github.com/onsi/gomega"
)

var _ = ginkgo.Describe("text mode", func() {
	ginkgo.It("should omit JSON when None", func() {
		gomega.Expect(TextModeNone.payloadValue()).To(gomega.BeEmpty())
		gomega.Expect(TextModeNone.String()).To(gomega.Equal("None"))
	})

	ginkgo.It("should map Normal to the REST API value", func() {
		gomega.Expect(TextModeNormal.payloadValue()).To(gomega.Equal("normal"))
		gomega.Expect(TextModeNormal.String()).To(gomega.Equal("Normal"))
	})

	ginkgo.It("should map Styled to the REST API value", func() {
		gomega.Expect(TextModeStyled.payloadValue()).To(gomega.Equal("styled"))
		gomega.Expect(TextModeStyled.String()).To(gomega.Equal("Styled"))
	})
})
