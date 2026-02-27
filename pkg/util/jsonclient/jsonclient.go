// Package jsonclient provides a JSON HTTP client for making HTTP requests
// that automatically marshal and unmarshal JSON payloads.
package jsonclient

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
)

// Client wraps http.Client for JSON operations.
type client struct {
	httpClient *http.Client
	headers    http.Header
	indent     string
}

// ContentType defines the default MIME type for JSON requests.
const ContentType = "application/json"

// HTTPClientErrorThreshold specifies the status code threshold for client errors (400+).
const HTTPClientErrorThreshold = 400

// ErrUnexpectedStatus indicates an unexpected HTTP response status.
var (
	ErrUnexpectedStatus = errors.New("got unexpected HTTP status")
)

// DefaultClient provides a singleton JSON client using http.DefaultClient.
var DefaultClient = NewClient()

// Get fetches a URL using GET and unmarshals the response into the provided object using DefaultClient.
func Get(url string, response any) error {
	if err := DefaultClient.Get(url, response); err != nil {
		return fmt.Errorf("getting JSON from %q: %w", url, err)
	}

	return nil
}

// Post sends a request as JSON and unmarshals the response into the provided object using DefaultClient.
func Post(url string, request, response any) error {
	if err := DefaultClient.Post(url, request, response); err != nil {
		return fmt.Errorf("posting JSON to %q: %w", url, err)
	}

	return nil
}

// NewClient creates a new JSON client using the default http.Client.
func NewClient() Client {
	return NewWithHTTPClient(http.DefaultClient)
}

// NewWithHTTPClient creates a new JSON client using the specified http.Client.
func NewWithHTTPClient(httpClient *http.Client) Client {
	return &client{
		httpClient: httpClient,
		headers: http.Header{
			"Content-Type": []string{ContentType},
		},
	}
}

// Headers returns the default headers for requests.
func (c *client) Headers() http.Header {
	return c.headers
}

// Get fetches a URL using GET and unmarshals the response into the provided object.
func (c *client) Get(url string, response any) error {
	req, err := http.NewRequestWithContext(context.Background(), http.MethodGet, url, http.NoBody)
	if err != nil {
		return fmt.Errorf("creating GET request for %q: %w", url, err)
	}

	for key, val := range c.headers {
		req.Header.Set(key, val[0])
	}

	//nolint:gosec // This is an HTTP client library; making HTTP requests is the intended functionality
	res, err := c.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("executing GET request to %q: %w", url, err)
	}

	defer func() { _ = res.Body.Close() }()

	return parseResponse(res, response)
}

// Post sends a request as JSON and unmarshals the response into the provided object.
func (c *client) Post(url string, request, response any) error {
	var err error

	var body []byte

	if strReq, ok := request.(string); ok {
		// If the request is a string, pass it through without serializing
		body = []byte(strReq)
	} else {
		body, err = json.MarshalIndent(request, "", c.indent)
		if err != nil {
			return fmt.Errorf("marshaling request to JSON: %w", err)
		}
	}

	req, err := http.NewRequestWithContext(
		context.Background(),
		http.MethodPost,
		url,
		bytes.NewReader(body),
	)
	if err != nil {
		return fmt.Errorf("creating POST request for %q: %w", url, err)
	}

	for key, val := range c.headers {
		req.Header.Set(key, val[0])
	}

	//nolint:gosec // This is an HTTP client library; making HTTP requests is the intended functionality
	res, err := c.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("sending POST request to %q: %w", url, err)
	}

	defer func() { _ = res.Body.Close() }()

	return parseResponse(res, response)
}

// ErrorResponse checks if an error is a JSON error and unmarshals its body into the response.
func (c *client) ErrorResponse(err error, response any) bool {
	var errMsg Error
	if errors.As(err, &errMsg) {
		return json.Unmarshal([]byte(errMsg.Body), response) == nil
	}

	return false
}

// parseResponse parses the HTTP response and unmarshals it into the provided object.
func parseResponse(res *http.Response, response any) error {
	body, err := io.ReadAll(res.Body)

	if res.StatusCode >= HTTPClientErrorThreshold {
		err = fmt.Errorf("%w: %v", ErrUnexpectedStatus, res.Status)
	}

	if err == nil {
		err = json.Unmarshal(body, response)
	}

	if err != nil {
		if body == nil {
			body = []byte{}
		}

		return Error{
			StatusCode: res.StatusCode,
			Body:       string(body),
			err:        err,
		}
	}

	return nil
}
