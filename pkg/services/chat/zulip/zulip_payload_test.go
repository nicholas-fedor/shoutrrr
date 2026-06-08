package zulip

import (
	"github.com/onsi/gomega"

	ginkgo "github.com/onsi/ginkgo/v2"
)

var _ = ginkgo.Describe("CreatePayload", func() {
	ginkgo.It("should create payload with type set to channel by default", func() {
		cfg := &Config{
			Stream: "general",
		}

		payload := CreatePayload(cfg, "test message")

		gomega.Expect(payload.Get("type")).To(gomega.Equal("channel"))
	})

	ginkgo.It("should set the to field from config stream for channel", func() {
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

		gomega.Expect(payload.Get("type")).To(gomega.Equal("channel"))
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

		gomega.Expect(payload.Encode()).To(gomega.ContainSubstring("type=channel"))
		gomega.Expect(payload.Encode()).To(gomega.ContainSubstring("to=general"))
		gomega.Expect(payload.Encode()).To(gomega.ContainSubstring("content=test+message"))
		gomega.Expect(payload.Encode()).To(gomega.ContainSubstring("topic=announcements"))
	})

	ginkgo.It("should use type=direct and JSON array for to when direct message", func() {
		cfg := &Config{
			Type: MessageTypeDirect,
			To:   "user1@example.com,user2@example.com",
		}

		payload := CreatePayload(cfg, "test message")

		gomega.Expect(payload.Get("type")).To(gomega.Equal("direct"))
		gomega.Expect(payload.Get("to")).To(gomega.Equal(`["user1@example.com","user2@example.com"]`))
		gomega.Expect(payload.Get("content")).To(gomega.Equal("test message"))
	})

	ginkgo.It("should fall back to stream for direct message recipients if to is empty", func() {
		cfg := &Config{
			Type:   MessageTypeDirect,
			Stream: "user@example.com",
		}

		payload := CreatePayload(cfg, "test message")

		gomega.Expect(payload.Get("type")).To(gomega.Equal("direct"))
		gomega.Expect(payload.Get("to")).To(gomega.Equal(`["user@example.com"]`))
	})

	ginkgo.It("should omit topic for direct messages even when set in config", func() {
		cfg := &Config{
			Type:  MessageTypeDirect,
			To:    "user@example.com",
			Topic: "should be omitted",
		}

		payload := CreatePayload(cfg, "test message")

		gomega.Expect(payload.Get("topic")).To(gomega.BeEmpty())
	})

	ginkgo.It("should include read_by_sender when true", func() {
		cfg := &Config{
			Stream:       "general",
			ReadBySender: true,
		}

		payload := CreatePayload(cfg, "test message")

		gomega.Expect(payload.Get("read_by_sender")).To(gomega.Equal("true"))
	})

	ginkgo.It("should not include read_by_sender when false", func() {
		cfg := &Config{
			Stream:       "general",
			ReadBySender: false,
		}

		payload := CreatePayload(cfg, "test message")

		gomega.Expect(payload.Get("read_by_sender")).To(gomega.BeEmpty())
	})

	ginkgo.It("should use explicit channel type", func() {
		cfg := &Config{
			Type:   MessageTypeChannel,
			Stream: "foo",
		}

		payload := CreatePayload(cfg, "test message")

		gomega.Expect(payload.Get("type")).To(gomega.Equal("channel"))
		gomega.Expect(payload.Get("to")).To(gomega.Equal("foo"))
	})

	ginkgo.It("should serialize numeric recipient IDs as JSON strings for direct messages", func() {
		cfg := &Config{
			Type: MessageTypeDirect,
			To:   "123,456",
		}

		payload := CreatePayload(cfg, "test message")

		gomega.Expect(payload.Get("type")).To(gomega.Equal("direct"))
		gomega.Expect(payload.Get("to")).To(gomega.Equal(`["123","456"]`))
	})

	ginkgo.It("should fall back to stream for numeric recipient IDs when to is empty", func() {
		cfg := &Config{
			Type:   MessageTypeDirect,
			Stream: "789",
		}

		payload := CreatePayload(cfg, "test message")

		gomega.Expect(payload.Get("type")).To(gomega.Equal("direct"))
		gomega.Expect(payload.Get("to")).To(gomega.Equal(`["789"]`))
	})
})

var _ = ginkgo.Describe("ValidateMessageType", func() {
	ginkgo.It("should accept channel type", func() {
		gomega.Expect(ValidateMessageType(MessageTypeChannel)).To(gomega.Succeed())
	})

	ginkgo.It("should accept direct type", func() {
		gomega.Expect(ValidateMessageType(MessageTypeDirect)).To(gomega.Succeed())
	})

	ginkgo.It("should accept empty type", func() {
		gomega.Expect(ValidateMessageType(MessageType(""))).To(gomega.Succeed())
	})

	ginkgo.It("should reject invalid type", func() {
		err := ValidateMessageType(MessageType("invalid"))
		gomega.Expect(err).To(gomega.HaveOccurred())
		gomega.Expect(err.Error()).To(gomega.ContainSubstring("invalid message type"))
	})
})
