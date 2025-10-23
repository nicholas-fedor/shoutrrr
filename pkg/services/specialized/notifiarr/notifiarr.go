package notifiarr

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"regexp"
	"strconv"
	"strings"

	"github.com/nicholas-fedor/shoutrrr/pkg/format"
	"github.com/nicholas-fedor/shoutrrr/pkg/services/standard"
	"github.com/nicholas-fedor/shoutrrr/pkg/types"
)

const (
	// APIBaseURL is the base URL for Notifiarr API.
	APIBaseURL = "https://notifiarr.com/api/v1/notification/passthrough"
)

// Error definitions for the Notifiarr service.
var (
	ErrSendFailed       = errors.New("failed to send notification to Notifiarr")
	ErrUnexpectedStatus = errors.New("server returned unexpected response status code")
	ErrInvalidAPIKey    = errors.New("invalid API key")
	ErrEmptyMessage     = errors.New("message is empty")
	ErrInvalidURL       = errors.New("invalid URL format")
	ErrInvalidChannelID = errors.New("invalid channel ID")
)

// mentionRegex is a compiled regular expression for parsing Discord user/role mentions.
var mentionRegex = regexp.MustCompile(`<@!?(\d+)>|<@&(\d+)>`)

// Service implements a Notifiarr notification service.
type Service struct {
	standard.Standard
	Config *Config
	pkr    format.PropKeyResolver
}

// Send delivers a notification message to Notifiarr.
func (service *Service) Send(message string, paramsPtr *types.Params) error {
	if message == "" {
		return ErrEmptyMessage
	}

	// Create a copy of the config to avoid modifying the original
	config := *service.Config

	var params types.Params
	if paramsPtr == nil {
		params = types.Params{}
	} else {
		params = *paramsPtr
	}

	// Filter params to only include valid config keys for config updates
	validConfigKeys := map[string]bool{
		"name":      true,
		"channel":   true,
		"color":     true,
		"thumbnail": true,
		"image":     true,
	}
	filteredParams := types.Params{}

	for k, v := range params {
		if validConfigKeys[k] {
			filteredParams[k] = v
		}
	}

	if err := service.pkr.UpdateConfigFromParams(&config, &filteredParams); err != nil {
		service.Logf("Failed to update params: %v", err)
	}

	// Create the payload
	payload, err := service.createPayload(message, params, &config)
	if err != nil {
		return fmt.Errorf("creating payload: %w", err)
	}

	// Send the notification
	if err := service.doSend(payload); err != nil {
		return fmt.Errorf("%w: %s", ErrSendFailed, err.Error())
	}

	return nil
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

// GetID returns the identifier for this service.
func (service *Service) GetID() string {
	return Scheme
}

// GetConfigURLFromCustom converts a custom webhook URL into a standard service URL.
func (*Service) GetConfigURLFromCustom(customURL *url.URL) (*url.URL, error) {
	// Copy the URL to modify
	webhookURL := *customURL
	if strings.HasPrefix(webhookURL.Scheme, Scheme) && len(webhookURL.Scheme) > len(Scheme) &&
		webhookURL.Scheme[len(Scheme)] == '+' {
		// Remove the scheme prefix if present
		webhookURL.Scheme = webhookURL.Scheme[len(Scheme)+1:]
	}

	// Parse config from webhook URL
	config, pkr, err := ConfigFromWebhookURL(webhookURL)
	if err != nil {
		return nil, err
	}

	// Generate and return the service URL
	return config.getURL(&pkr), nil
}

// parseChannelID parses the channel string to an integer.
func (service *Service) parseChannelID(channelStr string) (int, error) {
	var channelID int

	_, err := fmt.Sscanf(channelStr, "%d", &channelID)
	if err != nil {
		return 0, fmt.Errorf("invalid channel ID format '%s': %w", channelStr, ErrInvalidChannelID)
	}

	return channelID, nil
}

// parseMentions extracts Discord user and role mentions from the message content.
func (service *Service) parseMentions(message string) []string {
	var mentions []string

	matches := mentionRegex.FindAllStringSubmatch(message, -1)
	for _, match := range matches {
		if match[1] != "" {
			mentions = append(mentions, fmt.Sprintf("<@%s>", match[1]))
		} else if match[2] != "" {
			mentions = append(mentions, fmt.Sprintf("<@&%s>", match[2]))
		}
	}

	return mentions
}

// parseFields parses a JSON string into a slice of Field structs.
func (service *Service) parseFields(fieldsStr string) ([]Field, error) {
	var fields []Field
	if err := json.Unmarshal([]byte(fieldsStr), &fields); err != nil {
		return nil, fmt.Errorf("unmarshaling fields JSON: %w", err)
	}

	return fields, nil
}

// extractPingIDs extracts user and role IDs from mention strings.
func (service *Service) extractPingIDs(mentions []string) (int, int) {
	var pingUser, pingRole int

	for _, mention := range mentions {
		if strings.HasPrefix(mention, "<@") && strings.HasSuffix(mention, ">") {
			// Remove <@ and >
			idStr := mention[2 : len(mention)-1]
			if strings.HasPrefix(idStr, "&") {
				// Role mention: <@&123>
				roleIDStr := idStr[1:]
				if roleID, err := strconv.Atoi(roleIDStr); err == nil && pingRole == 0 {
					pingRole = roleID
				}
			} else {
				// User mention: <@123>
				if userID, err := strconv.Atoi(idStr); err == nil && pingUser == 0 {
					pingUser = userID
				}
			}
		}
	}

	return pingUser, pingRole
}

// createPayload creates the JSON payload for Notifiarr API.
func (service *Service) createPayload(
	message string,
	params types.Params,
	config *Config,
) ([]byte, error) {
	// Determine if this is an update based on the update parameter
	var updatePtr *bool

	if updateStr, exists := params["update"]; exists && updateStr != "" {
		switch updateStr {
		case "true":
			updatePtr = &[]bool{true}[0]
		case "false":
			updatePtr = &[]bool{false}[0]
		}
		// If updateStr is neither "true" nor "false", leave as nil
	}

	// Create the notification payload
	notification := NotificationPayload{
		Notification: NotificationData{
			Update: updatePtr,    // Optional boolean for updating existing messages
			Name:   config.Name,  // Required name of the custom app/script
			Event:  params["id"], // Optional unique ID for this notification
		},
	}

	// Check if there are any Discord fields to include
	hasChannel := config.Channel != ""
	hasColor := config.Color != ""
	hasThumbnail := config.Thumbnail != ""
	hasImage := config.Image != ""
	hasTitle := params[types.TitleKey] != ""
	hasIcon := params["icon"] != ""
	hasContent := params["content"] != ""
	hasDescription := message != ""
	hasFooter := params["footer"] != ""
	hasFields := params["fields"] != ""
	mentions := service.parseMentions(message)
	hasMentions := len(mentions) > 0

	if hasChannel || hasColor || hasThumbnail || hasImage || hasTitle || hasIcon || hasContent ||
		hasDescription ||
		hasFooter ||
		hasFields ||
		hasMentions {
		notification.Discord = &DiscordPayload{}

		// Add channel ID if configured
		if hasChannel {
			channelID, err := service.parseChannelID(config.Channel)
			if err != nil {
				return nil, fmt.Errorf("parsing channel ID: %w", err)
			}

			notification.Discord.IDs = &IDPayload{
				Channel: channelID,
			}
		}

		// Add color if configured
		if hasColor {
			notification.Discord.Color = config.Color
		}

		// Add images if configured
		if hasThumbnail || hasImage {
			notification.Discord.Images = &ImagePayload{
				Thumbnail: config.Thumbnail,
				Image:     config.Image,
			}
		}

		// Add text content
		textPayload := &TextPayload{
			Title:       params[types.TitleKey],
			Icon:        params["icon"],
			Content:     params["content"],
			Description: message,
			Footer:      params["footer"],
		}

		// Parse fields if provided
		if hasFields {
			fields, err := service.parseFields(params["fields"])
			if err != nil {
				return nil, fmt.Errorf("parsing fields: %w", err)
			}

			textPayload.Fields = fields
		}

		notification.Discord.Text = textPayload

		// Parse mentions from message content and add to ping
		if hasMentions {
			pingUser, pingRole := service.extractPingIDs(mentions)
			if pingUser > 0 || pingRole > 0 {
				notification.Discord.Ping = &PingPayload{
					PingUser: pingUser,
					PingRole: pingRole,
				}
			}
		}
	}

	// Marshal to JSON
	payloadBytes, err := json.Marshal(notification)
	if err != nil {
		return nil, fmt.Errorf("marshaling payload to JSON: %w", err)
	}

	return payloadBytes, nil
}

// doSend executes the HTTP request to send a notification to Notifiarr.
func (service *Service) doSend(payload []byte) error {
	// Build the API URL with API key
	apiURL := fmt.Sprintf("%s/%s", APIBaseURL, service.Config.APIKey)

	// Create background context for the request
	ctx := context.Background()

	// Create HTTP request with context
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, apiURL, bytes.NewBuffer(payload))
	if err != nil {
		return fmt.Errorf("creating HTTP request: %w", err)
	}

	// Set headers
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")

	// Send the HTTP request
	res, err := http.DefaultClient.Do(req)
	if err != nil {
		return fmt.Errorf("sending HTTP request: %w", err)
	}

	if res != nil && res.Body != nil {
		defer func() {
			_ = res.Body.Close()
		}()

		if body, err := io.ReadAll(res.Body); err == nil {
			service.Log("Server response: ", string(body))
		}
	}

	if res.StatusCode < 200 || res.StatusCode >= 300 {
		return fmt.Errorf("%w: %s", ErrUnexpectedStatus, res.Status)
	}

	return nil
}
