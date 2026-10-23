package gotify

import (
	"bytes"
	"context"
	"encoding/json/v2"
	"errors"
	"fmt"
	"net/http"

	"github.com/nicholas-fedor/shoutrrr/internal/httpbody"
	"github.com/nicholas-fedor/shoutrrr/internal/jsonopts"
	"github.com/nicholas-fedor/shoutrrr/internal/redact"
	"github.com/nicholas-fedor/shoutrrr/pkg/types"
	"github.com/nicholas-fedor/shoutrrr/pkg/util/jsonclient"
)

// Sender handles HTTP request execution and response processing.
type Sender interface {
	SendRequest(client types.HTTPClient, url string, request *MessageRequest, headers http.Header) error
}

// DefaultSender provides the default implementation of Sender.
type DefaultSender struct{}

// contextSender sends a request bounded by a context. [DefaultSender] implements it.
type contextSender interface {
	SendRequestContext(
		ctx context.Context,
		client types.HTTPClient,
		url string,
		request *MessageRequest,
		headers http.Header,
	) error
}

// SendRequest handles the HTTP request.
//
// It delegates to [DefaultSender.SendRequestContext] with [context.Background].
//
// Parameters:
//   - client: HTTP client to use for the request.
//   - url: the complete API endpoint URL to send the request to.
//   - request: the JSON payload to send in the request body.
//   - headers: optional headers to set on the request.
//
// Returns:
//   - error: the request error or the error the server reported.
func (s *DefaultSender) SendRequest(
	client types.HTTPClient,
	url string,
	request *MessageRequest,
	headers http.Header,
) error {
	return s.SendRequestContext(context.Background(), client, url, request, headers)
}

// SendRequestContext executes the HTTP POST request to the Gotify API endpoint,
// handling both successful responses and error conditions with appropriate error wrapping.
//
// Parameters:
//   - ctx: cancellation and deadline for the request.
//   - client: HTTP client to use for the request.
//   - url: the complete API endpoint URL to send the request to.
//   - request: the JSON payload to send in the request body.
//   - headers: optional headers to set on the request.
//
// Returns:
//   - error: the request error or the error the server reported. A request
//     error matches ctx's error when ctx ends the request.
func (s *DefaultSender) SendRequestContext(
	ctx context.Context,
	client types.HTTPClient,
	url string,
	request *MessageRequest,
	headers http.Header,
) error {
	// Prepare response structure to capture API response
	response := &messageResponse{}

	var err error

	if len(headers) == 0 {
		// Use JSON client for standard requests - this will handle error extraction
		jsonClient := jsonclient.NewContextClient(client)

		err = jsonClient.PostContext(ctx, url, request, response)
		if err != nil {
			if jsonErr, ok := errors.AsType[jsonclient.Error](err); ok {
				if gotifyErr := serverError(jsonErr.StatusCode, []byte(jsonErr.Body)); gotifyErr != nil {
					return gotifyErr
				}
			}

			return err
		}

		return nil
	}

	// Use direct HTTP client when custom headers are needed
	body, err := s.sendRequestWithHeaders(ctx, client, url, request, headers)
	if err != nil {
		return err
	}

	err = json.Unmarshal(body, response, jsonopts.Default)
	if err != nil {
		return fmt.Errorf("%w: %w", ErrParseResponse, err)
	}

	return nil
}

// handleResponseError checks the response status and extracts error information if present.
//
// Parameters:
//   - res: the response.
//   - body: the response body.
//
// Returns:
//   - error: the Gotify error in the body for an error status, or
//     [ErrUnexpectedStatus] with the status when the body has none, or nil.
func (s *DefaultSender) handleResponseError(res *http.Response, body []byte) error {
	if res.StatusCode < http.StatusBadRequest {
		return nil
	}

	if err := serverError(res.StatusCode, body); err != nil {
		return err
	}

	return fmt.Errorf("%w: %v", ErrUnexpectedStatus, res.Status)
}

// sendRequestWithHeaders sends a request with custom headers using the underlying HTTP client.
// This method is used when per-request headers are needed, bypassing the jsonclient
// to avoid modifying shared header state.
//
// Parameters:
//   - ctx: cancellation and deadline for the request.
//   - client: HTTP client to use.
//   - url: the complete API endpoint URL to send the request to.
//   - request: the JSON payload to send in the request body.
//   - headers: custom headers to set on the request.
//
// Returns:
//   - []byte: the response body.
//   - error: the marshaling, request, or response error.
func (s *DefaultSender) sendRequestWithHeaders(
	ctx context.Context,
	client types.HTTPClient,
	url string,
	request *MessageRequest,
	headers http.Header,
) ([]byte, error) {
	body, err := json.Marshal(request, jsonopts.Default)
	if err != nil {
		return nil, fmt.Errorf("%w: %w", ErrMarshalRequest, err)
	}

	req, err := http.NewRequestWithContext(
		ctx,
		http.MethodPost,
		url,
		bytes.NewReader(body),
	)
	if err != nil {
		return nil, fmt.Errorf("%w: %w", ErrCreateRequest, redact.URLError(err))
	}

	s.setRequestHeaders(req, headers)

	res, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("%w: %w", ErrSendRequest, redact.URLError(err))
	}

	defer func() { _ = res.Body.Close() }()

	body, err = httpbody.Read(res.Body, httpbody.MaxSize)
	if err != nil {
		return nil, fmt.Errorf("%w: %w", ErrReadResponse, err)
	}

	if err := s.handleResponseError(res, body); err != nil {
		return nil, err
	}

	return body, nil
}

// setRequestHeaders sets the Content-Type and custom headers on the HTTP request.
func (s *DefaultSender) setRequestHeaders(req *http.Request, headers http.Header) {
	req.Header.Set("Content-Type", "application/json")

	for key, values := range headers {
		for _, value := range values {
			req.Header.Add(key, value)
		}
	}
}

// serverError returns the Gotify error in the body of an error response,
// labeled as a client or server error by status.
//
// Parameters:
//   - statusCode: the response status code.
//   - body: the response body.
//
// Returns:
//   - error: the labeled Gotify error, or nil for a body without a Gotify
//     error.
func serverError(statusCode int, body []byte) error {
	errorRes := &responseError{}
	if json.Unmarshal(body, errorRes, jsonopts.Default) == nil && errorRes.Name != "" {
		if statusCode >= http.StatusInternalServerError {
			return fmt.Errorf("server error (%d): %w", statusCode, errorRes)
		}

		return fmt.Errorf("client error (%d): %w", statusCode, errorRes)
	}

	return nil
}
