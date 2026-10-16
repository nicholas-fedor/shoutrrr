package signal

import (
	"bytes"
	"context"
	"crypto/tls"
	"encoding/json/v2"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/nicholas-fedor/shoutrrr/internal/meta"
	"github.com/nicholas-fedor/shoutrrr/internal/redact"
	"github.com/nicholas-fedor/shoutrrr/internal/transport"
	"github.com/nicholas-fedor/shoutrrr/pkg/types"
)

const (
	contentType        = "application/json"
	defaultHTTPTimeout = 30 * time.Second
	// maxResponseBodySize caps how much of a /v2/send body is read. It holds
	// the failed recipients of a send to a full Signal group.
	maxResponseBodySize = 1 << 20
	// maxErrorBodySize caps how much of a plain error body goes into an error.
	maxErrorBodySize = 4 * 1024
)

// buildAPIURL constructs the Signal API endpoint URL from the configuration.
//
// Parameters:
//   - config: the service configuration
//
// Returns:
//   - string: the full API endpoint URL
func (s *Service) buildAPIURL(config *Config) string {
	scheme := "https"
	if config.DisableTLS {
		scheme = "http"
	}

	return (&url.URL{
		Scheme: scheme,
		Host:   net.JoinHostPort(config.Host, strconv.Itoa(config.Port)),
		Path:   "/v2/send",
	}).String()
}

// createRequest builds the HTTP request for the Signal API.
//
// Parameters:
//   - ctx: cancellation and deadline for the request, further bounded by [defaultHTTPTimeout].
//   - config: the service configuration.
//   - payload: the payload to send (passed as pointer for efficiency).
//
// Returns:
//   - *http.Request: the constructed HTTP request.
//   - context.CancelFunc: a function to cancel the request context.
//   - error: a marshaling or request creation error.
func (s *Service) createRequest(
	ctx context.Context,
	config *Config,
	payload *sendMessagePayload,
) (*http.Request, context.CancelFunc, error) {
	apiURL := s.buildAPIURL(config)

	jsonData, err := json.Marshal(payload)
	if err != nil {
		return nil, nil, fmt.Errorf("marshaling payload to JSON: %w", err)
	}

	ctx, cancel := context.WithTimeout(
		ctx,
		defaultHTTPTimeout,
	)

	req, err := http.NewRequestWithContext(
		ctx,
		http.MethodPost,
		apiURL,
		bytes.NewReader(jsonData),
	)
	if err != nil {
		cancel()

		return nil, nil, fmt.Errorf("creating HTTP request: %w", redact.URLError(err))
	}

	req.Header.Set("Content-Type", contentType)
	req.Header.Set("User-Agent", meta.UserAgent())
	s.setAuthentication(req, config)

	return req, cancel, nil
}

// newHTTPClient returns the default HTTP client for Signal requests.
//
// Parameters:
//   - skipTLSVerify: when true, skip certificate-chain and hostname verification
//
// Returns:
//   - types.HTTPClient: a client with a 30s timeout, TLS 1.2 minimum, and optional
//     skip-verify, whose transport honors the proxy environment variables.
func (s *Service) newHTTPClient(skipTLSVerify bool) types.HTTPClient {
	tlsConfig := &tls.Config{MinVersion: tls.VersionTLS12}
	if skipTLSVerify {
		tlsConfig.InsecureSkipVerify = true

		s.Log("Warning: TLS verification is disabled, making connections insecure")
	}

	return &http.Client{
		Timeout:   defaultHTTPTimeout,
		Transport: transport.New(tlsConfig),
	}
}

// handleSendResults logs the timestamp of each send result in a successful
// response and reports the recipients the sends could not reach. A response
// that cannot be read is logged, because the API accepted the message.
//
// Parameters:
//   - body: the raw response body.
//
// Returns:
//   - error: [ErrSendFailed] wrapping [ErrRecipientsFailed] with each failed
//     recipient and its reason, or nil.
func (s *Service) handleSendResults(body []byte) error {
	results, err := parseSendResults(body)
	if err != nil {
		s.Logf("Warning: failed to parse response: %v", err)

		return nil
	}

	var failed []string

	for _, result := range results {
		if result.Errors != nil && len(result.Errors.Recipients) > 0 {
			for _, failure := range result.Errors.Recipients {
				failed = append(failed, failure.describe())
			}

			continue
		}

		if result.Timestamp == nil {
			s.Log("Warning: response has no timestamp")

			continue
		}

		s.Logf("Message sent successfully at timestamp %d", *result.Timestamp)
	}

	if len(failed) > 0 {
		return fmt.Errorf("%w: %w: %s", ErrSendFailed, ErrRecipientsFailed, strings.Join(failed, ", "))
	}

	return nil
}

// parseSendResults reads the send results from a successful response: an
// array with one result per signal-cli send, or a single result object.
//
// Parameters:
//   - body: the raw response body.
//
// Returns:
//   - []sendResult: the send results.
//   - error: when the body is neither form.
func parseSendResults(body []byte) ([]sendResult, error) {
	body = bytes.TrimSpace(body)

	if len(body) > 0 && body[0] == '{' {
		var result sendResult
		if err := json.Unmarshal(body, &result); err != nil {
			return nil, fmt.Errorf("decoding send result: %w", err)
		}

		return []sendResult{result}, nil
	}

	var results []sendResult
	if err := json.Unmarshal(body, &results); err != nil {
		return nil, fmt.Errorf("decoding send results: %w", err)
	}

	return results, nil
}

// sendRequest executes the HTTP request and processes the response.
//
// Parameters:
//   - req: the HTTP request to execute
//   - config: the per-send configuration used to select the HTTP client
//
// Returns:
//   - error: if the request fails, returns a non-success status, or reports
//     recipients it could not reach, nil otherwise. A success response that
//     fails to read or exceeds [maxResponseBodySize] is logged without
//     checking its recipient results.
func (s *Service) sendRequest(req *http.Request, config *Config) error {
	client := s.httpClientFor(config)

	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("sending HTTP request: %w", redact.URLError(err))
	}

	defer func() { _ = resp.Body.Close() }()

	accepted := resp.StatusCode >= http.StatusOK && resp.StatusCode < http.StatusMultipleChoices

	body, err := io.ReadAll(io.LimitReader(resp.Body, maxResponseBodySize+1))
	if err != nil {
		if accepted {
			s.Logf("Warning: failed to read response body, so the recipient results were not checked: %v", err)

			return nil
		}

		return fmt.Errorf("reading response body: %w", err)
	}

	truncated := len(body) > maxResponseBodySize
	if truncated {
		body = body[:maxResponseBodySize]
	}

	if !accepted {
		return formatSendError(resp.StatusCode, body, truncated)
	}

	// The API accepted the message, but a cut body cannot be read for the
	// per-recipient results.
	if truncated {
		s.Logf("Warning: response exceeds %d bytes, so the recipient results were not checked", maxResponseBodySize)

		return nil
	}

	return s.handleSendResults(body)
}

// httpClientFor returns the client for this send.
// An injected client is used as-is. The default client matches config.SkipTLSVerify.
//
// Parameters:
//   - config: the per-send configuration
//
// Returns:
//   - types.HTTPClient: the client used for this request
func (s *Service) httpClientFor(config *Config) types.HTTPClient {
	if s.injectedHTTPClient && s.httpClient != nil {
		return s.httpClient
	}

	skip := config != nil && config.SkipTLSVerify
	initializedSkip := s.Config != nil && s.Config.SkipTLSVerify

	if s.httpClient != nil && skip == initializedSkip {
		return s.httpClient
	}

	return s.newHTTPClient(skip)
}

// setAuthentication configures HTTP authentication headers on the request.
// disabletls and skiptlsverify send Bearer or Basic credentials over an
// unverified or cleartext transport. Use those modes only on trusted networks.
//
// Parameters:
//   - req: the HTTP request to modify
//   - config: the service configuration containing credentials
func (s *Service) setAuthentication(req *http.Request, config *Config) {
	if config.Token != "" {
		req.Header.Set("Authorization", "Bearer "+config.Token)
	} else if config.User != "" {
		req.SetBasicAuth(config.User, config.Password)
	}
}

// formatSendError builds an error from a non-success API response.
//
// Parameters:
//   - statusCode: the HTTP status code
//   - body: the raw response body
//   - truncated: whether the body was cut at [maxResponseBodySize]
//
// Returns:
//   - error: ErrSendFailed wrapped with status and API details, or with the
//     trimmed body cut at [maxErrorBodySize]
func formatSendError(statusCode int, body []byte, truncated bool) error {
	suffix := ""
	if truncated {
		suffix = " [truncated]"
	}

	var apiErr sendErrorResponse
	if err := json.Unmarshal(body, &apiErr); err == nil && apiErr.Error != "" {
		var details []string
		if apiErr.Account != "" {
			details = append(details, "account "+apiErr.Account)
		}

		if len(apiErr.ChallengeTokens) > 0 {
			details = append(details, "challenge tokens: "+strings.Join(apiErr.ChallengeTokens, ","))
		}

		if len(details) > 0 {
			return fmt.Errorf(
				"%w: server returned status %d: %s (%s)%s",
				ErrSendFailed,
				statusCode,
				apiErr.Error,
				strings.Join(details, "; "),
				suffix,
			)
		}

		return fmt.Errorf("%w: server returned status %d: %s%s", ErrSendFailed, statusCode, apiErr.Error, suffix)
	}

	body = bytes.TrimSpace(body)
	if len(body) > maxErrorBodySize {
		body = body[:maxErrorBodySize]
		suffix = " [truncated]"
	}

	if len(body) > 0 {
		return fmt.Errorf(
			"%w: server returned status %d: %s%s",
			ErrSendFailed,
			statusCode,
			body,
			suffix,
		)
	}

	return fmt.Errorf("%w: server returned status %d%s", ErrSendFailed, statusCode, suffix)
}
