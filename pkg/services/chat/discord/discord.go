package discord

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"strings"

	"github.com/nicholas-fedor/shoutrrr/pkg/format"
	"github.com/nicholas-fedor/shoutrrr/pkg/services/standard"
	"github.com/nicholas-fedor/shoutrrr/pkg/types"
	"github.com/nicholas-fedor/shoutrrr/pkg/util"
)

const (
	ChunkSize      = 2000 // Maximum size of a single message chunk
	TotalChunkSize = 6000 // Maximum total size of all chunks
	ChunkCount     = 10   // Maximum number of chunks allowed
	MaxSearchRunes = 100  // Maximum number of runes to search for split position
	HooksBaseURL   = "https://discord.com/api/webhooks"
)

var (
	ErrUnknownAPIError  = errors.New("unknown error from Discord API")
	ErrUnexpectedStatus = errors.New("unexpected response status code")
	ErrInvalidURLPrefix = errors.New("URL must start with Discord webhook base URL")
	ErrInvalidWebhookID = errors.New("invalid webhook ID")
	ErrInvalidToken     = errors.New("invalid token")
	ErrEmptyURL         = errors.New("empty URL provided")
	ErrMalformedURL     = errors.New("malformed URL: missing webhook ID or token")
)

var limits = types.MessageLimit{
	ChunkSize:      ChunkSize,
	TotalChunkSize: TotalChunkSize,
	ChunkCount:     ChunkCount,
}

// Service implements a Discord notification service.
type Service struct {
	standard.Standard
	Config *Config
	pkr    format.PropKeyResolver
}

// Send delivers a notification message to Discord.
func (service *Service) Send(message string, params *types.Params) error {
	var firstErr error

	if service.Config.JSON {
		postURL := CreateAPIURLFromConfig(service.Config)
		if err := doSend([]byte(message), postURL); err != nil {
			return fmt.Errorf("sending JSON message: %w", err)
		}
	} else {
		batches := CreateItemsFromPlain(message, service.Config.SplitLines)
		for _, items := range batches {
			if err := service.sendItems(items, params); err != nil {
				service.Log(err)

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
func (service *Service) SendItems(items []types.MessageItem, params *types.Params) error {
	return service.sendItems(items, params)
}

func (service *Service) sendItems(items []types.MessageItem, params *types.Params) error {
	config := *service.Config
	if err := service.pkr.UpdateConfigFromParams(&config, params); err != nil {
		return fmt.Errorf("updating config from params: %w", err)
	}

	payload, err := CreatePayloadFromItems(items, config.Title, config.LevelColors())
	if err != nil {
		return fmt.Errorf("creating payload: %w", err)
	}

	payload.Username = config.Username
	payload.AvatarURL = config.Avatar

	payloadBytes, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("marshaling payload to JSON: %w", err)
	}

	postURL := CreateAPIURLFromConfig(&config)

	return doSend(payloadBytes, postURL)
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

// Initialize configures the service with a URL and logger.
func (service *Service) Initialize(configURL *url.URL, logger types.StdLogger) error {
	service.SetLogger(logger)
	service.Config = &Config{}
	service.pkr = format.NewPropKeyResolver(service.Config)

	if err := service.pkr.SetDefaultProps(service.Config); err != nil {
		return fmt.Errorf("setting default properties: %w", err)
	}

	if err := service.Config.SetURL(configURL); err != nil {
		return fmt.Errorf("setting config URL: %w", err)
	}

	return nil
}

// GetID provides the identifier for this service.
func (service *Service) GetID() string {
	return Scheme
}

// CreateAPIURLFromConfig builds a POST URL from the Discord configuration.
func CreateAPIURLFromConfig(config *Config) string {
	if config.WebhookID == "" || config.Token == "" {
		return "" // Invalid cases are caught in doSend
	}
	// Trim whitespace to prevent malformed URLs
	webhookID := strings.TrimSpace(config.WebhookID)
	token := strings.TrimSpace(config.Token)

	baseURL := fmt.Sprintf("%s/%s/%s", HooksBaseURL, webhookID, token)

	if config.ThreadID != "" {
		// Append thread_id as a query parameter
		query := url.Values{}
		query.Set("thread_id", strings.TrimSpace(config.ThreadID))

		return baseURL + "?" + query.Encode()
	}

	return baseURL
}

// doSend executes an HTTP POST request to deliver the payload to Discord.
//
//nolint:gosec,noctx
func doSend(payload []byte, postURL string) error {
	if postURL == "" {
		return ErrEmptyURL
	}

	parsedURL, err := url.ParseRequestURI(postURL)
	if err != nil {
		return fmt.Errorf("invalid URL: %w", err)
	}

	if !strings.HasPrefix(parsedURL.String(), HooksBaseURL) {
		return ErrInvalidURLPrefix
	}

	parts := strings.Split(strings.TrimPrefix(postURL, HooksBaseURL+"/"), "/")
	if len(parts) != 2 || parts[0] == "" || parts[1] == "" {
		return ErrMalformedURL
	}

	webhookID := strings.TrimSpace(parts[0])
	token := strings.TrimSpace(parts[1])
	safeURL := fmt.Sprintf("%s/%s/%s", HooksBaseURL, webhookID, token)

	res, err := http.Post(safeURL, "application/json", bytes.NewBuffer(payload))
	if err != nil {
		return fmt.Errorf("making HTTP POST request: %w", err)
	}

	if res == nil {
		return ErrUnknownAPIError
	}
	defer func() { _ = res.Body.Close() }()

	if res.StatusCode != http.StatusNoContent {
		return fmt.Errorf("%w: %s", ErrUnexpectedStatus, res.Status)
	}

	return nil
}
