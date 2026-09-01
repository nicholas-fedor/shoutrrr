package smtp

import (
	ginkgo "github.com/onsi/ginkgo/v2"
	gomega "github.com/onsi/gomega"
)

var _ = ginkgo.Describe("encMethod", func() {
	ginkgo.Describe("String", func() {
		ginkgo.It("should print the canonical name for each method", func() {
			gomega.Expect(EncMethods.None.String()).To(gomega.Equal("None"))
			gomega.Expect(EncMethods.ExplicitTLS.String()).To(gomega.Equal("ExplicitTLS"))
			gomega.Expect(EncMethods.ImplicitTLS.String()).To(gomega.Equal("ImplicitTLS"))
			gomega.Expect(EncMethods.Auto.String()).To(gomega.Equal("Auto"))
		})
	})

	ginkgo.Describe("useImplicitTLS", func() {
		ginkgo.It("should never use implicit TLS for None or ExplicitTLS", func() {
			gomega.Expect(useImplicitTLS(EncMethods.None, ImplicitTLSPort)).To(gomega.BeFalse())
			gomega.Expect(useImplicitTLS(EncMethods.None, 587)).To(gomega.BeFalse())
			gomega.Expect(useImplicitTLS(EncMethods.ExplicitTLS, ImplicitTLSPort)).To(gomega.BeFalse())
			gomega.Expect(useImplicitTLS(EncMethods.ExplicitTLS, 587)).To(gomega.BeFalse())
		})

		ginkgo.It("should always use implicit TLS for ImplicitTLS", func() {
			gomega.Expect(useImplicitTLS(EncMethods.ImplicitTLS, ImplicitTLSPort)).To(gomega.BeTrue())
			gomega.Expect(useImplicitTLS(EncMethods.ImplicitTLS, 25)).To(gomega.BeTrue())
		})

		ginkgo.It("should use implicit TLS for Auto only on port 465", func() {
			gomega.Expect(useImplicitTLS(EncMethods.Auto, ImplicitTLSPort)).To(gomega.BeTrue())
			gomega.Expect(useImplicitTLS(EncMethods.Auto, 587)).To(gomega.BeFalse())
			gomega.Expect(useImplicitTLS(EncMethods.Auto, 25)).To(gomega.BeFalse())
		})

		ginkgo.It("should return false for an unknown encryption value", func() {
			gomega.Expect(useImplicitTLS(encMethod(99), ImplicitTLSPort)).To(gomega.BeFalse())
		})
	})
})
