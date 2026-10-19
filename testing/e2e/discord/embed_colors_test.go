package e2e_test

import (
	"context"
	"net/url"
	"os"

	"github.com/onsi/ginkgo/v2"
	"github.com/onsi/gomega"

	"github.com/nicholas-fedor/shoutrrr/pkg/services/chat/discord"
	"github.com/nicholas-fedor/shoutrrr/pkg/types"
	"github.com/nicholas-fedor/shoutrrr/testing/e2e/internal/logger"
)

var _ = ginkgo.Describe("Discord E2E Embed Colors Test", func() {
	ginkgo.When("running e2e tests", func() {
		ginkgo.It("should send rich embed with colors (error level)", func() {
			envURL := os.Getenv("SHOUTRRR_DISCORD_URL")
			if envURL == "" {
				ginkgo.Skip("SHOUTRRR_DISCORD_URL not set, skipping embed colors test")

				return
			}

			serviceURL, _ := url.Parse(envURL)
			service := &discord.Service{}
			err := service.Initialize(serviceURL, logger.New())
			gomega.Expect(err).NotTo(gomega.HaveOccurred())

			items := []types.MessageItem{
				{Text: "E2E Test: Error level embed (red color)", Level: types.Error},
			}
			err = service.SendItemsContext(context.Background(), items, nil)
			gomega.Expect(err).NotTo(gomega.HaveOccurred())
		})
	})
})
