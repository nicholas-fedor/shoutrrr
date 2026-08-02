package e2e_test

import (
	"os"

	"github.com/onsi/ginkgo/v2"
	"github.com/onsi/gomega"

	"github.com/nicholas-fedor/shoutrrr/pkg/services/push/ntfy"
)

var _ = ginkgo.Describe("ntfy E2E Config Tests", func() {
	ginkgo.When("running e2e tests against a real ntfy server", func() {
		ginkgo.BeforeEach(func() {
			if !isNtfyServerAvailable() {
				ginkgo.Skip("ntfy server not available, skipping e2e tests")
			}
		})

		ginkgo.It("should initialize with default configuration", func() {
			serviceURLStr := buildServiceURL()
			service := initializeService(serviceURLStr)

			gomega.Expect(service.Config.Topic).NotTo(gomega.BeEmpty())
			gomega.Expect(service.Config.Host).NotTo(gomega.BeEmpty())
		})

		ginkgo.It("should initialize with credentials from URL", func() {
			serviceURLStr := buildServiceURL()
			service := initializeService(serviceURLStr)

			username := os.Getenv("SHOUTRRR_NTFY_USERNAME")
			password := os.Getenv("SHOUTRRR_NTFY_PASSWORD")

			if username != "" {
				gomega.Expect(service.Config.Username).To(gomega.Equal(username))
			}

			if password != "" {
				gomega.Expect(service.Config.Password).To(gomega.Equal(password))
			}
		})

		ginkgo.It("should initialize with DisableTLS when configured", func() {
			previous, hadPrevious := os.LookupEnv("SHOUTRRR_NTFY_DISABLE_TLS")
			_ = os.Setenv("SHOUTRRR_NTFY_DISABLE_TLS", "true")

			defer func() {
				if hadPrevious {
					_ = os.Setenv("SHOUTRRR_NTFY_DISABLE_TLS", previous)
				} else {
					_ = os.Unsetenv("SHOUTRRR_NTFY_DISABLE_TLS")
				}
			}()

			serviceURLStr := buildServiceURL()
			service := initializeService(serviceURLStr)

			gomega.Expect(service.Config.DisableTLS).To(gomega.BeTrue())
			gomega.Expect(service.Config.Scheme).To(gomega.Equal("http"))
		})

		ginkgo.It("should initialize with priority from URL", func() {
			serviceURLStr := addQueryParam(buildServiceURL(), "priority", "Max")
			service := initializeService(serviceURLStr)

			gomega.Expect(service.Config.Priority.String()).To(gomega.Equal("Max"))
		})

		ginkgo.It("should initialize with markdown enabled from URL", func() {
			serviceURLStr := addQueryParam(buildServiceURL(), "markdown", "yes")
			service := initializeService(serviceURLStr)

			gomega.Expect(service.Config.Markdown).To(gomega.BeTrue())
		})

		ginkgo.It("should initialize with tags from URL", func() {
			serviceURLStr := addQueryParam(buildServiceURL(), "tags", "warning,skull")
			service := initializeService(serviceURLStr)

			gomega.Expect(service.Config.Tags).To(gomega.ConsistOf("warning", "skull"))
		})

		ginkgo.It("should initialize with title from URL", func() {
			serviceURLStr := addQueryParam(buildServiceURL(), "title", "MyTitle")
			service := initializeService(serviceURLStr)

			gomega.Expect(service.Config.Title).To(gomega.Equal("MyTitle"))
		})
	})

	ginkgo.When("no server is configured", func() {
		ginkgo.It("should return the correct service ID", func() {
			service := &ntfy.Service{}
			gomega.Expect(service.GetID()).To(gomega.Equal("ntfy"))
		})
	})
})
