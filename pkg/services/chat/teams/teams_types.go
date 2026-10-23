package teams

// adaptivePayload is the wrapper for a Teams Power Automate webhook Adaptive Card message.
type adaptivePayload struct {
	// Type is the message type, "message".
	Type string `json:"type"`
	// Attachments holds the Adaptive Card attachment.
	Attachments []adaptiveAttachment `json:"attachments"`
}

// adaptiveAttachment represents a single Adaptive Card attachment.
type adaptiveAttachment struct {
	// ContentType is the attachment's media type, the Adaptive Card type.
	ContentType string `json:"contentType"`
	// ContentURL is the URL of remote content, sent as null because the card
	// is inline.
	ContentURL *string `json:"contentUrl"`
	// Content is the inline Adaptive Card.
	Content adaptiveCardContent `json:"content"`
}

// adaptiveCardContent is the Adaptive Card JSON body.
type adaptiveCardContent struct {
	// Schema is the URL of the Adaptive Card JSON schema.
	Schema string `json:"$schema"`
	// Type is the element type, "AdaptiveCard".
	Type string `json:"type"`
	// Version is the Adaptive Card schema version.
	Version string `json:"version"`
	// Body holds the card's blocks: the title, when set, and the message.
	Body []adaptiveBlock `json:"body"`
}

// adaptiveBlock is a single UI block within an Adaptive Card body.
type adaptiveBlock struct {
	// Type is the element type, such as "TextBlock".
	Type string `json:"type"`
	// Text is the block's text, left out when empty.
	Text string `json:"text,omitempty"`
	// Color is the text color, left out when empty.
	Color string `json:"color,omitempty"`
	// Weight is the font weight, such as "bolder", left out when empty.
	Weight string `json:"weight,omitempty"`
	// Size is the font size, left out when empty.
	Size string `json:"size,omitempty"`
	// Wrap wraps long text instead of cutting it off, left out when false.
	Wrap bool `json:"wrap,omitzero"`
}
