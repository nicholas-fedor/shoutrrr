package notifiarr

import (
	"bytes"
	"context"
	"encoding/json/v2"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"net/url"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/nicholas-fedor/shoutrrr/internal/httpclient"
	"github.com/nicholas-fedor/shoutrrr/internal/jsonopts"
	"github.com/nicholas-fedor/shoutrrr/internal/logging"
	"github.com/nicholas-fedor/shoutrrr/internal/redact"
	"github.com/nicholas-fedor/shoutrrr/pkg/format"
	"github.com/nicholas-fedor/shoutrrr/pkg/services/standard"
	"github.com/nicholas-fedor/shoutrrr/pkg/types"
)

// Service implements a Notifiarr notification service.
type Service struct {
	// Standard provides logging and the other shared service functionality.
	standard.Standard

	// client holds the injected HTTP client and the default client.
	client httpclient.Holder

	// Config holds the API key and the Discord options.
	Config *Config
	// pkr reads and writes the config props by their query keys.
	pkr format.PropKeyResolver
}

// presenceFlags holds boolean flags indicating presence of Discord fields.
type presenceFlags struct {
	// channel through mentions each report whether a send sets that Discord
	// field.
	channel, color, thumbnail, image, title, icon, content, description, footer, fields, mentions bool
}

// API endpoint and mention types.
const (
	// APIBaseURL is the base URL for Notifiarr API.
	APIBaseURL = "https://notifiarr.com/api/v1/notification/passthrough"
	// MentionTypeNone represents no mention type.
	MentionTypeNone = 0
	// MentionTypeUser represents a user mention type.
	MentionTypeUser = 1
	// MentionTypeRole represents a role mention type.
	MentionTypeRole = 2
)

// requestTimeout is the send budget of Notifiarr.
const requestTimeout = 30 * time.Second

// Send params that fill the payload. The config resolver skips them, and the
// payload code reads them from the params.
const (
	// iconParam sets the icon URL of the Discord embed.
	iconParam = "icon"
	// contentParam sets the text shown above the message.
	contentParam = "content"
	// footerParam sets the footer of the Discord embed.
	footerParam = "footer"
	// fieldsParam holds the fields of the Discord embed as a JSON array.
	fieldsParam = "fields"
	// updateParam asks Notifiarr to update the message with the same event ID.
	updateParam = "update"
	// idParam sets the event ID.
	idParam = "id"
)

// Errors returned by the Notifiarr service.
var (
	// ErrSendFailed indicates a failure to send a notification to Notifiarr.
	ErrSendFailed = errors.New("failed to send notification to Notifiarr")
	// ErrUnexpectedStatus indicates the server returned an unexpected response status code.
	ErrUnexpectedStatus = errors.New("server returned unexpected response status code")
	// ErrInvalidAPIKey indicates an invalid API key was provided.
	ErrInvalidAPIKey = errors.New("invalid API key")
	// ErrAPIKeyMissing indicates that the service URL has no API key.
	ErrAPIKeyMissing = errors.New("API key missing from service URL")
	// ErrAuthenticationFailed indicates authentication failure (e.g., 401 response).
	ErrAuthenticationFailed = errors.New("authentication failed")
	// ErrEmptyMessage indicates the message is empty.
	ErrEmptyMessage = errors.New("message is empty")
	// ErrInvalidURL indicates an invalid URL format.
	ErrInvalidURL = errors.New("invalid URL format")
	// ErrInvalidChannelID indicates an invalid channel ID.
	ErrInvalidChannelID = errors.New("invalid channel ID")
	// ErrNoDiscordFields indicates no Discord fields present.
	ErrNoDiscordFields = errors.New("no Discord fields present")
)

// mentionRegex is a compiled regular expression for parsing Discord user/role mentions.
var mentionRegex = regexp.MustCompile(`<@!?(\d+)>|<@&(\d+)>`)

// Compile-time checks that Service implements the interfaces the router relies on.
var (
	_ types.Service          = (*Service)(nil)
	_ types.ContextSender    = (*Service)(nil)
	_ types.HTTPClientSetter = (*Service)(nil)
	_ types.ServiceTimeout   = (*Service)(nil)
	_ types.CustomURLService = (*Service)(nil)
)

// HasAny returns true if any of the boolean fields are true, false otherwise.
//
// Returns:
//   - bool: true when a send sets any Discord field.
func (pf presenceFlags) HasAny() bool {
	return pf.channel || pf.color || pf.thumbnail || pf.image || pf.title || pf.icon ||
		pf.content ||
		pf.description ||
		pf.footer ||
		pf.fields ||
		pf.mentions
}

// ExtractPingIDs extracts user and role IDs from mention strings.
//
// Parameters:
//   - mentions: the mentions, such as <@123> for a user and <@&456> for a role.
//
// Returns:
//   - []int: the user IDs.
//   - []int: the role IDs.
func (s *Service) ExtractPingIDs(mentions []string) ([]int, []int) {
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

// GetID returns the identifier for this service.
//
// Returns:
//   - string: the service scheme, [Scheme].
func (s *Service) GetID() string {
	return Scheme
}

// GetServiceURLFromCustom converts a custom webhook URL into a standard service URL.
//
// A webhook URL without an API key converts without an error, to a service URL
// with an empty host.
//
// Parameters:
//   - serviceURL: the webhook URL, with or without the notifiarr+ scheme
//     prefix.
//
// Returns:
//   - *url.URL: the service URL.
//   - error: an error wrapping
//     [github.com/nicholas-fedor/shoutrrr/pkg/util.ErrInvalidQuery] for a
//     malformed query, or a query prop error.
func (*Service) GetServiceURLFromCustom(serviceURL *url.URL) (*url.URL, error) {
	// Copy the URL to modify
	webhookURL := *serviceURL
	if strings.HasPrefix(webhookURL.Scheme, Scheme) && len(webhookURL.Scheme) > len(Scheme) &&
		webhookURL.Scheme[len(Scheme)] == '+' {
		// Remove the scheme prefix if present
		webhookURL.Scheme = webhookURL.Scheme[len(Scheme)+1:]
	}

	// Parse config from webhook URL
	config, pkr, err := ConfigFromWebhookURL(&webhookURL)
	if err != nil {
		return nil, err
	}

	// Generate and return the service URL
	return config.getURL(&pkr), nil
}

// Initialize configures the service with a URL and logger.
//
// Parameters:
//   - serviceURL: the service URL.
//   - logger: the logger for service output.
//
// Returns:
//   - error: the error from applying the default props or reading the URL,
//     such as [ErrAPIKeyMissing].
func (s *Service) Initialize(serviceURL *url.URL, logger types.StdLogger) error {
	// Set the logger for the service
	s.SetLogger(logger)
	// Initialize service config
	s.Config = &Config{}
	// Initialize property key resolver
	s.pkr = format.NewPropKeyResolver(s.Config)

	// Set default properties
	if err := s.pkr.SetDefaultProps(s.Config); err != nil {
		return fmt.Errorf("setting default properties: %w", err)
	}

	// Set URL and return any error
	if err := s.Config.SetURL(serviceURL); err != nil {
		return fmt.Errorf("setting config URL: %w", err)
	}

	return nil
}

// ParseChannelID parses the channel string to an integer.
//
// The string must hold only decimal digits, so a sign or text around the
// digits is rejected. A Discord channel ID needs a 64-bit int, so on 32-bit
// platforms it is rejected as out of range.
//
// Parameters:
//   - channelStr: the channel ID from the config.
//
// Returns:
//   - int: the channel ID.
//   - error: [ErrInvalidChannelID] with the reason, such as invalid syntax or
//     a value out of range, when the string is not a channel ID.
func (s *Service) ParseChannelID(channelStr string) (int, error) {
	if channelStr == "" || strings.ContainsFunc(channelStr, func(r rune) bool { return r < '0' || r > '9' }) {
		return 0, fmt.Errorf("invalid channel ID format '%s': %w: %w", channelStr, ErrInvalidChannelID, strconv.ErrSyntax)
	}

	channelID, err := strconv.Atoi(channelStr)
	if err != nil {
		reason := err
		if numErr, ok := errors.AsType[*strconv.NumError](err); ok {
			reason = numErr.Err
		}

		return 0, fmt.Errorf("invalid channel ID format '%s': %w: %w", channelStr, ErrInvalidChannelID, reason)
	}

	return channelID, nil
}

// ParseFields parses a JSON string into a slice of Field structs.
//
// Parameters:
//   - fieldsStr: the fields as a JSON array.
//
// Returns:
//   - []Field: the fields.
//   - error: the decoding error.
func (s *Service) ParseFields(fieldsStr string) ([]Field, error) {
	var fields []Field
	if err := json.Unmarshal([]byte(fieldsStr), &fields, jsonopts.UserInput); err != nil {
		return nil, fmt.Errorf("unmarshaling fields JSON: %w", err)
	}

	return fields, nil
}

// ParseMentions extracts Discord user and role mentions from the message content.
//
// Parameters:
//   - message: the message.
//
// Returns:
//   - []string: the mentions in order, with user mentions written as <@ID>
//     and role mentions as <@&ID>.
func (s *Service) ParseMentions(message string) []string {
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

// Send delivers a notification message to Notifiarr.
//
// It delegates to [Service.SendContext] with [context.Background].
//
// Parameters:
//   - message: the message to send.
//   - paramsPtr: optional send params, which may override configuration fields
//     and set the Discord payload. The reserved message and level params are
//     accepted too. Any other param is an error.
//
// Returns:
//   - error: [types.ErrServiceNotInitialized] before Initialize,
//     [ErrEmptyMessage], the params failure, the error from the config's
//     checks, a payload error, or [ErrSendFailed] wrapping the send error.
func (s *Service) Send(message string, paramsPtr *types.Params) error {
	return s.SendContext(context.Background(), message, paramsPtr)
}

// SendContext delivers a notification message to Notifiarr. A context without
// a deadline is bounded by the budget that [Service.ServiceTimeout] reports.
//
// Parameters:
//   - ctx: cancellation and deadline for the request.
//   - message: the message to send.
//   - paramsPtr: optional send params, which may override configuration fields
//     and set the Discord payload. The reserved message and level params are
//     accepted too. Any other param is an error.
//
// Returns:
//   - error: [types.ErrServiceNotInitialized] before Initialize,
//     [ErrEmptyMessage], the params failure, the error from the config's
//     checks, a payload error, or [ErrSendFailed] wrapping the send error. A
//     request error matches ctx's error when ctx ends the request.
func (s *Service) SendContext(ctx context.Context, message string, paramsPtr *types.Params) error {
	if s.Config == nil {
		return types.ErrServiceNotInitialized
	}

	// Check for empty message
	if message == "" {
		return ErrEmptyMessage
	}

	ctx, cancel := httpclient.BoundContext(ctx, s.ServiceTimeout(paramsPtr))
	defer cancel()

	// Create a copy of the config to avoid modifying the original
	config := *s.Config

	var params types.Params
	// Handle nil params by creating empty map
	if paramsPtr == nil {
		params = types.Params{}
	} else {
		params = *paramsPtr
	}

	// The payload params fill the payload. Any other unknown param is an error.
	if err := s.pkr.UpdateConfigFromParams(&config, paramsPtr,
		iconParam, contentParam, footerParam, fieldsParam, updateParam, idParam); err != nil {
		return fmt.Errorf("updating config from params: %w", err)
	}

	if err := config.validate(); err != nil {
		return err
	}

	// Create the payload
	payload, err := s.createPayload(message, params, &config)
	if err != nil {
		return fmt.Errorf("creating payload: %w", err)
	}

	// Send the notification
	if err := s.doSend(ctx, &config, payload); err != nil {
		return fmt.Errorf("%w: %w", ErrSendFailed, err)
	}

	return nil
}

// ServiceTimeout reports the send budget of Notifiarr.
//
// Returns:
//   - time.Duration: [requestTimeout], whatever the params.
func (*Service) ServiceTimeout(*types.Params) time.Duration {
	return requestTimeout
}

// SetHTTPClient sets a custom HTTP client for the service. A nil client,
// including a nil *http.Client, selects the default client.
//
// Parameters:
//   - client: the HTTP client to use for requests, or nil for the default.
func (s *Service) SetHTTPClient(client types.HTTPClient) {
	s.client.Set(client)
}

// buildDiscordPayload constructs the Discord payload if any fields are present.
//
// Parameters:
//   - flags: the Discord fields the send sets.
//   - message: the message, used as the description.
//   - params: the send params, with the title, icon, content, footer, and
//     fields.
//   - config: the configuration with the channel, color, and images.
//
// Returns:
//   - *DiscordPayload: the Discord payload.
//   - error: [ErrNoDiscordFields] when the send sets none, or an error from
//     reading the channel ID or the fields.
func (s *Service) buildDiscordPayload(
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
		channelID, err := s.ParseChannelID(config.Channel)
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
		Icon:        params[iconParam],
		Content:     params[contentParam],
		Description: message,
		Footer:      params[footerParam],
	}

	if flags.fields {
		// Parse JSON fields string into Field structs
		fields, err := s.ParseFields(params[fieldsParam])
		if err != nil {
			return nil, fmt.Errorf("parsing fields: %w", err)
		}

		textPayload.Fields = fields
	}

	discord.Text = textPayload

	if flags.mentions {
		// Extract Discord mentions from message content
		mentions := s.ParseMentions(message)

		// Extract user and role IDs for ping setup (collects all mentions)
		pingUsers, pingRoles := s.ExtractPingIDs(mentions)
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
//
// Parameters:
//   - message: the message.
//   - params: the send params.
//   - config: the configuration for this send.
//
// Returns:
//   - []byte: the JSON payload.
//   - error: a Discord payload or encoding error.
func (s *Service) createPayload(
	message string,
	params types.Params,
	config *Config,
) ([]byte, error) {
	// Parse the update parameter from params
	updatePtr := ParseUpdateFlag(params)
	// Build the notification data structure
	notificationData := buildNotificationData(updatePtr, config, params)
	// Check presence flags for Discord fields
	flags := checkPresenceFlags(message, params, config, s)

	// Build the Discord payload if fields are present
	discord, err := s.buildDiscordPayload(flags, message, params, config)
	if err != nil {
		return nil, err
	}

	notification := NotificationPayload{
		Notification: notificationData,
		Discord:      discord,
	}

	// Marshal the notification to JSON
	payloadBytes, err := json.Marshal(notification, jsonopts.Default)
	if err != nil {
		return nil, fmt.Errorf("marshaling payload to JSON: %w", err)
	}

	return payloadBytes, nil
}

// doSend executes the HTTP request to send a notification to Notifiarr.
// It differentiates between authentication failures and other errors.
//
// Parameters:
//   - ctx: cancellation and deadline for the request.
//   - config: the config for this send, with the API key.
//   - payload: the JSON payload.
//
// Returns:
//   - error: the request error, [ErrAuthenticationFailed] for a 401 status, or
//     [ErrUnexpectedStatus] for another non-success status.
func (s *Service) doSend(ctx context.Context, config *Config, payload []byte) error {
	// The API key is escaped as one path segment. A slash, question mark, or
	// hash in it then cannot change the endpoint or start a query or fragment.
	apiURL := APIBaseURL + "/" + url.PathEscape(config.APIKey)

	// Create HTTP request with context
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, apiURL, bytes.NewBuffer(payload))
	if err != nil {
		return fmt.Errorf("creating HTTP request: %w", redact.URLError(err))
	}

	// Set headers
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")

	res, err := s.client.Client().Do(req)
	if err != nil {
		return fmt.Errorf("sending HTTP request: %w", redact.URLError(err))
	}

	if res != nil && res.Body != nil {
		defer func() {
			_ = res.Body.Close()
		}()

		// The body is not logged because it can echo the request, including its credentials.
		s.LogAttrs(ctx, slog.LevelDebug, "response received", slog.String(logging.KeyStatus, res.Status))
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

// ParseMention parses a single mention string and returns the type (0=none, 1=user, 2=role) and ID if valid.
//
// Parameters:
//   - mention: the mention, such as <@123>, <@!123>, or <@&456>.
//
// Returns:
//   - int: [MentionTypeUser], [MentionTypeRole], or [MentionTypeNone] for an
//     invalid mention.
//   - int: the user or role ID, or 0 for an invalid mention.
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

// ParseUpdateFlag parses the update parameter from params.
//
// Parameters:
//   - params: the send params.
//
// Returns:
//   - *bool: true or false for an update param of "true" or "false", or nil
//     otherwise.
func ParseUpdateFlag(params types.Params) *bool {
	if updateStr, exists := params[updateParam]; exists && updateStr != "" {
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
//
// Parameters:
//   - updatePtr: the update flag, or nil to leave it out.
//   - config: the configuration with the app name.
//   - params: the send params, whose id param becomes the event ID.
//
// Returns:
//   - NotificationData: the notification metadata.
func buildNotificationData(updatePtr *bool, config *Config, params types.Params) NotificationData {
	return NotificationData{
		Update: updatePtr,
		Name:   config.appName(),
		Event:  params[idParam],
	}
}

// checkPresenceFlags determines which Discord fields are present.
//
// Parameters:
//   - message: the message.
//   - params: the send params.
//   - config: the configuration for this send.
//   - service: the service, which finds the mentions in the message.
//
// Returns:
//   - presenceFlags: the Discord fields the send sets.
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
		icon:        params[iconParam] != "",
		content:     params[contentParam] != "",
		description: message != "",
		footer:      params[footerParam] != "",
		fields:      params[fieldsParam] != "",
		mentions:    len(service.ParseMentions(message)) > 0,
	}
}
