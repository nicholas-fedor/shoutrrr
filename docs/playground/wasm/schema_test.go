package main

import (
	"encoding/json"

	"github.com/onsi/ginkgo/v2"
	"github.com/onsi/gomega"

	"github.com/nicholas-fedor/shoutrrr/pkg/router"
)

var _ = ginkgo.Describe("Schema", func() {
	ginkgo.Describe("listServicesJSON", func() {
		ginkgo.It("returns valid JSON array", func() {
			result := listServicesJSON()

			var schemes []string

			err := json.Unmarshal([]byte(result), &schemes)
			gomega.Expect(err).ToNot(gomega.HaveOccurred())
			gomega.Expect(schemes).ToNot(gomega.BeEmpty())
		})

		ginkgo.It("contains known services", func() {
			result := listServicesJSON()

			var schemes []string

			_ = json.Unmarshal([]byte(result), &schemes)

			gomega.Expect(schemes).To(gomega.ContainElement("discord"))
			gomega.Expect(schemes).To(gomega.ContainElement("slack"))
			gomega.Expect(schemes).To(gomega.ContainElement("ntfy"))
			gomega.Expect(schemes).To(gomega.ContainElement("generic"))
			gomega.Expect(schemes).To(gomega.ContainElement("logger"))
		})

		ginkgo.It("matches router.ListServices", func() {
			r := router.ServiceRouter{}
			expected := r.ListServices()

			result := listServicesJSON()

			var schemes []string

			_ = json.Unmarshal([]byte(result), &schemes)
			gomega.Expect(schemes).To(gomega.HaveLen(len(expected)))
		})
	})

	ginkgo.Describe("configSchemaJSON", func() {
		ginkgo.It("returns valid schema for discord", func() {
			result := configSchemaJSON("discord")

			var schema configSchema

			err := json.Unmarshal([]byte(result), &schema)
			gomega.Expect(err).ToNot(gomega.HaveOccurred())
			gomega.Expect(schema.Service).To(gomega.Equal("discord"))
			gomega.Expect(schema.Scheme).To(gomega.Equal("discord"))
			gomega.Expect(schema.Fields).ToNot(gomega.BeEmpty())
		})

		ginkgo.It("returns valid schema for ntfy", func() {
			result := configSchemaJSON("ntfy")

			var schema configSchema

			err := json.Unmarshal([]byte(result), &schema)
			gomega.Expect(err).ToNot(gomega.HaveOccurred())
			gomega.Expect(schema.Service).To(gomega.Equal("ntfy"))
		})

		ginkgo.It("includes webhookURL field for generic service", func() {
			result := configSchemaJSON("generic")

			var schema configSchema

			_ = json.Unmarshal([]byte(result), &schema)

			var hasWebhookURL bool

			for _, field := range schema.Fields {
				if field.Name == "WebhookURL" {
					hasWebhookURL = true

					gomega.Expect(field.Required).To(gomega.BeTrue())
					gomega.Expect(field.Type).To(gomega.Equal("string"))

					break
				}
			}

			gomega.Expect(hasWebhookURL).To(gomega.BeTrue())
		})

		ginkgo.It("returns error for invalid service", func() {
			result := configSchemaJSON("nonexistent")

			var errResp errorResult

			err := json.Unmarshal([]byte(result), &errResp)
			gomega.Expect(err).ToNot(gomega.HaveOccurred())
			gomega.Expect(errResp.Error).ToNot(gomega.BeEmpty())
		})

		ginkgo.It("handles logger service with no fields", func() {
			result := configSchemaJSON("logger")

			var schema configSchema

			err := json.Unmarshal([]byte(result), &schema)
			gomega.Expect(err).ToNot(gomega.HaveOccurred())
			gomega.Expect(schema.Service).To(gomega.Equal("logger"))
		})
	})
})
