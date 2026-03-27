package main

import (
	"encoding/json"

	"github.com/onsi/ginkgo/v2"
	"github.com/onsi/gomega"

	"github.com/nicholas-fedor/shoutrrr/pkg/router"
)

var _ = ginkgo.Describe("Generator", func() {
	ginkgo.Describe("generateURLString", func() {
		ginkgo.It("generates discord URL with webhook and token", func() {
			configJSON := `{"WebhookID":"123456789","Token":"mytoken"}`
			result := generateURLString("discord", configJSON)

			var parsed map[string]string

			err := json.Unmarshal([]byte(result), &parsed)
			gomega.Expect(err).ToNot(gomega.HaveOccurred())
			gomega.Expect(parsed["url"]).To(gomega.ContainSubstring("discord://"))
			gomega.Expect(parsed["url"]).To(gomega.ContainSubstring("123456789"))
			gomega.Expect(parsed["url"]).To(gomega.ContainSubstring("mytoken"))
		})

		ginkgo.It("generates ntfy URL with host and path", func() {
			configJSON := `{"Host":"ntfy.sh","Path":"mytopic"}`
			result := generateURLString("ntfy", configJSON)

			var parsed map[string]string

			err := json.Unmarshal([]byte(result), &parsed)
			gomega.Expect(err).ToNot(gomega.HaveOccurred())
			gomega.Expect(parsed["url"]).To(gomega.ContainSubstring("ntfy://"))
			gomega.Expect(parsed["url"]).To(gomega.ContainSubstring("ntfy.sh"))
		})

		ginkgo.It("generates generic URL with webhook", func() {
			configJSON := `{"WebhookURL":"192.168.1.100:8123/api/webhook/abc123"}`
			result := generateURLString("generic", configJSON)

			var parsed map[string]string

			err := json.Unmarshal([]byte(result), &parsed)
			gomega.Expect(err).ToNot(gomega.HaveOccurred())
			gomega.Expect(parsed["url"]).To(gomega.ContainSubstring("generic://"))
			gomega.Expect(parsed["url"]).To(gomega.ContainSubstring("192.168.1.100"))
		})

		ginkgo.It("returns error for invalid service", func() {
			configJSON := `{}`
			result := generateURLString("nonexistent", configJSON)

			var errResp errorResult

			err := json.Unmarshal([]byte(result), &errResp)
			gomega.Expect(err).ToNot(gomega.HaveOccurred())
			gomega.Expect(errResp.Error).ToNot(gomega.BeEmpty())
		})

		ginkgo.It("returns error for invalid JSON", func() {
			result := generateURLString("discord", "not-json")

			var errResp errorResult

			err := json.Unmarshal([]byte(result), &errResp)
			gomega.Expect(err).ToNot(gomega.HaveOccurred())
			gomega.Expect(errResp.Error).ToNot(gomega.BeEmpty())
		})

		ginkgo.It("generates logger URL", func() {
			configJSON := `{}`
			result := generateURLString("logger", configJSON)

			var parsed map[string]string

			err := json.Unmarshal([]byte(result), &parsed)
			gomega.Expect(err).ToNot(gomega.HaveOccurred())
			gomega.Expect(parsed["url"]).To(gomega.Equal("logger://"))
		})
	})

	ginkgo.Describe("getServiceConfigFromService", func() {
		ginkgo.It("returns no config for newly created service", func() {
			r := router.ServiceRouter{}
			service, err := r.NewService("discord")
			gomega.Expect(err).ToNot(gomega.HaveOccurred())

			config, ok := getServiceConfigFromService(service)
			gomega.Expect(ok).To(gomega.BeFalse())
			gomega.Expect(config).To(gomega.BeNil())
		})
	})
})
