package e2e_test

import (
	"net/url"
	"os"

	"github.com/onsi/ginkgo/v2"
	"github.com/onsi/gomega"

	"github.com/nicholas-fedor/shoutrrr/internal/testutils"
	"github.com/nicholas-fedor/shoutrrr/pkg/services/push/signalgrid"
)

var _ = ginkgo.Describe("Signalgrid E2E Type and Critical Test", func() {
	ginkgo.When("running e2e tests", func() {
		ginkgo.It("should send a CRIT notification without critical delivery", func() {
			envURL := os.Getenv("SHOUTRRR_SIGNALGRID_URL")
			if envURL == "" {
				ginkgo.Skip("SHOUTRRR_SIGNALGRID_URL not set, skipping type test")

				return
			}

			serviceURL, err := url.Parse(envURL)
			gomega.Expect(err).NotTo(gomega.HaveOccurred())

			q := serviceURL.Query()
			q.Set("type", "CRIT")
			q.Set("title", "E2E CRIT")
			serviceURL.RawQuery = q.Encode()

			service := &signalgrid.Service{}
			err = service.Initialize(serviceURL, testutils.TestLogger())
			gomega.Expect(err).NotTo(gomega.HaveOccurred())

			err = service.Send("E2E Test: CRIT type without critical flag", nil)
			gomega.Expect(err).NotTo(gomega.HaveOccurred())
		})

		ginkgo.It("should send a critical notification", func() {
			envURL := os.Getenv("SHOUTRRR_SIGNALGRID_URL")
			if envURL == "" {
				ginkgo.Skip("SHOUTRRR_SIGNALGRID_URL not set, skipping critical test")

				return
			}

			serviceURL, err := url.Parse(envURL)
			gomega.Expect(err).NotTo(gomega.HaveOccurred())

			q := serviceURL.Query()
			q.Set("type", "INFO")
			q.Set("critical", "true")
			q.Set("title", "E2E Critical")
			serviceURL.RawQuery = q.Encode()

			service := &signalgrid.Service{}
			err = service.Initialize(serviceURL, testutils.TestLogger())
			gomega.Expect(err).NotTo(gomega.HaveOccurred())

			err = service.Send("E2E Test: critical delivery flag", nil)
			gomega.Expect(err).NotTo(gomega.HaveOccurred())
		})
	})
})
