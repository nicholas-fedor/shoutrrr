package e2e_test

import (
	"net/url"
	"os"
	"time"

	"github.com/onsi/ginkgo/v2"
	"github.com/onsi/gomega"

	"github.com/nicholas-fedor/shoutrrr/internal/testutils"
	"github.com/nicholas-fedor/shoutrrr/pkg/services/chat/discord"
	"github.com/nicholas-fedor/shoutrrr/pkg/types"
)

var _ = ginkgo.Describe("Discord E2E Complex Combination Test", func() {
	ginkgo.When("running e2e tests", func() {
		ginkgo.It("should send complex combination (embed with all features)", func() {
			envURL := os.Getenv("SHOUTRRR_DISCORD_URL")
			if envURL == "" {
				ginkgo.Skip("SHOUTRRR_DISCORD_URL not set, skipping complex combination test")

				return
			}

			serviceURL, _ := url.Parse(envURL)
			service := &discord.Service{}
			err := service.Initialize(serviceURL, testutils.TestLogger())
			gomega.Expect(err).NotTo(gomega.HaveOccurred())

			items := []types.MessageItem{
				{
					Text:      "E2E Test: Complex combination - embed with all features",
					Level:     types.Info,
					Timestamp: time.Now(),
					Fields: []types.Field{
						{Key: "embed_author_name", Value: "Shoutrrr E2E Test"},
						{
							Key:   "embed_author_url",
							Value: "https://github.com/nicholas-fedor/shoutrrr",
						},
						{
							Key:   "embed_author_icon_url",
							Value: "https://raw.githubusercontent.com/nicholas-fedor/shoutrrr/master/docs/assets/media/shoutrrr-180px.png",
						},
						{
							Key:   "embed_image_url",
							Value: "https://raw.githubusercontent.com/nicholas-fedor/shoutrrr/master/docs/assets/media/shoutrrr-logotype.png",
						},
						{
							Key:   "embed_thumbnail_url",
							Value: "https://raw.githubusercontent.com/nicholas-fedor/shoutrrr/master/docs/assets/media/shoutrrr-180px.png",
						},
						{Key: "Status", Value: "Complex Test"},
						{Key: "Features", Value: "Author, Images, Fields, Timestamp"},
						{Key: "Level", Value: "Info"},
					},
				},
			}
			err = service.SendItems(items, nil)
			gomega.Expect(err).NotTo(gomega.HaveOccurred())
		})
	})
})
