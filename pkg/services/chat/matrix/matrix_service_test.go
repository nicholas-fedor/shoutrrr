package matrix

import (
	"net/url"

	"github.com/onsi/gomega"

	ginkgo "github.com/onsi/ginkgo/v2"
)

var _ = ginkgo.Describe("Service", func() {
	ginkgo.Describe("GetID", func() {
		ginkgo.It("should return the matrix scheme identifier", func() {
			svc := &Service{}
			gomega.Expect(svc.GetID()).To(gomega.Equal(Scheme))
		})

		ginkgo.It("should always return 'matrix' regardless of service state", func() {
			svc := &Service{
				Config: &Config{
					Host: "matrix.example.com",
				},
			}
			gomega.Expect(svc.GetID()).To(gomega.Equal("matrix"))
		})
	})

	ginkgo.Describe("Initialize", func() {
		var svc *Service

		ginkgo.BeforeEach(func() {
			svc = &Service{}
		})

		ginkgo.Context("with invalid URL", func() {
			ginkgo.It("should return error when host is missing", func() {
				invalidURL := &url.URL{
					Scheme: "matrix",
					User:   url.UserPassword("user", "password"),
				}
				err := svc.Initialize(invalidURL, nil)
				gomega.Expect(err).To(gomega.MatchError(ErrMissingHost))
			})

			ginkgo.It("should return error when password is missing", func() {
				invalidURL := &url.URL{
					Scheme: "matrix",
					Host:   "matrix.example.com",
					User:   url.User("user"),
				}
				err := svc.Initialize(invalidURL, nil)
				gomega.Expect(err).To(gomega.MatchError(ErrMissingCredentials))
			})
		})

		ginkgo.Context("with valid URL but no client creation", func() {
			// Test that we can set config without triggering client initialization
			// The dummy URL check happens after config parsing, so we test config validation separately
			ginkgo.It("should reject URL without user info when credentials required", func() {
				// URL with host but no user - should fail at config validation
				invalidURL := &url.URL{
					Scheme: "matrix",
					Host:   "matrix.example.com",
				}
				err := svc.Initialize(invalidURL, nil)
				gomega.Expect(err).To(gomega.MatchError(ErrMissingCredentials))
			})
		})
	})

	ginkgo.Describe("Send", func() {
		var svc *Service

		ginkgo.BeforeEach(func() {
			svc = &Service{}
		})

		ginkgo.Context("when client is not initialized", func() {
			ginkgo.It("should return ErrClientNotInitialized", func() {
				svc.Config = &Config{}
				err := svc.Send("test message", nil)
				gomega.Expect(err).To(gomega.MatchError(ErrClientNotInitialized))
			})
		})

		ginkgo.Context("with initialized service but no client", func() {
			ginkgo.BeforeEach(func() {
				// Set config directly without initializing client
				svc.Config = &Config{
					Host: "matrix.example.com",
				}
				// Forcefully set client to nil to test the error path
				svc.client = nil
			})

			ginkgo.It("should return error when client is nil", func() {
				err := svc.Send("test message", nil)
				gomega.Expect(err).To(gomega.MatchError(ErrClientNotInitialized))
			})
		})
	})
})
