package discord

import (
	"context"
	"encoding/json/v2"
	"fmt"
	"net/url"
	"time"
	"unicode/utf8"

	"github.com/nicholas-fedor/shoutrrr/internal/httpclient"
	"github.com/nicholas-fedor/shoutrrr/internal/jsonopts"
	"github.com/nicholas-fedor/shoutrrr/internal/sequential"
	"github.com/nicholas-fedor/shoutrrr/pkg/format"
	"github.com/nicholas-fedor/shoutrrr/pkg/services/standard"
	"github.com/nicholas-fedor/shoutrrr/pkg/types"
	"github.com/nicholas-fedor/shoutrrr/pkg/util"
)

// Service implements a Discord notification service.
type Service struct {
	// Standard provides logging and the other shared service functionality.
	standard.Standard

	// HTTPClient sends the webhook requests when set. [Service.SetHTTPClient]
	// clears it, so whichever of the two the caller set last carries the
	// sends.
	//
	// Deprecated: Use [Service.SetHTTPClient].
	HTTPClient types.HTTPClient
	// Sleeper waits between retries. Initialize sets it to [RealSleeper] when
	// it is nil.
	Sleeper Sleeper

	// client holds the client from [Service.SetHTTPClient] and the default
	// client.
	client httpclient.Holder

	// Config holds the webhook and the message options.
	Config *Config
	// pkr reads and writes the config props by their query keys.
	pkr format.PropKeyResolver
}

// Message limits and the webhook endpoint.
const (
	// ChunkSize is the maximum size of a single message chunk.
	ChunkSize = 2000
	// TotalChunkSize is the maximum total size of all chunks.
	TotalChunkSize = 6000
	// ChunkCount is the maximum number of chunks allowed.
	ChunkCount = 10
	// MaxSearchRunes is the maximum number of runes to search for a split
	// position.
	MaxSearchRunes = 100
	// HooksBaseURL is the base URL of the webhook endpoints, on Discord API
	// v10. A URL without a version reaches Discord's default version. That
	// default is the deprecated v6, which reports rate-limit waits in
	// milliseconds.
	HooksBaseURL = "https://discord.com/api/v10/webhooks"
)

// limits are the chunk limits for splitting a plain message into items.
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
//
// Returns:
//   - string: the service scheme, [Scheme].
func (s *Service) GetID() string {
	return Scheme
}

// Initialize configures the service with a URL and logger.
// It keeps an HTTP client and a sleeper that are already set.
//
// Parameters:
//   - serviceURL: the service URL.
//   - logger: the logger for the service.
//
// Returns:
//   - error: the failure setting the default props or reading the URL.
func (s *Service) Initialize(serviceURL *url.URL, logger types.StdLogger) error {
	s.SetLogger(logger)

	s.Config = &Config{}

	s.pkr = format.NewPropKeyResolver(s.Config)

	if s.Sleeper == nil {
		s.Sleeper = RealSleeper{}
	}

	if err := s.pkr.SetDefaultProps(s.Config); err != nil {
		return fmt.Errorf("setting default properties: %w", err)
	}

	// The service's resolver applies the URL, so it records the props the URL
	// sets explicitly.
	if err := s.Config.setURL(&s.pkr, serviceURL); err != nil {
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
//   - error: [types.ErrServiceNotInitialized] before Initialize, the failures
//     of the message's batches joined, or nil on success.
func (s *Service) Send(message string, params *types.Params) error {
	return s.SendContext(context.Background(), message, params)
}

// SendContext delivers a notification message to Discord. The router calls it
// with its send deadline. A context without a deadline is bounded by the
// budget that [Service.ServiceTimeout] reports.
//
// Parameters:
//   - ctx: bounds each request and its retries.
//   - message: the message text, or a raw JSON payload when the json option is set.
//   - params: per-send parameters, applied to this send only.
//
// Returns:
//   - error: [types.ErrServiceNotInitialized] before Initialize, the failures
//     of the message's batches joined, or nil on success. Once ctx ends, the
//     remaining batches are not sent, and ctx's error is reported once.
func (s *Service) SendContext(ctx context.Context, message string, params *types.Params) error {
	if err := s.ready(); err != nil {
		return err
	}

	if message == "" {
		return ErrEmptyMessage
	}

	ctx, cancel := httpclient.BoundContext(ctx, s.ServiceTimeout(params))
	defer cancel()

	if s.Config.JSON {
		preparer := &JSONRequestPreparer{payload: []byte(message)}

		err := sendWithRetry(ctx, preparer, CreatePostURLFromConfig(s.Config), s.client.Prefer(s.HTTPClient), s.Sleeper)
		if err != nil {
			return fmt.Errorf("sending JSON message: %w", err)
		}

		return nil
	}

	config := *s.Config
	if err := s.pkr.UpdateConfigFromParams(&config, params); err != nil {
		return fmt.Errorf("updating config from params: %w", err)
	}

	batches := createItemsFromPlain(message, config.SplitLines, config.Title)
	level := s.plainLevel(&config, params)

	err := sequential.Send(
		ctx,
		batches,
		sequential.SharedDeadline,
		func(ctx context.Context, batch []types.MessageItem) error {
			// A batch sent as plain content has no embed to color, so it keeps its form.
			if level != types.Unknown && !sendsAsContent(batch, config.Title) {
				for i := range batch {
					batch[i].Level = level
				}
			}

			return s.sendItems(ctx, batch, params)
		},
	)
	if err != nil {
		return fmt.Errorf("failed to send discord notification: %w", err)
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
//   - error: [types.ErrServiceNotInitialized] before Initialize, the send
//     failure, or nil on success.
//
// Deprecated: Use [Service.SendItemsContext], which the router calls for rich
// messages. This method's *types.Params parameter does not match
// [types.RichSender], so the router never calls it.
func (s *Service) SendItems(items []types.MessageItem, params *types.Params) error {
	return s.sendItemBatches(context.Background(), items, params)
}

// SendItemsContext delivers message items with enhanced metadata and formatting to
// Discord, including embeds, fields, timestamps and file attachments. The router
// calls it for rich messages, with its send deadline. A context without a
// deadline is bounded by the budget that [Service.ServiceTimeout] reports.
//
// Parameters:
//   - ctx: bounds the request and its retries.
//   - items: the message items, sent as embeds with their fields and files.
//   - params: per-send parameters, applied to this send only.
//
// Returns:
//   - error: [types.ErrServiceNotInitialized] before Initialize, the send
//     failure, or nil on success.
func (s *Service) SendItemsContext(ctx context.Context, items []types.MessageItem, params types.Params) error {
	return s.sendItemBatches(ctx, items, &params)
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
// the default client. It clears the deprecated HTTPClient field, so the sends
// use this client until that field is set again.
//
// Parameters:
//   - client: the HTTP client, or nil for the default client. A nil
//     *http.Client also counts as nil.
func (s *Service) SetHTTPClient(client types.HTTPClient) {
	s.client.Set(client)
	s.HTTPClient = nil
}

// plainLevel returns the level that colors the embeds of a plain message. A
// send with a level param uses the color configured for that level, such as
// colorError, unless the service URL or the send params set the plain color.
// A message short enough to go out as plain content has no embed, so the
// level leaves it as it is.
//
// Parameters:
//   - config: the config for this send, with params already applied.
//   - params: the send params, or nil.
//
// Returns:
//   - types.MessageLevel: the level of the send, or [types.Unknown] to use the
//     plain color.
func (s *Service) plainLevel(config *Config, params *types.Params) types.MessageLevel {
	if params == nil {
		return types.Unknown
	}

	level, ok := params.Level()
	if !ok || !s.pkr.KeepsDefault(config, params, "color") {
		return types.Unknown
	}

	return level
}

// ready checks that the service has a config with a valid webhook ID and
// token. Send params cannot change either, so one check covers the send.
//
// Returns:
//   - error: [types.ErrServiceNotInitialized] without a config. An invalid
//     webhook ID or token gives an error such as [ErrInvalidWebhookID].
//     Otherwise nil.
func (s *Service) ready() error {
	if s.Config == nil {
		return types.ErrServiceNotInitialized
	}

	return s.Config.validate()
}

// sendConfig returns a copy of the service config with params applied.
//
// Parameters:
//   - params: per-send parameters.
//
// Returns:
//   - Config: the config for this send.
//   - error: the failure applying params, or nil.
func (s *Service) sendConfig(params *types.Params) (Config, error) {
	config := *s.Config
	if err := s.pkr.UpdateConfigFromParams(&config, params); err != nil {
		return Config{}, fmt.Errorf("updating config from params: %w", err)
	}

	return config, nil
}

// sendEmbedItems sends items with an embed for each of them, as multipart
// when any item carries a file.
//
// Parameters:
//   - ctx: bounds the request and its retries.
//   - items: the message items to send, at least one and at most [MaxEmbeds].
//   - params: per-send parameters, applied to a copy of the config.
//
// Returns:
//   - error: the params or send failure, or nil on success.
func (s *Service) sendEmbedItems(ctx context.Context, items []types.MessageItem, params *types.Params) error {
	config, err := s.sendConfig(params)
	if err != nil {
		return err
	}

	payload := createEmbedPayload(items, config.Title, config.LevelColors())

	return s.sendPayload(ctx, &config, &payload, items)
}

// sendItemBatches sends items in batches that each fit one Discord message:
// at most [MaxEmbeds] embeds whose text, with the title on the first, stays
// within [TotalChunkSize] characters. Items that fit in one batch are sent as
// [Service.sendItems] sends them. Otherwise every batch is sent as embeds, so a
// batch of one plain item matches the others and gets the embed length limit.
// A failed batch does not stop the batches after it, and failures other than
// the first are logged. Once ctx ends, the remaining batches are not sent. A
// context without a deadline is bounded by the budget that
// [Service.ServiceTimeout] reports.
//
// Parameters:
//   - ctx: bounds the requests and their retries.
//   - items: the message items to send.
//   - params: per-send parameters, applied to a copy of the config.
//
// Returns:
//   - error: [types.ErrServiceNotInitialized] before Initialize, the params
//     failure, the failures of the batches joined, or nil when every batch is
//     sent. Once ctx ends, the remaining batches are not sent, and ctx's error
//     is reported once.
func (s *Service) sendItemBatches(ctx context.Context, items []types.MessageItem, params *types.Params) error {
	if err := s.ready(); err != nil {
		return err
	}

	ctx, cancel := httpclient.BoundContext(ctx, s.ServiceTimeout(params))
	defer cancel()

	config, err := s.sendConfig(params)
	if err != nil {
		return err
	}

	batches := batchItems(items, config.Title, config.LevelColors())
	if len(batches) <= 1 {
		return s.sendItems(ctx, items, params)
	}

	return sequential.Send(ctx, batches, sequential.SharedDeadline,
		func(ctx context.Context, batch []types.MessageItem) error {
			return s.sendEmbedItems(ctx, batch, params)
		})
}

// sendItems builds the webhook payload for items and sends it, as multipart when
// any item carries a file. A single plain item is sent as the message content.
//
// Parameters:
//   - ctx: bounds the request and its retries.
//   - items: the message items to send.
//   - params: per-send parameters, applied to a copy of the config.
//
// Returns:
//   - error: the payload or send failure, or nil on success.
func (s *Service) sendItems(ctx context.Context, items []types.MessageItem, params *types.Params) error {
	config, err := s.sendConfig(params)
	if err != nil {
		return err
	}

	payload, err := CreatePayloadFromItems(items, config.Title, config.LevelColors())
	if err != nil {
		return fmt.Errorf("creating payload: %w", err)
	}

	return s.sendPayload(ctx, &config, &payload, items)
}

// sendPayload sends payload with the configured username and avatar, as
// multipart with the items' files when any item carries one.
//
// Parameters:
//   - ctx: bounds the request and its retries.
//   - config: the config for this send.
//   - payload: the webhook payload built from items.
//   - items: the message items, whose files are attached.
//
// Returns:
//   - error: the marshaling or send failure, or nil on success.
func (s *Service) sendPayload(
	ctx context.Context,
	config *Config,
	payload *WebhookPayload,
	items []types.MessageItem,
) error {
	payload.Username = config.Username
	payload.AvatarURL = config.Avatar

	files := make([]types.File, 0, len(items))

	for _, item := range items {
		if item.File != nil {
			files = append(files, *item.File)
		}
	}

	var preparer RequestPreparer

	if len(files) > 0 {
		preparer = &MultipartRequestPreparer{payload: payload, files: files}
	} else {
		payloadBytes, err := json.Marshal(payload, jsonopts.Default)
		if err != nil {
			return fmt.Errorf("marshaling payload to JSON: %w", err)
		}

		preparer = &JSONRequestPreparer{payload: payloadBytes}
	}

	return sendWithRetry(ctx, preparer, CreatePostURLFromConfig(config), s.client.Prefer(s.HTTPClient), s.Sleeper)
}

// batchItems groups items into batches that each fit one Discord message: at
// most [MaxEmbeds] items whose embeds, with the title on the first, stay within
// [TotalChunkSize] characters. An item over that total alone gets a batch of
// its own, which [createEmbedPayload] shortens to fit.
//
// Parameters:
//   - items: the message items.
//   - title: the title of each batch's first embed.
//   - colors: the embed color for each message level.
//
// Returns:
//   - [][]types.MessageItem: the batches, in order, or none for no items.
func batchItems(items []types.MessageItem, title string, colors [types.MessageLevelCount]uint) [][]types.MessageItem {
	budget := TotalChunkSize - utf8.RuneCountInString(util.Ellipsis(title, maxEmbedTitle))

	var (
		batches [][]types.MessageItem
		total   int
	)

	batch := make([]types.MessageItem, 0, MaxEmbeds)

	for i := range items {
		embed := newEmbed(&items[i], colors)
		length := embedLength(&embed)

		if len(batch) == MaxEmbeds || (len(batch) > 0 && total+length > budget) {
			batches = append(batches, batch)
			batch, total = make([]types.MessageItem, 0, MaxEmbeds), 0
		}

		batch = append(batch, items[i])
		total += length
	}

	if len(batch) > 0 {
		batches = append(batches, batch)
	}

	return batches
}

// CreateItemsFromPlain converts plain text into MessageItems suitable for Discord's webhook payload.
//
// Parameters:
//   - plain: the message text.
//   - splitLines: whether to send each line as its own embed.
//
// Returns:
//   - [][]types.MessageItem: the message items, grouped by request.
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
