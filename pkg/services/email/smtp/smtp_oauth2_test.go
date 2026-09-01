package smtp

import (
	"net/smtp"

	ginkgo "github.com/onsi/ginkgo/v2"
	gomega "github.com/onsi/gomega"
)

var _ = ginkgo.Describe("oauth2Auth", func() {
	ginkgo.Describe("newOAuth2Auth", func() {
		ginkgo.It("should store the username, access token, and host", func() {
			got := newOAuth2Auth("alice", "ya29.token", "smtp.gmail.com")
			auth, ok := got.(*oauth2Auth)
			gomega.Expect(ok).To(gomega.BeTrue())
			gomega.Expect(auth.username).To(gomega.Equal("alice"))
			gomega.Expect(auth.accessToken).To(gomega.Equal("ya29.token"))
			gomega.Expect(auth.host).To(gomega.Equal("smtp.gmail.com"))
		})
	})

	ginkgo.Describe("Start", func() {
		ginkgo.It("should send XOAUTH2 credentials as the initial response", func() {
			auth := newOAuth2Auth("alice", "ya29.token", "smtp.gmail.com")
			mech, resp, err := auth.Start(&smtp.ServerInfo{Name: "smtp.gmail.com", TLS: true})
			gomega.Expect(err).NotTo(gomega.HaveOccurred())
			gomega.Expect(mech).To(gomega.Equal("XOAUTH2"))
			gomega.Expect(string(resp)).To(gomega.Equal("user=alice\x01auth=Bearer ya29.token\x01\x01"))
		})

		ginkgo.It("should accept localhost without TLS", func() {
			auth := newOAuth2Auth("alice", "ya29.token", "localhost")
			_, _, err := auth.Start(&smtp.ServerInfo{Name: "localhost", TLS: false})
			gomega.Expect(err).NotTo(gomega.HaveOccurred())
		})

		ginkgo.It("should accept ::1 without TLS", func() {
			auth := newOAuth2Auth("alice", "ya29.token", "::1")
			_, _, err := auth.Start(&smtp.ServerInfo{Name: "::1", TLS: false})
			gomega.Expect(err).NotTo(gomega.HaveOccurred())
		})

		ginkgo.It("should reject an unencrypted remote server without leaking the token", func() {
			auth := newOAuth2Auth("alice", "ya29.token", "smtp.gmail.com")
			mech, resp, err := auth.Start(&smtp.ServerInfo{Name: "smtp.gmail.com", TLS: false})
			gomega.Expect(err).To(gomega.MatchError(errUnencryptedConnection))
			gomega.Expect(mech).To(gomega.BeEmpty())
			gomega.Expect(resp).To(gomega.BeNil())
			gomega.Expect(err.Error()).NotTo(gomega.ContainSubstring("ya29.token"))
		})

		ginkgo.It("should reject a mismatched host without leaking the token", func() {
			auth := newOAuth2Auth("alice", "ya29.token", "smtp.gmail.com")
			_, _, err := auth.Start(&smtp.ServerInfo{Name: "evil.example.com", TLS: true})
			gomega.Expect(err).To(gomega.MatchError(errWrongHostName))
			gomega.Expect(err.Error()).NotTo(gomega.ContainSubstring("ya29.token"))
		})
	})

	ginkgo.Describe("Next", func() {
		ginkgo.It("should send an empty continuation when the server issues a 334", func() {
			auth := newOAuth2Auth("alice", "ya29.token", "smtp.gmail.com").(*oauth2Auth)
			resp, err := auth.Next([]byte(`{"status":"400"}`), true)
			gomega.Expect(err).NotTo(gomega.HaveOccurred())
			gomega.Expect(resp).To(gomega.Equal([]byte{}))
			gomega.Expect(resp).NotTo(gomega.BeNil())
		})

		ginkgo.It("should not produce a further client response after a 235", func() {
			auth := newOAuth2Auth("alice", "ya29.token", "smtp.gmail.com").(*oauth2Auth)
			resp, err := auth.Next(nil, false)
			gomega.Expect(err).NotTo(gomega.HaveOccurred())
			gomega.Expect(resp).To(gomega.BeNil())
		})
	})
})
