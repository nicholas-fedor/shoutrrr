package gotify

import (
	"encoding/json/v2"
	"fmt"

	"github.com/nicholas-fedor/shoutrrr/internal/jsonopts"
)

// MessageRequest is the actual payload being sent to the Gotify API.
// This struct represents the JSON request body sent to the Gotify message endpoint,
// containing all notification data including message content, metadata, and custom extras.
type MessageRequest struct {
	// Extras holds the custom key-value pairs sent with the message.
	Extras map[string]any `json:"extras"`

	// Date is not sent.
	//
	// Deprecated: Gotify stamps each message with the server time, so a date has no effect.
	Date *string `json:"date,omitzero"`

	// Message is the message text.
	Message string `json:"message"`
	// Title is the message title.
	Title string `json:"title"`
	// Priority is the message priority, from -2 to 10.
	Priority int `json:"priority"`

	// omitPriority leaves the priority out of the JSON body, so Gotify applies
	// the application's default priority.
	omitPriority bool
}

// messageResponse represents the successful response from the Gotify API.
// This struct captures the server's confirmation of message acceptance,
// including the assigned ID and metadata about the created notification.
type messageResponse struct {
	// Extras echoes the extras sent with the message.
	Extras map[string]any `json:"extras"`
	// Message echoes the message text.
	Message string `json:"message"`
	// Title echoes the message title.
	Title string `json:"title"`
	// Date is the ISO 8601 time the server created the message.
	Date string `json:"date"`
	// Priority is the message's priority.
	Priority int `json:"priority"`
	// ID is the message's identifier.
	ID uint64 `json:"id"`
	// AppID is the identifier of the application that sent the message.
	AppID uint64 `json:"appid"`
}

// responseError represents an error response from the Gotify API.
// This struct captures structured error information returned by the server
// when a request fails, providing detailed error context.
type responseError struct {
	// Name is the error's name, the HTTP status text.
	Name string `json:"error"`
	// Description describes the error.
	Description string `json:"errorDescription"`
	// Code is the error code, the HTTP status code.
	Code uint64 `json:"errorCode"`
}

// Error implements the error interface for responseError.
// This method formats the error information into a consistent error message
// that can be returned as a standard Go error.
//
// Returns:
//   - string: the error's name, code, and description.
func (er *responseError) Error() string {
	return fmt.Sprintf("server responded with %v (%v): %v", er.Name, er.Code, er.Description)
}

// MarshalJSON writes the message body Gotify accepts: the message, title,
// and extras, and the priority unless the request leaves it out.
//
// Returns:
//   - []byte: the JSON body.
//   - error: an error encoding the extras.
func (r MessageRequest) MarshalJSON() ([]byte, error) {
	// body is the JSON body of a message.
	type body struct {
		Extras   map[string]any `json:"extras"`
		Priority *int           `json:"priority,omitzero"`
		Message  string         `json:"message"`
		Title    string         `json:"title"`
	}

	payload := body{Message: r.Message, Title: r.Title, Priority: &r.Priority, Extras: r.Extras}
	if r.omitPriority {
		payload.Priority = nil
	}

	data, err := json.Marshal(payload, jsonopts.Default)
	if err != nil {
		return nil, fmt.Errorf("encoding message: %w", err)
	}

	return data, nil
}
