package teams

// adaptivePayload is the wrapper for a Teams Power Automate webhook Adaptive Card message.
type adaptivePayload struct {
	Type        string               `json:"type"`
	Attachments []adaptiveAttachment `json:"attachments"`
}

// adaptiveAttachment represents a single Adaptive Card attachment.
type adaptiveAttachment struct {
	ContentType string              `json:"contentType"`
	ContentURL  *string             `json:"contentUrl"`
	Content     adaptiveCardContent `json:"content"`
}

// adaptiveCardContent is the Adaptive Card JSON body.
type adaptiveCardContent struct {
	Schema  string          `json:"$schema"`
	Type    string          `json:"type"`
	Version string          `json:"version"`
	Body    []adaptiveBlock `json:"body"`
}

// adaptiveBlock is a single UI block within an Adaptive Card body.
type adaptiveBlock struct {
	Type   string `json:"type"`
	Text   string `json:"text,omitempty"`
	Color  string `json:"color,omitempty"`
	Weight string `json:"weight,omitempty"`
	Size   string `json:"size,omitempty"`
	Wrap   bool   `json:"wrap,omitempty"`
}
