package smtp

import (
	"net/smtp"

	ginkgo "github.com/onsi/ginkgo/v2"
	gomega "github.com/onsi/gomega"
)

var _ = ginkgo.Describe("loginAuth", func() {
	ginkgo.Describe("newLoginAuth", func() {
		ginkgo.It("should store credentials and reset the response step", func() {
			got := newLoginAuth("alice", "s3cret", "mail.example.com")
			auth, ok := got.(*loginAuth)
			gomega.Expect(ok).To(gomega.BeTrue())
			gomega.Expect(auth.username).To(gomega.Equal("alice"))
			gomega.Expect(auth.password).To(gomega.Equal("s3cret"))
			gomega.Expect(auth.host).To(gomega.Equal("mail.example.com"))
			gomega.Expect(auth.respStep).To(gomega.BeZero())
		})
	})

	ginkgo.Describe("Start", func() {
		ginkgo.It("should accept TLS and a matching host", func() {
			auth := newLoginAuth("user", "pass", "mail.example.com")
			mech, resp, err := auth.Start(&smtp.ServerInfo{Name: "mail.example.com", TLS: true})
			gomega.Expect(err).NotTo(gomega.HaveOccurred())
			gomega.Expect(mech).To(gomega.Equal("LOGIN"))
			gomega.Expect(resp).To(gomega.BeNil())
		})

		ginkgo.It("should accept localhost without TLS", func() {
			auth := newLoginAuth("user", "pass", "localhost")
			_, _, err := auth.Start(&smtp.ServerInfo{Name: "localhost", TLS: false})
			gomega.Expect(err).NotTo(gomega.HaveOccurred())
		})

		ginkgo.It("should accept 127.0.0.1 without TLS", func() {
			auth := newLoginAuth("user", "pass", "127.0.0.1")
			_, _, err := auth.Start(&smtp.ServerInfo{Name: "127.0.0.1", TLS: false})
			gomega.Expect(err).NotTo(gomega.HaveOccurred())
		})

		ginkgo.It("should accept ::1 without TLS", func() {
			auth := newLoginAuth("user", "pass", "::1")
			_, _, err := auth.Start(&smtp.ServerInfo{Name: "::1", TLS: false})
			gomega.Expect(err).NotTo(gomega.HaveOccurred())
		})

		ginkgo.It("should reject an unencrypted remote server", func() {
			auth := newLoginAuth("user", "pass", "mail.example.com")
			_, _, err := auth.Start(&smtp.ServerInfo{Name: "mail.example.com", TLS: false})
			gomega.Expect(err).To(gomega.MatchError(errUnencryptedConnection))
		})

		ginkgo.It("should reject a mismatched host", func() {
			auth := newLoginAuth("user", "pass", "mail.example.com")
			_, _, err := auth.Start(&smtp.ServerInfo{Name: "evil.example.com", TLS: true})
			gomega.Expect(err).To(gomega.MatchError(errWrongHostName))
		})

		ginkgo.It("should reset respStep so a later Next starts at the username", func() {
			auth := newLoginAuth("u", "p", "localhost").(*loginAuth)
			_, err := auth.Next(nil, true)
			gomega.Expect(err).NotTo(gomega.HaveOccurred())

			_, _, err = auth.Start(&smtp.ServerInfo{Name: "localhost", TLS: false})
			gomega.Expect(err).NotTo(gomega.HaveOccurred())
			gomega.Expect(auth.respStep).To(gomega.BeZero())

			resp, err := auth.Next([]byte("Username:"), true)
			gomega.Expect(err).NotTo(gomega.HaveOccurred())
			gomega.Expect(string(resp)).To(gomega.Equal("u"))
		})
	})

	ginkgo.Describe("Next", func() {
		ginkgo.It("should send the username then the password and reject extra challenges", func() {
			auth := newLoginAuth("alice", "s3cret", "mail.example.com").(*loginAuth)

			resp, err := auth.Next([]byte("User Name\x00"), true)
			gomega.Expect(err).NotTo(gomega.HaveOccurred())
			gomega.Expect(string(resp)).To(gomega.Equal("alice"))

			resp, err = auth.Next([]byte("password:"), true)
			gomega.Expect(err).NotTo(gomega.HaveOccurred())
			gomega.Expect(string(resp)).To(gomega.Equal("s3cret"))

			resp, err = auth.Next([]byte("Username:"), true)
			gomega.Expect(err).To(gomega.MatchError(errUnexpectedServerChallenge))
			gomega.Expect(resp).To(gomega.BeNil())
			gomega.Expect(err.Error()).To(gomega.ContainSubstring("Username:"))

			resp, err = auth.Next(nil, false)
			gomega.Expect(err).NotTo(gomega.HaveOccurred())
			gomega.Expect(resp).To(gomega.BeNil())
		})

		ginkgo.It("should send passwords that contain spaces and unicode", func() {
			auth := newLoginAuth("alice", "päss word", "mail.example.com").(*loginAuth)

			resp, err := auth.Next(nil, true)
			gomega.Expect(err).NotTo(gomega.HaveOccurred())
			gomega.Expect(string(resp)).To(gomega.Equal("alice"))

			resp, err = auth.Next(nil, true)
			gomega.Expect(err).NotTo(gomega.HaveOccurred())
			gomega.Expect(string(resp)).To(gomega.Equal("päss word"))
		})
	})

	ginkgo.Describe("isLocalhost", func() {
		ginkgo.It("should recognize loopback identities", func() {
			gomega.Expect(isLocalhost("localhost")).To(gomega.BeTrue())
			gomega.Expect(isLocalhost("127.0.0.1")).To(gomega.BeTrue())
			gomega.Expect(isLocalhost("::1")).To(gomega.BeTrue())
		})

		ginkgo.It("should reject a remote hostname", func() {
			gomega.Expect(isLocalhost("example.com")).To(gomega.BeFalse())
			gomega.Expect(isLocalhost("")).To(gomega.BeFalse())
		})
	})
})
