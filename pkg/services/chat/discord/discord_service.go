package discord

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/nicholas-fedor/shoutrrr/pkg/format"
	"github.com/nicholas-fedor/shoutrrr/pkg/services/standard"
	"github.com/nicholas-fedor/shoutrrr/pkg/types"
	"github.com/nicholas-fedor/shoutrrr/pkg/util"
)

// Service implements a Discord notification service.
type Service struct {
	standard.Standard

	Config     *Config
	pkr        format.PropKeyResolver
	HTTPClient HTTPClient
	Sleeper    Sleeper
}

const (
	ChunkSize      = 2000 // Maximum size of a single message chunk
	TotalChunkSize = 6000 // Maximum total size of all chunks
	ChunkCount     = 10   // Maximum number of chunks allowed
	MaxSearchRunes = 100  // Maximum number of runes to search for split position
	HooksBaseURL   = "https://discord.com/api/webhooks"
)

var limits = types.MessageLimit{
	ChunkSize:      ChunkSize,
	TotalChunkSize: TotalChunkSize,
	ChunkCount:     ChunkCount,
}

// Compile-time checks that Service implements the interfaces the router relies on.
var (
	_ types.Service                 = (*Service)(nil)
	_ types.HTTPClientSetter        = (*Service)(nil)
	_ types.ContextSender           = (*Service)(nil)
	_ types.ContextAttachmentSender = (*Service)(nil)
	_ types.ServiceTimeout          = (*Service)(nil)
)

// GetID provides the identifier for this service.
func (s *Service) GetID() string {
	return Scheme
}

// Initialize configures the service with a URL and logger.
func (s *Service) Initialize(serviceURL *url.URL, logger types.StdLogger) error {
	s.SetLogger(logger)

	s.Config = &Config{}

	s.pkr = format.NewPropKeyResolver(s.Config)
	if s.HTTPClient == nil {
		s.HTTPClient = NewDefaultHTTPClient()
	}

	s.Sleeper = RealSleeper{} // Default sleeper

	if err := s.pkr.SetDefaultProps(s.Config); err != nil {
		return fmt.Errorf("setting default properties: %w", err)
	}

	if err := s.Config.SetURL(serviceURL); err != nil {
		return fmt.Errorf("setting config URL: %w", err)
	}

	return nil
}

// Send delivers a notification message to Discord without a deadline of its own.
//
// Parameters:
//   - message: the message text, or a raw JSON payload when the json option is set.
//   - params: per-send parameters, applied to this send only.
//
// Returns:
//   - error: the first failure among the message's batches, or nil on success.
func (s *Service) Send(message string, params *types.Params) error {
	return s.SendContext(context.Background(), message, params)
}

// SendContext delivers a notification message to Discord. The router calls it
// with its send deadline.
//
// Parameters:
//   - ctx: bounds each request and its retries.
//   - message: the message text, or a raw JSON payload when the json option is set.
//   - params: per-send parameters, applied to this send only.
//
// Returns:
//   - error: the first failure among the message's batches, or nil on success.
func (s *Service) SendContext(ctx context.Context, message string, params *types.Params) error {
	if message == "" {
		return ErrEmptyMessage
	}

	var firstErr error

	if s.Config.JSON {
		postURL := CreatePostURLFromConfig(s.Config)
		if err := s.doSend(ctx, []byte(message), postURL); err != nil {
			return fmt.Errorf("sending JSON message: %w", err)
		}
	} else {
		config := *s.Config
		if err := s.pkr.UpdateConfigFromParams(&config, params); err != nil {
			return fmt.Errorf("updating config from params: %w", err)
		}

		batches := createItemsFromPlain(message, config.SplitLines, config.Title)
		for _, batch := range batches {
			if err := s.sendItems(ctx, batch, params); err != nil {
				s.Log(err)

				if firstErr == nil {
					firstErr = err
				}
			}
		}
	}

	if firstErr != nil {
		return fmt.Errorf("failed to send discord notification: %w", firstErr)
	}

	return nil
}

// SendItems delivers message items with enhanced metadata and formatting to Discord.
//
// Parameters:
//   - items: the message items, sent as embeds with their fields and files.
//   - params: per-send parameters, applied to this send only.
//
// Returns:
//   - error: the send failure, or nil on success.
//
// Deprecated: Use [Service.SendItemsContext], which the router calls for rich
// messages. This method's *types.Params parameter does not match
// [types.RichSender], so the router never calls it.
func (s *Service) SendItems(items []types.MessageItem, params *types.Params) error {
	return s.sendItems(context.Background(), items, params)
}

// SendItemsContext delivers message items with enhanced metadata and formatting to
// Discord, including embeds, fields, timestamps and file attachments. The router
// calls it for rich messages, with its send deadline.
//
// Parameters:
//   - ctx: bounds the request and its retries.
//   - items: the message items, sent as embeds with their fields and files.
//   - params: per-send parameters, applied to this send only.
//
// Returns:
//   - error: the send failure, or nil on success.
func (s *Service) SendItemsContext(ctx context.Context, items []types.MessageItem, params types.Params) error {
	return s.sendItems(ctx, items, &params)
}

// ServiceTimeout reports the send budget the router gives Discord: the sender's
// retry limit. Discord's rate limits tell clients to wait for the Retry-After
// duration, which can last minutes, and a long message is sent as several
// batches, each with its own retries. Healthy sends finish well before it.
//
// Parameters:
//   - params: unused. The retry limit does not depend on send parameters.
//
// Returns:
//   - time.Duration: [maxRetryTimeout].
func (*Service) ServiceTimeout(*types.Params) time.Duration {
	return maxRetryTimeout
}

// SetHTTPClient sets a custom HTTP client for the service. A nil client restores
// the default client.
func (s *Service) SetHTTPClient(client types.HTTPClient) {
	if c, ok := client.(*http.Client); ok && c == nil {
		client = nil
	}

	if client == nil {
		client = NewDefaultHTTPClient()
	}

	s.HTTPClient = client
}

// doSend executes an HTTP POST request to deliver the payload to Discord.
//
// Parameters:
//   - ctx: bounds the request and its retries.
//   - payload: the JSON request body.
//   - postURL: the webhook URL.
//
// Returns:
//   - error: the validation or request failure, or nil on success.
func (s *Service) doSend(ctx context.Context, payload []byte, postURL string) error {
	if err := validateDiscordWebhookURL(postURL); err != nil {
		return err
	}

	preparer := &JSONRequestPreparer{payload: payload}

	return sendWithRetry(ctx, preparer, postURL, s.HTTPClient, s.Sleeper)
}

// doSendMultipart executes an HTTP POST request with multipart/form-data to deliver
// payload and files to Discord.
//
// Parameters:
//   - ctx: bounds the request and its retries.
//   - payload: the webhook payload sent as the JSON form part.
//   - files: the files attached to the request.
//   - postURL: the webhook URL.
//
// Returns:
//   - error: the validation or request failure, or nil on success.
func (s *Service) doSendMultipart(
	ctx context.Context,
	payload *WebhookPayload,
	files []types.File,
	postURL string,
) error {
	if err := validateDiscordWebhookURL(postURL); err != nil {
		return err
	}

	preparer := &MultipartRequestPreparer{
		payload: payload,
		files:   files,
	}

	return sendWithRetry(ctx, preparer, postURL, s.HTTPClient, s.Sleeper)
}

// sendItems builds the webhook payload for items and sends it, as multipart when
// any item carries a file.
//
// Parameters:
//   - ctx: bounds the request and its retries.
//   - items: the message items to send.
//   - params: per-send parameters, applied to a copy of the config.
//
// Returns:
//   - error: the payload or send failure, or nil on success.
func (s *Service) sendItems(ctx context.Context, items []types.MessageItem, params *types.Params) error {
	config := *s.Config
	if err := s.pkr.UpdateConfigFromParams(&config, params); err != nil {
		return fmt.Errorf("updating config from params: %w", err)
	}

	payload, err := CreatePayloadFromItems(items, config.Title, config.LevelColors())
	if err != nil {
		return fmt.Errorf("creating payload: %w", err)
	}

	payload.Username = config.Username
	payload.AvatarURL = config.Avatar

	postURL := CreatePostURLFromConfig(&config)

	// Check if any items have files
	fileCount := 0

	for _, item := range items {
		if item.File != nil {
			fileCount++
		}
	}

	files := make([]types.File, 0, fileCount)

	for _, item := range items {
		if item.File != nil {
			files = append(files, *item.File)
		}
	}

	hasFiles := len(files) > 0

	if hasFiles {
		return s.doSendMultipart(ctx, &payload, files, postURL)
	}

	payloadBytes, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("marshaling payload to JSON: %w", err)
	}

	return s.doSend(ctx, payloadBytes, postURL)
}

// CreateItemsFromPlain converts plain text into MessageItems suitable for Discord's webhook payload.
func CreateItemsFromPlain(plain string, splitLines bool) [][]types.MessageItem {
	return createItemsFromPlain(plain, splitLines, "")
}

// createItemsFromPlain converts plain text into batches of MessageItems, one batch
// per webhook request. Discord counts the title on a batch's first embed toward the
// same total text limit as the descriptions, so each batch's budget is reduced by
// the title's length.
//
// Parameters:
//   - plain: the message text.
//   - splitLines: whether to send each line as its own embed.
//   - title: the title added to the first embed of every batch.
//
// Returns:
//   - [][]types.MessageItem: the message items, grouped by request.
func createItemsFromPlain(plain string, splitLines bool, title string) [][]types.MessageItem {
	var batches [][]types.MessageItem

	batchLimits := limits
	batchLimits.TotalChunkSize = max(TotalChunkSize-utf8.RuneCountInString(title), 1)

	if splitLines {
		return util.MessageItemsFromLines(plain, batchLimits)
	}

	for {
		items, omitted := util.PartitionMessage(plain, batchLimits, MaxSearchRunes)
		batches = append(batches, items)

		if omitted == 0 {
			break
		}

		// PartitionMessage reports the omitted text in runes, so slice by runes.
		runes := []rune(plain)
		plain = string(runes[len(runes)-omitted:])
	}

	return batches
}

// validateDiscordWebhookURL validates the Discord webhook URL for security and correctness.
func validateDiscordWebhookURL(postURL string) error {
	if postURL == "" {
		return ErrEmptyURL
	}

	parsedURL, err := url.ParseRequestURI(postURL)
	if err != nil {
		return fmt.Errorf("invalid URL: %w", err)
	}

	if parsedURL.Scheme != "https" {
		return ErrInvalidScheme
	}

	if parsedURL.Host != "discord.com" {
		return ErrInvalidHost
	}

	if !strings.HasPrefix(parsedURL.Path, "/api/webhooks/") {
		return ErrInvalidURLPrefix
	}

	parts := strings.Split(strings.TrimPrefix(postURL, HooksBaseURL+"/"), "/")
	if len(parts) != 2 || parts[0] == "" || parts[1] == "" {
		return ErrMalformedURL
	}

	return nil
}
