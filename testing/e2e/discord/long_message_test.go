package e2e_test

import (
	"net/url"
	"os"
	"strings"

	"github.com/onsi/ginkgo/v2"
	"github.com/onsi/gomega"

	"github.com/nicholas-fedor/shoutrrr/internal/testutils"
	"github.com/nicholas-fedor/shoutrrr/pkg/services/chat/discord"
)

var _ = ginkgo.Describe("Discord E2E Long Message Test", func() {
	ginkgo.When("running e2e tests", func() {
		ginkgo.It("should send long message (chunking)", func() {
			envURL := os.Getenv("SHOUTRRR_DISCORD_URL")
			if envURL == "" {
				ginkgo.Skip("SHOUTRRR_DISCORD_URL not set, skipping long message test")

				return
			}

			serviceURL, _ := url.Parse(envURL)
			// Enable chunking by setting splitLines=false
			query := serviceURL.Query()
			query.Set("splitLines", "false")
			serviceURL.RawQuery = query.Encode()

			service := &discord.Service{}
			err := service.Initialize(serviceURL, testutils.TestLogger())
			gomega.Expect(err).NotTo(gomega.HaveOccurred())

			longMessage := "E2E Test: Long message for chunking - " + strings.Repeat(
				"This is a very long message that should be split into multiple messages. ",
				50,
			)
			err = service.Send(longMessage, nil)
			gomega.Expect(err).NotTo(gomega.HaveOccurred())
		})
	})
})
