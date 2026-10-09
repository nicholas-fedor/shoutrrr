package generic

import (
	"context"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/onsi/ginkgo/v2"
	"github.com/onsi/gomega"
	"github.com/stretchr/testify/mock"

	"github.com/nicholas-fedor/shoutrrr/internal/testutils"
	typesmocks "github.com/nicholas-fedor/shoutrrr/pkg/types/mocks"
)

var _ = ginkgo.Describe("send contexts", func() {
	var (
		service *Service
		client  *typesmocks.MockHTTPClient
	)

	ginkgo.BeforeEach(func() {
		service = &Service{}
		gomega.Expect(service.Initialize(
			testutils.URLMust("generic://hooks.example.invalid/webhook"),
			testutils.TestLogger(),
		)).To(gomega.Succeed())

		client = typesmocks.NewMockHTTPClient(ginkgo.GinkgoT())
		service.SetHTTPClient(client)
	})

	ginkgo.It("should bound a direct Send with the default send timeout", func() {
		var deadline time.Time

		client.EXPECT().Do(mock.Anything).RunAndReturn(func(req *http.Request) (*http.Response, error) {
			var ok bool

			deadline, ok = req.Context().Deadline()
			gomega.Expect(ok).To(gomega.BeTrue(), "the request must carry a deadline")

			return &http.Response{
				StatusCode: http.StatusOK,
				Status:     "200 OK",
				Body:       io.NopCloser(strings.NewReader("")),
			}, nil
		}).Once()

		start := time.Now()

		gomega.Expect(service.Send("message", nil)).To(gomega.Succeed())

		end := time.Now()

		gomega.Expect(deadline).To(gomega.BeTemporally(">=", start.Add(defaultSendTimeout)))
		gomega.Expect(deadline).To(gomega.BeTemporally("<=", end.Add(defaultSendTimeout)))
	})

	ginkgo.It("should stop the request when the caller's context is canceled", func() {
		ctx, cancel := context.WithCancel(context.Background())
		cancel()

		client.EXPECT().Do(mock.Anything).RunAndReturn(func(req *http.Request) (*http.Response, error) {
			return nil, req.Context().Err()
		}).Once()

		gomega.Expect(service.SendContext(ctx, "message", nil)).To(gomega.MatchError(context.Canceled))
	})
})
