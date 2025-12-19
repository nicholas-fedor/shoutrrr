package discord_test

import (
	"fmt"
	"strings"
	"time"

	"github.com/jarcoal/httpmock"

	ginkgo "github.com/onsi/ginkgo/v2"
	gomega "github.com/onsi/gomega"

	"github.com/nicholas-fedor/shoutrrr/pkg/services/chat/discord"
	"github.com/nicholas-fedor/shoutrrr/pkg/types"
)

var _ = ginkgo.Describe("Embeds", func() {
	var testService *discord.Service
	var dummyConfig discord.Config

	ginkgo.BeforeEach(func() {
		httpmock.Activate()
		dummyConfig = CreateDummyConfig()
		testService = CreateTestService(dummyConfig)
	})

	ginkgo.AfterEach(func() {
		httpmock.DeactivateAndReset()
	})

	ginkgo.Context("enhanced embed features", func() {
		ginkgo.It("should send an embed with default color", func() {
			SetupMockResponder(&dummyConfig, 204)
			items := CreateMessageItemWithLevel(
				"This is a test message",
				types.Info, // Use Info level to force embed creation
			)
			err := testService.SendItems(items, nil)
			gomega.Expect(err).NotTo(gomega.HaveOccurred())
			gomega.Expect(httpmock.GetTotalCallCount()).To(gomega.Equal(1))
		})

		ginkgo.It("should send embeds with different message levels and colors", func() {
			testCases := []struct {
				level types.MessageLevel
				text  string
			}{
				{types.Error, "Error message"},
				{types.Warning, "Warning message"},
				{types.Info, "Info message"},
				{types.Debug, "Debug message"},
			}

			for _, tc := range testCases {
				SetupMockResponder(&dummyConfig, 204)
				items := CreateMessageItemWithLevel(tc.text, tc.level)
				err := testService.SendItems(items, nil)
				gomega.Expect(err).NotTo(gomega.HaveOccurred())
			}
			gomega.Expect(httpmock.GetTotalCallCount()).To(gomega.Equal(4))
		})

		ginkgo.It("should send embed with author information", func() {
			SetupMockResponder(&dummyConfig, 204)
			fields := []types.Field{
				{Key: "embed_author_name", Value: "Test Author"},
				{Key: "embed_author_url", Value: "https://example.com"},
				{Key: "embed_author_icon_url", Value: "https://example.com/icon.png"},
			}
			items := CreateMessageItemWithFields("Message with author", fields)
			err := testService.SendItems(items, nil)
			gomega.Expect(err).NotTo(gomega.HaveOccurred())
			gomega.Expect(httpmock.GetTotalCallCount()).To(gomega.Equal(1))
		})

		ginkgo.It("should send embed with image and thumbnail", func() {
			SetupMockResponder(&dummyConfig, 204)
			fields := []types.Field{
				{Key: "embed_image_url", Value: "https://example.com/image.png"},
				{Key: "embed_thumbnail_url", Value: "https://example.com/thumb.png"},
			}
			items := CreateMessageItemWithFields("Message with media", fields)
			err := testService.SendItems(items, nil)
			gomega.Expect(err).NotTo(gomega.HaveOccurred())
			gomega.Expect(httpmock.GetTotalCallCount()).To(gomega.Equal(1))
		})

		ginkgo.It("should send embed with custom fields array", func() {
			SetupMockResponder(&dummyConfig, 204)
			fields := []types.Field{
				{Key: "Status", Value: "Active"},
				{Key: "Version", Value: "1.0.0"},
				{Key: "Environment", Value: "Production"},
				{Key: "Priority", Value: "High"},
			}
			items := CreateMessageItemWithFields(
				"Message with custom fields",
				fields,
			)
			err := testService.SendItems(items, nil)
			gomega.Expect(err).NotTo(gomega.HaveOccurred())
			gomega.Expect(httpmock.GetTotalCallCount()).To(gomega.Equal(1))
		})

		ginkgo.It("should send embed with timestamp", func() {
			SetupMockResponder(&dummyConfig, 204)
			items := []types.MessageItem{
				{
					Text:      "Message with timestamp",
					Timestamp: time.Now(),
				},
			}
			err := testService.SendItems(items, nil)
			gomega.Expect(err).NotTo(gomega.HaveOccurred())
			gomega.Expect(httpmock.GetTotalCallCount()).To(gomega.Equal(1))
		})

		ginkgo.It("should send multiple embeds with different features", func() {
			SetupMockResponder(&dummyConfig, 204)
			items := []types.MessageItem{
				{
					Text:  "First embed with author",
					Level: types.Info,
					Fields: []types.Field{
						{Key: "embed_author_name", Value: "Bot"},
						{Key: "Status", Value: "OK"},
					},
				},
				{
					Text:  "Second embed with image",
					Level: types.Warning,
					Fields: []types.Field{
						{Key: "embed_image_url", Value: "https://example.com/alert.png"},
						{Key: "Severity", Value: "Medium"},
					},
				},
			}
			err := testService.SendItems(items, nil)
			gomega.Expect(err).NotTo(gomega.HaveOccurred())
			gomega.Expect(httpmock.GetTotalCallCount()).To(gomega.Equal(1))
		})
	})

	ginkgo.Context("embed edge cases", func() {
		ginkgo.It("should handle embeds with empty descriptions", func() {
			items := []types.MessageItem{
				{
					Text:  "",
					Level: types.Info,
				},
			}
			SetupMockResponder(&dummyConfig, 204)
			err := testService.SendItems(items, nil)
			gomega.Expect(err).NotTo(gomega.HaveOccurred())
			gomega.Expect(httpmock.GetTotalCallCount()).To(gomega.Equal(1))
		})

		ginkgo.It("should handle embeds with very long field values", func() {
			longValue := strings.Repeat("A", 1024)
			items := []types.MessageItem{
				{
					Text: "Message with long field value",
					Fields: []types.Field{
						{Key: "long_field", Value: longValue},
					},
				},
			}
			SetupMockResponder(&dummyConfig, 204)
			err := testService.SendItems(items, nil)
			gomega.Expect(err).NotTo(gomega.HaveOccurred())
			gomega.Expect(httpmock.GetTotalCallCount()).To(gomega.Equal(1))
		})

		ginkgo.It("should handle embeds with many fields", func() {
			fields := make([]types.Field, 25) // More than Discord's typical limit
			for i := range fields {
				fields[i] = types.Field{
					Key:   fmt.Sprintf("field_%d", i),
					Value: fmt.Sprintf("value_%d", i),
				}
			}

			items := []types.MessageItem{
				{
					Text:   "Message with many fields",
					Fields: fields,
				},
			}
			SetupMockResponder(&dummyConfig, 204)
			err := testService.SendItems(items, nil)
			gomega.Expect(err).NotTo(gomega.HaveOccurred())
			gomega.Expect(httpmock.GetTotalCallCount()).To(gomega.Equal(1))
		})
	})
})
