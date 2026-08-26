package e2e_test

import (
	"net/url"
	"os"

	"github.com/onsi/ginkgo/v2"
	"github.com/onsi/gomega"

	"github.com/nicholas-fedor/shoutrrr/internal/testutils"
	"github.com/nicholas-fedor/shoutrrr/pkg/services/push/signalgrid"
)

var _ = ginkgo.Describe("Signalgrid E2E Title Test", func() {
	ginkgo.When("running e2e tests", func() {
		ginkgo.It("should send a notification with a title", func() {
			envURL := os.Getenv("SHOUTRRR_SIGNALGRID_URL")
			if envURL == "" {
				ginkgo.Skip("SHOUTRRR_SIGNALGRID_URL not set, skipping title test")

				return
			}

			serviceURL, err := url.Parse(envURL)
			gomega.Expect(err).NotTo(gomega.HaveOccurred())

			q := serviceURL.Query()
			q.Set("title", "E2E Alert")
			serviceURL.RawQuery = q.Encode()

			service := &signalgrid.Service{}
			err = service.Initialize(serviceURL, testutils.TestLogger())
			gomega.Expect(err).NotTo(gomega.HaveOccurred())

			err = service.Send("E2E Test: Message with title", nil)
			gomega.Expect(err).NotTo(gomega.HaveOccurred())
		})
	})
})
