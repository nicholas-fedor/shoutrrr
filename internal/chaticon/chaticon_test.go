package chaticon

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

// TestIsURL verifies that an icon is a URL only when it starts with an http or
// https scheme in either ASCII case. Any other icon is an emoji name. That
// holds for text before the scheme and for a scheme without slashes. It also
// holds for another scheme and for look-alike letters such as the long s.
func TestIsURL(t *testing.T) {
	t.Parallel()

	// tests lists icons with whether each is a URL.
	tests := []struct {
		name string
		icon string
		want bool
	}{
		{name: "https URL", icon: "https://example.com/logo.png", want: true},
		{name: "http URL", icon: "http://example.com/logo.png", want: true},
		{name: "upper-case scheme", icon: "HTTPS://example.com/logo.png", want: true},
		{name: "scheme alone", icon: "http://", want: true},
		{name: "URL after text", icon: "logo at https://example.com/logo.png"},
		{name: "scheme without slashes", icon: "http:logo"},
		{name: "https scheme cut short", icon: "https:/"},
		{name: "ftp URL", icon: "ftp://example.com/logo.png"},
		{name: "long s in the scheme", icon: "httpſ://example.com/logo.png"},
		{name: "emoji", icon: ":tanabata_tree:"},
		{name: "empty", icon: ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			assert.Equal(t, tt.want, IsURL(tt.icon))
		})
	}
}
