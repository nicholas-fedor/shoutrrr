package e2e_test

import (
	"net/url"
	"os"

	"github.com/onsi/ginkgo/v2"
	"github.com/onsi/gomega"

	"github.com/nicholas-fedor/shoutrrr/internal/testutils"
	"github.com/nicholas-fedor/shoutrrr/pkg/services/chat/discord"
)

var _ = ginkgo.Describe("Discord E2E Username Avatar Test", func() {
	ginkgo.When("running e2e tests", func() {
		ginkgo.It("should send message with custom username and avatar", func() {
			envURL := os.Getenv("SHOUTRRR_DISCORD_URL")
			if envURL == "" {
				ginkgo.Skip("SHOUTRRR_DISCORD_URL not set, skipping username/avatar test")

				return
			}

			demoURLStr := envURL + "?username=ShoutrrrDemo&avatar=https://raw.githubusercontent.com/nicholas-fedor/shoutrrr/master/docs/assets/media/shoutrrr-180px.png"
			demoURL, _ := url.Parse(demoURLStr)
			service := &discord.Service{}
			err := service.Initialize(demoURL, testutils.TestLogger())
			gomega.Expect(err).NotTo(gomega.HaveOccurred())

			err = service.Send("E2E Test: Custom username and avatar", nil)
			gomega.Expect(err).NotTo(gomega.HaveOccurred())
		})
	})
})
