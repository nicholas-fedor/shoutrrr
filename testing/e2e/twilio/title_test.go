package e2e_test

import (
	"net/url"
	"os"

	"github.com/onsi/ginkgo/v2"
	"github.com/onsi/gomega"

	"github.com/nicholas-fedor/shoutrrr/internal/testutils"
	"github.com/nicholas-fedor/shoutrrr/pkg/services/sms/twilio"
)

var _ = ginkgo.Describe("Twilio E2E Title Test", func() {
	ginkgo.When("running e2e tests", func() {
		ginkgo.It("should send an SMS with a title prepended", func() {
			envURL := os.Getenv("SHOUTRRR_TWILIO_URL")
			if envURL == "" {
				ginkgo.Skip("SHOUTRRR_TWILIO_URL not set, skipping title test")

				return
			}

			serviceURL, _ := url.Parse(envURL)

			// Append title query parameter
			q := serviceURL.Query()
			q.Set("title", "E2E Alert")
			serviceURL.RawQuery = q.Encode()

			service := &twilio.Service{}
			err := service.Initialize(serviceURL, testutils.TestLogger())
			gomega.Expect(err).NotTo(gomega.HaveOccurred())

			err = service.Send("E2E Test: Message with title", nil)
			gomega.Expect(err).NotTo(gomega.HaveOccurred())
		})
	})
})
