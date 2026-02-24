package e2e_test

import (
	"net/url"
	"os"

	"github.com/onsi/ginkgo/v2"
	"github.com/onsi/gomega"

	"github.com/nicholas-fedor/shoutrrr/internal/testutils"
	"github.com/nicholas-fedor/shoutrrr/pkg/services/sms/twilio"
)

var _ = ginkgo.Describe("Twilio E2E Service ID Test", func() {
	ginkgo.When("running e2e tests", func() {
		ginkgo.It("should return the correct service identifier", func() {
			envURL := os.Getenv("SHOUTRRR_TWILIO_URL")
			if envURL == "" {
				ginkgo.Skip("SHOUTRRR_TWILIO_URL not set, skipping service ID test")

				return
			}

			serviceURL, _ := url.Parse(envURL)
			service := &twilio.Service{}
			err := service.Initialize(serviceURL, testutils.TestLogger())
			gomega.Expect(err).NotTo(gomega.HaveOccurred())

			gomega.Expect(service.GetID()).To(gomega.Equal("twilio"))
		})
	})
})
