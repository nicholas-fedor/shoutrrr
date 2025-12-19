package discord

import (
	"time"

	"github.com/nicholas-fedor/shoutrrr/pkg/types"
	"github.com/nicholas-fedor/shoutrrr/pkg/util"
)

const (
	MaxEmbeds = 10
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
	Title     string          `json:"title,omitempty"`
	Content   string          `json:"description,omitempty"`
	URL       string          `json:"url,omitempty"`
	Timestamp string          `json:"timestamp,omitempty"`
	Color     uint            `json:"color,omitempty"`
	Footer    *embedFooter    `json:"footer,omitempty"`
	Author    *embedAuthor    `json:"author,omitempty"`
	Image     *embedImage     `json:"image,omitempty"`
	Thumbnail *embedThumbnail `json:"thumbnail,omitempty"`
	Fields    []embedField    `json:"fields,omitempty"`
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
	Inline bool   `json:"inline,omitempty"`
}

type attachment struct {
	ID       int    `json:"id"`
	Filename string `json:"filename"`
}

// hasEmbedFields checks if the fields contain any embed-specific fields.
func hasEmbedFields(fields []types.Field) bool {
	for _, field := range fields {
		switch field.Key {
		case "embed_author_name", "embed_author_url", "embed_author_icon_url",
			"embed_image_url", "embed_thumbnail_url":
			return true
		}
	}

	return false
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
			embedFields = append(embedFields, embedField{
				Name:  field.Key,
				Value: field.Value,
			})
		}
	}

	return author, image, thumbnail, embedFields
}

// CreatePayloadFromItems creates a JSON payload to be sent to the discord webhook API.
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
		!hasFiles {
		return WebhookPayload{
			Content: items[0].Text,
		}, nil
	}

	itemCount := util.Min(MaxEmbeds, len(items))

	embeds := make([]embedItem, 0, itemCount)

	var attachments []attachment

	for i, item := range items {
		if i >= itemCount {
			break
		}

		color := uint(0)
		if item.Level >= types.Unknown && int(item.Level) < len(colors) {
			color = colors[item.Level]
		}

		author, image, thumbnail, embedFields := processEmbedFields(item.Fields)

		embeddedItem := embedItem{
			Content:   item.Text,
			Color:     color,
			Author:    author,
			Image:     image,
			Thumbnail: thumbnail,
			Fields:    embedFields,
		}

		if item.Level != types.Unknown {
			embeddedItem.Footer = &embedFooter{
				Text: item.Level.String(),
			}
		}

		if !item.Timestamp.IsZero() {
			embeddedItem.Timestamp = item.Timestamp.UTC().Format(time.RFC3339)
		}

		embeds = append(embeds, embeddedItem)

		// Add file attachment if present
		if item.File != nil {
			attachments = append(attachments, attachment{
				ID:       len(attachments),
				Filename: item.File.Name,
			})
		}
	}

	// This should not happen, but it's better to leave the index check before dereferencing the array
	if len(embeds) > 0 {
		embeds[0].Title = title
	}

	return WebhookPayload{
		Embeds:      embeds,
		Attachments: attachments,
	}, nil
}
