package pushbullet

import (
	"io"
	"net/http"
	"strings"
	"testing"
)

// acceptingClient answers every request with HTTP 200 and a created push,
// without a network round trip. The reply declares its length, as
// Pushbullet's replies do, so reading it costs what reading a real reply costs.
type acceptingClient struct{}

// Do answers req with HTTP 200 and a created push.
//
// Parameters:
//   - req: the request.
//
// Returns:
//   - *http.Response: the reply.
//   - error: always nil.
func (acceptingClient) Do(req *http.Request) (*http.Response, error) {
	return &http.Response{
		StatusCode:    http.StatusOK,
		Status:        "200 OK",
		Body:          io.NopCloser(strings.NewReader(createdPush)),
		ContentLength: int64(len(createdPush)),
		Request:       req,
	}, nil
}

// BenchmarkSend measures a send to one target and to one target of each
// kind, through a client that accepts every push.
func BenchmarkSend(b *testing.B) {
	// targets lists the URL paths of each case.
	targets := []struct {
		name string
		path string
	}{
		{name: "one target", path: "/ujpah72o0sjAoRtnM0jc"},
		{name: "three targets", path: "/ujpah72o0sjAoRtnM0jc/alerts.team@example.com#deployments"},
	}

	for _, tt := range targets {
		b.Run(tt.name, func(b *testing.B) {
			service := newTestService(b, "pushbullet://"+testToken+tt.path)
			service.SetHTTPClient(acceptingClient{})

			b.ReportAllocs()

			for b.Loop() {
				if err := service.Send("Message", nil); err != nil {
					b.Fatal(err)
				}
			}
		})
	}
}

// BenchmarkSetTarget measures setting the target of a push, which each send
// does once for every target.
func BenchmarkSetTarget(b *testing.B) {
	// targets lists one target of each kind a URL can name.
	targets := []struct {
		name   string
		target string
	}{
		{name: "email", target: "alerts.team@example.com"},
		{name: "channel", target: "#deployments"},
		{name: "device", target: "ujpah72o0sjAoRtnM0jc"},
	}

	for _, tt := range targets {
		b.Run(tt.name, func(b *testing.B) {
			b.ReportAllocs()

			for b.Loop() {
				push := PushRequest{}
				push.SetTarget(tt.target)
			}
		})
	}
}
