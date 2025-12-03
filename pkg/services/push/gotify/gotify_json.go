package gotify

import "fmt"

// messageRequest is the actual payload being sent to the Gotify API.
// This struct represents the JSON request body sent to the Gotify message endpoint,
// containing all notification data including message content, metadata, and custom extras.
type messageRequest struct {
	Message  string         `json:"message"`  // The main notification message text content
	Title    string         `json:"title"`    // Notification title displayed in the Gotify interface
	Priority int            `json:"priority"` // Priority level (-2 to 1) affecting notification behavior
	Extras   map[string]any `json:"extras"`   // Additional custom key-value pairs for extended functionality
}

// messageResponse represents the successful response from the Gotify API.
// This struct captures the server's confirmation of message acceptance,
// including the assigned ID and metadata about the created notification.
type messageResponse struct {
	Message  string         `json:"message"`  // Echo of the sent message content
	Title    string         `json:"title"`    // Echo of the notification title
	Priority int            `json:"priority"` // Echo of the priority level set
	Extras   map[string]any `json:"extras"`   // Echo of any extras sent with the message
	ID       uint64         `json:"id"`       // Unique identifier assigned by Gotify to this message
	AppID    uint64         `json:"appid"`    // Application ID that sent the message
	Date     string         `json:"date"`     // ISO 8601 timestamp of when the message was created
}

// responseError represents an error response from the Gotify API.
// This struct captures structured error information returned by the server
// when a request fails, providing detailed error context.
type responseError struct {
	Name        string `json:"error"`            // Error name/type identifier
	Code        uint64 `json:"errorCode"`        // Numeric error code for programmatic handling
	Description string `json:"errorDescription"` // Human-readable error description
}

// Error implements the error interface for responseError.
// This method formats the error information into a consistent error message
// that can be returned as a standard Go error.
func (er *responseError) Error() string {
	return fmt.Sprintf("server responded with %v (%v): %v", er.Name, er.Code, er.Description)
}
