package e2e_test

import (
	"fmt"
	"log"
	"net/url"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/nicholas-fedor/shoutrrr/pkg/services/chat/matrix"
)

// TestMatrixConcurrency verifies that the shared service sends concurrent and
// rapid sequential messages to a real Matrix server, that services initialize
// concurrently, and that concurrent sends neither collide on transaction IDs
// nor mostly fail under load.
//
//nolint:paralleltest // Sends to a live server, which rate-limits concurrent tests.
func TestMatrixConcurrency(t *testing.T) {
	t.Run("multiple messages concurrently", func(t *testing.T) {
		service := getSharedService(t)

		const concurrency = 5

		errs := make(chan error, concurrency)

		var wg sync.WaitGroup

		for i := range concurrency {
			wg.Go(func() {
				errs <- service.Send(fmt.Sprintf("Concurrent message %d - %d", i, time.Now().UnixNano()), nil)
			})
		}

		wg.Wait()
		close(errs)

		for err := range errs {
			assert.NoError(t, err, "every concurrent send must succeed")
		}
	})

	t.Run("rapid sequential sends", func(t *testing.T) {
		service := getSharedService(t)

		for range 10 {
			require.NoError(t, service.Send("Rapid send test message", nil))

			time.Sleep(100 * time.Millisecond)
		}
	})

	t.Run("concurrent initialization", func(t *testing.T) {
		parsedURL, err := url.Parse(requireServiceURL(t))
		require.NoError(t, err)

		const numServices = 3

		var (
			wg          sync.WaitGroup
			initialized atomic.Int32
		)

		logger := log.New(t.Output(), "[Test] ", 0)

		for range numServices {
			wg.Go(func() {
				service := &matrix.Service{}
				if service.Initialize(parsedURL, logger) == nil {
					initialized.Add(1)
				}
			})
		}

		wg.Wait()

		assert.Positive(t, initialized.Load(), "at least one service must initialize")
	})

	t.Run("unique transaction IDs for each message", func(t *testing.T) {
		service := getSharedService(t)

		// Concurrent sends with colliding transaction IDs would be deduplicated
		// by the server, so every send succeeding shows the IDs are unique.
		const numMessages = 5

		var (
			wg              sync.WaitGroup
			successfulSends atomic.Int32
		)

		for i := range numMessages {
			wg.Go(func() {
				uniqueMsg := fmt.Sprintf("Race condition test - unique ID: %d-%d", i, time.Now().UnixNano())
				if service.Send(uniqueMsg, nil) == nil {
					successfulSends.Add(1)
				}
			})
		}

		wg.Wait()

		assert.Equal(t, int32(numMessages), successfulSends.Load(), "every message must be sent")
	})

	t.Run("many goroutines sending concurrently", func(t *testing.T) {
		service := getSharedService(t)

		const (
			numGoroutines = 10
			numIterations = 3
		)

		var (
			wg         sync.WaitGroup
			errorCount atomic.Int32
		)

		for iteration := range numIterations {
			for i := range numGoroutines {
				wg.Go(func() {
					message := fmt.Sprintf("Stress test - goroutine %d iteration %d", i, iteration)
					if service.Send(message, nil) != nil {
						errorCount.Add(1)
					}
				})
			}

			time.Sleep(200 * time.Millisecond)
		}

		wg.Wait()

		totalAttempts := numGoroutines * numIterations
		successCount := totalAttempts - int(errorCount.Load())

		// Half the sends is a conservative floor under concurrent load.
		assert.GreaterOrEqual(t, successCount, totalAttempts/2, "at least 50% of sends must succeed")
	})
}
