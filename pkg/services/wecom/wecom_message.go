package wecom

// RequestBody represents the payload sent to the WeCom webhook API.
type RequestBody struct {
	MsgType string      `json:"msgtype"`
	Text    TextContent `json:"text"`
}

// TextContent holds the text message content for WeCom.
type TextContent struct {
	Content             string   `json:"content"`
	MentionedList       []string `json:"mentioned_list,omitempty"`
	MentionedMobileList []string `json:"mentioned_mobile_list,omitempty"`
}

// Response represents the API response from WeCom.
type Response struct {
	ErrCode int    `json:"errcode"`
	ErrMsg  string `json:"errmsg"`
}
