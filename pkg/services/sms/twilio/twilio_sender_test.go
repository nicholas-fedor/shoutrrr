package twilio

import (
	"io"
	"net/http"
	"strings"

	"github.com/onsi/ginkgo/v2"
	"github.com/onsi/gomega"
)

var _ = ginkgo.Describe("Sender Unit Tests", func() {
	var (
		service    *Service
		mockClient *mockServiceHTTPClient
	)

	ginkgo.BeforeEach(func() {
		service = &Service{
			Config: &Config{
				AccountSID: "ACXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXX",
				AuthToken:  "authToken",
				FromNumber: "+15551234567",
				ToNumbers:  []string{"+15559876543"},
			},
			HTTPClient: &mockServiceHTTPClient{},
		}
		service.SetLogger(&mockLogger{})
		service.pkr = *initPKR(service.Config)
		mockClient = service.HTTPClient.(*mockServiceHTTPClient)
		mockClient.response = &http.Response{
			StatusCode: http.StatusCreated,
			Body:       io.NopCloser(strings.NewReader(`{"sid": "SM123"}`)),
		}
	})

	ginkgo.Describe("sendToRecipient", func() {
		ginkgo.It("should use MessagingServiceSid for MG-prefixed senders", func() {
			service.Config.FromNumber = "MGXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXX"
			mockClient.captureBody = true

			err := service.sendToRecipient(service.Config, "+15559876543", "Test")
			gomega.Expect(err).NotTo(gomega.HaveOccurred())
			gomega.Expect(mockClient.lastBody).To(gomega.ContainSubstring("MessagingServiceSid"))
			gomega.Expect(mockClient.lastBody).NotTo(gomega.ContainSubstring("From="))
		})

		ginkgo.It("should use From for regular phone numbers", func() {
			mockClient.captureBody = true

			err := service.sendToRecipient(service.Config, "+15559876543", "Test")
			gomega.Expect(err).NotTo(gomega.HaveOccurred())
			gomega.Expect(mockClient.lastBody).To(gomega.ContainSubstring("From="))
		})

		ginkgo.It("should set Basic Auth header", func() {
			mockClient.captureHeaders = true

			err := service.sendToRecipient(service.Config, "+15559876543", "Test")
			gomega.Expect(err).NotTo(gomega.HaveOccurred())

			username, password, ok := mockClient.lastRequest.BasicAuth()
			gomega.Expect(ok).To(gomega.BeTrue())
			gomega.Expect(username).To(gomega.Equal("ACXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXX"))
			gomega.Expect(password).To(gomega.Equal("authToken"))
		})

		ginkgo.It("should set the correct Content-Type header", func() {
			mockClient.captureHeaders = true

			err := service.sendToRecipient(service.Config, "+15559876543", "Test")
			gomega.Expect(err).NotTo(gomega.HaveOccurred())
			gomega.Expect(mockClient.lastRequest.Header.Get("Content-Type")).To(gomega.Equal(contentType))
		})
	})

	ginkgo.Describe("parseAPIError", func() {
		ginkgo.It("should parse a Twilio API error response", func() {
			res := &http.Response{
				StatusCode: http.StatusBadRequest,
				Status:     "400 Bad Request",
				Body:       io.NopCloser(strings.NewReader(`{"code": 21211, "message": "The 'To' number is not a valid phone number.", "status": 400}`)),
			}

			err := parseAPIError(res)
			gomega.Expect(err).To(gomega.HaveOccurred())
			gomega.Expect(err.Error()).To(gomega.ContainSubstring("The 'To' number is not a valid phone number."))
			gomega.Expect(err.Error()).To(gomega.ContainSubstring("21211"))
		})

		ginkgo.It("should handle non-JSON error response", func() {
			res := &http.Response{
				StatusCode: http.StatusInternalServerError,
				Status:     "500 Internal Server Error",
				Body:       io.NopCloser(strings.NewReader("Internal Server Error")),
			}

			err := parseAPIError(res)
			gomega.Expect(err).To(gomega.HaveOccurred())
			gomega.Expect(err.Error()).To(gomega.ContainSubstring("500"))
		})

		ginkgo.It("should handle empty body", func() {
			res := &http.Response{
				StatusCode: http.StatusBadRequest,
				Status:     "400 Bad Request",
				Body:       io.NopCloser(strings.NewReader("")),
			}

			err := parseAPIError(res)
			gomega.Expect(err).To(gomega.HaveOccurred())
		})
	})
})
