package gotify

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestBuildURL verifies that the message endpoint uses HTTPS unless TLS is
// disabled, keeps the port and subpath, and carries the token in the query
// unless the token goes in a header.
func TestBuildURL(t *testing.T) {
	t.Parallel()

	// tests lists configs with the endpoint each should produce.
	tests := []struct {
		name   string
		config Config
		want   string
	}{
		{name: "token in the query", config: Config{Host: testHost, Token: testToken}, want: "https://" + testHost + "/message?token=" + testToken},
		{name: "TLS disabled", config: Config{Host: testHost, Token: testToken, DisableTLS: true}, want: "http://" + testHost + "/message?token=" + testToken},
		{name: "port", config: Config{Host: testHost + ":8443", Token: testToken}, want: "https://" + testHost + ":8443/message?token=" + testToken},
		{name: "subpath", config: Config{Host: testHost, Path: "/gotify/", Token: testToken}, want: "https://" + testHost + "/gotify/message?token=" + testToken},
		{
			name:   "subpath without a trailing slash",
			config: Config{Host: testHost, Path: "/push/gotify", Token: testToken, UseHeader: true},
			want:   "https://" + testHost + "/push/gotify/message",
		},
		{name: "token in a header", config: Config{Host: testHost, Token: testToken, UseHeader: true}, want: "https://" + testHost + "/message"},
		{name: "Gotify 3 token", config: Config{Host: testHost, Token: enhancedToken}, want: "https://" + testHost + "/message?token=" + enhancedToken},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			apiURL, err := (&DefaultURLBuilder{}).BuildURL(&tt.config)

			require.NoError(t, err)
			assert.Equal(t, tt.want, apiURL)
		})
	}
}
