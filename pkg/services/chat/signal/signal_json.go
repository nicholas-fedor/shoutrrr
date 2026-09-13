package signal

// sendMessagePayload is the JSON body posted to POST /v2/send.
type sendMessagePayload struct {
	Message           string   `json:"message"`
	Number            string   `json:"number"`
	Recipients        []string `json:"recipients"`
	Base64Attachments []string `json:"base64_attachments,omitempty"`
	TextMode          string   `json:"text_mode,omitempty"`
	NotifySelf        *bool    `json:"notify_self,omitempty"`
}

// sendMessageResponse is the success body from the Signal REST API.
type sendMessageResponse struct {
	Timestamp int64 `json:"timestamp"`
}

// sendErrorResponse is the error body from a failed send.
type sendErrorResponse struct {
	Error           string   `json:"error"`
	ChallengeTokens []string `json:"challenge_tokens,omitempty"`
	Account         string   `json:"account,omitempty"`
}
