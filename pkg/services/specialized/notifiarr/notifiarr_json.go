package notifiarr

// Field represents a field in the Discord embed.
type Field struct {
	Title  string `json:"title"`  // Field title
	Text   string `json:"text"`   // Field text content
	Inline bool   `json:"inline"` // Whether the field is inline
}

// NotificationPayload represents the main payload structure for Notifiarr API.
type NotificationPayload struct {
	Notification NotificationData `json:"notification"`
	Discord      *DiscordPayload  `json:"discord,omitempty"`
}

// NotificationData contains the notification metadata.
type NotificationData struct {
	Update *bool  `json:"update,omitempty"` // Optional boolean for updating existing messages
	Name   string `json:"name"`             // Required name of the custom app/script
	Event  string `json:"event,omitempty"`  // Optional unique ID for this notification
}

// DiscordPayload contains Discord-specific configuration for notifications.
type DiscordPayload struct {
	Color  string        `json:"color,omitempty"`  // Optional color as 6-digit HTML color code
	Ping   *PingPayload  `json:"ping,omitempty"`   // Optional ping configuration
	Images *ImagePayload `json:"images,omitempty"` // Optional image configuration
	Text   *TextPayload  `json:"text,omitempty"`   // Optional text configuration
	IDs    *IDPayload    `json:"ids,omitempty"`    // Optional IDs configuration
}

// PingPayload contains ping configuration for Discord notifications.
type PingPayload struct {
	PingUser *int `json:"pingUser,omitempty"` // Optional user ID to ping
	PingRole *int `json:"pingRole,omitempty"` // Optional role ID to ping
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
	Fields      []Field `json:"fields,omitempty"`      // Optional fields array
	Footer      string  `json:"footer,omitempty"`      // Optional footer text
}

// IDPayload contains required IDs for Discord notifications.
type IDPayload struct {
	Channel int `json:"channel"` // Required channel ID
}
