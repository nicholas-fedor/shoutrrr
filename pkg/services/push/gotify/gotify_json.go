package gotify

import (
	"encoding/json/jsontext"
	"encoding/json/v2"
	"fmt"
)

// MessageRequest is a message for the Gotify message endpoint, sent as the
// JSON body of the request.
type MessageRequest struct {
	// Extras holds the custom key-value pairs sent with the message, left out
	// when nil.
	Extras Extras `json:"extras,omitzero"`

	// Date is not sent.
	//
	// Deprecated: Gotify stamps each message with the server time, so a date has no effect.
	Date *string `json:"-"`

	// Priority is the message priority, from -2 to 10. It is left out when nil,
	// so Gotify applies the application's default priority.
	Priority *int `json:"priority,omitzero"`
	// Message is the message text.
	Message string `json:"message"`
	// Title is the message title.
	Title string `json:"title"`
}

// Extras holds the custom key-value pairs of a message, such as
// client::display.
type Extras map[string]any

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

// MarshalJSONTo writes the extras as a JSON object with the encoder's options.
// Nil extras are written as null.
//
// Parameters:
//   - enc: the encoder to write the extras to.
//
// Returns:
//   - error: an error encoding a value of the extras.
func (e *Extras) MarshalJSONTo(enc *jsontext.Encoder) error {
	// As an any, the map takes json's path for a map[string]any. That path
	// skips the allocation for each entry that json's general map encoding
	// makes.
	var extras any = map[string]any(*e)

	return json.MarshalEncode(enc, &extras) //nolint:wrapcheck // json adds the value's path to its own error, and a wrapped one would hide it.
}
