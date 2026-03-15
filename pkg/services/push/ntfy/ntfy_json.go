package ntfy

import "fmt"

type apiResponseError struct {
	Code    int64  `json:"code"`
	Message string `json:"error"`
	Link    string `json:"link"`
}

func (e *apiResponseError) Error() string {
	msg := fmt.Sprintf("server response: %v (%v)", e.Message, e.Code)
	if e.Link != "" {
		return msg + ", see: " + e.Link
	}

	return msg
}
