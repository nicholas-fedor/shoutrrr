package telegram

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

// TestIsTokenValid verifies the bot token format. A token needs a numeric bot
// ID and a secret joined by one colon. The secret holds letters, digits,
// underscores, and dashes.
func TestIsTokenValid(t *testing.T) {
	t.Parallel()

	// tests lists tokens with whether each has a valid format.
	tests := []struct {
		name  string
		token string
		want  bool
	}{
		{name: "valid", token: "110201543:AAHdqTcvCH1vGWJxfSeofSAs0K5PALDsaw", want: true},
		{name: "underscores and dashes in the secret", token: "123:a_b-c", want: true},
		{name: "empty", token: ""},
		{name: "no colon", token: "110201543"},
		{name: "empty bot ID", token: ":secret"},
		{name: "bot ID not a number", token: "bot:secret"},
		{name: "empty secret", token: "123:"},
		{name: "secret with a space", token: "123:a b"},
		{name: "second colon", token: "123:abc:def"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			assert.Equal(t, tt.want, IsTokenValid(tt.token))
		})
	}
}
