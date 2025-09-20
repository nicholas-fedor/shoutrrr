package signal

// sendMessagePayload represents the JSON payload for sending a Signal message.
type sendMessagePayload struct {
	Message           string   `json:"message"`
	Number            string   `json:"number"`
	Recipients        []string `json:"recipients"`
	Base64Attachments []string `json:"base64_attachments,omitempty"`
}

// sendMessageResponse represents the response from the Signal REST API.
type sendMessageResponse struct {
	Timestamp int64 `json:"timestamp"`
}
