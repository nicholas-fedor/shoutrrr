package lark

import (
	"bytes"
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json/v2"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"strconv"
	"time"

	"github.com/nicholas-fedor/shoutrrr/internal/httpbody"
	"github.com/nicholas-fedor/shoutrrr/internal/httpclient"
	"github.com/nicholas-fedor/shoutrrr/internal/jsonopts"
	"github.com/nicholas-fedor/shoutrrr/internal/redact"
	"github.com/nicholas-fedor/shoutrrr/pkg/format"
	"github.com/nicholas-fedor/shoutrrr/pkg/services/standard"
	"github.com/nicholas-fedor/shoutrrr/pkg/types"
)

// Service sends notifications to Lark.
type Service struct {
	// Standard provides logging and the other shared service functionality.
	standard.Standard

	// client holds the injected HTTP client and the default client.
	client httpclient.Holder

	// Config holds the host, the bot token, and the message options.
	Config *Config
	// pkr reads and writes the config props by their query keys.
	pkr format.PropKeyResolver
}

// Constants for the Lark service configuration and limits.
const (
	// apiFormat is the webhook endpoint, formatted with the host and the bot
	// token.
	apiFormat = "https://%s/open-apis/bot/v2/hook/%s"
	// maxLength is the most bytes a message can hold.
	maxLength = 4096
	// defaultTime bounds each request of the default client and is the send
	// budget the router gives Lark.
	defaultTime = 30 * time.Second
)

// Hosts the custom bot webhooks are served from.
const (
	// larkHost is the host of Lark, the international service.
	larkHost = "open.larksuite.com"
	// feishuHost is the host of Feishu, the service in mainland China.
	feishuHost = "open.feishu.cn"
)

// Error variables for the Lark service.
var (
	// ErrInvalidHost indicates a host other than [larkHost] or [feishuHost].
	ErrInvalidHost = errors.New("invalid host, use 'open.larksuite.com' or 'open.feishu.cn'")
	// ErrNoPath indicates a service URL or send without the bot token.
	ErrNoPath = errors.New(
		"no path, path like 'xxx' in 'https://open.larksuite.com/open-apis/bot/v2/hook/xxx'",
	)
	// ErrLargeMessage indicates a message longer than [maxLength] bytes.
	ErrLargeMessage = errors.New("message exceeds the max length")
	// ErrMissingHost indicates a send whose configuration has no host.
	ErrMissingHost = errors.New("host is required but not specified in the configuration")
	// ErrSendFailed indicates a failed request, an error status, or an error
	// code from the Lark API.
	ErrSendFailed = errors.New("failed to send notification to Lark")
	// ErrInvalidSignature indicates that the request signature could not be
	// computed.
	ErrInvalidSignature = errors.New("failed to generate valid signature")
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
//   - error: the error from reading the URL, such as [ErrInvalidHost] or
//     [ErrNoPath].
func (s *Service) Initialize(serviceURL *url.URL, logger types.StdLogger) error {
	s.SetLogger(logger)
	s.client.SetDefault(defaultTime, nil)
	s.Config = &Config{}
	s.pkr = format.NewPropKeyResolver(s.Config)

	return s.Config.SetURL(serviceURL)
}

// Send delivers a notification message to Lark.
//
// It delegates to [Service.SendContext] with [context.Background].
//
// Parameters:
//   - message: the message to send.
//   - params: optional overrides for configuration fields.
//
// Returns:
//   - error: a validation, params, or send error.
func (s *Service) Send(message string, params *types.Params) error {
	return s.SendContext(context.Background(), message, params)
}

// SendContext delivers a notification message to Lark.
//
// Parameters:
//   - ctx: cancellation and deadline for the request.
//   - message: the message to send.
//   - params: optional overrides for configuration fields.
//
// Returns:
//   - error: a validation, params, or send error. A request error matches ctx's
//     error when ctx ends the request.
func (s *Service) SendContext(ctx context.Context, message string, params *types.Params) error {
	if len(message) > maxLength {
		return ErrLargeMessage
	}

	config := *s.Config
	if err := s.pkr.UpdateConfigFromParams(&config, params); err != nil {
		return fmt.Errorf("updating params: %w", err)
	}

	if config.Host != larkHost && config.Host != feishuHost {
		return ErrInvalidHost
	}

	if config.Path == "" {
		return ErrNoPath
	}

	return s.doSend(ctx, &config, message, params)
}

// ServiceTimeout returns the HTTP timeout used for a Lark send.
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

// doSend sends the notification to Lark using the configured API URL.
//
// Parameters:
//   - ctx: cancellation and deadline for the request.
//   - config: the configuration for this send.
//   - message: the message to send.
//   - params: the send params, which may set the title and link.
//
// Returns:
//   - error: [ErrMissingHost], a payload error, or the send error.
func (s *Service) doSend(ctx context.Context, config *Config, message string, params *types.Params) error {
	if config.Host == "" {
		return ErrMissingHost
	}

	postURL := fmt.Sprintf(apiFormat, config.Host, config.Path)

	payload, err := s.preparePayload(message, config, params)
	if err != nil {
		return err
	}

	return s.sendRequest(ctx, postURL, payload)
}

// genSign generates a signature for the request using the secret and timestamp.
//
// The signature is the HMAC-SHA256 of an empty message, keyed with the
// timestamp and the secret joined by a newline, in base64.
//
// Parameters:
//   - secret: the custom bot's signing secret.
//   - timestamp: the request time, in Unix seconds.
//
// Returns:
//   - string: the signature.
//   - error: an error wrapping [ErrInvalidSignature].
func (s *Service) genSign(secret string, timestamp int64) (string, error) {
	stringToSign := fmt.Sprintf("%v\n%s", timestamp, secret)

	h := hmac.New(sha256.New, []byte(stringToSign))
	if _, err := h.Write([]byte{}); err != nil {
		return "", fmt.Errorf("%w: computing HMAC: %w", ErrInvalidSignature, err)
	}

	return base64.StdEncoding.EncodeToString(h.Sum(nil)), nil
}

// getRequestBody constructs the request body for the Lark API, supporting rich content via params.
//
// Without a title the body is a text message. With one it is a post, with a
// "More Info" link when the params set link. A secret adds the timestamp and
// the signature.
//
// Parameters:
//   - message: the message to send.
//   - title: the post title, or empty for a text message.
//   - secret: the signing secret, or empty for an unsigned request.
//   - params: the send params, which may set link.
//
// Returns:
//   - *RequestBody: the request body.
func (s *Service) getRequestBody(
	message, title, secret string,
	params *types.Params,
) *RequestBody {
	body := &RequestBody{}

	if secret != "" {
		ts := time.Now().Unix()
		body.Timestamp = strconv.FormatInt(ts, 10)

		sign, err := s.genSign(secret, ts)
		if err != nil {
			sign = "" // Fallback to empty string on error
		}

		body.Sign = sign
	}

	if title == "" {
		body.MsgType = MsgTypeText
		body.Content.Text = message
	} else {
		body.MsgType = MsgTypePost
		//nolint:exhaustruct_v5 // Item fields are populated inline; Link is optional
		content := [][]Item{{{Tag: TagValueTextContent, Text: message}}}

		if params != nil {
			if link, ok := (*params)["link"]; ok && link != "" {
				content = append(
					content,

					[]Item{{Tag: TagValueLink, Text: "More Info", Link: link}},
				)
			}
		}

		//nolint:exhaustruct_v5 // Post fields are populated inline; Zh is optional
		body.Content.Post = &Post{
			En: &Message{
				Title:   title,
				Content: content,
			},
		}
	}

	return body
}

// handleResponse processes the API response and checks for errors.
//
// Parameters:
//   - resp: the API response. At most [httpbody.MaxSize] bytes of its body are read.
//
// Returns:
//   - error: [ErrSendFailed] for an error status or a nonzero code, or the
//     error from reading or decoding the body, which wraps [httpbody.ErrTooLarge]
//     for an oversized body.
func (s *Service) handleResponse(resp *http.Response) error {
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

	if response.Code != 0 {
		return fmt.Errorf(
			"%w: server returned code %d: %s",
			ErrSendFailed,
			response.Code,
			response.Msg,
		)
	}

	s.Logf("Notification sent successfully to %s", s.Config.Host)

	return nil
}

// preparePayload constructs and marshals the request payload for the Lark API.
//
// Parameters:
//   - message: the message to send.
//   - config: the configuration for this send, with the title and the secret.
//   - params: the send params.
//
// Returns:
//   - []byte: the JSON payload.
//   - error: the encoding error.
func (s *Service) preparePayload(
	message string,
	config *Config,
	params *types.Params,
) ([]byte, error) {
	body := s.getRequestBody(message, config.Title, config.Secret, params)

	data, err := json.Marshal(body, jsonopts.Default)
	if err != nil {
		return nil, fmt.Errorf("marshaling payload to JSON: %w", err)
	}

	return data, nil
}

// sendRequest performs the HTTP POST request to the Lark API and handles the response.
//
// Parameters:
//   - ctx: cancellation and deadline for the request.
//   - postURL: the webhook URL.
//   - payload: the JSON payload.
//
// Returns:
//   - error: the request error, or the error the Lark API reported.
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

	return s.handleResponse(resp)
}
