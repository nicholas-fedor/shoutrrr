package discord

import (
	"time"

	"github.com/onsi/ginkgo/v2"
	"github.com/onsi/gomega"

	"github.com/nicholas-fedor/shoutrrr/pkg/types"
)

var _ = ginkgo.Describe("Discord JSON Unit Tests", func() {
	ginkgo.Describe("hasEmbedFields function", func() {
		ginkgo.It("should return false for empty fields", func() {
			fields := []types.Field{}
			result := hasEmbedFields(fields)
			gomega.Expect(result).To(gomega.BeFalse())
		})

		ginkgo.It("should return true for regular fields only", func() {
			fields := []types.Field{
				{Key: "regular_field", Value: "value"},
				{Key: "another_field", Value: "another_value"},
			}
			result := hasEmbedFields(fields)
			gomega.Expect(result).To(gomega.BeTrue())
		})

		ginkgo.It("should return true when embed_author_name is present", func() {
			fields := []types.Field{
				{Key: "regular_field", Value: "value"},
				{Key: "embed_author_name", Value: "Author Name"},
			}
			result := hasEmbedFields(fields)
			gomega.Expect(result).To(gomega.BeTrue())
		})

		ginkgo.It("should return true when embed_author_url is present", func() {
			fields := []types.Field{
				{Key: "embed_author_url", Value: "https://example.com"},
			}
			result := hasEmbedFields(fields)
			gomega.Expect(result).To(gomega.BeTrue())
		})

		ginkgo.It("should return true when embed_author_icon_url is present", func() {
			fields := []types.Field{
				{Key: "embed_author_icon_url", Value: "https://example.com/icon.png"},
			}
			result := hasEmbedFields(fields)
			gomega.Expect(result).To(gomega.BeTrue())
		})

		ginkgo.It("should return true when embed_image_url is present", func() {
			fields := []types.Field{
				{Key: "embed_image_url", Value: "https://example.com/image.png"},
			}
			result := hasEmbedFields(fields)
			gomega.Expect(result).To(gomega.BeTrue())
		})

		ginkgo.It("should return true when embed_thumbnail_url is present", func() {
			fields := []types.Field{
				{Key: "embed_thumbnail_url", Value: "https://example.com/thumb.png"},
			}
			result := hasEmbedFields(fields)
			gomega.Expect(result).To(gomega.BeTrue())
		})

		ginkgo.It("should return true when multiple embed fields are present", func() {
			fields := []types.Field{
				{Key: "embed_author_name", Value: "Author"},
				{Key: "embed_image_url", Value: "https://example.com/image.png"},
				{Key: "embed_thumbnail_url", Value: "https://example.com/thumb.png"},
			}
			result := hasEmbedFields(fields)
			gomega.Expect(result).To(gomega.BeTrue())
		})
	})

	ginkgo.Describe("processEmbedFields function", func() {
		ginkgo.It("should return nil values for empty fields", func() {
			fields := []types.Field{}
			author, image, thumbnail, embedFields := processEmbedFields(fields)
			gomega.Expect(author).To(gomega.BeNil())
			gomega.Expect(image).To(gomega.BeNil())
			gomega.Expect(thumbnail).To(gomega.BeNil())
			gomega.Expect(embedFields).To(gomega.BeEmpty())
		})

		ginkgo.It("should process regular fields into embed fields", func() {
			fields := []types.Field{
				{Key: "field1", Value: "value1"},
				{Key: "field2", Value: "value2"},
			}
			author, image, thumbnail, embedFields := processEmbedFields(fields)
			gomega.Expect(author).To(gomega.BeNil())
			gomega.Expect(image).To(gomega.BeNil())
			gomega.Expect(thumbnail).To(gomega.BeNil())
			gomega.Expect(embedFields).To(gomega.HaveLen(2))
			gomega.Expect(embedFields[0].Name).To(gomega.Equal("field1"))
			gomega.Expect(embedFields[0].Value).To(gomega.Equal("value1"))
			gomega.Expect(embedFields[1].Name).To(gomega.Equal("field2"))
			gomega.Expect(embedFields[1].Value).To(gomega.Equal("value2"))
		})

		ginkgo.It("should process embed_author_name field", func() {
			fields := []types.Field{
				{Key: "embed_author_name", Value: "Test Author"},
			}
			author, image, thumbnail, embedFields := processEmbedFields(fields)
			gomega.Expect(author).NotTo(gomega.BeNil())
			gomega.Expect(author.Name).To(gomega.Equal("Test Author"))
			gomega.Expect(author.URL).To(gomega.Equal(""))
			gomega.Expect(author.IconURL).To(gomega.Equal(""))
			gomega.Expect(image).To(gomega.BeNil())
			gomega.Expect(thumbnail).To(gomega.BeNil())
			gomega.Expect(embedFields).To(gomega.BeEmpty())
		})

		ginkgo.It("should process embed_author_url field", func() {
			fields := []types.Field{
				{Key: "embed_author_url", Value: "https://example.com"},
			}
			author, image, thumbnail, embedFields := processEmbedFields(fields)
			gomega.Expect(author).NotTo(gomega.BeNil())
			gomega.Expect(author.Name).To(gomega.Equal(""))
			gomega.Expect(author.URL).To(gomega.Equal("https://example.com"))
			gomega.Expect(author.IconURL).To(gomega.Equal(""))
			gomega.Expect(image).To(gomega.BeNil())
			gomega.Expect(thumbnail).To(gomega.BeNil())
			gomega.Expect(embedFields).To(gomega.BeEmpty())
		})

		ginkgo.It("should process embed_author_icon_url field", func() {
			fields := []types.Field{
				{Key: "embed_author_icon_url", Value: "https://example.com/icon.png"},
			}
			author, image, thumbnail, embedFields := processEmbedFields(fields)
			gomega.Expect(author).NotTo(gomega.BeNil())
			gomega.Expect(author.Name).To(gomega.Equal(""))
			gomega.Expect(author.URL).To(gomega.Equal(""))
			gomega.Expect(author.IconURL).To(gomega.Equal("https://example.com/icon.png"))
			gomega.Expect(image).To(gomega.BeNil())
			gomega.Expect(thumbnail).To(gomega.BeNil())
			gomega.Expect(embedFields).To(gomega.BeEmpty())
		})

		ginkgo.It("should process complete embed author with all fields", func() {
			fields := []types.Field{
				{Key: "embed_author_name", Value: "Test Author"},
				{Key: "embed_author_url", Value: "https://example.com"},
				{Key: "embed_author_icon_url", Value: "https://example.com/icon.png"},
			}
			author, image, thumbnail, embedFields := processEmbedFields(fields)
			gomega.Expect(author).NotTo(gomega.BeNil())
			gomega.Expect(author.Name).To(gomega.Equal("Test Author"))
			gomega.Expect(author.URL).To(gomega.Equal("https://example.com"))
			gomega.Expect(author.IconURL).To(gomega.Equal("https://example.com/icon.png"))
			gomega.Expect(image).To(gomega.BeNil())
			gomega.Expect(thumbnail).To(gomega.BeNil())
			gomega.Expect(embedFields).To(gomega.BeEmpty())
		})

		ginkgo.It("should process embed_image_url field", func() {
			fields := []types.Field{
				{Key: "embed_image_url", Value: "https://example.com/image.png"},
			}
			author, image, thumbnail, embedFields := processEmbedFields(fields)
			gomega.Expect(author).To(gomega.BeNil())
			gomega.Expect(image).NotTo(gomega.BeNil())
			gomega.Expect(image.URL).To(gomega.Equal("https://example.com/image.png"))
			gomega.Expect(thumbnail).To(gomega.BeNil())
			gomega.Expect(embedFields).To(gomega.BeEmpty())
		})

		ginkgo.It("should process embed_thumbnail_url field", func() {
			fields := []types.Field{
				{Key: "embed_thumbnail_url", Value: "https://example.com/thumb.png"},
			}
			author, image, thumbnail, embedFields := processEmbedFields(fields)
			gomega.Expect(author).To(gomega.BeNil())
			gomega.Expect(image).To(gomega.BeNil())
			gomega.Expect(thumbnail).NotTo(gomega.BeNil())
			gomega.Expect(thumbnail.URL).To(gomega.Equal("https://example.com/thumb.png"))
			gomega.Expect(embedFields).To(gomega.BeEmpty())
		})

		ginkgo.It("should process mixed embed and regular fields", func() {
			fields := []types.Field{
				{Key: "embed_author_name", Value: "Test Author"},
				{Key: "embed_image_url", Value: "https://example.com/image.png"},
				{Key: "regular_field", Value: "regular_value"},
				{Key: "another_field", Value: "another_value"},
			}
			author, image, thumbnail, embedFields := processEmbedFields(fields)
			gomega.Expect(author).NotTo(gomega.BeNil())
			gomega.Expect(author.Name).To(gomega.Equal("Test Author"))
			gomega.Expect(image).NotTo(gomega.BeNil())
			gomega.Expect(image.URL).To(gomega.Equal("https://example.com/image.png"))
			gomega.Expect(thumbnail).To(gomega.BeNil())
			gomega.Expect(embedFields).To(gomega.HaveLen(2))
			gomega.Expect(embedFields[0].Name).To(gomega.Equal("regular_field"))
			gomega.Expect(embedFields[0].Value).To(gomega.Equal("regular_value"))
			gomega.Expect(embedFields[1].Name).To(gomega.Equal("another_field"))
			gomega.Expect(embedFields[1].Value).To(gomega.Equal("another_value"))
		})

		ginkgo.It("should handle duplicate embed fields by using last value", func() {
			fields := []types.Field{
				{Key: "embed_author_name", Value: "First Author"},
				{Key: "embed_author_name", Value: "Second Author"},
			}
			author, image, thumbnail, embedFields := processEmbedFields(fields)
			gomega.Expect(author).NotTo(gomega.BeNil())
			gomega.Expect(author.Name).To(gomega.Equal("Second Author"))
			gomega.Expect(image).To(gomega.BeNil())
			gomega.Expect(thumbnail).To(gomega.BeNil())
			gomega.Expect(embedFields).To(gomega.BeEmpty())
		})
	})

	ginkgo.Describe("CreatePayloadFromItems function", func() {
		var colors [types.MessageLevelCount]uint

		ginkgo.BeforeEach(func() {
			colors = [types.MessageLevelCount]uint{
				types.Unknown: 0,
				types.Debug:   0x0000ff,
				types.Info:    0x00ff00,
				types.Warning: 0xffff00,
				types.Error:   0xff0000,
			}
		})

		ginkgo.It("should return error for empty items", func() {
			items := []types.MessageItem{}
			_, err := CreatePayloadFromItems(items, "", colors)
			gomega.Expect(err).To(gomega.MatchError(ErrEmptyMessage))
		})

		ginkgo.It("should create simple content payload for single plain text item", func() {
			items := []types.MessageItem{
				{Text: "Simple message"},
			}
			payload, err := CreatePayloadFromItems(items, "", colors)
			gomega.Expect(err).ToNot(gomega.HaveOccurred())
			gomega.Expect(payload.Content).To(gomega.Equal("Simple message"))
			gomega.Expect(payload.Embeds).To(gomega.BeEmpty())
			gomega.Expect(payload.Attachments).To(gomega.BeEmpty())
		})

		ginkgo.It("should create embed payload when title is provided", func() {
			items := []types.MessageItem{
				{Text: "Message with title"},
			}
			payload, err := CreatePayloadFromItems(items, "Test Title", colors)
			gomega.Expect(err).ToNot(gomega.HaveOccurred())
			gomega.Expect(payload.Content).To(gomega.Equal(""))
			gomega.Expect(payload.Embeds).To(gomega.HaveLen(1))
			gomega.Expect(payload.Embeds[0].Title).To(gomega.Equal("Test Title"))
			gomega.Expect(payload.Embeds[0].Content).To(gomega.Equal("Message with title"))
		})

		ginkgo.It("should create embed payload when level is set", func() {
			items := []types.MessageItem{
				{Text: "Info message", Level: types.Info},
			}
			payload, err := CreatePayloadFromItems(items, "", colors)
			gomega.Expect(err).ToNot(gomega.HaveOccurred())
			gomega.Expect(payload.Content).To(gomega.Equal(""))
			gomega.Expect(payload.Embeds).To(gomega.HaveLen(1))
			gomega.Expect(payload.Embeds[0].Color).To(gomega.Equal(uint(0x00ff00)))
			gomega.Expect(payload.Embeds[0].Footer.Text).To(gomega.Equal("Info"))
		})

		ginkgo.It("should create embed payload when timestamp is set", func() {
			timestamp := time.Date(2023, 1, 1, 12, 0, 0, 0, time.UTC)
			items := []types.MessageItem{
				{Text: "Message with timestamp", Timestamp: timestamp},
			}
			payload, err := CreatePayloadFromItems(items, "", colors)
			gomega.Expect(err).ToNot(gomega.HaveOccurred())
			gomega.Expect(payload.Content).To(gomega.Equal(""))
			gomega.Expect(payload.Embeds).To(gomega.HaveLen(1))
			gomega.Expect(payload.Embeds[0].Timestamp).To(gomega.Equal("2023-01-01T12:00:00Z"))
		})

		ginkgo.It("should create embed payload when embed fields are present", func() {
			items := []types.MessageItem{
				{
					Text: "Message with embed fields",
					Fields: []types.Field{
						{Key: "embed_author_name", Value: "Test Author"},
					},
				},
			}
			payload, err := CreatePayloadFromItems(items, "", colors)
			gomega.Expect(err).ToNot(gomega.HaveOccurred())
			gomega.Expect(payload.Content).To(gomega.Equal(""))
			gomega.Expect(payload.Embeds).To(gomega.HaveLen(1))
			gomega.Expect(payload.Embeds[0].Author).NotTo(gomega.BeNil())
			gomega.Expect(payload.Embeds[0].Author.Name).To(gomega.Equal("Test Author"))
		})

		ginkgo.It("should create embed payload when file is attached", func() {
			items := []types.MessageItem{
				{
					Text: "Message with file",
					File: &types.File{Name: "test.txt", Data: []byte("content")},
				},
			}
			payload, err := CreatePayloadFromItems(items, "", colors)
			gomega.Expect(err).ToNot(gomega.HaveOccurred())
			gomega.Expect(payload.Content).To(gomega.Equal(""))
			gomega.Expect(payload.Embeds).To(gomega.HaveLen(1))
			gomega.Expect(payload.Attachments).To(gomega.HaveLen(1))
			gomega.Expect(payload.Attachments[0].Filename).To(gomega.Equal("test.txt"))
			gomega.Expect(payload.Attachments[0].ID).To(gomega.Equal(0))
		})

		ginkgo.It("should handle multiple items with embeds", func() {
			items := []types.MessageItem{
				{Text: "First message", Level: types.Info},
				{Text: "Second message", Level: types.Warning},
			}
			payload, err := CreatePayloadFromItems(items, "Multi Message", colors)
			gomega.Expect(err).ToNot(gomega.HaveOccurred())
			gomega.Expect(payload.Content).To(gomega.Equal(""))
			gomega.Expect(payload.Embeds).To(gomega.HaveLen(2))
			gomega.Expect(payload.Embeds[0].Title).To(gomega.Equal("Multi Message"))
			gomega.Expect(payload.Embeds[0].Color).To(gomega.Equal(uint(0x00ff00)))
			gomega.Expect(payload.Embeds[1].Color).To(gomega.Equal(uint(0xffff00)))
		})

		ginkgo.It("should limit embeds to MaxEmbeds", func() {
			items := make([]types.MessageItem, MaxEmbeds+5)
			for i := range items {
				items[i] = types.MessageItem{Text: "Message " + string(rune(i+'0'))}
			}
			payload, err := CreatePayloadFromItems(items, "", colors)
			gomega.Expect(err).ToNot(gomega.HaveOccurred())
			gomega.Expect(payload.Embeds).To(gomega.HaveLen(MaxEmbeds))
		})

		ginkgo.It("should process embed fields correctly in payload", func() {
			items := []types.MessageItem{
				{
					Text: "Message with complex embed",
					Fields: []types.Field{
						{Key: "embed_author_name", Value: "Author Name"},
						{Key: "embed_author_url", Value: "https://example.com"},
						{Key: "embed_image_url", Value: "https://example.com/image.png"},
						{Key: "embed_thumbnail_url", Value: "https://example.com/thumb.png"},
						{Key: "regular_field", Value: "regular_value"},
					},
				},
			}
			payload, err := CreatePayloadFromItems(items, "", colors)
			gomega.Expect(err).ToNot(gomega.HaveOccurred())
			gomega.Expect(payload.Embeds).To(gomega.HaveLen(1))
			embed := payload.Embeds[0]
			gomega.Expect(embed.Author).NotTo(gomega.BeNil())
			gomega.Expect(embed.Author.Name).To(gomega.Equal("Author Name"))
			gomega.Expect(embed.Author.URL).To(gomega.Equal("https://example.com"))
			gomega.Expect(embed.Image).NotTo(gomega.BeNil())
			gomega.Expect(embed.Image.URL).To(gomega.Equal("https://example.com/image.png"))
			gomega.Expect(embed.Thumbnail).NotTo(gomega.BeNil())
			gomega.Expect(embed.Thumbnail.URL).To(gomega.Equal("https://example.com/thumb.png"))
			gomega.Expect(embed.Fields).To(gomega.HaveLen(1))
			gomega.Expect(embed.Fields[0].Name).To(gomega.Equal("regular_field"))
			gomega.Expect(embed.Fields[0].Value).To(gomega.Equal("regular_value"))
		})

		ginkgo.It("should handle multiple files with correct IDs", func() {
			items := []types.MessageItem{
				{
					Text: "First file",
					File: &types.File{Name: "file1.txt", Data: []byte("content1")},
				},
				{
					Text: "Second file",
					File: &types.File{Name: "file2.txt", Data: []byte("content2")},
				},
			}
			payload, err := CreatePayloadFromItems(items, "", colors)
			gomega.Expect(err).ToNot(gomega.HaveOccurred())
			gomega.Expect(payload.Attachments).To(gomega.HaveLen(2))
			gomega.Expect(payload.Attachments[0].ID).To(gomega.Equal(0))
			gomega.Expect(payload.Attachments[0].Filename).To(gomega.Equal("file1.txt"))
			gomega.Expect(payload.Attachments[1].ID).To(gomega.Equal(1))
			gomega.Expect(payload.Attachments[1].Filename).To(gomega.Equal("file2.txt"))
		})

		ginkgo.It("should handle zero timestamp correctly", func() {
			items := []types.MessageItem{
				{Text: "Message with zero timestamp", Timestamp: time.Time{}},
			}
			payload, err := CreatePayloadFromItems(items, "", colors)
			gomega.Expect(err).ToNot(gomega.HaveOccurred())
			gomega.Expect(payload.Content).To(gomega.Equal("Message with zero timestamp"))
			gomega.Expect(payload.Embeds).To(gomega.BeEmpty())
		})

		ginkgo.It("should handle unknown level correctly", func() {
			items := []types.MessageItem{
				{Text: "Message with unknown level", Level: types.Unknown},
			}
			payload, err := CreatePayloadFromItems(items, "", colors)
			gomega.Expect(err).ToNot(gomega.HaveOccurred())
			gomega.Expect(payload.Content).To(gomega.Equal("Message with unknown level"))
			gomega.Expect(payload.Embeds).To(gomega.BeEmpty())
		})

		ginkgo.It("should handle out of bounds level correctly", func() {
			items := []types.MessageItem{
				{Text: "Message with invalid level", Level: types.MessageLevel(10)},
			}
			payload, err := CreatePayloadFromItems(items, "", colors)
			gomega.Expect(err).ToNot(gomega.HaveOccurred())
			gomega.Expect(payload.Content).To(gomega.Equal(""))
			gomega.Expect(payload.Embeds).To(gomega.HaveLen(1))
			gomega.Expect(payload.Embeds[0].Color).To(gomega.Equal(uint(0)))
		})
	})
})
