package jsonclient

import (
	"context"
	"io"
	"net/http"
	"strings"
	"testing"
)

// acceptingClient answers every request with HTTP 200 and an empty JSON
// object, without a network round trip.
type acceptingClient struct{}

// Do answers req with HTTP 200 and an empty JSON object.
//
// Parameters:
//   - req: the request.
//
// Returns:
//   - *http.Response: the reply.
//   - error: always nil.
func (acceptingClient) Do(req *http.Request) (*http.Response, error) {
	return &http.Response{
		Status:     "200 OK",
		StatusCode: http.StatusOK,
		Body:       io.NopCloser(strings.NewReader(`{}`)),
		Request:    req,
	}, nil
}

// BenchmarkRequest measures a GET and a POST through a client that holds an
// authentication header beside its content type, as the services' clients
// do. The POST body is built once, as a send builds it before its requests.
func BenchmarkRequest(b *testing.B) {
	// push is the POST body.
	push := &struct {
		Type  string `json:"type"`
		Title string `json:"title"`
		Body  string `json:"body"`
	}{Type: "note", Title: "Title", Body: "Message"}

	// requests lists the request of each case.
	requests := []struct {
		name string
		send func(context.Context, ContextClient) error
	}{
		{
			name: "get",
			send: func(ctx context.Context, c ContextClient) error {
				return c.GetContext(ctx, "https://api.example.com/v2/users/me", &struct{}{})
			},
		},
		{
			name: "post",
			send: func(ctx context.Context, c ContextClient) error {
				return c.PostContext(ctx, "https://api.example.com/v2/pushes", push, &struct{}{})
			},
		},
	}

	for _, tt := range requests {
		b.Run(tt.name, func(b *testing.B) {
			client := NewContextClient(acceptingClient{})
			client.Headers().Set("Access-Token", "o.abcdefghijklmnopqrstuvwxyz012345")

			b.ReportAllocs()

			for b.Loop() {
				if err := tt.send(b.Context(), client); err != nil {
					b.Fatal(err)
				}
			}
		})
	}
}
