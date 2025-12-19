package discord_test

import (
	"strings"

	"github.com/jarcoal/httpmock"

	ginkgo "github.com/onsi/ginkgo/v2"
	gomega "github.com/onsi/gomega"

	"github.com/nicholas-fedor/shoutrrr/pkg/services/chat/discord"
)

var _ = ginkgo.Describe("Threads", func() {
	ginkgo.BeforeEach(func() {
		httpmock.Activate()
	})

	ginkgo.AfterEach(func() {
		httpmock.DeactivateAndReset()
	})

	ginkgo.Context("thread functionality", func() {
		ginkgo.It("should post to existing thread with thread_id", func() {
			customConfig := CreateDummyConfig()
			customConfig.ThreadID = "1234567890123456789"
			customService := CreateTestService(customConfig)

			// The API URL should include the thread_id as a query parameter
			expectedURL := "https://discord.com/api/webhooks/123456789012345678/test-token-abcdefghijklmnopqrstuvwxyz123456?thread_id=1234567890123456789"
			httpmock.RegisterResponder("POST", expectedURL,
				httpmock.NewStringResponder(204, ""))

			err := customService.Send("Test message in thread", nil)
			gomega.Expect(err).NotTo(gomega.HaveOccurred())
		})

		ginkgo.It("should post file attachments to existing thread", func() {
			customConfig := CreateDummyConfig()
			customConfig.ThreadID = "1234567890123456789"
			customService := CreateTestService(customConfig)

			testData := []byte("existing thread file content")

			SetupMockResponder(&customConfig, 200)

			items := CreateMessageItemWithFile(
				"File message in existing thread",
				"existing-thread-file.txt",
				testData,
			)
			err := customService.SendItems(items, nil)
			gomega.Expect(err).NotTo(gomega.HaveOccurred())
			gomega.Expect(httpmock.GetTotalCallCount()).To(gomega.Equal(1))
		})

		ginkgo.Context("thread parameter edge cases", func() {
			ginkgo.It("should handle thread IDs with special characters", func() {
				customConfig := CreateDummyConfig()
				customConfig.ThreadID = "thread-123_special"
				customService := CreateTestService(customConfig)

				expectedURL := "https://discord.com/api/webhooks/123456789012345678/test-token-abcdefghijklmnopqrstuvwxyz123456?thread_id=thread-123_special"
				httpmock.RegisterResponder(
					"POST",
					expectedURL,
					httpmock.NewStringResponder(204, ""),
				)

				err := customService.Send("Test message", nil)
				gomega.Expect(err).NotTo(gomega.HaveOccurred())

				// Verify the URL was constructed correctly
				actualURL := discord.CreateAPIURLFromConfig(&customConfig)
				gomega.Expect(actualURL).To(gomega.Equal(expectedURL))
			})

			ginkgo.It("should handle very long thread IDs", func() {
				customConfig := CreateDummyConfig()
				customConfig.ThreadID = strings.Repeat("1", 100)
				customService := CreateTestService(customConfig)

				SetupMockResponder(&customConfig, 204)
				err := customService.Send("Test message", nil)
				gomega.Expect(err).NotTo(gomega.HaveOccurred())
			})
		})
	})
})
