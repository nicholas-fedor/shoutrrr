package zulip

import (
	"github.com/onsi/gomega"

	ginkgo "github.com/onsi/ginkgo/v2"
)

var _ = ginkgo.Describe("CreatePayload", func() {
	ginkgo.It("should create payload with type set to stream", func() {
		cfg := &Config{
			Stream: "general",
		}

		payload := CreatePayload(cfg, "test message")

		gomega.Expect(payload.Get("type")).To(gomega.Equal("stream"))
	})

	ginkgo.It("should set the to field from config stream", func() {
		cfg := &Config{
			Stream: "general",
		}

		payload := CreatePayload(cfg, "test message")

		gomega.Expect(payload.Get("to")).To(gomega.Equal("general"))
	})

	ginkgo.It("should set the content field from the message", func() {
		cfg := &Config{
			Stream: "general",
		}

		payload := CreatePayload(cfg, "Hello, World!")

		gomega.Expect(payload.Get("content")).To(gomega.Equal("Hello, World!"))
	})

	ginkgo.It("should include topic when config topic is set", func() {
		cfg := &Config{
			Stream: "general",
			Topic:  "announcements",
		}

		payload := CreatePayload(cfg, "test message")

		gomega.Expect(payload.Get("topic")).To(gomega.Equal("announcements"))
	})

	ginkgo.It("should omit topic when config topic is empty", func() {
		cfg := &Config{
			Stream: "general",
		}

		payload := CreatePayload(cfg, "test message")

		gomega.Expect(payload.Get("topic")).To(gomega.BeEmpty())
	})

	ginkgo.It("should handle empty stream name", func() {
		cfg := &Config{}

		payload := CreatePayload(cfg, "test message")

		gomega.Expect(payload.Get("type")).To(gomega.Equal("stream"))
		gomega.Expect(payload.Get("to")).To(gomega.BeEmpty())
		gomega.Expect(payload.Get("content")).To(gomega.Equal("test message"))
	})

	ginkgo.It("should handle empty message", func() {
		cfg := &Config{
			Stream: "general",
		}

		payload := CreatePayload(cfg, "")

		gomega.Expect(payload.Get("content")).To(gomega.BeEmpty())
	})

	ginkgo.It("should produce url.Values that can be encoded", func() {
		cfg := &Config{
			Stream: "general",
			Topic:  "announcements",
		}

		payload := CreatePayload(cfg, "test message")

		gomega.Expect(payload.Encode()).To(gomega.ContainSubstring("type=stream"))
		gomega.Expect(payload.Encode()).To(gomega.ContainSubstring("to=general"))
		gomega.Expect(payload.Encode()).To(gomega.ContainSubstring("content=test+message"))
		gomega.Expect(payload.Encode()).To(gomega.ContainSubstring("topic=announcements"))
	})
})
