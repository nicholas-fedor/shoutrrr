package twilio

import (
	"cmp"
	"context"
	"encoding/json/v2"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/nicholas-fedor/shoutrrr/internal/httpbody"
	"github.com/nicholas-fedor/shoutrrr/internal/jsonopts"
	"github.com/nicholas-fedor/shoutrrr/internal/logging"
	"github.com/nicholas-fedor/shoutrrr/internal/redact"
	"github.com/nicholas-fedor/shoutrrr/pkg/types"
)

// HTTPClient defines the interface for making HTTP requests.
//
// Deprecated: The service takes a [types.HTTPClient], which HTTPClient is an
// alias of. Use [types.HTTPClient].
//
//go:fix inline
type HTTPClient = types.HTTPClient

// apiErrorResponse represents an error response from the Twilio REST API.
type apiErrorResponse struct {
	// Message is the Twilio description of the error.
	Message string `json:"message"`
	// Code is the Twilio error code.
	Code int `json:"code"`
	// Status is the HTTP status code of the response.
	Status int `json:"status"`
}

// messageReply holds the part of a created message that a send logs.
type messageReply struct {
	// SID identifies the message in Twilio.
	SID string `json:"sid"`
}

// Settings for requests to the Twilio REST API.
const (
	// apiBaseURL is the accounts endpoint of the API. A message request adds
	// the account SID and Messages.json to it.
	apiBaseURL = "https://api.twilio.com/2010-04-01/Accounts"
	// contentType is the content type of the URL-encoded form request body.
	contentType = "application/x-www-form-urlencoded"
	// defaultHTTPTimeout is the part of the send budget for each recipient,
	// and the timeout of the client [DefaultHTTPClient] returns.
	defaultHTTPTimeout = 10 * time.Second
)

// DefaultHTTPClient returns a new HTTP client with a default timeout.
//
// Returns:
//   - HTTPClient: an [http.Client] with a timeout of [defaultHTTPTimeout].
//
// Deprecated: The service builds its own default client and bounds each send
// by its send budget. Inject an *http.Client with [Service.SetHTTPClient]
// instead.
func DefaultHTTPClient() types.HTTPClient {
	return &http.Client{
		Timeout: defaultHTTPTimeout,
	}
}

// encodeMessageForm encodes the form fields that every recipient's request of
// a send shares, the body and the sender.
//
// Parameters:
//   - config: the configuration for this send.
//   - body: the message body.
//
// Returns:
//   - string: the URL-encoded fields, without the recipient.
func encodeMessageForm(config *Config, body string) string {
	senderKey := "From"
	if config.usesMessagingService() {
		senderKey = "MessagingServiceSid"
	}

	return "Body=" + url.QueryEscape(body) + "&" + senderKey + "=" + url.QueryEscape(config.FromNumber)
}

// sendToRecipient sends an SMS message to a single recipient via the Twilio
// API. A created message is logged at Debug with its SID.
//
// Parameters:
//   - ctx: cancellation and deadline for the request.
//   - config: the configuration for this send.
//   - toNumber: the recipient phone number.
//   - form: the shared form fields from [encodeMessageForm].
//
// Returns:
//   - error: the request error, or [ErrSendFailed] for a non-success status.
func (s *Service) sendToRecipient(ctx context.Context, config *Config, toNumber, form string) error {
	apiURL := apiBaseURL + "/" + config.AccountSID + "/Messages.json"

	req, err := http.NewRequestWithContext(
		ctx,
		http.MethodPost,
		apiURL,
		strings.NewReader("To="+url.QueryEscape(toNumber)+"&"+form),
	)
	if err != nil {
		return fmt.Errorf("creating request: %w", redact.URLError(err))
	}

	req.Header.Set("Content-Type", contentType)
	req.SetBasicAuth(cmp.Or(config.APIKey, config.AccountSID), config.AuthToken)

	res, err := s.client.Prefer(s.HTTPClient).Do(req)
	if err != nil {
		return fmt.Errorf("sending request to Twilio API: %w", redact.URLError(err))
	}

	res.Body = httpbody.Limit(res.Body, res.ContentLength)
	defer func() { _ = res.Body.Close() }()

	if res.StatusCode < http.StatusOK || res.StatusCode >= http.StatusMultipleChoices {
		return parseAPIError(res)
	}

	// The reply only feeds a Debug record, so without Debug it is not decoded.
	if !s.Slog().Enabled(ctx, slog.LevelDebug) {
		return nil
	}

	// Twilio created the message, so a reply that cannot be read is only
	// logged.
	var reply messageReply
	if err := json.UnmarshalRead(res.Body, &reply, jsonopts.Default); err != nil {
		s.LogAttrs(ctx, slog.LevelDebug, "message created, reply not read", slog.Any(logging.KeyError, err))

		return nil
	}

	s.LogAttrs(ctx, slog.LevelDebug, "message created", slog.String(logging.KeyMessageID, reply.SID))

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
