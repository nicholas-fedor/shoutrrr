package e2e_test

import (
	"net/url"
	"os"

	"github.com/onsi/ginkgo/v2"
	"github.com/onsi/gomega"

	"github.com/nicholas-fedor/shoutrrr/internal/testutils"
	"github.com/nicholas-fedor/shoutrrr/pkg/services/chat/discord"
	"github.com/nicholas-fedor/shoutrrr/pkg/types"
)

var _ = ginkgo.Describe("Discord E2E Embed Images Test", func() {
	ginkgo.When("running e2e tests", func() {
		ginkgo.It("should send embed with images and thumbnails", func() {
			envURL := os.Getenv("SHOUTRRR_DISCORD_URL")
			if envURL == "" {
				ginkgo.Skip("SHOUTRRR_DISCORD_URL not set, skipping images test")

				return
			}

			serviceURL, _ := url.Parse(envURL)
			service := &discord.Service{}
			err := service.Initialize(serviceURL, testutils.TestLogger())
			gomega.Expect(err).NotTo(gomega.HaveOccurred())

			items := []types.MessageItem{
				{
					Text: "E2E Test: Embed with image and thumbnail",
					Fields: []types.Field{
						{
							Key:   "embed_image_url",
							Value: "https://raw.githubusercontent.com/nicholas-fedor/shoutrrr/master/docs/assets/media/shoutrrr-180px.png",
						},
						{
							Key:   "embed_thumbnail_url",
							Value: "https://raw.githubusercontent.com/nicholas-fedor/shoutrrr/master/docs/assets/media/shoutrrr-logotype.png",
						},
					},
				},
			}
			err = service.SendItems(items, nil)
			gomega.Expect(err).NotTo(gomega.HaveOccurred())
		})
	})
})
