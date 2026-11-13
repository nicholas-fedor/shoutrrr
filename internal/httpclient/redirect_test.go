package httpclient

import (
	"net/http"
	"net/http/httptest"
	"slices"
	"testing"

	"github.com/stretchr/testify/require"
)

// TestCheckRedirect verifies that a redirect is followed on the first
// request's host name, also from HTTP to HTTPS and to another port, and that a
// redirect to another host or from HTTPS to HTTP is refused. Host names match
// ignoring the case of ASCII letters only, because σ and ς fold to the same
// letter but name different hosts. A chain stops after ten requests.
func TestCheckRedirect(t *testing.T) {
	t.Parallel()

	// tests lists redirect chains, the earlier requests first, with the error
	// the last redirect gets.
	tests := []struct {
		wantErr error
		name    string
		target  string
		via     []string
	}{
		{name: "same host", via: []string{"https://gotify.example.com/message"}, target: "https://gotify.example.com/gotify/message"},
		{name: "host in another case", via: []string{"https://gotify.example.com/message"}, target: "https://Gotify.Example.COM/message"},
		{name: "HTTP to HTTPS", via: []string{"http://gotify.example.com/message"}, target: "https://gotify.example.com/message"},
		{name: "another port", via: []string{"http://gotify.example.com:8080/message"}, target: "https://gotify.example.com:8443/message"},
		{name: "another host", via: []string{"https://gotify.example.com/message"}, target: "https://other.example.com/message", wantErr: ErrRedirectRefused},
		{name: "another Unicode host in another case", via: []string{"https://\u03c3.example.com/message"}, target: "https://\u03c2.example.com/message", wantErr: ErrRedirectRefused},
		{name: "subdomain", via: []string{"https://example.com/message"}, target: "https://gotify.example.com/message", wantErr: ErrRedirectRefused},
		{name: "HTTPS to HTTP", via: []string{"https://gotify.example.com/message"}, target: "http://gotify.example.com/message", wantErr: ErrRedirectRefused},
		{
			name:    "HTTPS to HTTP after an upgrade",
			via:     []string{"http://gotify.example.com/message", "https://gotify.example.com/message"},
			target:  "http://gotify.example.com/message",
			wantErr: ErrRedirectRefused,
		},
		{
			name:    "ten requests",
			via:     slices.Repeat([]string{"https://gotify.example.com/message"}, 10),
			target:  "https://gotify.example.com/message",
			wantErr: errTooManyRedirects,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			via := make([]*http.Request, 0, len(tt.via))
			for _, rawURL := range tt.via {
				via = append(via, httptest.NewRequestWithContext(t.Context(), http.MethodPost, rawURL, nil))
			}

			err := checkRedirect(httptest.NewRequestWithContext(t.Context(), http.MethodPost, tt.target, nil), via)

			require.ErrorIs(t, err, tt.wantErr)
		})
	}
}
