package discord_test

import (
	"github.com/jarcoal/httpmock"

	ginkgo "github.com/onsi/ginkgo/v2"
	gomega "github.com/onsi/gomega"

	"github.com/nicholas-fedor/shoutrrr/pkg/services/chat/discord"
)

var _ = ginkgo.Describe("Config", func() {
	ginkgo.BeforeEach(func() {
		httpmock.Activate()
	})

	ginkgo.AfterEach(func() {
		httpmock.DeactivateAndReset()
	})

	ginkgo.Context("configuration options", func() {
		ginkgo.It("should handle custom username", func() {
			customConfig := CreateDummyConfig()
			customConfig.Username = "TestBot"
			customService := CreateTestService(customConfig)

			SetupMockResponderWithPayloadValidation(&customConfig, 204,
				func(payload discord.WebhookPayload) error {
					if err := ValidatePlainTextPayload("Test message")(payload); err != nil {
						return err
					}

					return ValidateUsername("TestBot")(payload)
				})
			err := customService.Send("Test message", nil)
			gomega.Expect(err).NotTo(gomega.HaveOccurred())
		})

		ginkgo.It("should handle custom avatar URL", func() {
			customConfig := CreateDummyConfig()
			customConfig.Avatar = "https://example.com/avatar.png"
			customService := CreateTestService(customConfig)

			SetupMockResponderWithPayloadValidation(&customConfig, 204,
				func(payload discord.WebhookPayload) error {
					if err := ValidatePlainTextPayload("Test message")(payload); err != nil {
						return err
					}

					return ValidateAvatarURL("https://example.com/avatar.png")(payload)
				})
			err := customService.Send("Test message", nil)
			gomega.Expect(err).NotTo(gomega.HaveOccurred())
		})

		ginkgo.It("should handle both username and avatar", func() {
			customConfig := CreateDummyConfig()
			customConfig.Username = "CustomBot"
			customConfig.Avatar = "https://example.com/bot-avatar.png"
			customService := CreateTestService(customConfig)

			SetupMockResponderWithPayloadValidation(&customConfig, 204,
				func(payload discord.WebhookPayload) error {
					if err := ValidatePlainTextPayload("Test message")(payload); err != nil {
						return err
					}
					if err := ValidateUsername("CustomBot")(payload); err != nil {
						return err
					}

					return ValidateAvatarURL("https://example.com/bot-avatar.png")(payload)
				})
			err := customService.Send("Test message", nil)
			gomega.Expect(err).NotTo(gomega.HaveOccurred())
		})
	})
})
