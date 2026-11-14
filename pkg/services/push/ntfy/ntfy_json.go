package ntfy

import "fmt"

// apiResponseError is the error body the ntfy server returns for a failed
// publish.
type apiResponseError struct {
	// Message describes the error.
	Message string `json:"error"`
	// Link is the URL of documentation about the error, or empty.
	Link string `json:"link"`
	// Code is ntfy's error code, such as 40301.
	Code int64 `json:"code"`
}

// publishRequest is the JSON body of a publish to a server's root, which
// names the topic in the body instead of the request path.
type publishRequest struct {
	// Topic is the topic name.
	Topic string `json:"topic"`
	// Message is the message text.
	Message string `json:"message"`
	// Markdown reports whether the message is formatted as Markdown.
	Markdown bool `json:"markdown,omitzero"`
}

// Error returns the server's message with its code and documentation link.
//
// Returns:
//   - string: the error message.
func (e *apiResponseError) Error() string {
	msg := fmt.Sprintf("server response: %v (%v)", e.Message, e.Code)
	if e.Link != "" {
		return msg + ", see: " + e.Link
	}

	return msg
}
