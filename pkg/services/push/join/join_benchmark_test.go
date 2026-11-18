package join

import (
	"io"
	"net/http"
	"strings"
	"testing"

	"github.com/nicholas-fedor/shoutrrr/pkg/types"
)

// acceptingClient answers every request with HTTP 200 and the reply Join
// gives an accepted push, without a network round trip.
type acceptingClient struct{}

// Do answers the request with HTTP 200 and a reply that reports success.
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
		Body:       io.NopCloser(strings.NewReader(acceptedBody)),
		Request:    req,
	}, nil
}

// BenchmarkSendContext measures a send with a title param. The client
// answers without the network, so the result is the work the service does
// around the request.
func BenchmarkSendContext(b *testing.B) {
	service := newTestService(b, "join://Token:apikey@join?devices=dev1")
	service.SetHTTPClient(acceptingClient{})

	params := types.Params{types.TitleKey: "Backup"}

	b.ReportAllocs()

	for b.Loop() {
		if err := service.SendContext(b.Context(), "Backup finished", &params); err != nil {
			b.Fatal(err)
		}
	}
}
