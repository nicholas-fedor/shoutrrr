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
	"github.com/nicholas-fedor/shoutrrr/pkg/types"
)

const (
	contentType        = "application/json"
	defaultHTTPTimeout = 30 * time.Second
	// maxResponseBodySize caps how much of a /v2/send body we buffer.
	maxResponseBodySize = 4 * 1024
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
//   - config: the service configuration
//   - payload: the payload to send (passed as pointer for efficiency)
//
// Returns:
//   - *http.Request: the constructed HTTP request
//   - context.CancelFunc: a function to cancel the request context
//   - error: if request creation fails, nil otherwise
func (s *Service) createRequest(
	config *Config,
	payload *sendMessagePayload,
) (*http.Request, context.CancelFunc, error) {
	apiURL := s.buildAPIURL(config)

	jsonData, err := json.Marshal(payload)
	if err != nil {
		return nil, nil, fmt.Errorf("marshaling payload to JSON: %w", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), defaultHTTPTimeout)

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, apiURL, bytes.NewReader(jsonData))
	if err != nil {
		cancel()

		return nil, nil, fmt.Errorf("creating HTTP request: %w", err)
	}

	req.Header.Set("Content-Type", contentType)
	req.Header.Set("User-Agent", meta.UserAgent())
	s.setAuthentication(req, config)

	return req, cancel, nil
}

// newHTTPClient returns the default HTTP client for Signal requests.
//
// Returns:
//   - An HTTP client with a 30s timeout, TLS 1.2 minimum, and optional skip-verify.
func (s *Service) newHTTPClient() types.HTTPClient {
	tlsConfig := &tls.Config{MinVersion: tls.VersionTLS12}
	if s.Config != nil && s.Config.SkipTLSVerify {
		tlsConfig.InsecureSkipVerify = true

		s.Log("Warning: TLS verification is disabled, making connections insecure")
	}

	transport := &http.Transport{TLSClientConfig: tlsConfig}
	if defaultTransport, ok := http.DefaultTransport.(*http.Transport); ok {
		transport = defaultTransport.Clone()
		transport.TLSClientConfig = tlsConfig
	}

	return &http.Client{
		Timeout:   defaultHTTPTimeout,
		Transport: transport,
	}
}

// parseResponse logs the success timestamp from the Signal API body.
//
// Parameters:
//   - body: the raw response body
func (s *Service) parseResponse(body []byte) {
	var response sendMessageResponse
	if err := json.Unmarshal(body, &response); err != nil {
		s.Logf("Warning: failed to parse response: %v", err)
	} else {
		s.Logf("Message sent successfully at timestamp %d", response.Timestamp)
	}
}

// sendRequest executes the HTTP request and processes the response.
//
// Parameters:
//   - req: the HTTP request to execute
//
// Returns:
//   - error: if the request fails or returns a non-success status, nil otherwise
func (s *Service) sendRequest(req *http.Request) error {
	client := s.httpClient
	if client == nil {
		client = s.newHTTPClient()
	}

	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("sending HTTP request: %w", err)
	}

	defer func() { _ = resp.Body.Close() }()

	body, err := io.ReadAll(io.LimitReader(resp.Body, maxResponseBodySize+1))
	if err != nil {
		return fmt.Errorf("reading response body: %w", err)
	}

	truncated := len(body) > maxResponseBodySize
	if truncated {
		body = body[:maxResponseBodySize]
	}

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return formatSendError(resp.StatusCode, body, truncated)
	}

	s.parseResponse(body)

	return nil
}

// setAuthentication configures HTTP authentication headers on the request.
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
//   - truncated: whether the body was cut at maxResponseBodySize
//
// Returns:
//   - error: ErrSendFailed wrapped with status and API details
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

	if len(body) > 0 {
		return fmt.Errorf(
			"%w: server returned status %d: %s%s",
			ErrSendFailed,
			statusCode,
			bytes.TrimSpace(body),
			suffix,
		)
	}

	return fmt.Errorf("%w: server returned status %d%s", ErrSendFailed, statusCode, suffix)
}
