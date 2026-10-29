package wecom

import (
	"bytes"
	"context"
	"encoding/json/v2"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/nicholas-fedor/shoutrrr/internal/httpbody"
	"github.com/nicholas-fedor/shoutrrr/internal/httpclient"
	"github.com/nicholas-fedor/shoutrrr/internal/jsonopts"
	"github.com/nicholas-fedor/shoutrrr/internal/redact"
	"github.com/nicholas-fedor/shoutrrr/pkg/format"
	"github.com/nicholas-fedor/shoutrrr/pkg/services/standard"
	"github.com/nicholas-fedor/shoutrrr/pkg/types"
)

// Service sends notifications to WeCom.
type Service struct {
	// Standard provides logging and the other shared service functionality.
	standard.Standard

	// client holds the injected HTTP client and the default client.
	client httpclient.Holder

	// Config holds the webhook key and the mention lists.
	Config *Config
	// pkr reads and writes the config props by their query keys.
	pkr format.PropKeyResolver
}

// Constants for the WeCom service configuration and limits.
const (
	// apiURL is the webhook endpoint, formatted with the webhook key.
	apiURL = "https://qyapi.weixin.qq.com/cgi-bin/webhook/send?key=%s"
	// maxLength is the most bytes the content of a text message can hold.
	maxLength = 2048
	// defaultTime is the send budget of WeCom.
	defaultTime = 30 * time.Second
)

// Error variables for the WeCom service.
var (
	// ErrLargeMessage indicates a message longer than [maxLength] bytes.
	ErrLargeMessage = fmt.Errorf("message exceeds the max length of %d bytes", maxLength)
	// ErrSendFailed indicates a failed request or an error the WeCom API
	// reported.
	ErrSendFailed = errors.New("failed to send notification to WeCom")
	// ErrKeyRequired indicates a send whose params leave the webhook key
	// empty.
	ErrKeyRequired = errors.New("webhook key is required")
)

// Compile-time checks that Service implements the interfaces the router relies on.
var (
	_ types.Service          = (*Service)(nil)
	_ types.ContextSender    = (*Service)(nil)
	_ types.HTTPClientSetter = (*Service)(nil)
	_ types.ServiceTimeout   = (*Service)(nil)
)

// GetID returns the service identifier.
//
// Returns:
//   - string: the service scheme, [Scheme].
func (s *Service) GetID() string {
	return Scheme
}

// Initialize configures the service with a URL and logger.
//
// Parameters:
//   - serviceURL: the service URL.
//   - logger: the logger for service output.
//
// Returns:
//   - error: the error from reading the URL, such as [ErrEmptyKey] or
//     [ErrInvalidKey].
func (s *Service) Initialize(serviceURL *url.URL, logger types.StdLogger) error {
	s.SetLogger(logger)
	s.Config = &Config{}
	s.pkr = format.NewPropKeyResolver(s.Config)

	return s.Config.SetURL(serviceURL)
}

// Send delivers a notification message to WeCom.
//
// It delegates to [Service.SendContext] with [context.Background].
//
// Parameters:
//   - message: the message to send.
//   - params: optional overrides for configuration fields.
//
// Returns:
//   - error: [types.ErrServiceNotInitialized] before Initialize, a validation,
//     params, or send error.
func (s *Service) Send(message string, params *types.Params) error {
	return s.SendContext(context.Background(), message, params)
}

// SendContext delivers a notification message to WeCom. A context without a
// deadline is bounded by the budget that [Service.ServiceTimeout] reports.
//
// Parameters:
//   - ctx: cancellation and deadline for the request.
//   - message: the message to send.
//   - params: optional overrides for configuration fields.
//
// Returns:
//   - error: [types.ErrServiceNotInitialized] before Initialize, a validation,
//     params, or send error, such as [ErrInvalidKey] for a key param with
//     characters a webhook key cannot have. A request error matches ctx's error
//     when ctx ends the request.
func (s *Service) SendContext(ctx context.Context, message string, params *types.Params) error {
	if s.Config == nil {
		return types.ErrServiceNotInitialized
	}

	if len(message) > maxLength {
		return ErrLargeMessage
	}

	ctx, cancel := httpclient.BoundContext(ctx, s.ServiceTimeout(params))
	defer cancel()

	config := *s.Config
	if err := s.pkr.UpdateConfigFromParams(&config, params); err != nil {
		return fmt.Errorf("updating params: %w", err)
	}

	if config.Key == "" {
		return ErrKeyRequired
	}

	// A key param replaces the URL's key, so it is validated here as well.
	if !isValidKey(config.Key) {
		return ErrInvalidKey
	}

	return s.doSend(ctx, config, message, params)
}

// ServiceTimeout reports the send budget of WeCom.
//
// Returns:
//   - time.Duration: [defaultTime], whatever the params.
func (*Service) ServiceTimeout(*types.Params) time.Duration {
	return defaultTime
}

// SetHTTPClient sets a custom HTTP client for the service. A nil client,
// including a nil *http.Client, selects the default client.
//
// Parameters:
//   - client: the HTTP client to use for requests, or nil for the default.
func (s *Service) SetHTTPClient(client types.HTTPClient) {
	s.client.Set(client)
}

// doSend sends the notification to WeCom using the configured API URL.
//
// Parameters:
//   - ctx: cancellation and deadline for the request.
//   - config: the configuration for this send.
//   - message: the message to send.
//   - params: the send params, which may set mentions.
//
// Returns:
//   - error: a payload error or the send error.
func (s *Service) doSend(ctx context.Context, config Config, message string, params *types.Params) error {
	postURL := fmt.Sprintf(apiURL, config.Key)

	payload, err := s.preparePayload(message, config, params)
	if err != nil {
		return err
	}

	return s.sendRequest(ctx, postURL, payload)
}

// getRequestBody constructs the request body for the WeCom API, with each
// comma-separated mention list sent as one entry per user ID or mobile number.
//
// Parameters:
//   - message: the message text.
//   - config: the configuration for this send.
//   - *types.Params: unused, because params are already applied to config.
//
// Returns:
//   - *RequestBody: the request body.
func (s *Service) getRequestBody(
	message string,
	config Config,
	_ *types.Params,
) *RequestBody {
	return &RequestBody{
		MsgType: "text",
		Text: TextContent{
			Content:             message,
			MentionedList:       splitMentions(config.MentionedList),
			MentionedMobileList: splitMentions(config.MentionedMobileList),
		},
	}
}

// splitMentions splits a comma-separated mention list into its entries, with
// the spaces around each entry trimmed and empty entries left out.
//
// Parameters:
//   - list: the comma-separated user IDs or mobile numbers, which may include @all.
//
// Returns:
//   - []string: the entries, or nil when the list has none.
func splitMentions(list string) []string {
	var mentions []string

	for mention := range strings.SplitSeq(list, ",") {
		if mention = strings.TrimSpace(mention); mention != "" {
			mentions = append(mentions, mention)
		}
	}

	return mentions
}

// handleResponse processes the API response and checks for errors.
//
// Parameters:
//   - ctx: the context of the send, passed to its log records.
//   - resp: the API response. At most [httpbody.MaxSize] bytes of its body are read.
//
// Returns:
//   - error: [ErrSendFailed] for an error status or a nonzero error code, or the
//     error from reading or decoding the body, which wraps [httpbody.ErrTooLarge]
//     for an oversized body.
func (s *Service) handleResponse(ctx context.Context, resp *http.Response) error {
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("%w: unexpected status %s", ErrSendFailed, resp.Status)
	}

	data, err := httpbody.Read(resp.Body, httpbody.MaxSize)
	if err != nil {
		return fmt.Errorf("reading response body: %w", err)
	}

	var response Response
	if err := json.Unmarshal(data, &response, jsonopts.Default); err != nil {
		return fmt.Errorf("unmarshaling response: %w", err)
	}

	if response.ErrCode != 0 {
		return fmt.Errorf(
			"%w: server returned error code %d: %s",
			ErrSendFailed,
			response.ErrCode,
			response.ErrMsg,
		)
	}

	s.Debug(ctx, "notification sent")

	return nil
}

// preparePayload constructs and marshals the request payload for the WeCom API.
//
// Parameters:
//   - message: the message to send.
//   - config: the configuration for this send, with the mention lists.
//   - params: the send params.
//
// Returns:
//   - []byte: the JSON payload.
//   - error: the encoding error.
func (s *Service) preparePayload(
	message string,
	config Config,
	params *types.Params,
) ([]byte, error) {
	body := s.getRequestBody(message, config, params)

	data, err := json.Marshal(body, jsonopts.Default)
	if err != nil {
		return nil, fmt.Errorf("marshaling payload to JSON: %w", err)
	}

	return data, nil
}

// sendRequest performs the HTTP POST request to the WeCom API and handles the response.
//
// Parameters:
//   - ctx: cancellation and deadline for the request.
//   - postURL: the webhook URL.
//   - payload: the JSON payload.
//
// Returns:
//   - error: the request error, or the error the WeCom API reported.
func (s *Service) sendRequest(ctx context.Context, postURL string, payload []byte) error {
	req, err := http.NewRequestWithContext(
		ctx,
		http.MethodPost,
		postURL,
		bytes.NewReader(payload),
	)
	if err != nil {
		return fmt.Errorf("creating HTTP request: %w", redact.URLError(err))
	}

	req.Header.Set("Content-Type", "application/json")

	client := s.client.Client()

	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("%w: making HTTP request: %w", ErrSendFailed, redact.URLError(err))
	}

	defer func() { _ = resp.Body.Close() }()

	return s.handleResponse(ctx, resp)
}
