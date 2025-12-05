package gotify

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"

	"github.com/nicholas-fedor/shoutrrr/pkg/util/jsonclient"
)

// Sender handles HTTP request execution and response processing.
type Sender interface {
	SendRequest(client *http.Client, url string, request *MessageRequest, headers http.Header) error
}

// DefaultSender provides the default implementation of Sender.
type DefaultSender struct{}

// SendRequest handles the HTTP request.
// This function executes the actual HTTP POST request to the Gotify API endpoint,
// handling both successful responses and error conditions with appropriate error wrapping.
// Parameters:
//   - client: HTTP client to use for the request
//   - url: The complete API endpoint URL to send the request to
//   - request: The JSON payload to send in the request body
//   - headers: Optional headers to set on the request
//
// Returns: error if the request fails or server returns an error, nil on success.
func (s *DefaultSender) SendRequest(
	client *http.Client,
	url string,
	request *MessageRequest,
	headers http.Header,
) error {
	// Prepare response structure to capture API response
	response := &messageResponse{}

	var err error
	if len(headers) > 0 {
		// Use direct HTTP client when custom headers are needed
		err = s.sendRequestWithHeaders(client, url, request, response, headers)
	} else {
		// Use JSON client for standard requests - this will handle error extraction
		jsonClient := jsonclient.NewWithHTTPClient(client)

		err = jsonClient.Post(url, request, response)
		if err != nil {
			// Try to extract structured error
			errorRes := &responseError{}
			if jsonClient.ErrorResponse(err, errorRes) {
				return fmt.Errorf("server error: %w", errorRes)
			}

			return fmt.Errorf("%s: %w", ErrSendFailed.Error(), err)
		}

		return nil
	}

	if err != nil {
		// Return generic error with context
		return fmt.Errorf("%s: %w", ErrSendFailed.Error(), err)
	}

	// Request completed successfully
	return nil
}

// sendRequestWithHeaders sends a request with custom headers using the underlying HTTP client.
// This method is used when per-request headers are needed, bypassing the jsonclient
// to avoid modifying shared header state.
// Parameters:
//   - client: HTTP client to use
//   - url: The complete API endpoint URL to send the request to
//   - request: The JSON payload to send in the request body
//   - response: The response structure to unmarshal into
//   - headers: Custom headers to set on the request
//
// Returns: error if the request fails or server returns an error, nil on success.
func (s *DefaultSender) sendRequestWithHeaders(
	client *http.Client,
	url string,
	request *MessageRequest,
	response *messageResponse,
	headers http.Header,
) error {
	// Marshal the request to JSON
	body, err := json.Marshal(request)
	if err != nil {
		return fmt.Errorf("%s: %w", ErrMarshalRequest.Error(), err)
	}

	// Create the HTTP request
	req, err := http.NewRequestWithContext(
		context.Background(),
		http.MethodPost,
		url,
		bytes.NewReader(body),
	)
	if err != nil {
		return fmt.Errorf("%s: %w", ErrCreateRequest.Error(), err)
	}

	// Set Content-Type header
	req.Header.Set("Content-Type", "application/json")

	// Set custom headers
	for key, values := range headers {
		for _, value := range values {
			req.Header.Add(key, value)
		}
	}

	// Execute the request
	res, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("%s: %w", ErrSendRequest.Error(), err)
	}

	defer func() { _ = res.Body.Close() }()

	// Read the response body
	body, err = io.ReadAll(res.Body)
	if err != nil {
		return fmt.Errorf("%s: %w", ErrReadResponse.Error(), err)
	}

	if res.StatusCode >= 400 { //nolint:mnd
		errorRes := &responseError{}
		if s.extractErrorResponse(body, errorRes) {
			return fmt.Errorf("server error: %w", errorRes)
		}

		return fmt.Errorf("%w: %v", ErrUnexpectedStatus, res.Status)
	}

	err = json.Unmarshal(body, response)
	if err != nil {
		return fmt.Errorf("%s: %w", ErrParseResponse.Error(), err)
	}

	return nil
}

// extractErrorResponse attempts to extract a structured error from a failed request.
func (s *DefaultSender) extractErrorResponse(body []byte, errorRes *responseError) bool {
	return json.Unmarshal(body, errorRes) == nil
}
