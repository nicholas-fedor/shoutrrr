package lark

import "encoding/json/jsontext"

// RequestBody represents the payload sent to the Lark API.
type RequestBody struct {
	MsgType   MsgType `json:"msg_type"`
	Content   Content `json:"content"`
	Timestamp string  `json:"timestamp,omitempty"`
	Sign      string  `json:"sign,omitempty"`
}

// MsgType defines the type of message to send.
type MsgType string

// Content holds the message content, supporting text or post formats.
type Content struct {
	Post *Post  `json:"post,omitzero"`
	Text string `json:"text,omitempty"`
}

// Post represents a rich post message with language-specific content.
type Post struct {
	Zh *Message `json:"zh_cn,omitzero"` // Chinese content
	En *Message `json:"en_us,omitzero"` // English content
}

// Message defines the structure of a post message.
type Message struct {
	Title   string   `json:"title"`
	Content [][]Item `json:"content"`
}

// Item represents a content element within a post message.
type Item struct {
	Tag  TagValue `json:"tag"`
	Text string   `json:"text,omitempty"`
	Link string   `json:"href,omitempty"`
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
	MsgTypeText MsgType = "text"
	MsgTypePost MsgType = "post"
)

// Constants for tag values supported by Lark.
//
//nolint:goconst // TagValueTextContent and MsgTypeText have same value but represent different concepts
const (
	TagValueTextContent TagValue = "text"
	TagValueLink        TagValue = "a"
)
