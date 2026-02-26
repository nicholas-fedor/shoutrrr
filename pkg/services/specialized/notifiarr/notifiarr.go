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
	"time"

	"github.com/nicholas-fedor/shoutrrr/pkg/format"
	"github.com/nicholas-fedor/shoutrrr/pkg/services/standard"
	"github.com/nicholas-fedor/shoutrrr/pkg/types"
)

const (
	// APIBaseURL is the base URL for Notifiarr API.
	APIBaseURL = "https://notifiarr.com/api/v1/notification/passthrough"
	// MentionTypeNone represents no mention type.
	MentionTypeNone = 0
	// MentionTypeUser represents a user mention type.
	MentionTypeUser = 1
	// MentionTypeRole represents a role mention type.
	MentionTypeRole = 2
	// requestTimeout is the timeout duration for HTTP requests to prevent hangs.
	requestTimeout = 30 // seconds
)

// ErrSendFailed indicates a failure to send a notification to Notifiarr.
var (
	ErrSendFailed = errors.New("failed to send notification to Notifiarr")
	// ErrUnexpectedStatus indicates the server returned an unexpected response status code.
	ErrUnexpectedStatus = errors.New("server returned unexpected response status code")
	// ErrInvalidAPIKey indicates an invalid API key was provided.
	ErrInvalidAPIKey = errors.New("invalid API key")
	// ErrAuthenticationFailed indicates authentication failure (e.g., 401 response).
	ErrAuthenticationFailed = errors.New("authentication failed")
	// ErrEmptyMessage indicates the message is empty.
	ErrEmptyMessage = errors.New("message is empty")
	// ErrInvalidURL indicates an invalid URL format.
	ErrInvalidURL = errors.New("invalid URL format")
	// ErrInvalidChannelID indicates an invalid channel ID.
	ErrInvalidChannelID = errors.New("invalid channel ID")
	// ErrNoDiscordFields indicates no Discord fields are present.
	ErrNoDiscordFields = errors.New("no Discord fields present")
)

// mentionRegex is a compiled regular expression for parsing Discord user/role mentions.
var mentionRegex = regexp.MustCompile(`<@!?(\d+)>|<@&(\d+)>`)

// Service implements a Notifiarr notification service.
type Service struct {
	standard.Standard

	Config *Config
	pkr    format.PropKeyResolver
}

// presenceFlags holds boolean flags indicating presence of Discord fields.
type presenceFlags struct {
	channel, color, thumbnail, image, title, icon, content, description, footer, fields, mentions bool
}

// HasAny returns true if any of the boolean fields are true, false otherwise.
func (pf presenceFlags) HasAny() bool {
	return pf.channel || pf.color || pf.thumbnail || pf.image || pf.title || pf.icon ||
		pf.content ||
		pf.description ||
		pf.footer ||
		pf.fields ||
		pf.mentions
}

// Send delivers a notification message to Notifiarr.
func (service *Service) Send(message string, paramsPtr *types.Params) error {
	// Check for empty message
	if message == "" {
		return ErrEmptyMessage
	}

	// Create a copy of the config to avoid modifying the original
	config := *service.Config

	var params types.Params
	// Handle nil params by creating empty map
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

	// Update config with filtered parameters
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
	// Set the logger for the service
	service.SetLogger(logger)
	// Initialize service config
	service.Config = &Config{}
	// Initialize property key resolver
	service.pkr = format.NewPropKeyResolver(service.Config)

	// Set default properties
	if err := service.pkr.SetDefaultProps(service.Config); err != nil {
		return fmt.Errorf("setting default properties: %w", err)
	}

	// Set URL and return any error
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

// ParseChannelID parses the channel string to an integer.
func (service *Service) ParseChannelID(channelStr string) (int, error) {
	var channelID int

	_, err := fmt.Sscanf(channelStr, "%d", &channelID)
	if err != nil {
		return 0, fmt.Errorf("invalid channel ID format '%s': %w", channelStr, ErrInvalidChannelID)
	}

	return channelID, nil
}

// ParseMentions extracts Discord user and role mentions from the message content.
func (service *Service) ParseMentions(message string) []string {
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

// ParseFields parses a JSON string into a slice of Field structs.
func (service *Service) ParseFields(fieldsStr string) ([]Field, error) {
	var fields []Field
	if err := json.Unmarshal([]byte(fieldsStr), &fields); err != nil {
		return nil, fmt.Errorf("unmarshaling fields JSON: %w", err)
	}

	return fields, nil
}

// ParseMention parses a single mention string and returns the type (0=none, 1=user, 2=role) and ID if valid.
func ParseMention(mention string) (int, int) {
	if !strings.HasPrefix(mention, "<@") || !strings.HasSuffix(mention, ">") {
		return MentionTypeNone, 0
	}

	idStr := mention[2 : len(mention)-1]
	if strings.HasPrefix(idStr, "&") {
		roleIDStr := idStr[1:]
		if roleID, err := strconv.Atoi(roleIDStr); err == nil {
			return MentionTypeRole, roleID
		}
	} else {
		// Handle user mentions, which may have a "!" prefix for nicknames
		userIDStr := strings.TrimPrefix(idStr, "!")

		if userID, err := strconv.Atoi(userIDStr); err == nil {
			return MentionTypeUser, userID
		}
	}

	return MentionTypeNone, 0
}

// ExtractPingIDs extracts user and role IDs from mention strings.
func (service *Service) ExtractPingIDs(mentions []string) ([]int, []int) {
	var pingUsers, pingRoles []int

	for _, mention := range mentions {
		mentionType, id := ParseMention(mention)
		switch mentionType {
		case MentionTypeUser:
			pingUsers = append(pingUsers, id)
		case MentionTypeRole:
			pingRoles = append(pingRoles, id)
		}
	}

	return pingUsers, pingRoles
}

// ParseUpdateFlag parses the update parameter from params.
func ParseUpdateFlag(params types.Params) *bool {
	if updateStr, exists := params["update"]; exists && updateStr != "" {
		switch updateStr {
		case "true":
			return &[]bool{true}[0]
		case "false":
			return &[]bool{false}[0]
		}
	}

	return nil
}

// buildNotificationData creates the notification data structure.
func buildNotificationData(updatePtr *bool, config *Config, params types.Params) NotificationData {
	return NotificationData{
		Update: updatePtr,
		Name:   config.Name,
		Event:  params["id"],
	}
}

// checkPresenceFlags determines which Discord fields are present.
func checkPresenceFlags(
	message string,
	params types.Params,
	config *Config,
	service *Service,
) presenceFlags {
	return presenceFlags{
		channel:     config.Channel != "",
		color:       config.Color != "",
		thumbnail:   config.Thumbnail != "",
		image:       config.Image != "",
		title:       params[types.TitleKey] != "",
		icon:        params["icon"] != "",
		content:     params["content"] != "",
		description: message != "",
		footer:      params["footer"] != "",
		fields:      params["fields"] != "",
		mentions:    len(service.ParseMentions(message)) > 0,
	}
}

// buildDiscordPayload constructs the Discord payload if any fields are present.
func (service *Service) buildDiscordPayload(
	flags presenceFlags,
	message string,
	params types.Params,
	config *Config,
) (*DiscordPayload, error) {
	if !flags.HasAny() {
		return nil, ErrNoDiscordFields
	}

	discord := &DiscordPayload{}

	if flags.channel {
		// Parse channel ID from config string to integer
		channelID, err := service.ParseChannelID(config.Channel)
		if err != nil {
			return nil, fmt.Errorf("parsing channel ID: %w", err)
		}

		discord.IDs = &IDPayload{Channel: channelID}
	}

	if flags.color {
		// Assign color from config
		discord.Color = config.Color
	}

	if flags.thumbnail || flags.image {
		// Set thumbnail and image URLs from config
		discord.Images = &ImagePayload{
			Thumbnail: config.Thumbnail,
			Image:     config.Image,
		}
	}

	// Construct text payload with title, icon, content, description, and footer from params
	textPayload := &TextPayload{
		Title:       params[types.TitleKey],
		Icon:        params["icon"],
		Content:     params["content"],
		Description: message,
		Footer:      params["footer"],
	}

	if flags.fields {
		// Parse JSON fields string into Field structs
		fields, err := service.ParseFields(params["fields"])
		if err != nil {
			return nil, fmt.Errorf("parsing fields: %w", err)
		}

		textPayload.Fields = fields
	}

	discord.Text = textPayload

	if flags.mentions {
		// Extract Discord mentions from message content
		mentions := service.ParseMentions(message)

		// Extract user and role IDs for ping setup (collects all mentions)
		pingUsers, pingRoles := service.ExtractPingIDs(mentions)
		if len(pingUsers) > 0 || len(pingRoles) > 0 {
			pingPayload := &PingPayload{}
			if len(pingUsers) > 0 {
				pingPayload.PingUser = &pingUsers[0] // Use first user mention
			}

			if len(pingRoles) > 0 {
				pingPayload.PingRole = &pingRoles[0] // Use first role mention
			}

			discord.Ping = pingPayload
		}
	}

	return discord, nil
}

// createPayload creates the JSON payload for Notifiarr API.
func (service *Service) createPayload(
	message string,
	params types.Params,
	config *Config,
) ([]byte, error) {
	// Parse the update parameter from params
	updatePtr := ParseUpdateFlag(params)
	// Build the notification data structure
	notificationData := buildNotificationData(updatePtr, config, params)
	// Check presence flags for Discord fields
	flags := checkPresenceFlags(message, params, config, service)

	// Build the Discord payload if fields are present
	discord, err := service.buildDiscordPayload(flags, message, params, config)
	if err != nil {
		return nil, err
	}

	notification := NotificationPayload{
		Notification: notificationData,
		Discord:      discord,
	}

	// Marshal the notification to JSON
	payloadBytes, err := json.Marshal(notification)
	if err != nil {
		return nil, fmt.Errorf("marshaling payload to JSON: %w", err)
	}

	return payloadBytes, nil
}

// doSend executes the HTTP request to send a notification to Notifiarr.
// It includes a timeout to prevent hangs and differentiates between authentication failures and other errors.
func (service *Service) doSend(payload []byte) error {
	// Build the API URL with API key
	apiURL := fmt.Sprintf("%s/%s", APIBaseURL, service.Config.APIKey)

	// Create context with timeout to prevent request hangs
	ctx, cancel := context.WithTimeout(
		context.Background(),
		time.Duration(requestTimeout)*time.Second,
	)
	defer cancel()

	// Create HTTP request with context and timeout
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

	// Check for authentication failure (401 Unauthorized)
	if res.StatusCode == http.StatusUnauthorized {
		return ErrAuthenticationFailed
	}

	// Check for other unexpected status codes
	if res.StatusCode < 200 || res.StatusCode >= 300 {
		return fmt.Errorf("%w: %s", ErrUnexpectedStatus, res.Status)
	}

	return nil
}
