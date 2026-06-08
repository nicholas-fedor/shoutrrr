package e2e_test

import (
	"net/url"
	"strings"

	"github.com/onsi/ginkgo/v2"
	"github.com/onsi/gomega"

	"github.com/nicholas-fedor/shoutrrr/internal/testutils"
	"github.com/nicholas-fedor/shoutrrr/pkg/services/chat/zulip"
	"github.com/nicholas-fedor/shoutrrr/pkg/types"
)

var _ = ginkgo.Describe("Zulip E2E Title and Topic Handling", func() {
	ginkgo.When("running e2e tests against a real Zulip server", func() {
		ginkgo.It("should use title as topic when no topic is configured", func() {
			serviceURLStr := buildServiceURL()
			if serviceURLStr == "" {
				ginkgo.Skip("Zulip server not configured, skipping title-as-topic test")
			}

			serviceURL, err := url.Parse(serviceURLStr)
			gomega.Expect(err).NotTo(gomega.HaveOccurred())

			service := &zulip.Service{}
			err = service.Initialize(serviceURL, testutils.TestLogger())
			gomega.Expect(err).NotTo(gomega.HaveOccurred())

			err = service.Send("E2E Test: title used as topic", &types.Params{
				"title": "e2e-title-as-topic",
			})
			gomega.Expect(err).NotTo(gomega.HaveOccurred())
		})

		ginkgo.It("should preserve explicit topic when title is also provided", func() {
			serviceURLStr := buildServiceURL()
			if serviceURLStr == "" {
				ginkgo.Skip("Zulip server not configured, skipping topic-preservation test")
			}

			serviceURL, err := url.Parse(serviceURLStr)
			gomega.Expect(err).NotTo(gomega.HaveOccurred())

			service := &zulip.Service{}
			err = service.Initialize(serviceURL, testutils.TestLogger())
			gomega.Expect(err).NotTo(gomega.HaveOccurred())

			err = service.Send("E2E Test: body content", &types.Params{
				"topic": "e2e-explicit-topic",
				"title": "E2E Notification Title",
			})
			gomega.Expect(err).NotTo(gomega.HaveOccurred())
		})

		ginkgo.It("should send a message with a long title used as topic", func() {
			serviceURLStr := buildServiceURL()
			if serviceURLStr == "" {
				ginkgo.Skip("Zulip server not configured, skipping long title test")
			}

			serviceURL, err := url.Parse(serviceURLStr)
			gomega.Expect(err).NotTo(gomega.HaveOccurred())

			service := &zulip.Service{}
			err = service.Initialize(serviceURL, testutils.TestLogger())
			gomega.Expect(err).NotTo(gomega.HaveOccurred())

			longTitle := strings.Repeat("x", 50)
			err = service.Send("E2E Test: long title as topic", &types.Params{
				"title": longTitle,
			})
			gomega.Expect(err).NotTo(gomega.HaveOccurred())
		})

		ginkgo.It("should send message with topic only and no title", func() {
			serviceURLStr := buildServiceURL()
			if serviceURLStr == "" {
				ginkgo.Skip("Zulip server not configured, skipping topic-only test")
			}

			serviceURL, err := url.Parse(serviceURLStr)
			gomega.Expect(err).NotTo(gomega.HaveOccurred())

			service := &zulip.Service{}
			err = service.Initialize(serviceURL, testutils.TestLogger())
			gomega.Expect(err).NotTo(gomega.HaveOccurred())

			err = service.Send("E2E Test: topic only", &types.Params{
				"topic": "e2e-topic-only",
			})
			gomega.Expect(err).NotTo(gomega.HaveOccurred())
		})

		ginkgo.It("should send message with title only and no topic", func() {
			serviceURLStr := buildServiceURL()
			if serviceURLStr == "" {
				ginkgo.Skip("Zulip server not configured, skipping title-only test")
			}

			serviceURL, err := url.Parse(serviceURLStr)
			gomega.Expect(err).NotTo(gomega.HaveOccurred())

			service := &zulip.Service{}
			err = service.Initialize(serviceURL, testutils.TestLogger())
			gomega.Expect(err).NotTo(gomega.HaveOccurred())

			err = service.Send("E2E Test: title only", &types.Params{
				"title": "e2e-title-only",
			})
			gomega.Expect(err).NotTo(gomega.HaveOccurred())
		})
	})
})
