package discord

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"math"
	"mime/multipart"
	"net/http"
	"strconv"
	"time"

	"github.com/nicholas-fedor/shoutrrr/pkg/types"
)

const (
	// defaultHTTPTimeout is the default timeout for HTTP requests to Discord.
	defaultHTTPTimeout = 30 * time.Second

	// serverErrorStatusCode is the HTTP status code for server errors (5xx).
	serverErrorStatusCode = 500
)

// HTTPClient defines the interface for HTTP operations to enable dependency injection and testing.
type HTTPClient interface {
	Do(req *http.Request) (*http.Response, error)
}

// Sleeper defines the interface for sleep operations to enable testing.
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

		_, err = fw.Write(file.Data)
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

// sendWithRetry executes an HTTP request with exponential backoff retries.
func sendWithRetry(
	ctx context.Context,
	preparer RequestPreparer,
	url string,
	httpClient HTTPClient,
	sleeper Sleeper,
) error {
	const (
		MaxRetries      = 5
		BaseBackoff     = time.Second
		MaxBackoff      = 64 * time.Second
		MaxRetryTimeout = 5 * time.Minute
		BackoffBase     = 2
	)

	startTime := time.Now()

	for attempt := 0; attempt <= MaxRetries; attempt++ {
		// Check if we've exceeded the maximum retry timeout
		if time.Since(startTime) > MaxRetryTimeout {
			return fmt.Errorf(
				"max retry timeout exceeded after %v: %w",
				MaxRetryTimeout,
				ErrMaxRetries,
			)
		}

		// Prepare the request
		req, err := preparer.PrepareRequest(ctx, url)
		if err != nil {
			return fmt.Errorf("preparing request: %w", err)
		}

		// Make the request
		res, err := httpClient.Do(req)
		if err != nil {
			return fmt.Errorf("making HTTP POST request: %w", err)
		}

		if res == nil {
			return ErrUnknownAPIError
		}

		defer func() { _ = res.Body.Close() }()

		// Handle rate limit response
		if res.StatusCode == http.StatusTooManyRequests {
			if err := handleRateLimitResponse(res, attempt, startTime, sleeper); err != nil {
				return err
			}

			continue
		}

		// Retry on server errors (5xx) but not on client errors (4xx)
		if res.StatusCode >= serverErrorStatusCode {
			if attempt < MaxRetries {
				// Exponential backoff for server errors
				wait := time.Duration(
					math.Min(
						math.Pow(BackoffBase, float64(attempt))*float64(BaseBackoff),
						float64(MaxBackoff),
					),
				)
				sleeper.Sleep(wait)

				_ = res.Body.Close()

				continue
			}

			_ = res.Body.Close()

			return ErrMaxRetries
		}

		if res.StatusCode != http.StatusNoContent && res.StatusCode != http.StatusOK {
			return fmt.Errorf("%w: %s", ErrUnexpectedStatus, res.Status)
		}

		return nil
	}

	return ErrMaxRetries
}

// handleRateLimitResponse handles rate limiting logic for 429 responses.
func handleRateLimitResponse(
	res *http.Response,
	attempt int,
	startTime time.Time,
	sleeper Sleeper,
) error {
	const (
		MaxRetryTimeout = 5 * time.Minute
		BaseBackoff     = time.Second
		MaxBackoff      = 64 * time.Second
		BackoffBase     = 2
	)

	retryAfter := res.Header.Get("Retry-After")
	if retryAfter != "" {
		if seconds, err := strconv.Atoi(retryAfter); err == nil {
			wait := time.Duration(seconds) * time.Second
			// Don't wait longer than remaining timeout
			if wait > MaxRetryTimeout-time.Since(startTime) {
				_ = res.Body.Close()

				return fmt.Errorf(
					"retry-after wait time %v would exceed max retry timeout %v: %w",
					wait,
					MaxRetryTimeout,
					ErrRateLimited,
				)
			}

			sleeper.Sleep(wait)

			_ = res.Body.Close()

			return nil // Continue retry
		}
	}
	// Fallback to exponential backoff
	wait := time.Duration(
		math.Min(math.Pow(BackoffBase, float64(attempt))*float64(BaseBackoff), float64(MaxBackoff)),
	)
	// Don't wait longer than remaining timeout
	if wait > MaxRetryTimeout-time.Since(startTime) {
		_ = res.Body.Close()

		return fmt.Errorf(
			"backoff wait time %v would exceed max retry timeout %v: %w",
			wait,
			MaxRetryTimeout,
			ErrRateLimited,
		)
	}

	sleeper.Sleep(wait)

	_ = res.Body.Close()

	return nil // Continue retry
}
