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

var _ = ginkgo.Describe("Discord E2E Multiple Embeds Test", func() {
	ginkgo.When("running e2e tests", func() {
		ginkgo.It("should send multiple embeds", func() {
			envURL := os.Getenv("SHOUTRRR_DISCORD_URL")
			if envURL == "" {
				ginkgo.Skip("SHOUTRRR_DISCORD_URL not set, skipping multiple embeds test")

				return
			}

			serviceURL, _ := url.Parse(envURL)
			service := &discord.Service{}
			err := service.Initialize(serviceURL, logger.New())
			gomega.Expect(err).NotTo(gomega.HaveOccurred())

			items := []types.MessageItem{
				{
					Text:  "E2E Test: First embed in multiple embeds",
					Level: types.Info,
					Fields: []types.Field{
						{Key: "embed_author_name", Value: "First Embed"},
					},
				},
				{
					Text:  "E2E Test: Second embed in multiple embeds",
					Level: types.Warning,
					Fields: []types.Field{
						{Key: "embed_author_name", Value: "Second Embed"},
					},
				},
			}
			err = service.SendItemsContext(context.Background(), items, nil)
			gomega.Expect(err).NotTo(gomega.HaveOccurred())
		})
	})
})
