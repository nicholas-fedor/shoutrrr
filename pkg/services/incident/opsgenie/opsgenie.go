package opsgenie

import (
	"bytes"
	"context"
	"encoding/json/v2"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"strconv"
	"unicode/utf8"

	"github.com/nicholas-fedor/shoutrrr/internal/httpbody"
	"github.com/nicholas-fedor/shoutrrr/internal/httpclient"
	"github.com/nicholas-fedor/shoutrrr/internal/jsonopts"
	"github.com/nicholas-fedor/shoutrrr/internal/redact"
	"github.com/nicholas-fedor/shoutrrr/pkg/format"
	"github.com/nicholas-fedor/shoutrrr/pkg/services/standard"
	"github.com/nicholas-fedor/shoutrrr/pkg/types"
	"github.com/nicholas-fedor/shoutrrr/pkg/util"
)

// Service provides OpsGenie as a notification service.
type Service struct {
	// Standard provides logging and the other shared service functionality.
	standard.Standard

	// client holds the injected HTTP client and the default client.
	client httpclient.Holder

	// Config holds the API key, the host, and the alert fields.
	Config *Config
	// pkr reads and writes the config props by their query keys.
	pkr format.PropKeyResolver
}

// Settings and limits of the Alert API.
const (
	// alertEndpointPath is the OpsGenie API endpoint path for sending alerts.
	alertEndpointPath = "/v2/alerts"
	// MaxMessageLength is the maximum length of the alert message field in OpsGenie.
	MaxMessageLength = 130
	// httpSuccessMax is the maximum HTTP status code for a successful response.
	httpSuccessMax = 299
)

// ErrUnexpectedStatus indicates that OpsGenie returned an unexpected HTTP status code.
var ErrUnexpectedStatus = errors.New("OpsGenie notification returned unexpected HTTP status code")

// Compile-time checks that Service implements the interfaces the router relies on.
var (
	_ types.Service          = (*Service)(nil)
	_ types.ContextSender    = (*Service)(nil)
	_ types.HTTPClientSetter = (*Service)(nil)
	_ types.ServiceTimeout   = (*Service)(nil)
)

// levelPriorities maps each known message level to the Opsgenie priority it is
// sent with when the send keeps the default priority. P1 is left for alerts
// configured as critical.
var levelPriorities = map[types.MessageLevel]string{
	types.Debug:   "P5",
	types.Info:    "P4",
	types.Warning: "P3",
	types.Error:   "P2",
}

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
//   - error: the error from reading the URL, such as [ErrAPIKeyMissing] or
//     [ErrInvalidPort].
func (s *Service) Initialize(serviceURL *url.URL, logger types.StdLogger) error {
	s.SetLogger(logger)
	s.Config = &Config{}
	s.pkr = format.NewPropKeyResolver(s.Config)

	return s.Config.setURL(&s.pkr, serviceURL)
}

// Send delivers a notification message to OpsGenie.
//
// It delegates to [Service.SendContext] with [context.Background].
//
// Parameters:
//   - message: the message to send.
//   - params: optional overrides for configuration fields.
//
// Returns:
//   - error: [types.ErrServiceNotInitialized] before Initialize, the error
//     from the params or the config's checks, a payload error, the request
//     error, or [ErrUnexpectedStatus] for a non-success status.
func (s *Service) Send(message string, params *types.Params) error {
	return s.SendContext(context.Background(), message, params)
}

// SendContext delivers a notification message to OpsGenie as a new alert.
// See: https://docs.opsgenie.com/docs/alert-api#create-alert
// A context without a deadline is bounded by the budget that
// [Service.ServiceTimeout] reports.
//
// Parameters:
//   - ctx: cancellation and deadline for the request.
//   - message: the message to send.
//   - params: optional overrides for configuration fields.
//
// Returns:
//   - error: [types.ErrServiceNotInitialized] before Initialize, the error
//     from the params or the config's checks, a payload error, the request
//     error, or [ErrUnexpectedStatus] for a non-success status. A request
//     error matches ctx's error when ctx ends the request.
func (s *Service) SendContext(ctx context.Context, message string, params *types.Params) error {
	if s.Config == nil {
		return types.ErrServiceNotInitialized
	}

	ctx, cancel := httpclient.BoundContext(ctx, s.ServiceTimeout(params))
	defer cancel()

	if params == nil {
		params = &types.Params{}
	}

	// Params apply to this send only, so they update a copy of the config.
	config := *s.Config

	if err := s.pkr.UpdateConfigFromParams(&config, params); err != nil {
		return fmt.Errorf("updating payload fields from params: %w", err)
	}

	s.applyLevel(&config, params)

	if err := config.validate(); err != nil {
		return err
	}

	config.applyDefaults()

	payload, err := newAlertPayload(message, &config)
	if err != nil {
		return err
	}

	serviceURL := (&url.URL{
		Scheme: "https",
		Host:   util.JoinURLHost(config.Host, strconv.FormatUint(uint64(config.Port), 10)),
		Path:   alertEndpointPath,
	}).String()

	return s.sendAlert(ctx, serviceURL, config.APIKey, &payload)
}

// SetHTTPClient sets a custom HTTP client for the service. A nil client,
// including a nil *http.Client, selects the default client.
//
// Parameters:
//   - client: the HTTP client to use for requests, or nil for the default.
func (s *Service) SetHTTPClient(client types.HTTPClient) {
	s.client.Set(client)
}

// applyLevel sets the priority of an alert from its level param, debug to P5,
// info to P4, warning to P3, and error to P2. A priority set in the service URL
// or the send params takes precedence.
//
// Parameters:
//   - config: the config for this send, with params already applied.
//   - params: the send params, which are never nil here.
func (s *Service) applyLevel(config *Config, params *types.Params) {
	level, ok := params.Level()
	if !ok {
		return
	}

	if levelPriority, mapped := levelPriorities[level]; mapped && s.pkr.KeepsDefault(config, params, "priority") {
		config.Priority = levelPriority
	}
}

// newAlertPayload creates a new alert payload for OpsGenie based on the message and parameters.
//
// The title, or the message when there is no title, becomes the alert
// message, cut to [MaxMessageLength] characters. The message also becomes the
// description when a title is set or when it is cut.
//
// Parameters:
//   - message: the notification message.
//   - payloadFields: the config for this send, with its params applied.
//
// Returns:
//   - AlertPayload: the alert.
//   - error: [ErrUnexpectedEntityType] for a visibleTo entry that is not a
//     team or user.
func newAlertPayload(message string, payloadFields *Config) (AlertPayload, error) {
	// The Alert API accepts only teams and users in visibleTo, while responders
	// may also be escalations and schedules.
	for _, entity := range payloadFields.VisibleTo {
		if entity.Type != "team" && entity.Type != "user" {
			return AlertPayload{}, fmt.Errorf("%w: visibleTo cannot include %q", ErrUnexpectedEntityType, entity.Type)
		}
	}

	// Use `Message` for the title if available, or if the message is too long
	// Use `Description` for the message in these scenarios
	title := payloadFields.Title
	description := message

	if title == "" {
		title = message
		if utf8.RuneCountInString(message) <= MaxMessageLength {
			description = ""
		}
	}

	// The alert message is limited to MaxMessageLength characters, so it is cut
	// on a character boundary.
	if utf8.RuneCountInString(title) > MaxMessageLength {
		title = string([]rune(title)[:MaxMessageLength])
	}

	if payloadFields.Description != "" && description != "" {
		description += "\n"
	}

	result := AlertPayload{
		Message:     title,
		Alias:       payloadFields.Alias,
		Description: description + payloadFields.Description,
		Responders:  payloadFields.Responders,
		VisibleTo:   payloadFields.VisibleTo,
		Actions:     payloadFields.Actions,
		Tags:        payloadFields.Tags,
		Details:     payloadFields.Details,
		Entity:      payloadFields.Entity,
		Source:      payloadFields.Source,
		Priority:    payloadFields.Priority,
		User:        payloadFields.User,
		Note:        payloadFields.Note,
	}

	return result, nil
}

// sendAlert sends an alert to OpsGenie using the specified URL and API key.
//
// Parameters:
//   - ctx: cancellation and deadline for the request.
//   - serviceURL: the alert API URL.
//   - apiKey: the OpsGenie API key.
//   - payload: the alert payload.
//
// Returns:
//   - error: a marshaling or request error, or [ErrUnexpectedStatus] for a
//     non-success status.
func (s *Service) sendAlert(ctx context.Context, serviceURL, apiKey string, payload *AlertPayload) error {
	jsonBody, err := json.Marshal(payload, jsonopts.Default)
	if err != nil {
		return fmt.Errorf("marshaling alert payload to JSON: %w", err)
	}

	jsonBuffer := bytes.NewBuffer(jsonBody)

	req, err := http.NewRequestWithContext(
		ctx,
		http.MethodPost,
		serviceURL,
		jsonBuffer,
	)
	if err != nil {
		return fmt.Errorf("creating HTTP request: %w", redact.URLError(err))
	}

	req.Header.Add("Authorization", "GenieKey "+apiKey)
	req.Header.Add("Content-Type", "application/json")

	resp, err := s.client.Client().Do(req)
	if err != nil {
		return fmt.Errorf("failed to send notification to OpsGenie: %w", redact.URLError(err))
	}

	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode < http.StatusOK || resp.StatusCode > httpSuccessMax {
		body, err := httpbody.Read(resp.Body, httpbody.MaxSize)
		if err != nil && !errors.Is(err, httpbody.ErrTooLarge) {
			return fmt.Errorf(
				"%w: %d, cannot read body: %w",
				ErrUnexpectedStatus,
				resp.StatusCode,
				err,
			)
		}

		return fmt.Errorf(
			"%w: %d - %s",
			ErrUnexpectedStatus,
			resp.StatusCode,
			body,
		)
	}

	return nil
}
