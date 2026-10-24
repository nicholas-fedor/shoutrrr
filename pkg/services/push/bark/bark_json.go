package bark

// PushPayload represents the JSON payload sent to the Bark server API.
// This structure contains all notification parameters that can be customized.
type PushPayload struct {
	// Body is the notification text.
	Body string `json:"body"`
	// DeviceKey is the key of the device that receives the notification.
	DeviceKey string `json:"device_key"`
	// Title is the notification title.
	Title string `json:"title"`
	// Sound is the name of the notification sound. It is omitted when empty.
	Sound string `json:"sound,omitempty"`
	// Badge is the number displayed next to the app icon. It is omitted only
	// when nil, so a badge of 0 is still sent.
	Badge *int64 `json:"badge,omitzero"`
	// Icon is the URL of the notification icon. It is omitted when empty.
	Icon string `json:"icon,omitempty"`
	// Group is the group of the notification. It is omitted when empty.
	Group string `json:"group,omitempty"`
	// URL is the URL to open when the notification is tapped. It is omitted
	// when empty.
	URL string `json:"url,omitempty"`
	// Category is a field that Bark reserves for future use. It is omitted
	// when empty.
	Category string `json:"category,omitempty"`
	// Copy is the text to copy to the clipboard when the notification is
	// tapped. It is omitted when empty.
	Copy string `json:"copy,omitempty"`
}

// APIResponse represents the response structure returned by the Bark server API.
// The server responds with a JSON object containing status code, message, and timestamp.
//
//nolint:errname // APIResponse name is mandated by the Bark API response format
type APIResponse struct {
	// Message is the server's description of the result.
	Message string `json:"message"`
	// Code is the status code of the result. Any code other than 200 is a
	// failure.
	Code int64 `json:"code"`
	// Timestamp is the server time of the response.
	Timestamp int64 `json:"timestamp"`
}

// Error returns the error message from the Bark API response.
// This method implements the error interface for APIResponse.
//
// Returns:
//   - A formatted error string containing the server message.
func (e *APIResponse) Error() string {
	return "server response: " + e.Message
}
