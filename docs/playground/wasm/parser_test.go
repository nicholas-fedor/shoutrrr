package main

import (
	"encoding/json"

	"github.com/onsi/ginkgo/v2"
	"github.com/onsi/gomega"
)

var _ = ginkgo.Describe("Parser", func() {
	ginkgo.Describe("parseURLString", func() {
		ginkgo.It("parses discord URL", func() {
			result := parseURLString("discord://token@123456789")

			var parsed parseResult

			err := json.Unmarshal([]byte(result), &parsed)
			gomega.Expect(err).ToNot(gomega.HaveOccurred())
			gomega.Expect(parsed.Service).To(gomega.Equal("discord"))
			gomega.Expect(parsed.Config).ToNot(gomega.BeEmpty())
		})

		ginkgo.It("parses ntfy URL", func() {
			result := parseURLString("ntfy://ntfy.sh/mytopic")

			var parsed parseResult

			err := json.Unmarshal([]byte(result), &parsed)
			gomega.Expect(err).ToNot(gomega.HaveOccurred())
			gomega.Expect(parsed.Service).To(gomega.Equal("ntfy"))
		})

		ginkgo.It("parses generic URL with webhook", func() {
			result := parseURLString("generic://192.168.1.100:8123/api/webhook/abc123")

			var parsed parseResult

			err := json.Unmarshal([]byte(result), &parsed)
			gomega.Expect(err).ToNot(gomega.HaveOccurred())
			gomega.Expect(parsed.Service).To(gomega.Equal("generic"))
			gomega.Expect(parsed.Config["WebhookURL"]).To(gomega.ContainSubstring("192.168.1.100"))
		})

		ginkgo.It("returns error for invalid URL", func() {
			result := parseURLString("not-a-valid-url")

			var errResp errorResult

			err := json.Unmarshal([]byte(result), &errResp)
			gomega.Expect(err).ToNot(gomega.HaveOccurred())
			gomega.Expect(errResp.Error).ToNot(gomega.BeEmpty())
		})

		ginkgo.It("handles URL with query parameters", func() {
			result := parseURLString("discord://token@webhook?color=0x50D9ff&splitlines=Yes")

			var parsed parseResult

			err := json.Unmarshal([]byte(result), &parsed)
			gomega.Expect(err).ToNot(gomega.HaveOccurred())
			gomega.Expect(parsed.Config).To(gomega.HaveKeyWithValue("Color", "0x50D9ff"))
			gomega.Expect(parsed.Config).To(gomega.HaveKeyWithValue("SplitLines", "Yes"))
		})
	})

	ginkgo.Describe("validateURLString", func() {
		ginkgo.It("returns valid for discord URL", func() {
			result := validateURLString("discord://token@123456789")

			var valid map[string]bool

			err := json.Unmarshal([]byte(result), &valid)
			gomega.Expect(err).ToNot(gomega.HaveOccurred())
			gomega.Expect(valid["valid"]).To(gomega.BeTrue())
		})

		ginkgo.It("returns valid for ntfy URL", func() {
			result := validateURLString("ntfy://ntfy.sh/mytopic")

			var valid map[string]bool

			err := json.Unmarshal([]byte(result), &valid)
			gomega.Expect(err).ToNot(gomega.HaveOccurred())
			gomega.Expect(valid["valid"]).To(gomega.BeTrue())
		})

		ginkgo.It("returns error for invalid URL", func() {
			result := validateURLString("not-valid")

			var errResp errorResult

			err := json.Unmarshal([]byte(result), &errResp)
			gomega.Expect(err).ToNot(gomega.HaveOccurred())
			gomega.Expect(errResp.Error).ToNot(gomega.BeEmpty())
		})

		ginkgo.It("returns error for unknown service", func() {
			result := validateURLString("unknown://something")

			var errResp errorResult

			err := json.Unmarshal([]byte(result), &errResp)
			gomega.Expect(err).ToNot(gomega.HaveOccurred())
			gomega.Expect(errResp.Error).ToNot(gomega.BeEmpty())
		})
	})
})
