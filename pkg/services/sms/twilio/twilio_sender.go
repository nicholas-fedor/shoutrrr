package twilio

import (
	"context"
	"encoding/json/v2"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/nicholas-fedor/shoutrrr/internal/httpbody"
	"github.com/nicholas-fedor/shoutrrr/internal/jsonopts"
	"github.com/nicholas-fedor/shoutrrr/internal/redact"
)

// HTTPClient defines the interface for making HTTP requests.
type HTTPClient interface {
	Do(req *http.Request) (*http.Response, error)
}

// apiErrorResponse represents an error response from the Twilio REST API.
type apiErrorResponse struct {
	Message string `json:"message"`
	Code    int    `json:"code"`
	Status  int    `json:"status"`
}

const (
	apiBaseURL         = "https://api.twilio.com/2010-04-01/Accounts"
	contentType        = "application/x-www-form-urlencoded"
	defaultHTTPTimeout = 10 * time.Second
	msgServicePrefix   = "MG"
)

// DefaultHTTPClient returns a new HTTP client with a default timeout.
func DefaultHTTPClient() HTTPClient {
	return &http.Client{
		Timeout: defaultHTTPTimeout,
	}
}

// sendToRecipient sends an SMS message to a single recipient via the Twilio API.
//
// Parameters:
//   - ctx: cancellation and deadline for the request, further bounded by [defaultHTTPTimeout].
//   - config: the configuration for this send.
//   - toNumber: the recipient phone number.
//   - message: the message to send.
//
// Returns:
//   - error: the request error, or [ErrSendFailed] for a non-success status.
func (s *Service) sendToRecipient(ctx context.Context, config *Config, toNumber, message string) error {
	body := message
	if config.Title != "" {
		body = config.Title + "\n" + message
	}

	apiURL := fmt.Sprintf("%s/%s/Messages.json", apiBaseURL, config.AccountSID)

	data := url.Values{}
	data.Set("To", toNumber)
	data.Set("Body", body)

	// Use MessagingServiceSid if the sender looks like a Messaging Service SID,
	// otherwise use it as a regular From number.
	if strings.HasPrefix(config.FromNumber, msgServicePrefix) {
		data.Set("MessagingServiceSid", config.FromNumber)
	} else {
		data.Set("From", config.FromNumber)
	}

	ctx, cancel := context.WithTimeout(ctx, defaultHTTPTimeout)
	defer cancel()

	req, err := http.NewRequestWithContext(
		ctx,
		http.MethodPost,
		apiURL,
		strings.NewReader(data.Encode()),
	)
	if err != nil {
		return fmt.Errorf("creating request: %w", redact.URLError(err))
	}

	req.Header.Set("Content-Type", contentType)
	req.SetBasicAuth(config.AccountSID, config.AuthToken)

	res, err := s.HTTPClient.Do(req)
	if err != nil {
		return fmt.Errorf("sending request to Twilio API: %w", redact.URLError(err))
	}

	defer func() { _ = res.Body.Close() }()

	if res.StatusCode < http.StatusOK || res.StatusCode >= http.StatusMultipleChoices {
		return parseAPIError(res)
	}

	return nil
}

// parseAPIError reads the Twilio API error response body and returns a descriptive error.
//
// Parameters:
//   - res: the error response. At most [httpbody.MaxSize] bytes of its body are read.
//
// Returns:
//   - error: [ErrSendFailed] with the Twilio message and code, or with the
//     status when the body holds no Twilio error.
func parseAPIError(res *http.Response) error {
	respBody, err := httpbody.Read(res.Body, httpbody.MaxSize)
	if err != nil && !errors.Is(err, httpbody.ErrTooLarge) {
		return fmt.Errorf("%w: response status %q (failed to read body)", ErrSendFailed, res.Status)
	}

	var apiErr apiErrorResponse

	err = json.Unmarshal(respBody, &apiErr, jsonopts.Default)
	if err == nil && apiErr.Message != "" {
		return fmt.Errorf("%w: %s (code %d)", ErrSendFailed, apiErr.Message, apiErr.Code)
	}

	return fmt.Errorf("%w: response status %q", ErrSendFailed, res.Status)
}
