package lark

import "encoding/json/jsontext"

// RequestBody represents the payload sent to the Lark API.
type RequestBody struct {
	// MsgType is the message type, text or post.
	MsgType MsgType `json:"msg_type"`
	// Content is the message content for MsgType.
	Content Content `json:"content"`
	// Timestamp is the signing time in Unix seconds, left out for an unsigned
	// request.
	Timestamp string `json:"timestamp,omitempty"`
	// Sign is the request signature, left out for an unsigned request.
	Sign string `json:"sign,omitempty"`
}

// MsgType defines the type of message to send.
type MsgType string

// Content holds the message content, supporting text or post formats.
type Content struct {
	// Post is the rich post of a post message, left out for a text message.
	Post *Post `json:"post,omitzero"`
	// Text is the text of a text message, left out for a post message.
	Text string `json:"text,omitempty"`
}

// Post represents a rich post message with language-specific content.
type Post struct {
	// Zh is the Simplified Chinese content, left out when nil.
	Zh *Message `json:"zh_cn,omitzero"`
	// En is the English content, left out when nil.
	En *Message `json:"en_us,omitzero"`
}

// Message defines the structure of a post message.
type Message struct {
	// Title is the post title.
	Title string `json:"title"`
	// Content is the post body, as paragraphs of items.
	Content [][]Item `json:"content"`
}

// Item represents a content element within a post message.
type Item struct {
	// Tag is the item type, text or a link.
	Tag TagValue `json:"tag"`
	// Text is the item's text, or the link's label.
	Text string `json:"text,omitempty"`
	// Link is the URL of a link item, left out for text.
	Link string `json:"href,omitempty"`
}

// TagValue specifies the type of content item.
type TagValue string

// Response represents the API response from Lark.
type Response struct {
	// Msg is the result message, which describes the error when Code is not 0.
	Msg string `json:"msg"`
	// Data is the response payload, kept undecoded because a send only checks
	// the code and message.
	Data jsontext.Value `json:"data"`
	// Code is 0 for a successful request.
	Code int `json:"code"`
}

// Constants for message types supported by Lark.
const (
	// MsgTypeText is a plain text message.
	MsgTypeText MsgType = "text"
	// MsgTypePost is a rich post message with a title.
	MsgTypePost MsgType = "post"
)

// Constants for tag values supported by Lark.
//
//nolint:goconst // TagValueTextContent and MsgTypeText have same value but represent different concepts
const (
	// TagValueTextContent is a text item.
	TagValueTextContent TagValue = "text"
	// TagValueLink is a link item.
	TagValueLink TagValue = "a"
)
