package e2e_test

import (
	"context"
	"net/url"
	"strings"
	"time"

	"github.com/onsi/ginkgo/v2"
	"github.com/onsi/gomega"

	"github.com/nicholas-fedor/shoutrrr/internal/testutils"
	"github.com/nicholas-fedor/shoutrrr/pkg/services/chat/zulip"
	"github.com/nicholas-fedor/shoutrrr/pkg/types"
)

var _ = ginkgo.Describe("Zulip E2E Error Handling", func() {
	ginkgo.When("the server is unreachable", func() {
		ginkgo.It("should fail gracefully with a connection error", func() {
			serviceURL, err := url.Parse("zulip://bot@example.com:secret-key@10.255.255.1:44399")
			gomega.Expect(err).NotTo(gomega.HaveOccurred())

			service := &zulip.Service{}
			err = service.Initialize(serviceURL, testutils.TestLogger())
			gomega.Expect(err).NotTo(gomega.HaveOccurred())

			sendCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
			defer cancel()

			err = service.SendWithContext(sendCtx, "E2E Test: unreachable", nil)
			gomega.Expect(err).To(gomega.HaveOccurred())
		})
	})

	ginkgo.When("sending with context cancellation", func() {
		ginkgo.It("should abort when context is already canceled", func() {
			serviceURL, err := url.Parse("zulip://bot@example.com:secret-key@10.255.255.1:44399")
			gomega.Expect(err).NotTo(gomega.HaveOccurred())

			service := &zulip.Service{}
			err = service.Initialize(serviceURL, testutils.TestLogger())
			gomega.Expect(err).NotTo(gomega.HaveOccurred())

			ctx, cancel := context.WithCancel(context.Background())
			cancel()

			err = service.SendWithContext(ctx, "E2E Test: canceled", nil)
			gomega.Expect(err).To(gomega.HaveOccurred())
		})
	})

	ginkgo.When("sending with invalid credentials", func() {
		ginkgo.It("should fail with authentication error from a valid server", func() {
			serviceURLStr := buildServiceURL()
			if serviceURLStr == "" {
				ginkgo.Skip("Zulip server not configured, skipping auth error test")

				return
			}

			parsedURL, err := url.Parse(serviceURLStr)
			gomega.Expect(err).NotTo(gomega.HaveOccurred())

			parsedURL.User = url.UserPassword("bot@example.com", "wrong-key-invalid-key")

			service := &zulip.Service{}
			err = service.Initialize(parsedURL, testutils.TestLogger())
			gomega.Expect(err).NotTo(gomega.HaveOccurred())

			err = service.Send("E2E Test: invalid credentials", nil)
			gomega.Expect(err).To(gomega.HaveOccurred())
			gomega.Expect(err.Error()).To(gomega.ContainSubstring("401"))
		})
	})

	ginkgo.When("sending with a non-existent stream", func() {
		ginkgo.It("should fail with error for invalid stream", func() {
			serviceURLStr := buildServiceURL()
			if serviceURLStr == "" {
				ginkgo.Skip("Zulip server not configured, skipping invalid stream test")

				return
			}

			serviceURL, err := url.Parse(serviceURLStr)
			gomega.Expect(err).NotTo(gomega.HaveOccurred())

			service := &zulip.Service{}
			err = service.Initialize(serviceURL, testutils.TestLogger())
			gomega.Expect(err).NotTo(gomega.HaveOccurred())

			params := &types.Params{
				"stream": "this-stream-definitely-does-not-exist-" + strings.Repeat("x", 20),
			}

			err = service.Send("E2E Test: invalid stream", params)
			gomega.Expect(err).To(gomega.HaveOccurred())
		})
	})
})
