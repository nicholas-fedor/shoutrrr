package httpclient

import (
	"cmp"
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
// letter but name different hosts. A redirect that changes the method is
// refused unless it is a 303, which points to the response to the original
// request. A chain stops after ten requests.
func TestCheckRedirect(t *testing.T) {
	t.Parallel()

	// tests lists redirect chains, the earlier requests first, with the error
	// the last redirect gets. The earlier requests are POSTs unless viaMethod
	// names another method. The last request is a POST unless method names
	// another, and follows a 307 unless status names another redirect.
	tests := []struct {
		wantErr   error
		name      string
		target    string
		method    string
		viaMethod string
		via       []string
		status    int
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
			name:    "POST resent as GET after a 302",
			via:     []string{"https://gotify.example.com/message"},
			target:  "https://gotify.example.com/elsewhere",
			method:  http.MethodGet,
			status:  http.StatusFound,
			wantErr: ErrRedirectRefused,
		},
		{
			name:    "POST resent as GET after a 301",
			via:     []string{"https://gotify.example.com/message"},
			target:  "https://gotify.example.com/elsewhere",
			method:  http.MethodGet,
			status:  http.StatusMovedPermanently,
			wantErr: ErrRedirectRefused,
		},
		{
			name:   "POST result fetched after a 303",
			via:    []string{"https://gotify.example.com/message"},
			target: "https://gotify.example.com/result",
			method: http.MethodGet,
			status: http.StatusSeeOther,
		},
		{
			name:      "PUT resent as GET after a 302",
			via:       []string{"https://gotify.example.com/message"},
			target:    "https://gotify.example.com/elsewhere",
			method:    http.MethodGet,
			viaMethod: http.MethodPut,
			status:    http.StatusFound,
			wantErr:   ErrRedirectRefused,
		},
		{
			name:      "GET after a 302",
			via:       []string{"https://gotify.example.com/version"},
			target:    "https://gotify.example.com/v2/version",
			method:    http.MethodGet,
			viaMethod: http.MethodGet,
			status:    http.StatusFound,
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
				via = append(via, httptest.NewRequestWithContext(t.Context(), cmp.Or(tt.viaMethod, http.MethodPost), rawURL, nil))
			}

			req := httptest.NewRequestWithContext(t.Context(), cmp.Or(tt.method, http.MethodPost), tt.target, nil)
			req.Response = &http.Response{StatusCode: cmp.Or(tt.status, http.StatusTemporaryRedirect)}

			err := checkRedirect(req, via)

			require.ErrorIs(t, err, tt.wantErr)
		})
	}
}

// TestCheckRedirectEmptyMethod verifies that a 302 after a request with an
// empty method, which net/http sends as a GET, is followed. net/http names the
// next request's method GET, so the method does not change.
func TestCheckRedirectEmptyMethod(t *testing.T) {
	t.Parallel()

	previous := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "https://gotify.example.com/version", nil)
	previous.Method = ""

	req := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "https://gotify.example.com/v2/version", nil)
	req.Response = &http.Response{StatusCode: http.StatusFound}

	require.NoError(t, checkRedirect(req, []*http.Request{previous}))
}
