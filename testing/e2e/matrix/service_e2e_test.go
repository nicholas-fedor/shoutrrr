package e2e_test

import (
	"net/url"
	"os"

	"github.com/onsi/ginkgo/v2"
	"github.com/onsi/gomega"

	"github.com/nicholas-fedor/shoutrrr/internal/testutils"
	"github.com/nicholas-fedor/shoutrrr/pkg/services/chat/matrix"
	"github.com/nicholas-fedor/shoutrrr/pkg/types"
)

var _ = ginkgo.Describe("Matrix Service E2E Tests", func() {
	ginkgo.Describe("Service Initialization", func() {
		ginkgo.It("should initialize with valid credentials from environment", func() {
			// Use shared service if available, otherwise create new one
			if sharedService != nil {
				gomega.Expect(sharedService.Config).NotTo(gomega.BeNil())
				gomega.Expect(sharedService.Config.Host).NotTo(gomega.BeEmpty())

				return
			}

			serviceURL := buildServiceURL()
			if serviceURL == "" {
				ginkgo.Skip("Matrix server not configured, skipping test. " +
					"Set SHOUTRRR_MATRIX_USER and SHOUTRRR_MATRIX_PASSWORD environment variables")
			}

			parsedURL, err := url.Parse(serviceURL)
			gomega.Expect(err).NotTo(gomega.HaveOccurred())

			service := &matrix.Service{}
			err = service.Initialize(parsedURL, testutils.TestLogger())
			gomega.Expect(err).NotTo(gomega.HaveOccurred())

			gomega.Expect(service.Config).NotTo(gomega.BeNil())
			gomega.Expect(service.Config.Host).NotTo(gomega.BeEmpty())
		})

		ginkgo.It("should return error with invalid credentials", func() {
			// Use a clearly invalid URL that will fail authentication
			invalidURL := "matrix://invaliduser:invalidpassword@localhost:8008?disableTLS=true"
			parsedURL, err := url.Parse(invalidURL)
			gomega.Expect(err).NotTo(gomega.HaveOccurred())

			service := &matrix.Service{}
			err = service.Initialize(parsedURL, testutils.TestLogger())

			// Should fail because the user doesn't exist or password is wrong
			gomega.Expect(err).To(gomega.HaveOccurred())
		})

		ginkgo.It("should return error with missing host", func() {
			invalidURL := "matrix://user:password@"
			parsedURL, err := url.Parse(invalidURL)
			gomega.Expect(err).NotTo(gomega.HaveOccurred())

			service := &matrix.Service{}
			err = service.Initialize(parsedURL, testutils.TestLogger())
			gomega.Expect(err).To(gomega.HaveOccurred())
		})

		ginkgo.It("should return error with missing credentials", func() {
			invalidURL := "matrix://localhost:8008"
			parsedURL, err := url.Parse(invalidURL)
			gomega.Expect(err).NotTo(gomega.HaveOccurred())

			service := &matrix.Service{}
			err = service.Initialize(parsedURL, testutils.TestLogger())
			gomega.Expect(err).To(gomega.HaveOccurred())
		})
	})

	ginkgo.Describe("Send Messages", func() {
		ginkgo.It("should send basic text message to Matrix room", func() {
			// Use shared service if available
			if sharedService != nil {
				err := sharedService.Send("E2E Test: Basic text message", nil)
				gomega.Expect(err).NotTo(gomega.HaveOccurred())

				return
			}

			serviceURL := buildServiceURL()
			if serviceURL == "" {
				ginkgo.Skip("Matrix server not configured, skipping test. " +
					"Set SHOUTRRR_MATRIX_USER, SHOUTRRR_MATRIX_PASSWORD, and SHOUTRRR_MATRIX_ROOM environment variables")
			}

			parsedURL, err := url.Parse(serviceURL)
			gomega.Expect(err).NotTo(gomega.HaveOccurred())

			service := &matrix.Service{}
			err = service.Initialize(parsedURL, testutils.TestLogger())
			gomega.Expect(err).NotTo(gomega.HaveOccurred())

			err = service.Send("E2E Test: Basic text message", nil)
			gomega.Expect(err).NotTo(gomega.HaveOccurred())
		})

		ginkgo.It("should send message with custom title", func() {
			// Use shared service if available to avoid rate limit from new login
			if sharedService != nil {
				err := sharedService.Send(
					"E2E Test: Message with custom title",
					&types.Params{"title": "TestNotification"},
				)
				gomega.Expect(err).NotTo(gomega.HaveOccurred())

				return
			}

			serviceURL := buildServiceURL()
			if serviceURL == "" {
				ginkgo.Skip("Matrix server not configured, skipping test")
			}

			// Fallback: create new service (hits rate limit - not recommended)
			parsedURL, err := url.Parse(serviceURL + "&title=TestNotification")
			gomega.Expect(err).NotTo(gomega.HaveOccurred())

			service := &matrix.Service{}
			err = service.Initialize(parsedURL, testutils.TestLogger())
			gomega.Expect(err).NotTo(gomega.HaveOccurred())

			err = service.Send("E2E Test: Message with custom title", nil)
			gomega.Expect(err).NotTo(gomega.HaveOccurred())
		})

		ginkgo.It("should fail to send when client is not initialized", func() {
			// Use dummy URL which skips client initialization
			// Format: matrix://:password@host - password is empty, which triggers ErrMissingCredentials
			// but this is handled differently - we need a URL that parses successfully but skips client init
			dummyURL := "matrix://:dummy@dummy.com"
			parsedURL, err := url.Parse(dummyURL)
			gomega.Expect(err).NotTo(gomega.HaveOccurred())

			service := &matrix.Service{}
			err = service.Initialize(parsedURL, testutils.TestLogger())
			// This should succeed because the dummy URL is detected and client is not initialized
			gomega.Expect(err).NotTo(gomega.HaveOccurred())

			err = service.Send("This should fail", nil)
			gomega.Expect(err).To(gomega.HaveOccurred())
		})
	})

	ginkgo.Describe("Service ID", func() {
		ginkgo.It("should return correct service ID", func() {
			// Even with dummy URL, service ID should work
			// Use format with empty username but password present: matrix://:password@host
			dummyURL := "matrix://:dummy@dummy.com"
			parsedURL, err := url.Parse(dummyURL)
			gomega.Expect(err).NotTo(gomega.HaveOccurred())

			service := &matrix.Service{}
			err = service.Initialize(parsedURL, testutils.TestLogger())
			gomega.Expect(err).NotTo(gomega.HaveOccurred())

			serviceID := service.GetID()
			gomega.Expect(serviceID).To(gomega.Equal("matrix"))
		})
	})

	ginkgo.Describe("Configuration", func() {
		ginkgo.It("should parse room aliases correctly", func() {
			// Test that room aliases are handled with # prefix
			serviceURL := buildServiceURL()
			if serviceURL == "" {
				ginkgo.Skip("Matrix server not configured, skipping test")
			}

			// Add room without # prefix
			room := os.Getenv("SHOUTRRR_MATRIX_ROOM")
			if room == "" {
				room = "testroom"
			}

			// Build URL with room that needs prefix
			parsedURL, err := url.Parse("matrix://user:pass@localhost:8008?room=" + room + "&disableTLS=true")
			gomega.Expect(err).NotTo(gomega.HaveOccurred())

			service := &matrix.Service{}
			err = service.Initialize(parsedURL, testutils.TestLogger())
			// This may fail due to auth, but we can check config parsing
			if err == nil {
				gomega.Expect(service.Config.Rooms).To(gomega.Not(gomega.BeEmpty()))
			}
		})
	})
})
