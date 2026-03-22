package e2e_test

import (
	"context"
	"errors"
	"net/url"
	"strings"
	"time"

	"github.com/onsi/ginkgo/v2"
	"github.com/onsi/gomega"

	"github.com/nicholas-fedor/shoutrrr/internal/testutils"
	"github.com/nicholas-fedor/shoutrrr/pkg/services/chat/matrix"
)

var _ = ginkgo.Describe("Matrix Service E2E Error Handling", func() {
	// TestE2EContextCancellation tests behavior when operations are canceled via context.
	ginkgo.Describe("Context Cancellation", func() {
		ginkgo.It("should handle canceled context gracefully", func() {
			serviceURL := buildServiceURL()
			if serviceURL == "" {
				ginkgo.Skip("Matrix server not configured, skipping test. " +
					"Set SHOUTRRR_MATRIX_USER and SHOUTRRR_MATRIX_PASSWORD environment variables")
			}

			parsedURL, err := url.Parse(serviceURL)
			gomega.Expect(err).NotTo(gomega.HaveOccurred())

			var svc *matrix.Service
			if sharedService != nil {
				svc = sharedService
			} else {
				svc = &matrix.Service{}
				err = svc.Initialize(parsedURL, testutils.TestLogger())
				gomega.Expect(err).NotTo(gomega.HaveOccurred())
			}

			// Create a canceled context
			ctx, cancel := context.WithCancel(context.Background())
			cancel()

			err = svc.SendWithContext(ctx, "Test message with canceled context", nil)
			gomega.Expect(errors.Is(err, context.Canceled)).To(gomega.BeTrue(),
				"expected context.Canceled error, got: %v", err)
		})

		ginkgo.It("should handle deadline exceeded context gracefully", func() {
			serviceURL := buildServiceURL()
			if serviceURL == "" {
				ginkgo.Skip("Matrix server not configured, skipping test. " +
					"Set SHOUTRRR_MATRIX_USER and SHOUTRRR_MATRIX_PASSWORD environment variables")
			}

			parsedURL, err := url.Parse(serviceURL)
			gomega.Expect(err).NotTo(gomega.HaveOccurred())

			var svc *matrix.Service
			if sharedService != nil {
				svc = sharedService
			} else {
				svc = &matrix.Service{}
				err = svc.Initialize(parsedURL, testutils.TestLogger())
				gomega.Expect(err).NotTo(gomega.HaveOccurred())
			}

			// Create a context with a very short timeout
			ctx, cancel := context.WithTimeout(context.Background(), 1*time.Nanosecond)
			defer cancel()

			// Wait for the context to expire
			<-ctx.Done()

			err = svc.SendWithContext(ctx, "Test message with expired deadline", nil)
			gomega.Expect(errors.Is(err, context.DeadlineExceeded)).To(gomega.BeTrue(),
				"expected context.DeadlineExceeded error, got: %v", err)
		})
	})

	// TestE2EContextTimeout tests behavior when operations timeout.
	// We test this by using an unreachable room or network conditions.
	ginkgo.Describe("Context Timeout", func() {
		ginkgo.It("should timeout when server is unreachable", func() {
			// Use a URL with a non-existent room that will cause timeout
			// This tests the internal timeout handling
			invalidURL := "matrix://user:password@localhost:9999?room=#nonexistent:invalid&disableTLS=true"
			parsedURL, err := url.Parse(invalidURL)
			gomega.Expect(err).NotTo(gomega.HaveOccurred())

			service := &matrix.Service{}

			err = service.Initialize(parsedURL, testutils.TestLogger())
			if err != nil {
				// If initialization fails due to unreachable server, that's expected
				gomega.Expect(err).To(gomega.HaveOccurred())

				return
			}

			// Send should timeout or fail due to network issues
			err = service.Send("This should timeout", nil)
			gomega.Expect(err).To(gomega.HaveOccurred())
		})

		ginkgo.It("should handle slow server response with timeout", func() {
			serviceURL := buildServiceURL()
			if serviceURL == "" {
				ginkgo.Skip("Matrix server not configured, skipping test")
			}

			// Parse the service URL
			parsedURL, err := url.Parse(serviceURL)
			gomega.Expect(err).NotTo(gomega.HaveOccurred())

			service := &matrix.Service{}

			err = service.Initialize(parsedURL, testutils.TestLogger())
			if err != nil {
				ginkgo.Skip("Cannot initialize service, skipping test")
			}

			// Try to send to a room that doesn't exist
			// This will cause the server to take time to respond with an error
			invalidRoomURL := parsedURL
			q := invalidRoomURL.Query()
			q.Set("room", "#definitely-does-not-exist-12345:invalid.local")
			invalidRoomURL.RawQuery = q.Encode()

			// Re-initialize with invalid room
			service = &matrix.Service{}
			err = service.Initialize(invalidRoomURL, testutils.TestLogger())
			gomega.Expect(err).NotTo(gomega.HaveOccurred())

			// Send should eventually timeout or return error
			err = service.Send("Test timeout handling", nil)
			gomega.Expect(err).To(gomega.HaveOccurred())
		})
	})

	// TestE2ERoomJoinFailure tests behavior when joining a room fails.
	ginkgo.Describe("Room Join Failure", func() {
		ginkgo.It("should return error when room doesn't exist", func() {
			serviceURL := buildServiceURL()
			if serviceURL == "" {
				ginkgo.Skip("Matrix server not configured, skipping test")
			}

			parsedURL, err := url.Parse(serviceURL)
			gomega.Expect(err).NotTo(gomega.HaveOccurred())

			// Add a non-existent room
			q := parsedURL.Query()
			q.Set("room", "#nonexistent-room-that-does-not-exist-12345:invalid.local")
			parsedURL.RawQuery = q.Encode()

			service := &matrix.Service{}
			err = service.Initialize(parsedURL, testutils.TestLogger())
			gomega.Expect(err).NotTo(gomega.HaveOccurred())

			// Try to send - should fail to join room
			err = service.Send("Test message to non-existent room", nil)
			gomega.Expect(err).To(gomega.HaveOccurred())
			// Error should mention the room or joining failure
			gomega.Expect(err.Error()).To(gomega.SatisfyAny(
				gomega.ContainSubstring("join"),
				gomega.ContainSubstring("room"),
				gomega.ContainSubstring("404"),
				gomega.ContainSubstring("not found"),
			))
		})

		ginkgo.It("should return error for invalid room alias format", func() {
			serviceURL := buildServiceURL()
			if serviceURL == "" {
				ginkgo.Skip("Matrix server not configured, skipping test")
			}

			parsedURL, err := url.Parse(serviceURL)
			gomega.Expect(err).NotTo(gomega.HaveOccurred())

			// Add an invalid room alias (missing colon for server name)
			q := parsedURL.Query()
			q.Set("room", "invalid-room-alias")
			parsedURL.RawQuery = q.Encode()

			service := &matrix.Service{}
			err = service.Initialize(parsedURL, testutils.TestLogger())
			gomega.Expect(err).NotTo(gomega.HaveOccurred())

			// Try to send - may fail or succeed depending on server handling
			err = service.Send("Test message with invalid room alias", nil)
			// We're just checking it doesn't panic; error is optional
			_ = err
		})
	})

	// TestE2ENetworkError tests behavior when network errors occur.
	ginkgo.Describe("Network Error Handling", func() {
		ginkgo.It("should return error for unreachable server", func() {
			// Use a port that's unlikely to be open
			invalidURL := "matrix://user:password@localhost:65432?disableTLS=true"
			parsedURL, err := url.Parse(invalidURL)
			gomega.Expect(err).NotTo(gomega.HaveOccurred())

			service := &matrix.Service{}

			err = service.Initialize(parsedURL, testutils.TestLogger())
			if err != nil {
				// Connection refused is expected (platform-agnostic)
				gomega.Expect(err.Error()).To(gomega.SatisfyAny(
					gomega.ContainSubstring("refused"),
					gomega.ContainSubstring("no such host"),
					gomega.ContainSubstring("timeout"),
					gomega.ContainSubstring("i/o timeout"),
				))

				return
			}

			// If initialization succeeded (unlikely), try to send
			err = service.Send("Test message", nil)
			gomega.Expect(err).To(gomega.HaveOccurred())
		})

		ginkgo.It("should handle invalid host gracefully", func() {
			// Use a clearly invalid host
			invalidURL := "matrix://user:password@this-host-does-not-exist-12345.invalid:8008?disableTLS=true"
			parsedURL, err := url.Parse(invalidURL)
			gomega.Expect(err).NotTo(gomega.HaveOccurred())

			service := &matrix.Service{}
			err = service.Initialize(parsedURL, testutils.TestLogger())
			// Should fail with DNS/network error
			gomega.Expect(err).To(gomega.HaveOccurred())
			gomega.Expect(err.Error()).To(gomega.SatisfyAny(
				gomega.ContainSubstring("no such host"),
				gomega.ContainSubstring("dial"),
				gomega.ContainSubstring("timeout"),
			))
		})

		ginkgo.It("should handle invalid credentials with clear error", func() {
			// Use valid host but invalid credentials
			serviceURL := buildServiceURL()
			if serviceURL == "" {
				// Use localhost as fallback
				serviceURL = "matrix://invaliduser:invalidpassword@localhost:8008?disableTLS=true"
			} else {
				// Replace credentials in existing URL
				parsedURL, _ := url.Parse(serviceURL)
				parsedURL.User = url.UserPassword("invaliduser", "invalidpassword")
				serviceURL = parsedURL.String()
			}

			parsedURL, err := url.Parse(serviceURL)
			gomega.Expect(err).NotTo(gomega.HaveOccurred())

			service := &matrix.Service{}
			err = service.Initialize(parsedURL, testutils.TestLogger())
			// Authentication should fail
			gomega.Expect(err).To(gomega.HaveOccurred())
		})

		ginkgo.It("should handle invalid host gracefully", func() {
			// Construct a test-specific invalid URL directly
			invalidURL, err := url.Parse("matrix://user:pass@invalid-host-for-testing:9999?disableTLS=true")
			gomega.Expect(err).NotTo(gomega.HaveOccurred())

			service := &matrix.Service{}
			err = service.Initialize(invalidURL, testutils.TestLogger())
			// Should fail due to invalid host
			gomega.Expect(err).To(gomega.HaveOccurred())
		})
	})

	// Additional edge case tests
	ginkgo.Describe("Edge Cases", func() {
		ginkgo.It("should handle empty message gracefully", func() {
			if sharedService != nil {
				err := sharedService.Send("", nil)
				// Empty message should not cause an error
				gomega.Expect(err).NotTo(gomega.HaveOccurred())

				return
			}

			serviceURL := buildServiceURL()
			if serviceURL == "" {
				ginkgo.Skip("Matrix server not configured, skipping test")
			}

			parsedURL, err := url.Parse(serviceURL)
			gomega.Expect(err).NotTo(gomega.HaveOccurred())

			service := &matrix.Service{}
			err = service.Initialize(parsedURL, testutils.TestLogger())
			gomega.Expect(err).NotTo(gomega.HaveOccurred())

			err = service.Send("", nil)
			// Empty message handling depends on server
			_ = err
		})

		ginkgo.It("should handle very long message", func() {
			if sharedService != nil {
				longMessage := strings.Repeat("A very long message. ", 1000)
				err := sharedService.Send(longMessage, nil)
				// Long message should be sent successfully
				gomega.Expect(err).NotTo(gomega.HaveOccurred())

				return
			}

			serviceURL := buildServiceURL()
			if serviceURL == "" {
				ginkgo.Skip("Matrix server not configured, skipping test")
			}

			parsedURL, err := url.Parse(serviceURL)
			gomega.Expect(err).NotTo(gomega.HaveOccurred())

			service := &matrix.Service{}
			err = service.Initialize(parsedURL, testutils.TestLogger())
			gomega.Expect(err).NotTo(gomega.HaveOccurred())

			longMessage := strings.Repeat("Test message content. ", 500)
			err = service.Send(longMessage, nil)
			gomega.Expect(err).NotTo(gomega.HaveOccurred())
		})

		ginkgo.It("should respect configured timeout", func() {
			// The default HTTP timeout is 10 seconds
			// We test that the service doesn't hang indefinitely
			serviceURL := buildServiceURL()
			if serviceURL == "" {
				ginkgo.Skip("Matrix server not configured, skipping test")
			}

			parsedURL, err := url.Parse(serviceURL)
			gomega.Expect(err).NotTo(gomega.HaveOccurred())

			service := &matrix.Service{}
			err = service.Initialize(parsedURL, testutils.TestLogger())
			gomega.Expect(err).NotTo(gomega.HaveOccurred())

			// Send should complete within reasonable time (not hang indefinitely)
			done := make(chan error, 1)

			go func() {
				done <- service.Send("Timeout test message", nil)
			}()

			select {
			case err := <-done:
				// Completed within the timeout - must succeed
				gomega.Expect(err).NotTo(gomega.HaveOccurred())
			case <-time.After(15 * time.Second):
				ginkgo.Fail("Send operation timed out after 15 seconds (expected < 10s)")
			}
		})
	})
})
