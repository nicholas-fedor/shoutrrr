package e2e_test

import (
	"net/url"
	"os"

	"github.com/onsi/ginkgo/v2"
	"github.com/onsi/gomega"

	"github.com/nicholas-fedor/shoutrrr/internal/testutils"
	"github.com/nicholas-fedor/shoutrrr/pkg/services/chat/discord"
)

var _ = ginkgo.Describe("Discord E2E Special Characters Test", func() {
	ginkgo.When("running e2e tests", func() {
		ginkgo.It("should send message with special characters", func() {
			envURL := os.Getenv("SHOUTRRR_DISCORD_URL")
			if envURL == "" {
				ginkgo.Skip("SHOUTRRR_DISCORD_URL not set, skipping special characters test")

				return
			}

			serviceURL, _ := url.Parse(envURL)
			service := &discord.Service{}
			err := service.Initialize(serviceURL, testutils.TestLogger())
			gomega.Expect(err).NotTo(gomega.HaveOccurred())

			specialMessage := "E2E Test: Special characters - éñüñ 中文 🚀 @everyone #channel <@123456> ||spoiler|| `code`"
			err = service.Send(specialMessage, nil)
			gomega.Expect(err).NotTo(gomega.HaveOccurred())
		})
	})
})
