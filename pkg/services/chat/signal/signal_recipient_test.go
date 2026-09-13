package signal

import (
	"github.com/onsi/ginkgo/v2"
	"github.com/onsi/gomega"
)

var _ = ginkgo.Describe("recipients", func() {
	ginkgo.It("should parse valid phone numbers", func() {
		recipients, err := parseRecipients([]string{"+1234567890", "+0987654321"})
		gomega.Expect(err).NotTo(gomega.HaveOccurred())
		gomega.Expect(recipients).To(gomega.Equal([]string{"+1234567890", "+0987654321"}))
	})

	ginkgo.It("should parse valid group IDs", func() {
		recipients, err := parseRecipients([]string{"group.testgroup", "group.abcdef123"})
		gomega.Expect(err).NotTo(gomega.HaveOccurred())
		gomega.Expect(recipients).
			To(gomega.Equal([]string{"group.testgroup", "group.abcdef123"}))
	})

	ginkgo.It("should parse group IDs with base64 characters", func() {
		recipients, err := parseRecipients([]string{"group.ABCD/EFGH=", "group.xyz+abc"})
		gomega.Expect(err).NotTo(gomega.HaveOccurred())
		gomega.Expect(recipients).
			To(gomega.Equal([]string{"group.ABCD/EFGH=", "group.xyz+abc"}))
	})

	ginkgo.It("should parse mixed phone numbers and group IDs", func() {
		recipients, err := parseRecipients(
			[]string{"+1234567890", "group.testgroup", "+0987654321"},
		)
		gomega.Expect(err).NotTo(gomega.HaveOccurred())
		gomega.Expect(recipients).
			To(gomega.Equal([]string{"+1234567890", "group.testgroup", "+0987654321"}))
	})

	ginkgo.It("should parse usernames", func() {
		recipients, err := parseRecipients([]string{"u:someuser.123"})
		gomega.Expect(err).NotTo(gomega.HaveOccurred())
		gomega.Expect(recipients).To(gomega.Equal([]string{"u:someuser.123"}))
	})

	ginkgo.It("should return error for invalid recipients", func() {
		_, err := parseRecipients([]string{"invalid-recipient"})
		gomega.Expect(err).To(gomega.HaveOccurred())
		gomega.Expect(err.Error()).To(gomega.ContainSubstring("invalid recipient"))
	})

	ginkgo.It("should return error for mixed valid and invalid recipients", func() {
		_, err := parseRecipients([]string{"+1234567890", "invalid-recipient"})
		gomega.Expect(err).To(gomega.HaveOccurred())
		gomega.Expect(err.Error()).To(gomega.ContainSubstring("invalid recipient"))
	})

	ginkgo.It("should return error for empty recipient list", func() {
		_, err := parseRecipients([]string{})
		gomega.Expect(err).To(gomega.MatchError(ErrNoRecipients))
	})

	ginkgo.It("should handle group IDs split across path segments", func() {
		recipients, err := parseRecipients([]string{"group.ABCD", "EFGH="})
		gomega.Expect(err).NotTo(gomega.HaveOccurred())
		gomega.Expect(recipients).To(gomega.Equal([]string{"group.ABCD/EFGH="}))
	})
})
