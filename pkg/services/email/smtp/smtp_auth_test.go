package smtp

import (
	"net/smtp"

	ginkgo "github.com/onsi/ginkgo/v2"
	gomega "github.com/onsi/gomega"
)

var _ = ginkgo.Describe("newAuth", func() {
	ginkgo.It("should return a nil mechanism when Auth is None", func() {
		got, err := newAuth(&Config{Auth: AuthTypes.None})
		gomega.Expect(err).NotTo(gomega.HaveOccurred())
		gomega.Expect(got).To(gomega.BeNil())
	})

	ginkgo.It("should return PlainAuth when Auth is Plain", func() {
		got, err := newAuth(&Config{
			Auth:     AuthTypes.Plain,
			Username: "user",
			Password: "pass",
			Host:     "mail.example.com",
		})
		gomega.Expect(err).NotTo(gomega.HaveOccurred())
		gomega.Expect(got).NotTo(gomega.BeNil())

		want := smtp.PlainAuth("", "user", "pass", "mail.example.com")
		gomega.Expect(got).To(gomega.BeAssignableToTypeOf(want))
	})

	ginkgo.It("should return CRAMMD5Auth when Auth is CRAMMD5", func() {
		got, err := newAuth(&Config{
			Auth:     AuthTypes.CRAMMD5,
			Username: "user",
			Password: "pass",
		})
		gomega.Expect(err).NotTo(gomega.HaveOccurred())
		gomega.Expect(got).NotTo(gomega.BeNil())
		gomega.Expect(got).To(gomega.BeAssignableToTypeOf(smtp.CRAMMD5Auth("user", "pass")))
	})

	ginkgo.It("should return oauth2Auth when Auth is OAuth2", func() {
		got, err := newAuth(&Config{
			Auth:     AuthTypes.OAuth2,
			Username: "user",
			Password: "token",
		})
		gomega.Expect(err).NotTo(gomega.HaveOccurred())

		_, ok := got.(*oauth2Auth)
		gomega.Expect(ok).To(gomega.BeTrue())
	})

	ginkgo.It("should return loginAuth when Auth is Login", func() {
		got, err := newAuth(&Config{
			Auth:     AuthTypes.Login,
			Username: "user",
			Password: "pass",
			Host:     "mail.example.com",
		})
		gomega.Expect(err).NotTo(gomega.HaveOccurred())

		_, ok := got.(*loginAuth)
		gomega.Expect(ok).To(gomega.BeTrue())
	})

	ginkgo.It("should fail when Auth is Unknown", func() {
		got, err := newAuth(&Config{Auth: AuthTypes.Unknown})
		gomega.Expect(got).To(gomega.BeNil())
		gomega.Expect(err).To(matchFailure(FailAuthType))
		gomega.Expect(err.Error()).To(gomega.ContainSubstring("Unknown"))
	})

	ginkgo.It("should fail when Auth is not a known method", func() {
		got, err := newAuth(&Config{Auth: authType(99)})
		gomega.Expect(got).To(gomega.BeNil())
		gomega.Expect(err).To(matchFailure(FailAuthType))
	})
})
