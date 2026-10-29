// Package jsonclient provides a JSON HTTP client for making HTTP requests
// that automatically marshal and unmarshal JSON payloads.
//
// Request bodies are encoded with sorted map keys, and a nil slice or map
// encodes as an empty array or object. Responses are decoded with member names
// that match field names exactly. A repeated member name fails the decode.
// Invalid UTF-8 is replaced with U+FFFD in both directions.
package jsonclient

import (
	"bytes"
	"context"
	"encoding/json/v2"
	"errors"
	"fmt"
	"net/http"

	"github.com/nicholas-fedor/shoutrrr/internal/httpbody"
	"github.com/nicholas-fedor/shoutrrr/internal/httpclient"
	"github.com/nicholas-fedor/shoutrrr/internal/jsonopts"
	"github.com/nicholas-fedor/shoutrrr/internal/redact"
	"github.com/nicholas-fedor/shoutrrr/pkg/types"
)

// Client defines the interface for JSON HTTP operations.
type Client interface {
	Get(url string, response any) error
	Post(url string, request, response any) error
	Headers() http.Header
	ErrorResponse(err error, response any) bool
}

// ContextClient is a [Client] whose requests honor a context.
//
// The context bounds the whole request, so a canceled context or an expired
// deadline stops the request and its error matches the context's error.
type ContextClient interface {
	Client

	// GetContext fetches url using GET and unmarshals the response into response.
	GetContext(ctx context.Context, url string, response any) error
	// PostContext sends request as JSON to url and unmarshals the response into response.
	PostContext(ctx context.Context, url string, request, response any) error
}

// Error contains additional HTTP/JSON details.
type Error struct {
	err        error
	Body       string
	StatusCode int
}

// client wraps an HTTP client for JSON operations.
type client struct {
	// headers are set on every request.
	headers http.Header
	// http holds the HTTP client that sends the requests, or the default
	// client when none was given.
	http httpclient.Holder
}

// ContentType defines the default MIME type for JSON requests.
const ContentType = "application/json"

// HTTPClientErrorThreshold specifies the status code threshold for client errors (400+).
const HTTPClientErrorThreshold = 400

// ErrUnexpectedStatus indicates an unexpected HTTP response status.
var ErrUnexpectedStatus = errors.New("got unexpected HTTP status")

// DefaultClient provides a singleton JSON client using http.DefaultClient.
//
// Deprecated: Use NewWithHTTPClient with an explicit client instead.
// This will continue to use http.DefaultClient.
var DefaultClient = NewWithHTTPClient(http.DefaultClient)

// Error returns the string representation of the error.
func (je Error) Error() string {
	return je.String()
}

// String provides a human-readable description of the error.
func (je Error) String() string {
	if je.err == nil {
		return fmt.Sprintf("unknown error (HTTP %v)", je.StatusCode)
	}

	return je.err.Error()
}

// Unwrap returns the underlying error, such as [ErrUnexpectedStatus] or a JSON
// decoding error, so callers can match it with errors.Is and errors.As.
//
// Returns:
//   - error: the underlying error, or nil when there is none.
func (je Error) Unwrap() error {
	return je.err
}

// ErrorBody extracts the request body from an error if it's a jsonclient.Error.
func ErrorBody(e error) string {
	if jsonError, ok := errors.AsType[Error](e); ok {
		return jsonError.Body
	}

	return ""
}

// NewClient creates a new JSON client using the default http.Client.
//
// Deprecated: Use NewWithHTTPClient with an explicit client instead.
//
//go:fix inline
func NewClient() Client {
	return NewWithHTTPClient(http.DefaultClient)
}

// NewWithHTTPClient creates a new JSON client using the specified HTTP client.
//
// The returned client also implements [ContextClient]. Use [NewContextClient]
// to get that type directly.
//
// Parameters:
//   - httpClient: the HTTP client that sends the requests. Nil, or a nil
//     *http.Client, sends through a default client. A nil value of another
//     type is used as given, and a runtime panic from it fails the request
//     with [types.ErrNilHTTPClient].
//
// Returns:
//   - Client: a client that sends JSON requests through httpClient.
func NewWithHTTPClient(httpClient types.HTTPClient) Client {
	return NewContextClient(httpClient)
}

// NewContextClient creates a JSON client whose requests honor a context.
//
// Parameters:
//   - httpClient: the HTTP client that sends the requests, handled as for
//     [NewWithHTTPClient].
//
// Returns:
//   - ContextClient: a client that sends JSON requests through httpClient.
func NewContextClient(httpClient types.HTTPClient) ContextClient {
	c := &client{
		headers: http.Header{
			"Content-Type": []string{ContentType},
		},
		http: httpclient.Holder{},
	}
	c.http.Set(httpClient)

	return c
}

// Get fetches a URL using GET and unmarshals the response into the provided object using DefaultClient.
//
// Deprecated: Create a Client with NewWithHTTPClient and call Get on it instead.
func Get(url string, response any) error {
	if err := DefaultClient.Get(url, response); err != nil {
		return fmt.Errorf("getting JSON: %w", err)
	}

	return nil
}

// Post sends a request as JSON and unmarshals the response into the provided object using DefaultClient.
//
// Deprecated: Create a Client with NewWithHTTPClient and call Post on it instead.
func Post(url string, request, response any) error {
	if err := DefaultClient.Post(url, request, response); err != nil {
		return fmt.Errorf("posting JSON: %w", err)
	}

	return nil
}

// ErrorResponse checks if an error is a JSON error and unmarshals its body into the response.
func (c *client) ErrorResponse(err error, response any) bool {
	if errMsg, ok := errors.AsType[Error](err); ok {
		return json.Unmarshal([]byte(errMsg.Body), response, jsonopts.Default) == nil
	}

	return false
}

// Get fetches a URL using GET and unmarshals the response into the provided object.
//
// It delegates to [client.GetContext] with [context.Background].
//
// Parameters:
//   - url: the URL to fetch.
//   - response: the value the JSON response is decoded into.
//
// Returns:
//   - error: an [Error] for an error status or invalid JSON, or the request error.
func (c *client) Get(url string, response any) error {
	return c.GetContext(context.Background(), url, response)
}

// GetContext fetches a URL using GET and unmarshals the response into the provided object.
//
// Parameters:
//   - ctx: cancellation and deadline for the request.
//   - url: the URL to fetch.
//   - response: the value the JSON response is decoded into.
//
// Returns:
//   - error: an [Error] for an error status or invalid JSON, or the request error,
//     which matches ctx's error when ctx ends the request.
func (c *client) GetContext(ctx context.Context, url string, response any) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, http.NoBody)
	if err != nil {
		return fmt.Errorf("creating GET request: %w", redact.URLError(err))
	}

	for key, val := range c.headers {
		req.Header.Set(key, val[0])
	}

	res, err := c.http.Client().Do(req)
	if err != nil {
		return fmt.Errorf("executing GET request: %w", redact.URLError(err))
	}

	defer func() { _ = res.Body.Close() }()

	return parseResponse(res, response)
}

// Headers returns the default headers for requests.
func (c *client) Headers() http.Header {
	return c.headers
}

// Post sends a request as JSON and unmarshals the response into the provided object.
//
// It delegates to [client.PostContext] with [context.Background].
//
// Parameters:
//   - url: the URL to post to.
//   - request: the request body. A string is sent as is, and any other value is encoded as JSON.
//   - response: the value the JSON response is decoded into.
//
// Returns:
//   - error: an [Error] for an error status or invalid JSON, or the request error.
func (c *client) Post(url string, request, response any) error {
	return c.PostContext(context.Background(), url, request, response)
}

// PostContext sends a request as JSON and unmarshals the response into the provided object.
//
// Parameters:
//   - ctx: cancellation and deadline for the request.
//   - url: the URL to post to.
//   - request: the request body. A string is sent as is, and any other value is encoded as JSON.
//   - response: the value the JSON response is decoded into.
//
// Returns:
//   - error: an [Error] for an error status or invalid JSON, or the request error,
//     which matches ctx's error when ctx ends the request.
func (c *client) PostContext(ctx context.Context, url string, request, response any) error {
	var err error

	var body []byte

	if strReq, ok := request.(string); ok {
		// If the request is a string, pass it through without serializing
		body = []byte(strReq)
	} else {
		body, err = json.Marshal(request, jsonopts.Default)
		if err != nil {
			return fmt.Errorf("marshaling request to JSON: %w", err)
		}
	}

	req, err := http.NewRequestWithContext(
		ctx,
		http.MethodPost,
		url,
		bytes.NewReader(body),
	)
	if err != nil {
		return fmt.Errorf("creating POST request: %w", redact.URLError(err))
	}

	for key, val := range c.headers {
		req.Header.Set(key, val[0])
	}

	res, err := c.http.Client().Do(req)
	if err != nil {
		return fmt.Errorf("sending POST request: %w", redact.URLError(err))
	}

	defer func() { _ = res.Body.Close() }()

	return parseResponse(res, response)
}

// parseResponse parses the HTTP response and unmarshals it into the provided object.
//
// Parameters:
//   - res: the response. At most [httpbody.MaxSize] bytes of its body are read.
//   - response: the value the JSON body is decoded into.
//
// Returns:
//   - error: an [Error] holding the body read. It wraps [ErrUnexpectedStatus]
//     for an error status. It also wraps the read error, which is
//     [httpbody.ErrTooLarge] for a body larger than [httpbody.MaxSize], on any
//     status. Otherwise it wraps the decoding error.
func parseResponse(res *http.Response, response any) error {
	body, err := httpbody.Read(res.Body, httpbody.MaxSize)

	if res.StatusCode >= HTTPClientErrorThreshold {
		if err != nil {
			err = fmt.Errorf("%w: %v: %w", ErrUnexpectedStatus, res.Status, err)
		} else {
			err = fmt.Errorf("%w: %v", ErrUnexpectedStatus, res.Status)
		}
	}

	if err == nil {
		err = json.Unmarshal(body, response, jsonopts.Default)
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
