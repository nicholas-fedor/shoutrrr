package e2e_test

import (
	"net/url"
	"os"

	"github.com/onsi/ginkgo/v2"
	"github.com/onsi/gomega"

	"github.com/nicholas-fedor/shoutrrr/internal/testutils"
	"github.com/nicholas-fedor/shoutrrr/pkg/services/push/mqtt"
)

var _ = ginkgo.Describe("MQTT E2E Basic Test", func() {
	ginkgo.When("running e2e tests", func() {
		ginkgo.It("should send basic text content", func() {
			envURL := os.Getenv("SHOUTRRR_MQTT_URL")
			if envURL == "" {
				ginkgo.Skip("SHOUTRRR_MQTT_URL not set, skipping basic text test")

				return
			}

			serviceURL, err := url.Parse(envURL)
			gomega.Expect(err).NotTo(gomega.HaveOccurred())

			service := &mqtt.Service{}
			err = service.Initialize(serviceURL, testutils.TestLogger())
			gomega.Expect(err).NotTo(gomega.HaveOccurred())

			err = service.Send("E2E Test: Basic text content notification", nil)
			gomega.Expect(err).NotTo(gomega.HaveOccurred())

			err = service.Close()
			gomega.Expect(err).NotTo(gomega.HaveOccurred())
		})
	})
})
