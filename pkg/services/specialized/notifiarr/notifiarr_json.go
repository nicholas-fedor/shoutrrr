package notifiarr

// Field represents a field in the Discord embed.
type Field struct {
	// Title is the field title.
	Title string `json:"title"`
	// Text is the field's text.
	Text string `json:"text"`
	// Inline shows the field beside other inline fields.
	Inline bool `json:"inline"`
}

// NotificationPayload represents the main payload structure for Notifiarr API.
type NotificationPayload struct {
	// Discord holds the Discord message, left out when nil.
	Discord *DiscordPayload `json:"discord,omitzero"`
	// Notification holds the notification metadata.
	Notification NotificationData `json:"notification"`
}

// NotificationData contains the notification metadata.
type NotificationData struct {
	// Update edits the message sent earlier with the same Event instead of
	// sending a new one, and is left out when nil.
	Update *bool `json:"update,omitzero"`
	// Name is the name of the custom app or script that sends the
	// notification. It is required.
	Name string `json:"name"`
	// Event is a unique ID for the notification, left out when empty.
	Event string `json:"event,omitempty"`
}

// DiscordPayload contains Discord-specific configuration for notifications.
type DiscordPayload struct {
	// Ping holds the user and role to mention, left out when nil.
	Ping *PingPayload `json:"ping,omitzero"`
	// Images holds the thumbnail and image URLs, left out when nil.
	Images *ImagePayload `json:"images,omitzero"`
	// Text holds the message text, left out when nil.
	Text *TextPayload `json:"text,omitzero"`
	// IDs holds the channel to send to, left out when nil.
	IDs *IDPayload `json:"ids,omitzero"`
	// Color is the embed color as a 6-digit HTML color code, left out when
	// empty.
	Color string `json:"color,omitempty"`
}

// PingPayload contains ping configuration for Discord notifications.
type PingPayload struct {
	// PingUser is the Discord user ID to mention, left out when nil.
	PingUser *int `json:"pingUser,omitzero"`
	// PingRole is the Discord role ID to mention, left out when nil.
	PingRole *int `json:"pingRole,omitzero"`
}

// ImagePayload contains image URLs for Discord notifications.
type ImagePayload struct {
	// Thumbnail is the thumbnail URL, left out when empty.
	Thumbnail string `json:"thumbnail,omitempty"`
	// Image is the image URL, left out when empty.
	Image string `json:"image,omitempty"`
}

// TextPayload contains text content for Discord notifications.
type TextPayload struct {
	// Title is the message title, left out when empty.
	Title string `json:"title,omitempty"`
	// Icon is the icon URL, left out when empty.
	Icon string `json:"icon,omitempty"`
	// Content is the text above the embed, left out when empty.
	Content string `json:"content,omitempty"`
	// Description is the embed's body, left out when empty.
	Description string `json:"description,omitempty"`
	// Footer is the footer text, left out when empty.
	Footer string `json:"footer,omitempty"`
	// Fields holds the embed's fields, left out when empty.
	Fields []Field `json:"fields,omitempty"`
}

// IDPayload contains required IDs for Discord notifications.
type IDPayload struct {
	// Channel is the Discord channel ID to send to. It is required.
	Channel int `json:"channel"`
}
