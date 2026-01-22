package discord

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"maps"
	"math"
	"mime/multipart"
	"net"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/nicholas-fedor/shoutrrr/pkg/types"
)

const (
	// defaultHTTPTimeout is the default timeout for HTTP requests to Discord.
	defaultHTTPTimeout = 30 * time.Second

	// serverErrorStatusCode is the HTTP status code for server errors (5xx).
	serverErrorStatusCode = 500
)

const (
	maxRetries      = 5
	baseBackoff     = time.Second
	maxBackoff      = 64 * time.Second
	maxRetryTimeout = 5 * time.Minute
	backoffBase     = 2
)

// HTTPClient defines the interface for HTTP operations.
type HTTPClient interface {
	Do(req *http.Request) (*http.Response, error)
}

// Sleeper defines the interface for sleep operations.
type Sleeper interface {
	Sleep(d time.Duration)
}

// RealSleeper is the default implementation using time.Sleep.
type RealSleeper struct{}

func (RealSleeper) Sleep(d time.Duration) {
	time.Sleep(d)
}

// DefaultHTTPClient is the default implementation using http.DefaultClient with timeout.
type DefaultHTTPClient struct {
	client *http.Client
}

// NewDefaultHTTPClient creates a new default HTTP client with a reasonable timeout.
func NewDefaultHTTPClient() *DefaultHTTPClient {
	return &DefaultHTTPClient{
		client: &http.Client{
			Timeout: defaultHTTPTimeout, // Default timeout for Discord requests
		},
	}
}

// Do performs the HTTP request.
func (c *DefaultHTTPClient) Do(req *http.Request) (*http.Response, error) {
	resp, err := c.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("performing HTTP request: %w", err)
	}

	return resp, nil
}

// RequestPreparer defines the interface for preparing HTTP requests.
type RequestPreparer interface {
	PrepareRequest(ctx context.Context, url string) (*http.Request, error)
}

// JSONRequestPreparer prepares JSON requests.
type JSONRequestPreparer struct {
	payload []byte
}

// PrepareRequest creates a JSON POST request.
func (p *JSONRequestPreparer) PrepareRequest(
	ctx context.Context,
	url string,
) (*http.Request, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(p.payload))
	if err != nil {
		return nil, fmt.Errorf("creating HTTP request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("User-Agent", "shoutrrr")

	return req, nil
}

// MultipartRequestPreparer prepares multipart/form-data requests.
type MultipartRequestPreparer struct {
	payload WebhookPayload
	files   []types.File
}

// PrepareRequest creates a multipart POST request.
func (p *MultipartRequestPreparer) PrepareRequest(
	ctx context.Context,
	url string,
) (*http.Request, error) {
	var body bytes.Buffer

	writer := multipart.NewWriter(&body)

	// Add payload as JSON
	payloadBytes, err := json.Marshal(p.payload)
	if err != nil {
		return nil, fmt.Errorf("marshaling payload to JSON: %w", err)
	}

	err = writer.WriteField("payload_json", string(payloadBytes))
	if err != nil {
		return nil, fmt.Errorf("writing payload_json field: %w", err)
	}

	// Add files
	for i, file := range p.files {
		fw, err := writer.CreateFormFile(fmt.Sprintf("files[%d]", i), file.Name)
		if err != nil {
			return nil, fmt.Errorf("creating form file for %s: %w", file.Name, err)
		}

		_, err = io.Copy(fw, bytes.NewReader(file.Data))
		if err != nil {
			return nil, fmt.Errorf("writing file data for %s: %w", file.Name, err)
		}
	}

	if err := writer.Close(); err != nil {
		return nil, fmt.Errorf("closing multipart writer: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, &body)
	if err != nil {
		return nil, fmt.Errorf("creating HTTP request: %w", err)
	}

	req.Header.Set("Content-Type", writer.FormDataContentType())
	req.Header.Set("User-Agent", "shoutrrr")

	return req, nil
}

// isTransientError checks if an error is transient and should be retried.
func isTransientError(err error) bool {
	if err == nil {
		return false
	}

	if errors.Is(err, context.Canceled) {
		return false
	}

	if errors.Is(err, context.DeadlineExceeded) {
		return true
	}

	var netErr net.Error
	if errors.As(err, &netErr) {
		return netErr.Timeout() ||
			netErr.Temporary() //nolint:staticcheck // Temporary is deprecated but still used for compatibility
	}

	var urlErr *url.Error
	if errors.As(err, &urlErr) {
		return isTransientError(urlErr.Err)
	}

	errStr := err.Error()

	return strings.Contains(errStr, "connection refused") ||
		strings.Contains(errStr, "connection reset")
}

// executeWithTransportRetry executes an HTTP request with transport-level retries.
func executeWithTransportRetry(
	ctx context.Context,
	httpClient HTTPClient,
	bodyBytes []byte,
	req *http.Request,
	sleeper Sleeper,
) (*http.Response, error) {
	const maxTransportRetries = 3

	for transportAttempt := 0; transportAttempt <= maxTransportRetries; transportAttempt++ {
		newReq, err := http.NewRequestWithContext(
			ctx,
			req.Method,
			req.URL.String(),
			bytes.NewReader(bodyBytes),
		)
		if err != nil {
			return nil, fmt.Errorf("creating retry request: %w", err)
		}

		maps.Copy(newReq.Header, req.Header)

		res, err := httpClient.Do(newReq)
		if err != nil {
			if isTransientError(err) && transportAttempt < maxTransportRetries {
				wait := time.Duration(
					math.Min(
						math.Pow(backoffBase, float64(transportAttempt))*float64(baseBackoff),
						float64(maxBackoff),
					),
				)

				select {
				case <-ctx.Done():
					return nil, fmt.Errorf("making HTTP POST request: %w", ctx.Err())
				default:
					sleeper.Sleep(wait)
				}

				continue
			}

			return nil, fmt.Errorf("making HTTP POST request: %w", err)
		}

		return res, nil
	}

	return nil, fmt.Errorf("making HTTP POST request: %w", ErrMaxRetries)
}

// handleServerError handles 5xx server errors with retry logic.
func handleServerError(
	ctx context.Context,
	res *http.Response,
	attempt int,
	startTime time.Time,
	sleeper Sleeper,
) error {
	if res.StatusCode < serverErrorStatusCode {
		return nil
	}

	if attempt >= maxRetries {
		_ = res.Body.Close()

		return ErrMaxRetries
	}

	wait := time.Duration(
		math.Min(
			math.Pow(backoffBase, float64(attempt))*float64(baseBackoff),
			float64(maxBackoff),
		),
	)

	if err := waitWithTimeout(ctx, wait, startTime, sleeper); err != nil {
		_ = res.Body.Close()

		return err
	}

	_ = res.Body.Close()

	return nil // Continue retry
}

// sendWithRetry executes an HTTP request with exponential backoff retries.
func sendWithRetry(
	ctx context.Context,
	preparer RequestPreparer,
	url string,
	httpClient HTTPClient,
	sleeper Sleeper,
) error {
	startTime := time.Now()

	for attempt := 0; attempt <= maxRetries; attempt++ {
		// Check if we've exceeded the maximum retry timeout
		if time.Since(startTime) > maxRetryTimeout {
			return fmt.Errorf(
				"max retry timeout exceeded after %v: %w",
				maxRetryTimeout,
				ErrMaxRetries,
			)
		}

		// Prepare the request
		req, err := preparer.PrepareRequest(ctx, url)
		if err != nil {
			return fmt.Errorf("preparing request: %w", err)
		}

		// Read the request body to preserve it for retries
		bodyBytes, err := io.ReadAll(req.Body)
		if err != nil {
			return fmt.Errorf("reading request body: %w", err)
		}

		_ = req.Body.Close()

		// Execute request with transport retries
		res, err := executeWithTransportRetry(ctx, httpClient, bodyBytes, req, sleeper)
		if err != nil {
			return err
		}

		if res == nil {
			return ErrUnknownAPIError
		}

		defer func() { _ = res.Body.Close() }()

		// Handle rate limit response
		if res.StatusCode == http.StatusTooManyRequests {
			if err := handleRateLimitResponse(ctx, res, attempt, startTime, sleeper); err != nil {
				return err
			}

			continue
		}

		// Handle server errors
		if err := handleServerError(ctx, res, attempt, startTime, sleeper); err != nil {
			return err
		}

		if res.StatusCode >= serverErrorStatusCode {
			continue
		}

		if res.StatusCode != http.StatusNoContent && res.StatusCode != http.StatusOK {
			return fmt.Errorf("%w: %s", ErrUnexpectedStatus, res.Status)
		}

		return nil
	}

	return ErrMaxRetries
}

// waitWithTimeout waits for the specified duration with timeout and context cancellation checks.
func waitWithTimeout(
	ctx context.Context,
	wait time.Duration,
	startTime time.Time,
	sleeper Sleeper,
) error {
	// Don't wait longer than remaining timeout
	if wait > maxRetryTimeout-time.Since(startTime) {
		return fmt.Errorf(
			"wait time %v would exceed max retry timeout %v: %w",
			wait,
			maxRetryTimeout,
			ErrRateLimited,
		)
	}

	select {
	case <-ctx.Done():
		return fmt.Errorf("context canceled during wait: %w", ctx.Err())
	default:
		sleeper.Sleep(wait)
	}

	return nil
}

// handleRateLimitResponse handles rate limiting logic for 429 responses.
func handleRateLimitResponse(
	ctx context.Context,
	res *http.Response,
	attempt int,
	startTime time.Time,
	sleeper Sleeper,
) error {
	retryAfter := res.Header.Get("Retry-After")
	if retryAfter != "" {
		if seconds, err := strconv.ParseFloat(retryAfter, 64); err == nil {
			wait := time.Duration(seconds * float64(time.Second))
			if err := waitWithTimeout(ctx, wait, startTime, sleeper); err != nil {
				_ = res.Body.Close()

				return err
			}

			_ = res.Body.Close()

			return nil // Continue retry
		}
	}
	// Fallback to exponential backoff
	wait := time.Duration(
		math.Min(math.Pow(backoffBase, float64(attempt))*float64(baseBackoff), float64(maxBackoff)),
	)
	if err := waitWithTimeout(ctx, wait, startTime, sleeper); err != nil {
		_ = res.Body.Close()

		return err
	}

	_ = res.Body.Close()

	return nil // Continue retry
}
