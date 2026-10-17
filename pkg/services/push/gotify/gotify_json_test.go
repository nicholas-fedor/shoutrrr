package gotify

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

// TestResponseErrorError verifies that a Gotify error response reads as its
// name, code, and description.
func TestResponseErrorError(t *testing.T) {
	t.Parallel()

	err := &responseError{Name: "Unauthorized", Code: 401, Description: "you need to provide a valid access token or user credentials to access this api"}

	assert.EqualError(t, err, "server responded with Unauthorized (401): you need to provide a valid access token or user credentials to access this api")
}
