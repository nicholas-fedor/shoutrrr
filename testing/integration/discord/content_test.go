package discord_test

import (
	"strings"

	"github.com/jarcoal/httpmock"

	ginkgo "github.com/onsi/ginkgo/v2"
	gomega "github.com/onsi/gomega"

	"github.com/nicholas-fedor/shoutrrr/pkg/services/chat/discord"
)

var _ = ginkgo.Describe("Content", func() {
	var testService *discord.Service
	var dummyConfig discord.Config

	ginkgo.BeforeEach(func() {
		httpmock.Activate()
		dummyConfig = CreateDummyConfig()
		testService = CreateTestService(dummyConfig)
	})

	ginkgo.AfterEach(func() {
		httpmock.DeactivateAndReset()
	})

	ginkgo.Context("content field support (plain text messages)", func() {
		ginkgo.It("should send a simple plain text message successfully", func() {
			message := "Hello, Discord!"
			SetupMockResponder(&dummyConfig, 204)
			err := testService.Send(message, nil)
			gomega.Expect(err).NotTo(gomega.HaveOccurred())

			// Verify the request was made to the correct URL
			gomega.Expect(httpmock.GetTotalCallCount()).To(gomega.Equal(1))
		})

		ginkgo.It("should send plain text with special characters successfully", func() {
			message := "Special chars: éñüñ 中文 🚀 @everyone #channel"
			SetupMockResponder(&dummyConfig, 204)
			err := testService.Send(message, nil)
			gomega.Expect(err).NotTo(gomega.HaveOccurred())

			// Verify the request was made
			gomega.Expect(httpmock.GetTotalCallCount()).To(gomega.Equal(1))
		})

		ginkgo.It("should send long plain text messages successfully", func() {
			longMessage := strings.Repeat("This is a long message. ", 100)
			SetupMockResponder(&dummyConfig, 204)
			err := testService.Send(longMessage, nil)
			gomega.Expect(err).NotTo(gomega.HaveOccurred())

			// Verify the request was made
			gomega.Expect(httpmock.GetTotalCallCount()).To(gomega.Equal(1))
		})

		ginkgo.It("should handle empty messages with error", func() {
			SetupMockResponder(&dummyConfig, 204)
			err := testService.Send("", nil)
			gomega.Expect(err).To(gomega.HaveOccurred())
		})

		ginkgo.It("should handle whitespace-only messages successfully", func() {
			SetupMockResponder(&dummyConfig, 204)
			err := testService.Send("   \n\t   ", nil)
			gomega.Expect(err).NotTo(gomega.HaveOccurred())

			// Verify the request was made
			gomega.Expect(httpmock.GetTotalCallCount()).To(gomega.Equal(1))
		})

		ginkgo.Context("malformed payload handling", func() {
			ginkgo.It("should handle extremely long content fields gracefully", func() {
				// Test with content that exceeds Discord's limits but should be handled by chunking
				veryLongContent := strings.Repeat(
					"This is a very long message that should be chunked. ",
					500,
				)
				SetupMockResponder(&dummyConfig, 204)
				err := testService.Send(veryLongContent, nil)
				gomega.Expect(err).NotTo(gomega.HaveOccurred())
				gomega.Expect(httpmock.GetTotalCallCount()).To(gomega.BeNumerically(">=", 1))
			})

			ginkgo.It("should handle content with special Unicode characters", func() {
				specialContent := "Special chars: éñüñ 中文 🚀 @everyone #channel 😀 🎉"
				SetupMockResponder(&dummyConfig, 204)
				err := testService.Send(specialContent, nil)
				gomega.Expect(err).NotTo(gomega.HaveOccurred())
				gomega.Expect(httpmock.GetTotalCallCount()).To(gomega.Equal(1))
			})

			ginkgo.It("should handle content with null bytes", func() {
				contentWithNull := "Content with null byte: \x00 in the middle"
				SetupMockResponder(&dummyConfig, 204)
				err := testService.Send(contentWithNull, nil)
				gomega.Expect(err).NotTo(gomega.HaveOccurred())
				gomega.Expect(httpmock.GetTotalCallCount()).To(gomega.Equal(1))
			})

			ginkgo.It("should handle content with control characters", func() {
				contentWithControl := "Content with control chars: \n\r\t"
				SetupMockResponder(&dummyConfig, 204)
				err := testService.Send(contentWithControl, nil)
				gomega.Expect(err).NotTo(gomega.HaveOccurred())
				gomega.Expect(httpmock.GetTotalCallCount()).To(gomega.Equal(1))
			})
		})
	})
})
