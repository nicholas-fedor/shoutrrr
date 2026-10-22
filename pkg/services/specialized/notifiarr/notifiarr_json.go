package notifiarr

// Field represents a field in the Discord embed.
type Field struct {
	Title  string `json:"title"`  // Field title
	Text   string `json:"text"`   // Field text content
	Inline bool   `json:"inline"` // Whether the field is inline
}

// NotificationPayload represents the main payload structure for Notifiarr API.
type NotificationPayload struct {
	Discord      *DiscordPayload  `json:"discord,omitzero"`
	Notification NotificationData `json:"notification"`
}

// NotificationData contains the notification metadata.
type NotificationData struct {
	Update *bool  `json:"update,omitzero"` // Optional boolean for updating existing messages
	Name   string `json:"name"`            // Required name of the custom app/script
	Event  string `json:"event,omitempty"` // Optional unique ID for this notification
}

// DiscordPayload contains Discord-specific configuration for notifications.
type DiscordPayload struct {
	Ping   *PingPayload  `json:"ping,omitzero"`   // Optional ping configuration
	Images *ImagePayload `json:"images,omitzero"` // Optional image configuration
	Text   *TextPayload  `json:"text,omitzero"`   // Optional text configuration
	IDs    *IDPayload    `json:"ids,omitzero"`    // Optional IDs configuration
	Color  string        `json:"color,omitempty"` // Optional color as 6-digit HTML color code
}

// PingPayload contains ping configuration for Discord notifications.
type PingPayload struct {
	PingUser *int `json:"pingUser,omitzero"` // Optional user ID to ping
	PingRole *int `json:"pingRole,omitzero"` // Optional role ID to ping
}

// ImagePayload contains image URLs for Discord notifications.
type ImagePayload struct {
	Thumbnail string `json:"thumbnail,omitempty"` // Optional thumbnail URL
	Image     string `json:"image,omitempty"`     // Optional image URL
}

// TextPayload contains text content for Discord notifications.
type TextPayload struct {
	Title       string  `json:"title,omitempty"`       // Optional notification title
	Icon        string  `json:"icon,omitempty"`        // Optional icon URL
	Content     string  `json:"content,omitempty"`     // Optional content
	Description string  `json:"description,omitempty"` // Optional description
	Footer      string  `json:"footer,omitempty"`      // Optional footer text
	Fields      []Field `json:"fields,omitempty"`      // Optional fields array
}

// IDPayload contains required IDs for Discord notifications.
type IDPayload struct {
	Channel int `json:"channel"` // Required channel ID
}
