package discord

import (
	"time"
	"unicode/utf8"

	"github.com/nicholas-fedor/shoutrrr/pkg/types"
	"github.com/nicholas-fedor/shoutrrr/pkg/util"
)

// WebhookPayload is the webhook endpoint payload.
type WebhookPayload struct {
	Content     string       `json:"content,omitempty"`
	Embeds      []embedItem  `json:"embeds,omitempty"`
	Username    string       `json:"username,omitempty"`
	AvatarURL   string       `json:"avatar_url,omitempty"`
	Attachments []attachment `json:"attachments,omitempty"`
}

// JSON is the actual notification payload.
type embedItem struct {
	Footer    *embedFooter    `json:"footer,omitzero"`
	Author    *embedAuthor    `json:"author,omitzero"`
	Image     *embedImage     `json:"image,omitzero"`
	Thumbnail *embedThumbnail `json:"thumbnail,omitzero"`
	Title     string          `json:"title,omitempty"`
	Content   string          `json:"description,omitempty"`
	URL       string          `json:"url,omitempty"`
	Timestamp string          `json:"timestamp,omitempty"`
	Fields    []embedField    `json:"fields,omitempty"`
	Color     uint            `json:"color,omitzero"`
}

type embedFooter struct {
	Text    string `json:"text"`
	IconURL string `json:"icon_url,omitempty"`
}

type embedAuthor struct {
	Name    string `json:"name,omitempty"`
	URL     string `json:"url,omitempty"`
	IconURL string `json:"icon_url,omitempty"`
}

type embedImage struct {
	URL string `json:"url,omitempty"`
}

type embedThumbnail struct {
	URL string `json:"url,omitempty"`
}

type embedField struct {
	Name   string `json:"name"`
	Value  string `json:"value"`
	Inline bool   `json:"inline,omitzero"`
}

type attachment struct {
	Filename string `json:"filename"`
	ID       int    `json:"id"`
}

const (
	MaxEmbeds = 10
)

// Embed limits from Discord's message documentation, in characters.
const (
	// maxEmbedTitle is the longest embed title.
	maxEmbedTitle = 256
	// maxEmbedDescription is the longest embed description.
	maxEmbedDescription = 4096
	// maxEmbedFields is the most fields an embed holds.
	maxEmbedFields = 25
	// maxFieldName is the longest embed field name.
	maxFieldName = 256
	// maxFieldValue is the longest embed field value.
	maxFieldValue = 1024
	// maxFooterText is the longest embed footer text.
	maxFooterText = 2048
	// maxAuthorName is the longest embed author name.
	maxAuthorName = 256
)

// hasEmbedFields checks if the fields contain any fields that should trigger embed processing.
func hasEmbedFields(fields []types.Field) bool {
	return len(fields) > 0
}

// processEmbedFields processes MessageItem fields into embed components.
func processEmbedFields(
	fields []types.Field,
) (*embedAuthor, *embedImage, *embedThumbnail, []embedField) {
	var (
		author      *embedAuthor
		image       *embedImage
		thumbnail   *embedThumbnail
		embedFields []embedField
	)

	for _, field := range fields {
		switch field.Key {
		case "embed_author_name":
			if author == nil {
				author = &embedAuthor{}
			}

			author.Name = field.Value
		case "embed_author_url":
			if author == nil {
				author = &embedAuthor{}
			}

			author.URL = field.Value
		case "embed_author_icon_url":
			if author == nil {
				author = &embedAuthor{}
			}

			author.IconURL = field.Value
		case "embed_image_url":
			image = &embedImage{URL: field.Value}
		case "embed_thumbnail_url":
			thumbnail = &embedThumbnail{URL: field.Value}
		default:
			// Regular fields become embed fields
			//nolint:exhaustruct_v5 // Inline is optional and defaults to false
			embedFields = append(embedFields, embedField{
				Name:  field.Key,
				Value: field.Value,
			})
		}
	}

	return author, image, thumbnail, embedFields
}

// CreatePayloadFromItems creates a JSON payload to be sent to the discord webhook API.
//
// A single plain item becomes the message content, and any other items become
// embeds, at most [MaxEmbeds] of them. Items past that limit are left out, so
// callers send them in further payloads.
//
// Parameters:
//   - items: the message items.
//   - title: the title of the first embed.
//   - colors: the embed color for each message level.
//
// Returns:
//   - WebhookPayload: the payload.
//   - error: [ErrEmptyMessage] for no items or a single plain item without text.
func CreatePayloadFromItems(
	items []types.MessageItem,
	title string,
	colors [types.MessageLevelCount]uint,
) (WebhookPayload, error) {
	if len(items) < 1 {
		return WebhookPayload{}, ErrEmptyMessage
	}

	// Check if we can use content field for plain text messages
	// Only if no special embed fields are present and no files
	hasFiles := false

	for _, item := range items {
		if item.File != nil {
			hasFiles = true

			break
		}
	}

	if len(items) == 1 && title == "" && items[0].Level == types.Unknown &&
		items[0].Timestamp.IsZero() &&
		!hasEmbedFields(items[0].Fields) &&
		!hasFiles &&
		utf8.RuneCountInString(items[0].Text) <= ChunkSize {
		if items[0].Text == "" {
			return WebhookPayload{}, ErrEmptyMessage
		}

		return WebhookPayload{
			Content: items[0].Text,
		}, nil
	}

	return createEmbedPayload(items, title, colors), nil
}

// createEmbedPayload creates a webhook payload with an embed for each item, at
// most [MaxEmbeds] of them, and an attachment entry for each of their files.
// Each embed is kept within Discord's per-field limits, and the embeds within
// [TotalChunkSize] characters together, as [fitEmbed] shortens an embed that
// does not fit in the characters left.
//
// Parameters:
//   - items: the message items, at least one.
//   - title: the title of the first embed.
//   - colors: the embed color for each message level.
//
// Returns:
//   - WebhookPayload: the payload.
func createEmbedPayload(
	items []types.MessageItem,
	title string,
	colors [types.MessageLevelCount]uint,
) WebhookPayload {
	itemCount := util.Min(MaxEmbeds, len(items))

	embeds := make([]embedItem, 0, itemCount)

	var attachments []attachment

	for i := range items[:itemCount] {
		item := &items[i]
		embeds = append(embeds, newEmbed(item, colors))

		if item.File != nil {
			attachments = append(attachments, attachment{
				ID:       len(attachments),
				Filename: item.File.Name,
			})
		}
	}

	embeds[0].Title = util.Ellipsis(title, maxEmbedTitle)

	remaining := TotalChunkSize
	for i := range embeds {
		fitEmbed(&embeds[i], remaining)
		remaining = max(remaining-embedLength(&embeds[i]), 0)
	}

	return WebhookPayload{
		Embeds:      embeds,
		Attachments: attachments,
	}
}

// newEmbed creates the embed for item, with its text, fields, and footer
// shortened to Discord's limits and fields past the 25th left out.
//
// Parameters:
//   - item: the message item.
//   - colors: the embed color for each message level.
//
// Returns:
//   - embedItem: the embed, without a title.
func newEmbed(item *types.MessageItem, colors [types.MessageLevelCount]uint) embedItem {
	color := uint(0)
	if item.Level >= types.Unknown && int(item.Level) < len(colors) {
		color = colors[item.Level]
	}

	author, image, thumbnail, embedFields := processEmbedFields(item.Fields)

	if author != nil {
		author.Name = util.Ellipsis(author.Name, maxAuthorName)
	}

	embedFields = embedFields[:util.Min(len(embedFields), maxEmbedFields)]
	for i := range embedFields {
		embedFields[i].Name = util.Ellipsis(embedFields[i].Name, maxFieldName)
		embedFields[i].Value = util.Ellipsis(embedFields[i].Value, maxFieldValue)
	}

	//nolint:exhaustruct_v5 // Fields are conditionally set below
	embed := embedItem{
		Content:   util.Ellipsis(item.Text, maxEmbedDescription),
		Color:     color,
		Author:    author,
		Image:     image,
		Thumbnail: thumbnail,
		Fields:    embedFields,
	}

	if item.Level != types.Unknown {
		//nolint:exhaustruct_v5 // IconURL is optional
		embed.Footer = &embedFooter{
			Text: util.Ellipsis(item.Level.String(), maxFooterText),
		}
	}

	if !item.Timestamp.IsZero() {
		embed.Timestamp = item.Timestamp.UTC().Format(time.RFC3339)
	}

	return embed
}

// embedLength returns the characters of an embed that count toward Discord's
// total for a message: its title, description, field names and values, footer
// text, and author name.
//
// Parameters:
//   - embed: the embed.
//
// Returns:
//   - int: the character count.
func embedLength(embed *embedItem) int {
	length := utf8.RuneCountInString(embed.Title) + utf8.RuneCountInString(embed.Content)

	for _, field := range embed.Fields {
		length += utf8.RuneCountInString(field.Name) + utf8.RuneCountInString(field.Value)
	}

	if embed.Footer != nil {
		length += utf8.RuneCountInString(embed.Footer.Text)
	}

	if embed.Author != nil {
		length += utf8.RuneCountInString(embed.Author.Name)
	}

	return length
}

// fitEmbed shortens an embed to at most budget characters, as [embedLength]
// counts them, by dropping fields from the end, then shortening the
// description, the author name, and the footer text in turn. An author or
// footer left without text is removed, since Discord requires their text. A
// negative budget counts as zero.
//
// Parameters:
//   - embed: the embed to shorten.
//   - budget: the most characters the embed may have.
func fitEmbed(embed *embedItem, budget int) {
	budget = max(budget, 0)

	for len(embed.Fields) > 0 && embedLength(embed) > budget {
		embed.Fields = embed.Fields[:len(embed.Fields)-1]
	}

	// shorten returns text without the characters the embed is over budget.
	shorten := func(text string) string {
		return util.Ellipsis(text, utf8.RuneCountInString(text)-(embedLength(embed)-budget))
	}

	if embedLength(embed) > budget {
		embed.Content = shorten(embed.Content)
	}

	if embed.Author != nil && embedLength(embed) > budget {
		if embed.Author.Name = shorten(embed.Author.Name); embed.Author.Name == "" {
			embed.Author = nil
		}
	}

	if embed.Footer != nil && embedLength(embed) > budget {
		if embed.Footer.Text = shorten(embed.Footer.Text); embed.Footer.Text == "" {
			embed.Footer = nil
		}
	}
}
