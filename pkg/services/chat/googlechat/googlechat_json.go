package googlechat

// JSON is the actual payload being sent to the Google Chat API.
type JSON struct {
	// Text is the message.
	Text string `json:"text"`
}
