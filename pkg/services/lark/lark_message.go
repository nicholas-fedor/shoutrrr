package lark

// RequestBody represents the payload sent to the Lark API.
type RequestBody struct {
	MsgType   MsgType `json:"msg_type"`
	Content   Content `json:"content"`
	Timestamp string  `json:"timestamp,omitempty"`
	Sign      string  `json:"sign,omitempty"`
}

// MsgType defines the type of message to send.
type MsgType string

// Constants for message types supported by Lark.
const (
	MsgTypeText MsgType = "text"
	MsgTypePost MsgType = "post"
)

// Content holds the message content, supporting text or post formats.
type Content struct {
	Text string `json:"text,omitempty"`
	Post *Post  `json:"post,omitempty"`
}

// Post represents a rich post message with language-specific content.
type Post struct {
	Zh *Message `json:"zh_cn,omitempty"` // Chinese content
	En *Message `json:"en_us,omitempty"` // English content
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

// Constants for tag values supported by Lark.
const (
	TagValueText TagValue = "text"
	TagValueLink TagValue = "a"
)

// Response represents the API response from Lark.
type Response struct {
	Code int    `json:"code"`
	Msg  string `json:"msg"`
	Data any    `json:"data"`
}
