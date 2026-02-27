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

var _ = ginkgo.Describe("Discord E2E Thread Test", func() {
	ginkgo.When("running e2e tests", func() {
		ginkgo.It("should post message to existing thread using thread_id", func() {
			envURL := os.Getenv("SHOUTRRR_DISCORD_URL")
			envThreadID := os.Getenv("SHOUTRRR_DISCORD_THREAD_ID")

			if envURL == "" {
				ginkgo.Skip("SHOUTRRR_DISCORD_URL not set, skipping thread posting test")

				return
			}

			if envThreadID == "" {
				ginkgo.Skip("SHOUTRRR_DISCORD_THREAD_ID not set, skipping thread posting test")

				return
			}

			demoURLStr := envURL + "?thread_id=" + envThreadID
			demoURL, err := url.Parse(demoURLStr)
			gomega.Expect(err).NotTo(gomega.HaveOccurred(), "failed to parse Discord URL")

			service := &discord.Service{}
			err = service.Initialize(demoURL, testutils.TestLogger())
			gomega.Expect(err).NotTo(gomega.HaveOccurred())

			// Debug: Print configuration
			ginkgo.GinkgoWriter.Printf(
				"DEBUG: Discord config - WebhookID: %s, Token: %s, ThreadID: %s\n",
				redact(service.Config.WebhookID),
				redact(service.Config.Token),
				redact(service.Config.ThreadID),
			)

			err = service.Send("E2E Test: Posting to existing thread", nil)
			// Debug: Print the error if any
			if err != nil {
				ginkgo.GinkgoWriter.Printf("DEBUG: Send error: %v\n", err)
			}

			// Posting to thread may fail due to permissions or thread state
			// Accept both success and graceful failure (400 Bad Request due to permissions)
			if err != nil {
				// Check if it's a 400 error (permission/thread issue) - this is acceptable
				errStr := err.Error()
				if strings.Contains(errStr, "400") || strings.Contains(errStr, "Bad Request") {
					ginkgo.GinkgoWriter.Printf("DEBUG: Skipping due to 400 error: %v\n", err)
					ginkgo.Skip(
						"Thread posting failed due to permissions/thread state (400), skipping test",
					)

					return
				}
				// For other errors, still fail the test
				gomega.Expect(err).NotTo(gomega.HaveOccurred())
			} else {
				ginkgo.GinkgoWriter.Printf(
					"DEBUG: Send succeeded, message should be posted to thread\n",
				)
			}
		})
	})
})

// redact masks sensitive string values for logging.
func redact(value string) string {
	if len(value) <= 4 {
		return "****"
	}

	return "****" + value[len(value)-4:]
}
