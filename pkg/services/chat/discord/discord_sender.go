package discord

import (
	"bytes"
	"context"
	"encoding/json/v2"
	"errors"
	"fmt"
	"io"
	"maps"
	"math"
	"mime/multipart"
	"net"
	"net/http"
	"strings"
	"time"

	"github.com/nicholas-fedor/shoutrrr/internal/httpclient"
	"github.com/nicholas-fedor/shoutrrr/internal/jsonopts"
	"github.com/nicholas-fedor/shoutrrr/internal/meta"
	"github.com/nicholas-fedor/shoutrrr/internal/redact"
	"github.com/nicholas-fedor/shoutrrr/pkg/types"
)

// HTTPClient defines the interface for HTTP operations.
//
// Deprecated: The service takes a [types.HTTPClient], which HTTPClient is an
// alias of. Use [types.HTTPClient].
//
//go:fix inline
type HTTPClient = types.HTTPClient

// Sleeper defines the interface for sleep operations.
type Sleeper interface {
	// Sleep waits for d.
	//
	// Parameters:
	//   - d: how long to wait.
	Sleep(d time.Duration)
}

// RealSleeper is the default implementation using time.Sleep.
type RealSleeper struct{}

// DefaultHTTPClient is an HTTP client that sends through an [http.Client]
// with a timeout. The zero value is ready to use.
//
// Deprecated: The service builds its own default client and bounds each send
// by its send budget. Inject an *http.Client with [Service.SetHTTPClient]
// instead.
type DefaultHTTPClient struct {
	// client sends the requests, with [defaultHTTPTimeout] as its timeout, or
	// is nil to send through a new client with that timeout.
	client *http.Client
}

// RequestPreparer defines the interface for preparing HTTP requests.
type RequestPreparer interface {
	// PrepareRequest creates a POST request to requestURL.
	//
	// Parameters:
	//   - ctx: the context of the request.
	//   - requestURL: the webhook URL.
	//
	// Returns:
	//   - *http.Request: the request.
	//   - error: the failure building the request.
	PrepareRequest(ctx context.Context, requestURL string) (*http.Request, error)
}

// JSONRequestPreparer prepares JSON requests.
type JSONRequestPreparer struct {
	// payload is the JSON request body.
	payload []byte
}

// MultipartRequestPreparer prepares multipart/form-data requests.
type MultipartRequestPreparer struct {
	// payload is sent as JSON in the payload_json form field.
	payload *WebhookPayload
	// files are sent as the files[n] form parts, in order.
	files []types.File
}

// HTTP settings for requests to Discord.
const (
	// defaultHTTPTimeout is the timeout of [DefaultHTTPClient].
	defaultHTTPTimeout = 30 * time.Second

	// serverErrorStatusCode is the HTTP status code for server errors (5xx).
	serverErrorStatusCode = 500
)

// Retry limits and backoff for sends.
const (
	// maxRetries is the most retries after the first attempt for a send that
	// gets a rate limit or server error response.
	maxRetries = 5
	// baseBackoff is the wait before the first retry.
	baseBackoff = time.Second
	// maxBackoff caps the wait between retries.
	maxBackoff = 64 * time.Second
	// maxRetryTimeout limits how long a send keeps retrying. It is checked
	// before each attempt and before each wait for a rate limit or server
	// error. It does not cut short a request in flight or the transport
	// retries within an attempt.
	maxRetryTimeout = 5 * time.Minute
	// backoffBase is the factor by which the wait grows with each retry.
	backoffBase = 2
)

// Sleep waits for d with [time.Sleep].
//
// Parameters:
//   - d: how long to wait.
func (RealSleeper) Sleep(d time.Duration) {
	time.Sleep(d)
}

// SleepContext waits for d or until ctx is done, whichever comes first.
//
// Parameters:
//   - ctx: ends the wait early when it is done.
//   - d: how long to wait.
//
// Returns:
//   - error: the context's error when it ends the wait, or nil after waiting d.
func (RealSleeper) SleepContext(ctx context.Context, d time.Duration) error {
	timer := time.NewTimer(d)
	defer timer.Stop()

	select {
	case <-ctx.Done():
		return fmt.Errorf("waiting: %w", ctx.Err())
	case <-timer.C:
		return nil
	}
}

// NewDefaultHTTPClient creates a new default HTTP client with a reasonable timeout.
//
// Returns:
//   - *DefaultHTTPClient: a client with [defaultHTTPTimeout] as its timeout.
//
// Deprecated: The service builds its own default client and bounds each send
// by its send budget. Inject an *http.Client with [Service.SetHTTPClient]
// instead.
func NewDefaultHTTPClient() *DefaultHTTPClient {
	return &DefaultHTTPClient{
		client: &http.Client{
			Timeout: defaultHTTPTimeout, // Default timeout for Discord requests
		},
	}
}

// Do performs the HTTP request.
//
// Parameters:
//   - req: the request to send.
//
// Returns:
//   - *http.Response: the response.
//   - error: the transport failure, with the request URL redacted.
func (c *DefaultHTTPClient) Do(req *http.Request) (*http.Response, error) {
	client := &http.Client{Timeout: defaultHTTPTimeout}
	if c != nil && c.client != nil {
		client = c.client
	}

	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("performing HTTP request: %w", redact.URLError(err))
	}

	return resp, nil
}

// PrepareRequest creates a JSON POST request.
//
// Parameters:
//   - ctx: the context of the request.
//   - requestURL: the webhook URL.
//
// Returns:
//   - *http.Request: the request, with the payload as its body.
//   - error: the failure creating the request, with the URL redacted.
func (p *JSONRequestPreparer) PrepareRequest(
	ctx context.Context,
	requestURL string,
) (*http.Request, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, requestURL, bytes.NewReader(p.payload))
	if err != nil {
		return nil, fmt.Errorf("creating HTTP request: %w", redact.URLError(err))
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("User-Agent", meta.UserAgent())

	return req, nil
}

// PrepareRequest creates a multipart POST request. The payload goes in the
// payload_json field as JSON, and each file in a files[n] part.
//
// Parameters:
//   - ctx: the context of the request.
//   - requestURL: the webhook URL.
//
// Returns:
//   - *http.Request: the request, with the multipart form as its body.
//   - error: the failure encoding the form or creating the request.
func (p *MultipartRequestPreparer) PrepareRequest(
	ctx context.Context,
	requestURL string,
) (*http.Request, error) {
	var body bytes.Buffer

	writer := multipart.NewWriter(&body)

	// Add payload as JSON
	payloadBytes, err := json.Marshal(p.payload, jsonopts.Default)
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

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, requestURL, &body)
	if err != nil {
		return nil, fmt.Errorf("creating HTTP request: %w", redact.URLError(err))
	}

	req.Header.Set("Content-Type", writer.FormDataContentType())
	req.Header.Set("User-Agent", meta.UserAgent())

	return req, nil
}

// isTransientError checks if an error is transient and should be retried.
// A canceled context is not transient, and a passed deadline is. A
// [net.Error] that reports a timeout or a temporary failure is transient.
// Any other error, including a [net.Error] that reports neither, is transient
// when its text mentions a refused or reset connection.
//
// Parameters:
//   - err: the transport error, or nil.
//
// Returns:
//   - bool: whether a retry may succeed.
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

	if netErr, ok := errors.AsType[net.Error](err); ok &&
		(netErr.Timeout() || netErr.Temporary()) { //nolint:staticcheck // Temporary is deprecated but still used for compatibility
		return true
	}

	errStr := err.Error()

	return strings.Contains(errStr, "connection refused") ||
		strings.Contains(errStr, "connection reset")
}

// executeWithTransportRetry executes an HTTP request with transport-level retries.
// Each attempt sends a new request with the method, URL, and headers of req
// and bodyBytes as its body. A transient error, as [isTransientError] judges
// it, is retried up to three times with an exponential backoff.
//
// Parameters:
//   - ctx: the context of each request and wait.
//   - httpClient: sends the requests.
//   - bodyBytes: the request body.
//   - req: the request whose method, URL, and headers are sent.
//   - sleeper: waits between attempts.
//
// Returns:
//   - *http.Response: the response, whatever its status.
//   - error: the last transport error, with the URL redacted. Otherwise the
//     failure creating a request or waiting.
func executeWithTransportRetry(
	ctx context.Context,
	httpClient types.HTTPClient,
	bodyBytes []byte,
	req *http.Request,
	sleeper Sleeper,
) (*http.Response, error) {
	const maxTransportRetries = 3

	for transportAttempt := 0; ; transportAttempt++ {
		newReq, err := http.NewRequestWithContext(
			ctx,
			req.Method,
			req.URL.String(),
			bytes.NewReader(bodyBytes),
		)
		if err != nil {
			return nil, fmt.Errorf("creating retry request: %w", redact.URLError(err))
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

				if err := sleepContext(ctx, sleeper, wait); err != nil {
					return nil, fmt.Errorf("making HTTP POST request: %w", err)
				}

				continue
			}

			return nil, fmt.Errorf("making HTTP POST request: %w", redact.URLError(err))
		}

		return res, nil
	}
}

// handleServerError handles 5xx server errors with retry logic.
// A response below 500 is left as it is. For a 5xx response it waits an
// exponential backoff before the next attempt and closes the body. After the
// last attempt it closes the body and returns [ErrMaxRetries] without
// waiting.
//
// Parameters:
//   - ctx: bounds the wait.
//   - res: the response.
//   - attempt: the attempt that got the response, from zero.
//   - startTime: when the send started, for the retry time limit.
//   - sleeper: waits between attempts.
//
// Returns:
//   - error: [ErrMaxRetries] after the last attempt, or the wait's failure.
//     It is nil for a response below 500 and when the next attempt may start.
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
// A rate limit or server error response is retried up to [maxRetries] times,
// while [maxRetryTimeout] allows. A 200 or 204 response is a success, and any
// other status fails at once.
//
// Parameters:
//   - ctx: bounds the requests and the waits between them.
//   - preparer: creates the request for each attempt.
//   - requestURL: the webhook URL.
//   - httpClient: sends the requests.
//   - sleeper: waits between attempts.
//
// Returns:
//   - error: nil on success. An error matching [ErrMaxRetries] when the
//     retries run out, or when [maxRetryTimeout] has passed before an attempt.
//     An error wrapping [ErrRateLimited] when a retry wait would outlast
//     [maxRetryTimeout] or the context's deadline. An error wrapping
//     [ErrUnexpectedStatus] for any other status. Otherwise the request or
//     wait failure.
func sendWithRetry(
	ctx context.Context,
	preparer RequestPreparer,
	requestURL string,
	httpClient types.HTTPClient,
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
		req, err := preparer.PrepareRequest(ctx, requestURL)
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

		// Handle rate limit response
		if res.StatusCode == http.StatusTooManyRequests {
			if err := handleRateLimitResponse(ctx, res, attempt, startTime, sleeper); err != nil {
				_ = res.Body.Close()

				return err
			}

			_ = res.Body.Close()

			continue
		}

		// Handle server errors
		if err := handleServerError(ctx, res, attempt, startTime, sleeper); err != nil {
			_ = res.Body.Close()

			return err
		}

		if res.StatusCode >= serverErrorStatusCode {
			_ = res.Body.Close()

			continue
		}

		if res.StatusCode != http.StatusNoContent && res.StatusCode != http.StatusOK {
			_ = res.Body.Close()

			return fmt.Errorf("%w: %s", ErrUnexpectedStatus, res.Status)
		}

		_ = res.Body.Close()

		return nil
	}

	return ErrMaxRetries
}

// waitWithTimeout waits for the specified duration with timeout and context
// cancellation checks. A wait that would outlast the retry limit or the
// context's deadline fails at once instead of sleeping.
//
// Parameters:
//   - ctx: the send's context, whose deadline bounds the wait.
//   - wait: how long to wait before retrying.
//   - startTime: when the send started, for the retry limit.
//   - sleeper: performs the wait.
//
// Returns:
//   - error: an [ErrRateLimited] error when the wait cannot finish in time, the
//     context's error when it is already done, or nil after waiting.
func waitWithTimeout(
	ctx context.Context,
	wait time.Duration,
	startTime time.Time,
	sleeper Sleeper,
) error {
	// A context that is already done reports its own error
	if err := ctx.Err(); err != nil {
		return fmt.Errorf("context canceled before wait: %w", err)
	}

	// Don't wait longer than remaining timeout
	if wait > maxRetryTimeout-time.Since(startTime) {
		return fmt.Errorf(
			"wait time %v would exceed max retry timeout %v: %w",
			wait,
			maxRetryTimeout,
			ErrRateLimited,
		)
	}

	// Don't wait past the send's deadline, since the retry would be canceled anyway
	if deadline, ok := ctx.Deadline(); ok && wait > time.Until(deadline) {
		return fmt.Errorf(
			"wait time %v would exceed the send deadline: %w",
			wait,
			ErrRateLimited,
		)
	}

	return sleepContext(ctx, sleeper, wait)
}

// sleepContext waits for duration with sleeper. A sleeper that implements
// SleepContext, such as [RealSleeper], stops waiting when ctx is done. Other
// sleepers wait the full duration.
//
// Parameters:
//   - ctx: the send's context.
//   - sleeper: performs the wait.
//   - duration: how long to wait.
//
// Returns:
//   - error: the context's error when it is done before or during the wait, or nil
//     after waiting.
func sleepContext(ctx context.Context, sleeper Sleeper, duration time.Duration) error {
	if err := ctx.Err(); err != nil {
		return fmt.Errorf("context canceled before wait: %w", err)
	}

	if contextSleeper, ok := sleeper.(interface {
		SleepContext(ctx context.Context, d time.Duration) error
	}); ok {
		if err := contextSleeper.SleepContext(ctx, duration); err != nil {
			return fmt.Errorf("context canceled during wait: %w", err)
		}

		return nil
	}

	sleeper.Sleep(duration)

	return nil
}

// handleRateLimitResponse handles rate limiting logic for 429 responses.
//
// It waits for the Retry-After duration, in whole seconds or until an HTTP
// date, or an exponential backoff when the response has no usable Retry-After,
// before the next attempt. After the last attempt it returns [ErrMaxRetries]
// without waiting.
//
// Parameters:
//   - ctx: bounds the wait.
//   - res: the 429 response, whose body is closed.
//   - attempt: the attempt that got the response, from zero.
//   - startTime: when the send started, for the retry time limit.
//   - sleeper: waits between attempts.
//
// Returns:
//   - error: [ErrMaxRetries] after the last attempt, the wait's failure, or nil
//     when the next attempt may start.
func handleRateLimitResponse(
	ctx context.Context,
	res *http.Response,
	attempt int,
	startTime time.Time,
	sleeper Sleeper,
) error {
	if attempt >= maxRetries {
		_ = res.Body.Close()

		return ErrMaxRetries
	}

	if wait, ok := httpclient.RetryAfter(res.Header); ok {
		if err := waitWithTimeout(ctx, wait, startTime, sleeper); err != nil {
			_ = res.Body.Close()

			return err
		}

		_ = res.Body.Close()

		return nil // Continue retry
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
