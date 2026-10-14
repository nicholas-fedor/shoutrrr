package bark

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

// TestAPIResponseError verifies that an API response reports the server's
// message as its error text.
func TestAPIResponseError(t *testing.T) {
	t.Parallel()

	response := &APIResponse{Code: 400, Message: "failed to get device token"}

	assert.EqualError(t, response, "server response: failed to get device token")
}
