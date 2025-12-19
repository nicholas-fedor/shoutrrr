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

var _ = ginkgo.Describe("Discord E2E File Attachment Test", func() {
	ginkgo.When("running e2e tests", func() {
		ginkgo.It("should send file attachment", func() {
			envURL := os.Getenv("SHOUTRRR_DISCORD_URL")
			if envURL == "" {
				ginkgo.Skip("SHOUTRRR_DISCORD_URL not set, skipping file attachment test")

				return
			}

			serviceURL, _ := url.Parse(envURL)
			service := &discord.Service{}
			err := service.Initialize(serviceURL, testutils.TestLogger())
			gomega.Expect(err).NotTo(gomega.HaveOccurred())

			testData := []byte("E2E test file content for file attachment feature")
			items := []types.MessageItem{
				{
					Text: "E2E Test: File attachment",
					File: &types.File{Name: "e2e_test.txt", Data: testData},
				},
			}
			err = service.SendItems(items, nil)
			gomega.Expect(err).NotTo(gomega.HaveOccurred())
		})
	})
})
