package e2e_test

import (
	"net/url"
	"os"

	"github.com/onsi/ginkgo/v2"
	"github.com/onsi/gomega"

	"github.com/nicholas-fedor/shoutrrr/internal/testutils"
	"github.com/nicholas-fedor/shoutrrr/pkg/services/chat/zulip"
	"github.com/nicholas-fedor/shoutrrr/pkg/types"
)

var _ = ginkgo.Describe("Zulip E2E Basic Tests", func() {
	ginkgo.When("running e2e tests against a real Zulip server", func() {
		ginkgo.It("should send a basic text message to a stream", func() {
			serviceURLStr := buildServiceURL()
			if serviceURLStr == "" {
				ginkgo.Skip("Zulip server not configured, skipping basic message test")
			}

			serviceURL, err := url.Parse(serviceURLStr)
			gomega.Expect(err).NotTo(gomega.HaveOccurred())

			service := &zulip.Service{}
			err = service.Initialize(serviceURL, testutils.TestLogger())
			gomega.Expect(err).NotTo(gomega.HaveOccurred())

			err = service.Send("E2E Test: Basic text notification", nil)
			gomega.Expect(err).NotTo(gomega.HaveOccurred())
		})

		ginkgo.It("should send a message with a custom topic via params", func() {
			serviceURLStr := buildServiceURL()
			if serviceURLStr == "" {
				ginkgo.Skip("Zulip server not configured, skipping topic test")
			}

			serviceURL, err := url.Parse(serviceURLStr)
			gomega.Expect(err).NotTo(gomega.HaveOccurred())

			service := &zulip.Service{}
			err = service.Initialize(serviceURL, testutils.TestLogger())
			gomega.Expect(err).NotTo(gomega.HaveOccurred())

			err = service.Send("E2E Test: Custom topic notification", &types.Params{
				"topic": "e2e-testing",
			})
			gomega.Expect(err).NotTo(gomega.HaveOccurred())
		})

		ginkgo.It("should send a message with stream via params override", func() {
			serviceURLStr := buildServiceURL()
			if serviceURLStr == "" {
				ginkgo.Skip("Zulip server not configured, skipping stream params test")
			}

			serviceURL, err := url.Parse(serviceURLStr)
			gomega.Expect(err).NotTo(gomega.HaveOccurred())

			service := &zulip.Service{}
			err = service.Initialize(serviceURL, testutils.TestLogger())
			gomega.Expect(err).NotTo(gomega.HaveOccurred())

			// Override stream via params using the same stream from config
			stream := os.Getenv("SHOUTRRR_ZULIP_STREAM")
			if stream == "" {
				stream = "general"
			}

			err = service.Send("E2E Test: Stream via params notification", &types.Params{
				"stream": stream,
			})
			gomega.Expect(err).NotTo(gomega.HaveOccurred())
		})
	})

	ginkgo.When("no server is configured", func() {
		ginkgo.It("should return the correct service ID", func() {
			service := &zulip.Service{}
			gomega.Expect(service.GetID()).To(gomega.Equal("zulip"))
		})
	})
})
