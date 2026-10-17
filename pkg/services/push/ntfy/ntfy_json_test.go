package ntfy

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

// TestAPIResponseErrorError verifies that an ntfy error response reads as its
// message and code, with its documentation link when it has one.
func TestAPIResponseErrorError(t *testing.T) {
	t.Parallel()

	// tests lists error responses with the message each should produce.
	tests := []struct {
		name string
		want string
		err  apiResponseError
	}{
		{
			name: "with a link",
			err:  apiResponseError{Code: 40301, Message: "forbidden", Link: "https://ntfy.sh/docs/publish/#authentication"},
			want: "server response: forbidden (40301), see: https://ntfy.sh/docs/publish/#authentication",
		},
		{name: "without a link", err: apiResponseError{Code: 42901, Message: "limit reached: too many requests"}, want: "server response: limit reached: too many requests (42901)"},
		{name: "empty", want: "server response:  (0)"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			assert.EqualError(t, &tt.err, tt.want)
		})
	}
}
