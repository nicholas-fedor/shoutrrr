package slack

import (
	"io"
	"net/http"
	"strconv"
	"strings"
	"testing"
)

// acceptingClient answers every request with HTTP 200 and body, without a
// network round trip. The reply declares its length, as Slack's replies do,
// so reading it costs what reading a real reply costs.
type acceptingClient struct {
	// body is the reply body.
	body string
}

// Do answers req with HTTP 200 and the client's body.
//
// Parameters:
//   - req: the request.
//
// Returns:
//   - *http.Response: the reply.
//   - error: always nil.
func (c acceptingClient) Do(req *http.Request) (*http.Response, error) {
	return &http.Response{
		StatusCode:    http.StatusOK,
		Status:        "200 OK",
		Body:          io.NopCloser(strings.NewReader(c.body)),
		ContentLength: int64(len(c.body)),
		Request:       req,
	}, nil
}

// BenchmarkCreateJSONPayload measures building a payload, which each send
// does once.
func BenchmarkCreateJSONPayload(b *testing.B) {
	config := &Config{Title: "Backup", Color: "good", Channel: "C0123456789"}

	for _, lineCount := range []int{1, 10, 100, 1000} {
		message := strings.TrimSuffix(strings.Repeat("Backup of volume finished\n", lineCount), "\n")

		b.Run(strconv.Itoa(lineCount), func(b *testing.B) {
			b.ReportAllocs()

			for b.Loop() {
				CreateJSONPayload(config, message)
			}
		})
	}
}

// BenchmarkToken measures the request forms of a token read through
// SetFromProp, which each API send and each webhook send asks for once.
func BenchmarkToken(b *testing.B) {
	bot := parsedToken(b, "xoxb:123456789012-1234567890123-4mt0t4l1YL3g1T5L4cK70k3N")
	hook := parsedToken(b, "hook:AAAAAAAAA-BBBBBBBBB-123456789123456789123456")

	b.Run("authorization", func(b *testing.B) {
		b.ReportAllocs()

		for b.Loop() {
			_ = bot.Authorization()
		}
	})

	b.Run("webhook_url", func(b *testing.B) {
		b.ReportAllocs()

		for b.Loop() {
			_ = hook.WebhookURL()
		}
	})
}

// BenchmarkSend measures an API send and a webhook send through a client
// that accepts each request, so only the service's own work counts.
func BenchmarkSend(b *testing.B) {
	for _, tc := range []struct {
		name   string
		rawURL string
		body   string
	}{
		{name: "api", rawURL: testAPIServiceURL, body: `{"ok":true}`},
		{name: "webhook", rawURL: testWebhookServiceURL, body: "ok"},
	} {
		b.Run(tc.name, func(b *testing.B) {
			service := newTestService(b, tc.rawURL, acceptingClient{body: tc.body}, io.Discard)

			b.ReportAllocs()

			for b.Loop() {
				if err := service.Send("Backup finished", nil); err != nil {
					b.Fatal(err)
				}
			}
		})
	}
}
