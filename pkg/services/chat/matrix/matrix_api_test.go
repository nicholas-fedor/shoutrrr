package matrix

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

// TestAPIResErrorMessage verifies that a Matrix API error reports its message,
// whatever its error code.
func TestAPIResErrorMessage(t *testing.T) {
	t.Parallel()

	// tests lists API errors with the message each should report.
	tests := []struct {
		name string
		err  *apiResError
		want string
	}{
		{name: "message and code", err: &apiResError{Message: "test error message", Code: "M_UNKNOWN"}, want: "test error message"},
		{name: "code only", err: &apiResError{Code: "M_NOT_FOUND"}, want: ""},
		{name: "message only", err: &apiResError{Message: "custom error"}, want: "custom error"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			assert.Equal(t, tt.want, tt.err.Error())
		})
	}
}

// TestNewUserIdentifier verifies that a user identifier carries the user and
// the user identifier type.
func TestNewUserIdentifier(t *testing.T) {
	t.Parallel()

	for _, user := range []string{"testuser", ""} {
		id := newUserIdentifier(user)

		assert.Equal(t, user, id.User)
		assert.Equal(t, idTypeUser, id.Type)
	}
}
