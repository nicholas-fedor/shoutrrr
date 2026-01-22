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

var _ = ginkgo.Describe("Discord E2E Multiple Files Test", func() {
	ginkgo.When("running e2e tests", func() {
		ginkgo.It("should send multiple file attachments", func() {
			envURL := os.Getenv("SHOUTRRR_DISCORD_URL")
			if envURL == "" {
				ginkgo.Skip("SHOUTRRR_DISCORD_URL not set, skipping multiple files test")

				return
			}

			serviceURL, _ := url.Parse(envURL)
			service := &discord.Service{}
			err := service.Initialize(serviceURL, testutils.TestLogger())
			gomega.Expect(err).NotTo(gomega.HaveOccurred())

			file1Data := []byte("Content of first test file")
			file2Data := []byte("Content of second test file")
			items := []types.MessageItem{
				{
					Text: "E2E Test: Multiple file attachments - File 1",
					File: &types.File{Name: "test1.txt", Data: file1Data},
				},
				{
					Text: "E2E Test: Multiple file attachments - File 2",
					File: &types.File{Name: "test2.txt", Data: file2Data},
				},
			}
			err = service.SendItems(items, nil)
			gomega.Expect(err).NotTo(gomega.HaveOccurred())
		})
	})
})
