package e2e_test

import (
	"net/url"
	"strings"

	"github.com/onsi/ginkgo/v2"
	"github.com/onsi/gomega"

	"github.com/nicholas-fedor/shoutrrr/internal/testutils"
	"github.com/nicholas-fedor/shoutrrr/pkg/services/chat/zulip"
)

var _ = ginkgo.Describe("Zulip E2E Message Types", func() {
	ginkgo.When("sending different message types", func() {
		ginkgo.It("should send a long message", func() {
			serviceURLStr := buildServiceURL()
			if serviceURLStr == "" {
				ginkgo.Skip("Zulip server not configured, skipping long message test")
			}

			serviceURL, err := url.Parse(serviceURLStr)
			gomega.Expect(err).NotTo(gomega.HaveOccurred())

			service := &zulip.Service{}
			err = service.Initialize(serviceURL, testutils.TestLogger())
			gomega.Expect(err).NotTo(gomega.HaveOccurred())

			longMessage := strings.Repeat("This is a long message. ", 100)
			err = service.Send(longMessage, nil)
			gomega.Expect(err).NotTo(gomega.HaveOccurred())
		})

		ginkgo.It("should send a message with unicode characters", func() {
			serviceURLStr := buildServiceURL()
			if serviceURLStr == "" {
				ginkgo.Skip("Zulip server not configured, skipping unicode test")
			}

			serviceURL, err := url.Parse(serviceURLStr)
			gomega.Expect(err).NotTo(gomega.HaveOccurred())

			service := &zulip.Service{}
			err = service.Initialize(serviceURL, testutils.TestLogger())
			gomega.Expect(err).NotTo(gomega.HaveOccurred())

			err = service.Send("E2E Test: Unicode — 日本語 🎉 émojis ñ", nil) //nolint:gosmopolitan // unicode test message
			gomega.Expect(err).NotTo(gomega.HaveOccurred())
		})

		ginkgo.It("should send a message with special characters", func() {
			serviceURLStr := buildServiceURL()
			if serviceURLStr == "" {
				ginkgo.Skip("Zulip server not configured, skipping special chars test")
			}

			serviceURL, err := url.Parse(serviceURLStr)
			gomega.Expect(err).NotTo(gomega.HaveOccurred())

			service := &zulip.Service{}
			err = service.Initialize(serviceURL, testutils.TestLogger())
			gomega.Expect(err).NotTo(gomega.HaveOccurred())

			err = service.Send("E2E Test: Special <>&\"' \t\n\r characters", nil)
			gomega.Expect(err).NotTo(gomega.HaveOccurred())
		})

		ginkgo.It("should send a message with markdown formatting", func() {
			serviceURLStr := buildServiceURL()
			if serviceURLStr == "" {
				ginkgo.Skip("Zulip server not configured, skipping markdown test")
			}

			serviceURL, err := url.Parse(serviceURLStr)
			gomega.Expect(err).NotTo(gomega.HaveOccurred())

			service := &zulip.Service{}
			err = service.Initialize(serviceURL, testutils.TestLogger())
			gomega.Expect(err).NotTo(gomega.HaveOccurred())

			markdown := `**Bold text** and *italic text* and ~~strikethrough~~` + "\n" +
				"```go\npackage main\n```" + "\n" +
				"- List item 1\n- List item 2\n" + "\n" +
				"[Link](https://example.com)"

			err = service.Send(markdown, nil)
			gomega.Expect(err).NotTo(gomega.HaveOccurred())
		})

		ginkgo.It("should return an error for an empty message", func() {
			serviceURLStr := buildServiceURL()
			if serviceURLStr == "" {
				ginkgo.Skip("Zulip server not configured, skipping empty message test")
			}

			serviceURL, err := url.Parse(serviceURLStr)
			gomega.Expect(err).NotTo(gomega.HaveOccurred())

			service := &zulip.Service{}
			err = service.Initialize(serviceURL, testutils.TestLogger())
			gomega.Expect(err).NotTo(gomega.HaveOccurred())

			err = service.Send("", nil)
			gomega.Expect(err).To(gomega.HaveOccurred())
		})
	})
})
