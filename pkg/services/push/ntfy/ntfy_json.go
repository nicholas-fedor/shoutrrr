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
