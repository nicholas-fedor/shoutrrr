package e2e_test

import (
	"net/url"
	"os"

	"github.com/onsi/ginkgo/v2"
	"github.com/onsi/gomega"

	"github.com/nicholas-fedor/shoutrrr/internal/testutils"
	"github.com/nicholas-fedor/shoutrrr/pkg/services/chat/discord"
)

var _ = ginkgo.Describe("Discord E2E JSON Mode Test", func() {
	ginkgo.When("running e2e tests", func() {
		ginkgo.It("should send message in JSON mode", func() {
			envURL := os.Getenv("SHOUTRRR_DISCORD_URL")
			if envURL == "" {
				ginkgo.Skip("SHOUTRRR_DISCORD_URL not set, skipping JSON mode test")

				return
			}

			demoURLStr := envURL + "?json=true"
			demoURL, _ := url.Parse(demoURLStr)
			service := &discord.Service{}
			err := service.Initialize(demoURL, testutils.TestLogger())
			gomega.Expect(err).NotTo(gomega.HaveOccurred())

			jsonPayload := `{"content": "E2E Test: Raw JSON message demonstrating JSON mode"}`
			err = service.Send(jsonPayload, nil)
			gomega.Expect(err).NotTo(gomega.HaveOccurred())
		})
	})
})
