package slack

import (
	"strings"

	"github.com/nicholas-fedor/shoutrrr/internal/chaticon"
)

// MessagePayload used within the Slack service.
type MessagePayload struct {
	// Text is the message text, the title, shown above the attachments.
	Text string `json:"text"`
	// BotName overrides the name the message is posted as, left out when
	// empty.
	BotName string `json:"username,omitempty"`
	// ThreadTS is the ts value of the parent message to reply to, left out
	// when empty.
	ThreadTS string `json:"thread_ts,omitempty"`
	// Channel is the channel ID to post to, left out for a webhook.
	Channel string `json:"channel,omitempty"`
	// IconEmoji overrides the icon with an emoji, left out when empty.
	IconEmoji string `json:"icon_emoji,omitempty"`
	// IconURL overrides the icon with an image URL, left out when empty.
	IconURL string `json:"icon_url,omitempty"`
	// Blocks holds layout blocks, left out when empty.
	Blocks []block `json:"blocks,omitempty"`
	// Attachments holds the attachments built from the message lines, left out
	// when empty.
	Attachments []attachment `json:"attachments,omitempty"`
}

// block is a Slack layout block.
type block struct {
	// Type is the block type.
	Type string `json:"type"`
	// Text is the block's text.
	Text blockText `json:"text"`
}

// blockText is the text object of a layout block.
type blockText struct {
	// Type is the text format, such as plain_text or mrkdwn.
	Type string `json:"type"`
	// Text is the text.
	Text string `json:"text"`
}

// attachment is a legacy secondary attachment, which shows a colored border
// beside its text.
type attachment struct {
	// Title is the attachment title, left out when empty.
	Title string `json:"title,omitempty"`
	// Fallback is the plain-text summary for clients that cannot show
	// attachments, left out when empty.
	Fallback string `json:"fallback,omitempty"`
	// Text is the attachment text, one line of the message. The last of
	// [MaxAttachments] attachments holds the rest of the message.
	Text string `json:"text"`
	// Color is the border color, left out when empty.
	Color string `json:"color,omitempty"`
	// Footer is the footer text, left out when empty.
	Footer string `json:"footer,omitempty"`
	// Fields holds the attachment's fields, left out when empty.
	Fields []legacyField `json:"fields,omitempty"`
	// Time is the attachment's timestamp in Unix seconds, left out when zero.
	Time int `json:"ts,omitzero"`
}

// legacyField is a field of a legacy attachment, shown as a title and a value.
type legacyField struct {
	// Title is the field title.
	Title string `json:"title"`
	// Value is the field value.
	Value string `json:"value"`
	// Short lets the field share a row with another short field, left out
	// when false.
	Short bool `json:"short,omitzero"`
}

// APIResponse is the default generic response message sent from the API.
type APIResponse struct {
	// Error is the error code when Ok is false.
	Error string `json:"error"`
	// Warning lists warnings for a request that succeeded.
	Warning string `json:"warning"`
	// MetaData holds additional details about the response.
	MetaData struct {
		// Warnings lists the warnings in detail.
		Warnings []string `json:"warnings"`
	} `json:"response_metadata"`
	// Ok reports whether the request succeeded.
	Ok bool `json:"ok"`
}

// Constants for Slack API limits.
const (
	// MaxAttachments is the maximum number of attachments allowed by Slack API.
	MaxAttachments = 100
)

// CreateJSONPayload builds a payload for the Slack chat.postMessage API or a
// webhook, with one attachment for each line of the message. A message with
// more lines than [MaxAttachments] puts the rest in the last attachment, and
// a trailing line break adds no attachment.
//
// Parameters:
//   - config: the configuration for this send.
//   - message: the message text.
//
// Returns:
//   - MessagePayload: the payload to send.
func CreateJSONPayload(config *Config, message string) MessagePayload {
	// The texts are collected first so the attachments are allocated once, at
	// their exact count. The last attachment keeps the rest of the message
	// with its line breaks.
	var texts [MaxAttachments]string

	count := 0
	for rest := message; rest != ""; count++ {
		line, after, _ := strings.Cut(rest, "\n")
		if count == MaxAttachments-1 {
			line, after = rest, ""
		}

		texts[count] = line
		rest = after
	}

	atts := make([]attachment, count)
	for i, text := range texts[:count] {
		atts[i].Text = text
		atts[i].Color = config.Color
	}

	payload := MessagePayload{
		ThreadTS:    config.ThreadTS,
		Text:        config.Title,
		BotName:     config.BotName,
		Attachments: atts,
		Blocks:      nil,
		Channel:     "",
		IconEmoji:   "",
		IconURL:     "",
	}

	payload.SetIcon(config.Icon)

	if config.Channel != "webhook" {
		payload.Channel = config.Channel
	}

	return payload
}

// SetIcon sets the icon URL or the icon emoji and clears the other.
//
// Parameters:
//   - icon: a URL for [MessagePayload.IconURL], an emoji name for
//     [MessagePayload.IconEmoji], or empty to clear both.
func (p *MessagePayload) SetIcon(icon string) {
	if chaticon.IsURL(icon) {
		p.IconURL, p.IconEmoji = icon, ""

		return
	}

	p.IconURL, p.IconEmoji = "", icon
}
