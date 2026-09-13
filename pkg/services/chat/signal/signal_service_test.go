package signal

import (
	"context"
	"errors"
	"net"
	"net/http"
	"net/url"

	"github.com/jarcoal/httpmock"
	"github.com/onsi/ginkgo/v2"
	"github.com/onsi/gomega"

	"github.com/nicholas-fedor/shoutrrr/internal/meta"
	"github.com/nicholas-fedor/shoutrrr/pkg/types"
)

var _ = ginkgo.Describe("service", func() {
	var signal *Service

	ginkgo.BeforeEach(func() {
		signal = &Service{}
	})

	ginkgo.It("should return the correct service ID", func() {
		gomega.Expect(signal.GetID()).To(gomega.Equal("signal"))
	})

	ginkgo.Describe("custom HTTP client", func() {
		ginkgo.It("should keep an injected client through Initialize", func() {
			stub := &stubHTTPClient{}
			signal.SetHTTPClient(stub)

			serviceURL, err := url.Parse("signal://localhost:8080/+1234567890/+0987654321")
			gomega.Expect(err).NotTo(gomega.HaveOccurred())
			gomega.Expect(signal.Initialize(serviceURL, logger)).NotTo(gomega.HaveOccurred())
			gomega.Expect(signal.httpClient).To(gomega.BeIdenticalTo(stub))
		})

		ginkgo.It("should use a client injected after Initialize", func() {
			serviceURL, err := url.Parse("signal://localhost:8080/+1234567890/+0987654321")
			gomega.Expect(err).NotTo(gomega.HaveOccurred())
			gomega.Expect(signal.Initialize(serviceURL, logger)).NotTo(gomega.HaveOccurred())

			stub := &stubHTTPClient{}
			signal.SetHTTPClient(stub)

			gomega.Expect(signal.Send("hi", nil)).NotTo(gomega.HaveOccurred())
			gomega.Expect(stub.req).NotTo(gomega.BeNil())
			gomega.Expect(stub.req.URL.Path).To(gomega.Equal("/v2/send"))
		})

		ginkgo.It("should use DialContext from an injected HTTP client", func() {
			serviceURL, err := url.Parse("signal://localhost:8080/+1234567890/+0987654321")
			gomega.Expect(err).NotTo(gomega.HaveOccurred())
			gomega.Expect(signal.Initialize(serviceURL, logger)).NotTo(gomega.HaveOccurred())

			blocked := errors.New("destination blocked")

			signal.SetHTTPClient(&http.Client{
				Transport: &http.Transport{
					DialContext: func(_ context.Context, _, _ string) (net.Conn, error) {
						return nil, blocked
					},
				},
			})

			err = signal.Send("hi", nil)
			gomega.Expect(err).To(gomega.HaveOccurred())
			gomega.Expect(err).To(gomega.MatchError(blocked))
		})
	})

	ginkgo.Describe("sending the payload", func() {
		var captured capturedRequest

		ginkgo.BeforeEach(func() {
			httpmock.Activate()

			captured = capturedRequest{}
		})
		ginkgo.AfterEach(func() {
			httpmock.DeactivateAndReset()
		})

		ginkgo.It("should not report an error if the server accepts the payload", func() {
			initMocked(signal, "signal://localhost:8080/+1234567890/+0987654321")
			setupCapture(http.StatusOK, `{"timestamp": 1234567890}`, &captured)

			err := signal.Send("Test message", nil)
			gomega.Expect(err).NotTo(gomega.HaveOccurred())
			gomega.Expect(captured.userAgent).To(gomega.Equal(meta.UserAgent()))
			gomega.Expect(captured.payload.Message).To(gomega.Equal("Test message"))
			gomega.Expect(captured.payload.TextMode).To(gomega.BeEmpty())
			gomega.Expect(captured.payload.NotifySelf).To(gomega.BeNil())
		})

		ginkgo.It("should send text_mode styled from the URL", func() {
			initMocked(signal, "signal://localhost:8080/+1234567890/+0987654321?textmode=styled")
			setupCapture(http.StatusOK, `{"timestamp": 1}`, &captured)

			err := signal.Send("*hello*", nil)
			gomega.Expect(err).NotTo(gomega.HaveOccurred())
			gomega.Expect(captured.payload.TextMode).To(gomega.Equal("styled"))
			gomega.Expect(captured.payload.Message).To(gomega.Equal("*hello*"))
		})

		ginkgo.It("should send text_mode normal from the URL", func() {
			initMocked(signal, "signal://localhost:8080/+1234567890/+0987654321?textmode=normal")
			setupCapture(http.StatusOK, `{"timestamp": 1}`, &captured)

			err := signal.Send("plain", nil)
			gomega.Expect(err).NotTo(gomega.HaveOccurred())
			gomega.Expect(captured.payload.TextMode).To(gomega.Equal("normal"))
		})

		ginkgo.It("should apply textmode from send params", func() {
			initMocked(signal, "signal://localhost:8080/+1234567890/+0987654321")
			setupCapture(http.StatusOK, `{"timestamp": 1}`, &captured)

			params := types.Params{"textmode": "styled"}
			err := signal.Send("hi", &params)
			gomega.Expect(err).NotTo(gomega.HaveOccurred())
			gomega.Expect(captured.payload.TextMode).To(gomega.Equal("styled"))
		})

		ginkgo.It("should prepend a plain title", func() {
			initMocked(signal, "signal://localhost:8080/+1234567890/+0987654321?title=Alert")
			setupCapture(http.StatusOK, `{"timestamp": 1}`, &captured)

			err := signal.Send("body", nil)
			gomega.Expect(err).NotTo(gomega.HaveOccurred())
			gomega.Expect(captured.payload.Message).To(gomega.Equal("Alert\nbody"))
		})

		ginkgo.It("should bold the title when styled", func() {
			initMocked(
				signal,
				"signal://localhost:8080/+1234567890/+0987654321?title=Alert&textmode=styled",
			)
			setupCapture(http.StatusOK, `{"timestamp": 1}`, &captured)

			err := signal.Send("body", nil)
			gomega.Expect(err).NotTo(gomega.HaveOccurred())
			gomega.Expect(captured.payload.Message).To(gomega.Equal("**Alert**\nbody"))
		})

		ginkgo.It("should send a title-only message when the body is empty", func() {
			initMocked(signal, "signal://localhost:8080/+1234567890/+0987654321?title=Alert")
			setupCapture(http.StatusOK, `{"timestamp": 1}`, &captured)

			err := signal.Send("", nil)
			gomega.Expect(err).NotTo(gomega.HaveOccurred())
			gomega.Expect(captured.payload.Message).To(gomega.Equal("Alert"))
		})

		ginkgo.It("should send notify_self false when notifyself=no", func() {
			initMocked(signal, "signal://localhost:8080/+1234567890/+0987654321?notifyself=no")
			setupCapture(http.StatusOK, `{"timestamp": 1}`, &captured)

			err := signal.Send("hi", nil)
			gomega.Expect(err).NotTo(gomega.HaveOccurred())
			gomega.Expect(captured.payload.NotifySelf).NotTo(gomega.BeNil())
			gomega.Expect(*captured.payload.NotifySelf).To(gomega.BeFalse())
		})

		ginkgo.It("should report an error if the server returns an error", func() {
			initMocked(signal, "signal://localhost:8080/+1234567890/+0987654321")
			setupCapture(http.StatusBadRequest, `{"error": "Bad Request"}`, &captured)

			err := signal.Send("Test message", nil)
			gomega.Expect(err).To(gomega.HaveOccurred())
			gomega.Expect(err.Error()).To(gomega.ContainSubstring("server returned status 400"))
			gomega.Expect(err.Error()).To(gomega.ContainSubstring("Bad Request"))
		})

		ginkgo.It("should include challenge tokens and account on 429", func() {
			initMocked(signal, "signal://localhost:8080/+1234567890/+0987654321")
			setupCapture(
				http.StatusTooManyRequests,
				`{"error":"rate limited","challenge_tokens":["abc","def"],"account":"+1234567890"}`,
				&captured,
			)

			err := signal.Send("Test message", nil)
			gomega.Expect(err).To(gomega.HaveOccurred())
			gomega.Expect(err.Error()).To(gomega.ContainSubstring("account +1234567890"))
			gomega.Expect(err.Error()).To(gomega.ContainSubstring("challenge tokens: abc,def"))
		})

		ginkgo.It("should handle attachments in parameters", func() {
			initMocked(signal, "signal://localhost:8080/+1234567890/+0987654321")
			setupCapture(http.StatusOK, `{"timestamp": 1234567890}`, &captured)

			params := types.Params{
				"attachments": "base64data1,base64data2",
			}

			err := signal.Send("Test message", &params)
			gomega.Expect(err).NotTo(gomega.HaveOccurred())
			gomega.Expect(captured.payload.Base64Attachments).
				To(gomega.Equal([]string{"base64data1", "base64data2"}))
		})

		ginkgo.It("should send a data URI as a single attachment", func() {
			initMocked(signal, "signal://localhost:8080/+1234567890/+0987654321")
			setupCapture(http.StatusOK, `{"timestamp": 1}`, &captured)

			params := types.Params{
				"attachments": "data:image/png;base64,iVBOR",
			}

			err := signal.Send("Test message", &params)
			gomega.Expect(err).NotTo(gomega.HaveOccurred())
			gomega.Expect(captured.payload.Base64Attachments).
				To(gomega.Equal([]string{"data:image/png;base64,iVBOR"}))
		})

		ginkgo.It("should handle different response formats", func() {
			initMocked(signal, "signal://localhost:8080/+1234567890/+0987654321")
			setupCapture(http.StatusCreated, `{"timestamp": "1234567890"}`, &captured)

			err := signal.Send("Test message", nil)
			gomega.Expect(err).NotTo(gomega.HaveOccurred())
		})

		ginkgo.It("should handle server errors gracefully", func() {
			initMocked(signal, "signal://localhost:8080/+1234567890/+0987654321")
			setupCapture(
				http.StatusInternalServerError,
				`{"error": "Internal Server Error"}`,
				&captured,
			)

			err := signal.Send("Test message", nil)
			gomega.Expect(err).To(gomega.HaveOccurred())
			gomega.Expect(err.Error()).To(gomega.ContainSubstring("server returned status 500"))
			gomega.Expect(err.Error()).To(gomega.ContainSubstring("Internal Server Error"))
		})

		ginkgo.It("should reject unknown send params", func() {
			initMocked(signal, "signal://localhost:8080/+1234567890/+0987654321")

			params := types.Params{"unknown": "value"}
			err := signal.Send("Test message", &params)
			gomega.Expect(err).To(gomega.HaveOccurred())
			gomega.Expect(err.Error()).To(gomega.ContainSubstring("not a valid config key"))
		})

		ginkgo.It("should return error when no recipients configured", func() {
			signal.Config = &Config{
				Host:       "localhost",
				Port:       8080,
				Source:     "+1234567890",
				Recipients: []string{},
			}

			err := signal.Send("Test message", nil)
			gomega.Expect(err).To(gomega.MatchError(ErrNoRecipients))
		})
	})
})
