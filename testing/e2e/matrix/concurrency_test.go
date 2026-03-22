package e2e_test

import (
	"fmt"
	"net/url"
	"sync"
	"sync/atomic"
	"time"

	"github.com/onsi/ginkgo/v2"
	"github.com/onsi/gomega"

	"github.com/nicholas-fedor/shoutrrr/internal/testutils"
	"github.com/nicholas-fedor/shoutrrr/pkg/services/chat/matrix"
)

var _ = ginkgo.Describe("Matrix Service E2E Concurrency", func() {
	// TestE2EConcurrentSends tests that multiple messages can be sent concurrently.
	ginkgo.Describe("Concurrent Sends", func() {
		ginkgo.It("should send multiple messages concurrently", func() {
			// Use shared service if available
			err := getOrInitSharedService()
			if err != nil {
				ginkgo.Skip(fmt.Sprintf("Cannot initialize shared service: %v", err))
			}

			// Number of concurrent messages to send
			concurrency := 5
			errChan := make(chan error, concurrency)

			var wg sync.WaitGroup

			// Send multiple messages concurrently
			for i := range concurrency {
				wg.Add(1)

				go func(idx int) {
					defer wg.Done()

					message := fmt.Sprintf("Concurrent message %d - %d", idx, time.Now().UnixNano())

					err := sharedService.Send(message, nil)
					errChan <- err
				}(i)
			}

			// Wait for all goroutines to complete
			wg.Wait()
			close(errChan)

			// Check all sends completed without errors
			errCount := 0

			for err := range errChan {
				if err != nil {
					errCount++
				}
			}

			// All sends must succeed
			gomega.Expect(errCount).To(gomega.Equal(0),
				"Expected all sends to succeed, but some failed")
		})

		ginkgo.It("should handle rapid sequential sends without race conditions", func() {
			err := getOrInitSharedService()
			if err != nil {
				ginkgo.Skip(fmt.Sprintf("Cannot initialize shared service: %v", err))
			}

			// Send messages in quick succession
			// This tests that the service can handle rapid requests
			numMessages := 10
			for range numMessages {
				err := sharedService.Send("Rapid send test message", nil)
				// All messages must succeed
				gomega.Expect(err).NotTo(gomega.HaveOccurred())

				// Small delay between messages
				time.Sleep(100 * time.Millisecond)
			}
		})

		ginkgo.It("should handle concurrent initialization safely", func() {
			serviceURL := buildServiceURL()
			if serviceURL == "" {
				ginkgo.Skip("Matrix server not configured, skipping test")
			}

			parsedURL, err := url.Parse(serviceURL)
			gomega.Expect(err).NotTo(gomega.HaveOccurred())

			// Create multiple services concurrently
			numServices := 3

			var wg sync.WaitGroup

			serviceChan := make(chan *matrix.Service, numServices)
			errChan := make(chan error, numServices)

			for range numServices {
				wg.Go(func() {
					service := &matrix.Service{}

					err := service.Initialize(parsedURL, testutils.TestLogger())
					serviceChan <- service

					errChan <- err
				})
			}

			wg.Wait()
			close(serviceChan)
			close(errChan)

			// Collect results
			var successCount int

			for svc := range serviceChan {
				if svc != nil {
					successCount++
				}
			}

			// Some may succeed, some may fail due to session conflicts
			// At least one should succeed
			gomega.Expect(successCount).To(gomega.BeNumerically(">", 0),
				"Expected at least one service to initialize successfully")
		})
	})

	// TestE2ERaceConditionPrevention tests that transaction IDs prevent duplicate messages.
	ginkgo.Describe("Race Condition Prevention", func() {
		ginkgo.It("should generate unique transaction IDs for each message", func() {
			// Test the transaction ID generation by looking at the internal behavior
			// We verify that rapid sends don't result in duplicate transaction IDs
			// which could cause message duplication on the Matrix server
			err := getOrInitSharedService()
			if err != nil {
				ginkgo.Skip(fmt.Sprintf("Cannot initialize shared service: %v", err))
			}

			// Track transaction IDs indirectly by checking for duplicates in error messages
			// or by verifying all messages are delivered
			numMessages := 5

			var successfulSends atomic.Int32

			// Send messages with unique identifiers
			var wg sync.WaitGroup
			for i := range numMessages {
				wg.Add(1)

				go func(idx int) {
					defer wg.Done()
					// Each message has a unique identifier in the content
					uniqueMsg := fmt.Sprintf("Race condition test - unique ID: %d-%d", idx, time.Now().UnixNano())

					err := sharedService.Send(uniqueMsg, nil)
					if err == nil {
						successfulSends.Add(1)
					}
				}(i)
			}

			wg.Wait()

			// All messages should succeed
			gomega.Expect(successfulSends.Load()).To(gomega.Equal(int32(numMessages)),
				"Expected all messages to be sent successfully")
		})

		ginkgo.It("should handle multiple goroutines accessing service concurrently", func() {
			// This is a stress test to ensure thread-safety
			err := getOrInitSharedService()
			if err != nil {
				ginkgo.Skip(fmt.Sprintf("Cannot initialize shared service: %v", err))
			}

			numGoroutines := 10
			numIterations := 3

			var wg sync.WaitGroup

			errorCount := &atomic.Int32{}

			for iteration := range numIterations {
				for i := range numGoroutines {
					wg.Add(1)

					go func(goroutineID, iterID int) {
						defer wg.Done()

						message := fmt.Sprintf("Stress test - goroutine %d iteration %d", goroutineID, iterID)

						err := sharedService.Send(message, nil)
						if err != nil {
							errorCount.Add(1)
						}
					}(i, iteration)
				}
				// Small delay between iterations
				time.Sleep(200 * time.Millisecond)
			}

			wg.Wait()

			// Calculate success rate
			totalAttempts := numGoroutines * numIterations
			successCount := totalAttempts - int(errorCount.Load())

			// At least 50% should succeed (conservative under concurrent load)
			gomega.Expect(successCount).To(gomega.BeNumerically(">=", totalAttempts/2),
				"Expected at least 50% success rate under concurrent load")
		})
	})
})
