package discord

import (
	"context"
	"encoding/json"
	"fmt"
	"net/url"
	"strings"

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

// GetID provides the identifier for this service.
func (s *Service) GetID() string {
	return Scheme
}

// Initialize configures the service with a URL and logger.
func (s *Service) Initialize(configURL *url.URL, logger types.StdLogger) error {
	s.SetLogger(logger)

	s.Config = &Config{}
	s.pkr = format.NewPropKeyResolver(s.Config)
	s.HTTPClient = NewDefaultHTTPClient() // Default client for backward compatibility
	s.Sleeper = RealSleeper{}             // Default sleeper

	if err := s.pkr.SetDefaultProps(s.Config); err != nil {
		return fmt.Errorf("setting default properties: %w", err)
	}

	if err := s.Config.SetURL(configURL); err != nil {
		return fmt.Errorf("setting config URL: %w", err)
	}

	return nil
}

// Send delivers a notification message to Discord.
func (s *Service) Send(message string, params *types.Params) error {
	if message == "" {
		return ErrEmptyMessage
	}

	var firstErr error

	if s.Config.JSON {
		postURL := CreateAPIURLFromConfig(s.Config)
		if err := s.doSend([]byte(message), postURL); err != nil {
			return fmt.Errorf("sending JSON message: %w", err)
		}
	} else {
		config := *s.Config
		if err := s.pkr.UpdateConfigFromParams(&config, params); err != nil {
			return fmt.Errorf("updating config from params: %w", err)
		}

		batches := CreateItemsFromPlain(message, config.SplitLines)
		for _, batch := range batches {
			if err := s.sendItems(batch, params); err != nil {
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
func (s *Service) SendItems(items []types.MessageItem, params *types.Params) error {
	return s.sendItems(items, params)
}

// CreateItemsFromPlain converts plain text into MessageItems suitable for Discord's webhook payload.
func CreateItemsFromPlain(plain string, splitLines bool) [][]types.MessageItem {
	var batches [][]types.MessageItem

	if splitLines {
		return util.MessageItemsFromLines(plain, limits)
	}

	for {
		items, omitted := util.PartitionMessage(plain, limits, MaxSearchRunes)
		batches = append(batches, items)

		if omitted == 0 {
			break
		}

		plain = plain[len(plain)-omitted:]
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

// doSend executes an HTTP POST request to deliver the payload to Discord.
func (s *Service) doSend(payload []byte, postURL string) error {
	if err := validateDiscordWebhookURL(postURL); err != nil {
		return err
	}

	ctx := context.Background()

	preparer := &JSONRequestPreparer{payload: payload}

	return sendWithRetry(ctx, preparer, postURL, s.HTTPClient, s.Sleeper)
}

// doSendMultipart executes an HTTP POST request with multipart/form-data to deliver payload and files to Discord.
func (s *Service) doSendMultipart(
	payload *WebhookPayload,
	files []types.File,
	postURL string,
) error {
	if err := validateDiscordWebhookURL(postURL); err != nil {
		return err
	}

	ctx := context.Background()

	preparer := &MultipartRequestPreparer{
		payload: payload,
		files:   files,
	}

	return sendWithRetry(ctx, preparer, postURL, s.HTTPClient, s.Sleeper)
}

func (s *Service) sendItems(items []types.MessageItem, params *types.Params) error {
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

	postURL := CreateAPIURLFromConfig(&config)

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
		return s.doSendMultipart(&payload, files, postURL)
	}

	payloadBytes, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("marshaling payload to JSON: %w", err)
	}

	return s.doSend(payloadBytes, postURL)
}
