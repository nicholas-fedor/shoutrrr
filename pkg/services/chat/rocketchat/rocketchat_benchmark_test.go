package rocketchat

import (
	"net/http"
	"testing"

	"github.com/nicholas-fedor/shoutrrr/pkg/types"
)

// acceptingClient answers every request with HTTP 200 without a network round
// trip. Rocket.Chat answers a webhook it accepts the same way.
type acceptingClient struct{}

// Do answers the request with HTTP 200 and no body, since a send reads the
// body only for an error status.
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
		Body:       http.NoBody,
		Request:    req,
	}, nil
}

// BenchmarkSendContext measures a send with a channel param. The client
// answers without the network, so the result is the work the service does
// around the request.
func BenchmarkSendContext(b *testing.B) {
	service := newTestService(b, nil)
	service.SetHTTPClient(acceptingClient{})

	params := types.Params{"channel": "#alerts"}

	b.ReportAllocs()

	for b.Loop() {
		if err := service.SendContext(b.Context(), "Backup finished", &params); err != nil {
			b.Fatal(err)
		}
	}
}
