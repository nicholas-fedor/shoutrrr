package bark

// PushPayload represents the JSON payload sent to the Bark server API.
// This structure contains all notification parameters that can be customized.
type PushPayload struct {
	Body      string `json:"body"`
	DeviceKey string `json:"device_key"`
	Title     string `json:"title"`
	Sound     string `json:"sound,omitempty"`
	Badge     *int64 `json:"badge,omitempty"`
	Icon      string `json:"icon,omitempty"`
	Group     string `json:"group,omitempty"`
	URL       string `json:"url,omitempty"`
	Category  string `json:"category,omitempty"`
	Copy      string `json:"copy,omitempty"`
}

// APIResponse represents the response structure returned by the Bark server API.
// The server responds with a JSON object containing status code, message, and timestamp.
//
//nolint:errname // APIResponse name is mandated by the Bark API response format
type APIResponse struct {
	Code      int64  `json:"code"`
	Message   string `json:"message"`
	Timestamp int64  `json:"timestamp"`
}

// Error returns the error message from the Bark API response.
// This method implements the error interface for APIResponse.
//
// Returns:
//   - A formatted error string containing the server message.
func (e *APIResponse) Error() string {
	return "server response: " + e.Message
}
