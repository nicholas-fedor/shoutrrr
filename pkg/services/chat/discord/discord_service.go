package discord

import (
	"context"
	"encoding/json/v2"
	"fmt"
	"net/url"
	"slices"
	"strings"
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

// sendState holds what one send resolves once and every request of the send
// uses.
type sendState struct {
	// postURL is the webhook URL built from config.
	postURL string
	// mentions are the mentions the send's next request carries, read from
	// config. They are empty once a request has gone out.
	mentions mentionSet
	// config is the service config with the send's params applied, checked
	// as described on [Config]. Its trimmed ThreadName names the thread the
	// send's next request creates, and is cleared once a request may have
	// created it. Its ThreadID becomes the created thread once the reply
	// names it.
	config Config
	// multipleRequests is true for a send that takes more than one request,
	// whose first reply then names the thread the rest go into.
	multipleRequests bool
	// threadUnknown is true once the request that was to create the send's
	// thread failed without a definite rejection, so the thread may exist
	// but its ID is unknown. Later requests then fail without being sent.
	threadUnknown bool
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
//   - error: [types.ErrServiceNotInitialized] before Initialize, or the send's
//     failures joined.
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
//   - error: [types.ErrServiceNotInitialized] before Initialize, or the send's
//     failures joined. A send that ctx ends reports ctx's error once.
func (s *Service) SendContext(ctx context.Context, message string, params *types.Params) error {
	send, err := s.prepareSend(params)
	if err != nil {
		return err
	}

	if message == "" {
		return ErrEmptyMessage
	}

	ctx, cancel := httpclient.BoundContext(ctx, s.ServiceTimeout(params))
	defer cancel()

	if send.config.JSON {
		preparer := &JSONRequestPreparer{payload: []byte(message)}

		if _, err = sendWithRetry(ctx, preparer, send.postURL, s.client.Prefer(s.HTTPClient), s.Sleeper, nil); err != nil {
			return fmt.Errorf("sending JSON message: %w", err)
		}

		return nil
	}

	items := plainItems(message, send.config.SplitLines)

	// Splitting leaves out blank lines and chunks, so a message of only
	// whitespace leaves nothing to post.
	if len(items) == 0 {
		return ErrEmptyMessage
	}

	// A message sent as plain content has no embed to color, so it keeps its
	// form. The level is set before batching, since its footer counts toward
	// each message's total.
	if level := s.plainLevel(&send.config, params); level != types.Unknown && !send.sendsAsContent(items) {
		for i := range items {
			items[i].Level = level
		}
	}

	if err := s.sendBatches(ctx, send, items); err != nil {
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
//   - error: [types.ErrServiceNotInitialized] before Initialize, or the send
//     failure.
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
//   - error: [types.ErrServiceNotInitialized] before Initialize, or the send
//     failure.
func (s *Service) SendItemsContext(ctx context.Context, items []types.MessageItem, params types.Params) error {
	return s.sendItemBatches(ctx, items, &params)
}

// ServiceTimeout reports the send budget the router gives Discord, which is
// the sender's retry limit of 5 minutes.
//
// Discord's rate limits can ask a client to wait minutes for the Retry-After
// duration. A long message is sent as several batches, and each batch has its
// own retries. Healthy sends finish well before the limit.
//
// Parameters:
//   - params: unused. The retry limit does not depend on send parameters.
//
// Returns:
//   - time.Duration: 5 minutes.
func (*Service) ServiceTimeout(*types.Params) time.Duration {
	return maxRetryTimeout
}

// SetHTTPClient sets a custom HTTP client for the service.
//
// A nil client restores the default client. It clears the deprecated HTTPClient
// field, so the sends use this client until that field is set again.
//
// Parameters:
//   - client: the HTTP client, or nil for the default client. A nil
//     *http.Client also counts as nil.
func (s *Service) SetHTTPClient(client types.HTTPClient) {
	s.client.Set(client)
	s.HTTPClient = nil
}

// plainLevel returns the level that colors the embeds of a plain message.
//
// A send with a level param uses the color configured for that level, such as
// colorError. A plain color set by the service URL or the send params wins
// over the level. A message short enough to go out as plain content has no
// embed, so the level leaves it as it is.
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

// prepareSend resolves the config and the webhook URL for one send.
//
// The config is a copy of the service config with params applied, checked as
// reading a service URL checks it.
//
// Parameters:
//   - params: per-send parameters.
//
// Returns:
//   - *sendState: the config and webhook URL for this send.
//   - error: [types.ErrServiceNotInitialized] without a config, or the
//     failure applying params or checking the config.
func (s *Service) prepareSend(params *types.Params) (*sendState, error) {
	if s.Config == nil {
		return nil, types.ErrServiceNotInitialized
	}

	config := *s.Config
	if err := s.pkr.UpdateConfigFromParams(&config, params); err != nil {
		return nil, fmt.Errorf("updating config from params: %w", err)
	}

	if err := config.validate(); err != nil {
		return nil, err
	}

	mentions, err := config.mentions()
	if err != nil {
		return nil, err
	}

	config.ThreadName = strings.TrimSpace(config.ThreadName)

	return &sendState{
		postURL:          config.postURL(),
		mentions:         mentions,
		config:           config,
		multipleRequests: false,
		threadUnknown:    false,
	}, nil
}

// sendBatches sends items in batches that each fit one Discord message.
//
// A batch holds at most [MaxEmbeds] embeds, and their text stays within
// [TotalChunkSize] characters, counting the title on the first and each
// embed's footer. Items that fit in one batch go out as one message, where a
// single plain item is sent as the message content. Otherwise every batch is
// sent as embeds, so a batch of one plain item matches the others and gets
// the embed length limit. A failed batch does not stop the batches after it.
//
// Parameters:
//   - ctx: bounds the requests and their retries.
//   - send: the config and webhook URL for this send.
//   - items: the message items to send.
//
// Returns:
//   - error: the send's failures joined. A send that ctx ends reports ctx's
//     error once.
func (s *Service) sendBatches(ctx context.Context, send *sendState, items []types.MessageItem) error {
	batches := batchItems(items, send.config.Title, send.config.LevelColors())
	if len(batches) <= 1 {
		return s.sendItems(ctx, send, items)
	}

	send.multipleRequests = true

	return sequential.Send(ctx, batches, sequential.SharedDeadline,
		func(ctx context.Context, batch []types.MessageItem) error {
			payload := createEmbedPayload(batch, send.config.Title, send.config.LevelColors())

			return s.sendPayload(ctx, send, &payload, batch)
		})
}

// sendItemBatches sends rich message items with params applied, in batches
// that each fit one Discord message. A context without a deadline is bounded
// by the budget that [Service.ServiceTimeout] reports.
//
// Parameters:
//   - ctx: bounds the requests and their retries.
//   - items: the message items to send.
//   - params: per-send parameters, applied to a copy of the config.
//
// Returns:
//   - error: [types.ErrServiceNotInitialized] before Initialize, or the send's
//     failures joined. A send that ctx ends reports ctx's error once.
func (s *Service) sendItemBatches(ctx context.Context, items []types.MessageItem, params *types.Params) error {
	send, err := s.prepareSend(params)
	if err != nil {
		return err
	}

	ctx, cancel := httpclient.BoundContext(ctx, s.ServiceTimeout(params))
	defer cancel()

	return s.sendBatches(ctx, send, items)
}

// sendItems builds the webhook payload for items and sends it as the only
// request of its send, as multipart when any item carries a file. A single
// plain item is sent as the message content when it fits there with the
// send's mentions in front, and as an embed otherwise.
//
// Parameters:
//   - ctx: bounds the request and its retries.
//   - send: the config and webhook URL for this send.
//   - items: the message items to send.
//
// Returns:
//   - error: the payload or send failure, or nil on success.
func (s *Service) sendItems(ctx context.Context, send *sendState, items []types.MessageItem) error {
	payload, err := CreatePayloadFromItems(items, send.config.Title, send.config.LevelColors())
	if err != nil {
		return fmt.Errorf("creating payload: %w", err)
	}

	if payload.Content != "" && !send.sendsAsContent(items) {
		payload = createEmbedPayload(items, send.config.Title, send.config.LevelColors())
	}

	return s.sendPayload(ctx, send, &payload, items)
}

// sendPayload sends payload with the configured username and avatar, as
// multipart with the items' files when any item carries one. The send's
// mentions and thread name go on the request when the send still holds them,
// which is only for its first request, so that only it creates a thread.
//
// Parameters:
//   - ctx: bounds the request and its retries.
//   - send: the config and webhook URL for this send.
//   - payload: the webhook payload built from items.
//   - items: the message items, whose files are attached.
//
// Returns:
//   - error: [ErrThreadUnknown] without a request when the thread for the
//     request is unknown, the marshaling or send failure, or nil on success.
func (s *Service) sendPayload(
	ctx context.Context,
	send *sendState,
	payload *WebhookPayload,
	items []types.MessageItem,
) error {
	if send.threadUnknown {
		return ErrThreadUnknown
	}

	payload.Username = send.config.Username
	payload.AvatarURL = send.config.Avatar
	payload.ThreadName = send.config.ThreadName
	send.mentions.apply(payload)

	// Most sends carry no files, so the slice is only made for one that does.
	var files []types.File

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

	// Only a send whose later requests go into the new thread needs the reply
	// that names it.
	var reply *createdMessage
	if send.config.ThreadName != "" && send.multipleRequests {
		reply = &createdMessage{ChannelID: ""}
	}

	mayHaveCreated, err := sendWithRetry(ctx, preparer, send.postURL, s.client.Prefer(s.HTTPClient), s.Sleeper, reply)
	send.finishRequest(reply, mayHaveCreated)

	return err
}

// finishRequest moves the send past one request.
//
// A request that created no message, as the sender reports, leaves the
// mentions and the thread name to the next request. After a request that may
// have created one, later requests carry neither, so the send pings and
// creates a thread once.
//
// A send that takes several requests and names a thread goes into the thread
// that reply names, by a snowflake. When the request may have created the
// thread but reply names none, the thread's ID is unknown. Later requests
// then fail with [ErrThreadUnknown] without being sent, rather than going
// without a thread.
//
// Parameters:
//   - reply: the message the request created, or nil when the send did not
//     read it.
//   - mayHaveCreated: whether the request may have created a message.
func (s *sendState) finishRequest(reply *createdMessage, mayHaveCreated bool) {
	if !mayHaveCreated {
		return
	}

	s.mentions = mentionSet{}
	s.config.ThreadName = ""

	if reply == nil {
		return
	}

	// A failed request leaves reply empty, so it names no thread either.
	if !isSnowflake(reply.ChannelID) {
		s.threadUnknown = true

		return
	}

	s.config.ThreadID = reply.ChannelID
	s.postURL = s.config.postURL()
}

// sendsAsContent reports whether items go out as the message content, which
// holds the send's mentions in front of the text. A message that fits as
// content alone but not with the mentions goes as an embed instead.
//
// Parameters:
//   - items: the message items of one request.
//
// Returns:
//   - bool: whether the items are sent as the message content.
func (s *sendState) sendsAsContent(items []types.MessageItem) bool {
	return sendsAsContent(items, s.config.Title) && s.mentions.fits(items[0].Text)
}

// batchItems groups items into batches that each fit one Discord message.
//
// A batch holds at most [MaxEmbeds] items, and their embeds stay within
// [TotalChunkSize] characters with the title counted on the first. An item
// over that total gets a batch of its own, and its embed is shortened to fit
// when the payload is built.
//
// Parameters:
//   - items: the message items.
//   - title: the title of each batch's first embed.
//   - colors: the embed color for each message level.
//
// Returns:
//   - [][]types.MessageItem: the batches in order, or none for no items.
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
//   - [][]types.MessageItem: the message items, grouped by request, or none
//     for a message of only whitespace.
func CreateItemsFromPlain(plain string, splitLines bool) [][]types.MessageItem {
	return batchItems(plainItems(plain, splitLines), "", [types.MessageLevelCount]uint{})
}

// plainItems splits plain text into message items, in order.
//
// With split lines, each line becomes an item for an embed whose description
// holds up to 4096 characters, so a longer line is shortened to that length.
// Without them, the text is split into chunks of at most [ChunkSize]
// characters, at a space or line break near the limit when there is one, and
// that space or line break is left out. Discord treats text of only
// whitespace as empty, so blank lines and chunks are left out, and a message
// of only whitespace gives no items.
//
// Parameters:
//   - plain: the message text.
//   - splitLines: whether to send each line as its own embed.
//
// Returns:
//   - []types.MessageItem: the message items, or none for a message of only
//     whitespace.
func plainItems(plain string, splitLines bool) []types.MessageItem {
	if splitLines {
		lineLimits := limits
		lineLimits.ChunkSize = maxEmbedDescription

		return slices.Concat(util.MessageItemsFromLines(plain, lineLimits)...)
	}

	var items []types.MessageItem

	for {
		chunks, omitted := util.PartitionMessage(plain, limits, MaxSearchRunes)

		for _, chunk := range chunks {
			if strings.TrimSpace(chunk.Text) != "" {
				items = append(items, chunk)
			}
		}

		if omitted == 0 {
			return items
		}

		// PartitionMessage reports the omitted text in runes, so slice by runes.
		runes := []rune(plain)
		plain = string(runes[len(runes)-omitted:])
	}
}
