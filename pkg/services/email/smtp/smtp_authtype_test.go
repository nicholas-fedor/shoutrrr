package smtp

import (
	ginkgo "github.com/onsi/ginkgo/v2"
	gomega "github.com/onsi/gomega"
)

var _ = ginkgo.Describe("authType", func() {
	ginkgo.Describe("String", func() {
		ginkgo.It("should print the canonical name for each method", func() {
			gomega.Expect(AuthTypes.None.String()).To(gomega.Equal("None"))
			gomega.Expect(AuthTypes.Plain.String()).To(gomega.Equal("Plain"))
			gomega.Expect(AuthTypes.CRAMMD5.String()).To(gomega.Equal("CRAMMD5"))
			gomega.Expect(AuthTypes.Unknown.String()).To(gomega.Equal("Unknown"))
			gomega.Expect(AuthTypes.OAuth2.String()).To(gomega.Equal("OAuth2"))
			gomega.Expect(AuthTypes.Login.String()).To(gomega.Equal("Login"))
		})
	})
})
