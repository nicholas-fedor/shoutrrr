package jsonclient

import (
	"errors"
	"fmt"
)

// Error contains additional HTTP/JSON details.
type Error struct {
	StatusCode int
	Body       string
	err        error
}

// Error returns the string representation of the error.
func (je Error) Error() string {
	return je.String()
}

// String provides a human-readable description of the error.
func (je Error) String() string {
	if je.err == nil {
		return fmt.Sprintf("unknown error (HTTP %v)", je.StatusCode)
	}

	return je.err.Error()
}

// ErrorBody extracts the request body from an error if it’s a jsonclient.Error.
func ErrorBody(e error) string {
	var jsonError Error
	if errors.As(e, &jsonError) {
		return jsonError.Body
	}

	return ""
}
