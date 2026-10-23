package wecom

// RequestBody represents the payload sent to the WeCom webhook API.
type RequestBody struct {
	// MsgType is the message type, "text" for a text message.
	MsgType string `json:"msgtype"`
	// Text is the text message.
	Text TextContent `json:"text"`
}

// TextContent holds the text message content for WeCom.
type TextContent struct {
	// Content is the message.
	Content string `json:"content"`
	// MentionedList lists the user IDs to mention, left out when empty.
	MentionedList []string `json:"mentioned_list,omitempty"`
	// MentionedMobileList lists the mobile numbers of the users to mention,
	// left out when empty.
	MentionedMobileList []string `json:"mentioned_mobile_list,omitempty"`
}

// Response represents the API response from WeCom.
type Response struct {
	// ErrMsg describes the result, "ok" on success.
	ErrMsg string `json:"errmsg"`
	// ErrCode is the result code, 0 on success.
	ErrCode int `json:"errcode"`
}
