package signal

import (
	"github.com/onsi/ginkgo/v2"
	"github.com/onsi/gomega"
)

var _ = ginkgo.Describe("payload", func() {
	ginkgo.Describe("composeMessage", func() {
		ginkgo.It("should return the body when title is empty", func() {
			gomega.Expect(composeMessage("", "body", false)).To(gomega.Equal("body"))
		})

		ginkgo.It("should prepend a plain title", func() {
			gomega.Expect(composeMessage("Alert", "body", false)).To(gomega.Equal("Alert\nbody"))
		})

		ginkgo.It("should bold the title when styled", func() {
			gomega.Expect(composeMessage("Alert", "body", true)).To(gomega.Equal("**Alert**\nbody"))
		})

		ginkgo.It("should send title only when the body is empty", func() {
			gomega.Expect(composeMessage("Alert", "", false)).To(gomega.Equal("Alert"))
		})
	})

	ginkgo.Describe("parseTimestamp", func() {
		ginkgo.It("should accept an unquoted JSON number", func() {
			n, err := parseTimestamp([]byte(`{"timestamp": 1234567890}`))
			gomega.Expect(err).NotTo(gomega.HaveOccurred())
			gomega.Expect(n).To(gomega.Equal(int64(1234567890)))
		})

		ginkgo.It("should accept a quoted JSON number", func() {
			n, err := parseTimestamp([]byte(`{"timestamp": "1234567890"}`))
			gomega.Expect(err).NotTo(gomega.HaveOccurred())
			gomega.Expect(n).To(gomega.Equal(int64(1234567890)))
		})
	})

	ginkgo.Describe("apiRecipients", func() {
		ginkgo.It("should strip the u: prefix from usernames", func() {
			gomega.Expect(apiRecipients([]string{"u:someuser.123", "+1234567890"})).
				To(gomega.Equal([]string{"someuser.123", "+1234567890"}))
		})
	})

	ginkgo.Describe("parseAttachments", func() {
		ginkgo.It("should return nil for empty input", func() {
			gomega.Expect(parseAttachments("")).To(gomega.BeNil())
			gomega.Expect(parseAttachments("   ")).To(gomega.BeNil())
		})

		ginkgo.It("should split comma-separated raw base64", func() {
			gomega.Expect(parseAttachments("a, b")).To(gomega.Equal([]string{"a", "b"}))
		})

		ginkgo.It("should keep a data URI as a single attachment", func() {
			gomega.Expect(parseAttachments("data:image/png;base64,iVBOR")).
				To(gomega.Equal([]string{"data:image/png;base64,iVBOR"}))
		})
	})
})
