package pushover

import (
	"context"
	"encoding/json/v2"
	"fmt"
	"net/http"
	"net/url"
	"strconv"
	"strings"

	"github.com/nicholas-fedor/shoutrrr/internal/httpbody"
	"github.com/nicholas-fedor/shoutrrr/internal/httpclient"
	"github.com/nicholas-fedor/shoutrrr/internal/jsonopts"
	"github.com/nicholas-fedor/shoutrrr/internal/redact"
	"github.com/nicholas-fedor/shoutrrr/pkg/format"
	"github.com/nicholas-fedor/shoutrrr/pkg/services/standard"
	"github.com/nicholas-fedor/shoutrrr/pkg/types"
)

// Service provides the Pushover notification service.
type Service struct {
	// Standard provides logging and the other shared service functionality.
	standard.Standard

	// Config holds the keys, the devices, and the message options.
	Config *Config
	// Client sends the requests when set. It stays nil after Initialize until
	// the caller sets it, and the sends use the default client meanwhile.
	// [Service.SetHTTPClient] clears it, so whichever of the two the caller
	// set last carries the sends.
	//
	// Deprecated: Use [Service.SetHTTPClient].
	Client *http.Client
	// client holds the client from [Service.SetHTTPClient] and the default
	// client.
	client httpclient.Holder
	// pkr reads and writes the config props by their query keys.
	pkr format.PropKeyResolver
}

// apiResponse is the part of a Message API response that reports the result.
type apiResponse struct {
	// Errors describes each invalid input of a rejected request.
	Errors []string `json:"errors"`
	// Status is 1 for an accepted request.
	Status int `json:"status"`
}

// Settings for requests to the Pushover Message API.
const (
	// hookURL is the Pushover API endpoint for sending messages.
	hookURL = "https://api.pushover.net/1/messages.json"
	// contentType is the form encoding the Message API takes.
	contentType = "application/x-www-form-urlencoded"
	// statusAccepted is the status the Message API reports for an accepted
	// request.
	statusAccepted = 1
)

// Compile-time checks that Service implements the interfaces the router relies on.
var (
	_ types.Service          = (*Service)(nil)
	_ types.ContextSender    = (*Service)(nil)
	_ types.HTTPClientSetter = (*Service)(nil)
	_ types.ServiceTimeout   = (*Service)(nil)
)

// levelPriorities maps each known message level to the Pushover priority it is
// sent with when the send keeps the default priority.
var levelPriorities = map[types.MessageLevel]int8{
	types.Debug:   -1,
	types.Info:    0,
	types.Warning: 0,
	types.Error:   1,
}

// GetID returns the service identifier.
//
// Returns:
//   - string: the scheme, pushover.
func (s *Service) GetID() string {
	return Scheme
}

// Initialize configures the service with a URL and logger. It keeps an HTTP
// client that is already set.
//
// Parameters:
//   - serviceURL: the service URL with the token and the user key.
//   - logger: the logger for the service's output.
//
// Returns:
//   - error: the error from reading the URL, such as [ErrUserMissing].
func (s *Service) Initialize(serviceURL *url.URL, logger types.StdLogger) error {
	s.SetLogger(logger)
	s.Config = &Config{}

	s.pkr = format.NewPropKeyResolver(s.Config)

	if err := s.Config.setURL(&s.pkr, serviceURL); err != nil {
		return err
	}

	return nil
}

// Send delivers a notification message to Pushover.
//
// It delegates to [Service.SendContext] with [context.Background].
//
// Parameters:
//   - message: the message to send.
//   - params: optional overrides for configuration fields.
//
// Returns:
//   - error: [types.ErrServiceNotInitialized] before Initialize, the error
//     from the params or the config's checks, such as [ErrInvalidTimestamp],
//     or the send error.
func (s *Service) Send(message string, params *types.Params) error {
	return s.SendContext(context.Background(), message, params)
}

// SendContext delivers a notification message to the configured Pushover devices
// in one request. A context without a deadline is bounded by the budget that
// [Service.ServiceTimeout] reports.
//
// Parameters:
//   - ctx: cancellation and deadline for the request.
//   - message: the message to send.
//   - params: optional overrides for configuration fields.
//
// Returns:
//   - error: [types.ErrServiceNotInitialized] before Initialize, the error
//     from the params or the config's checks, such as [ErrInvalidTimestamp],
//     or the send error. A request error matches ctx's error when ctx ends the
//     request.
func (s *Service) SendContext(ctx context.Context, message string, params *types.Params) error {
	if s.Config == nil {
		return types.ErrServiceNotInitialized
	}

	ctx, cancel := httpclient.BoundContext(ctx, s.ServiceTimeout(params))
	defer cancel()

	// Params apply to this send only, so they update a copy of the service config.
	configCopy := *s.Config
	config := &configCopy

	if err := s.pkr.UpdateConfigFromParams(config, params); err != nil {
		return fmt.Errorf("updating config from params: %w", err)
	}

	s.applyLevel(config, params)

	if err := config.validate(); err != nil {
		return err
	}

	device := strings.Join(config.Devices, ",")
	if err := s.sendToDevice(ctx, device, message, config); err != nil {
		return fmt.Errorf("failed to send notifications to pushover devices: %w", err)
	}

	return nil
}

// SetHTTPClient sets a custom HTTP client for the service. A nil client,
// including a nil *http.Client, selects the default client. It clears the
// deprecated Client field, so the sends use this client until that field is
// set again.
//
// Parameters:
//   - client: the HTTP client to use for requests, or nil for the default.
func (s *Service) SetHTTPClient(client types.HTTPClient) {
	s.client.Set(client)
	s.Client = nil
}

// applyLevel sets the priority of a send from its level param, debug to low
// (-1), info and warning to normal (0), and error to high (1), which bypasses
// quiet hours. Emergency priority needs retry settings, so no level maps to it.
// A priority set in the service URL or the send params takes precedence.
//
// Parameters:
//   - config: the config for this send, with params already applied.
//   - params: the send params, or nil.
func (s *Service) applyLevel(config *Config, params *types.Params) {
	if params == nil {
		return
	}

	level, ok := params.Level()
	if !ok {
		return
	}

	if levelPriority, mapped := levelPriorities[level]; mapped && s.pkr.KeepsDefault(config, params, "priority") {
		config.Priority = levelPriority
	}
}

// sendToDevice sends a notification to the given Pushover devices.
//
// Parameters:
//   - ctx: cancellation and deadline for the request.
//   - device: the comma-separated device names, or empty for all devices.
//   - message: the message to send.
//   - config: the configuration for this send.
//
// Returns:
//   - error: an encryption error, the request error, or an error wrapping
//     [ErrSendFailed] for a rejected request, with the errors the API reported.
func (s *Service) sendToDevice(ctx context.Context, device, message string, config *Config) error {
	key, err := parseEncryptionKey(config.EncryptionKey)
	if err != nil {
		return err
	}

	if key != nil {
		message, err = encryptField(message, key)
		if err != nil {
			return err
		}
	}

	data := url.Values{}
	data.Set("device", device)
	data.Set("user", config.User)
	data.Set("token", config.Token)
	data.Set("message", message)

	if config.Title != "" {
		title := config.Title
		if key != nil {
			title, err = encryptField(title, key)
			if err != nil {
				return err
			}
		}

		data.Set("title", title)
	}

	if key != nil {
		data.Set("encrypted", "1")
	}

	if config.Priority >= -2 && config.Priority <= 1 {
		data.Set("priority", strconv.FormatInt(int64(config.Priority), 10))
	}

	// The timestamp is not one of the fields that end-to-end encryption covers.
	if config.Timestamp > 0 {
		data.Set("timestamp", strconv.FormatInt(config.Timestamp, 10))
	}

	req, err := http.NewRequestWithContext(
		ctx,
		http.MethodPost,
		hookURL,
		strings.NewReader(data.Encode()),
	)
	if err != nil {
		return fmt.Errorf("creating request: %w", redact.URLError(err))
	}

	req.Header.Set("Content-Type", contentType)

	res, err := s.client.Prefer(s.Client).Do(req)
	if err != nil {
		return fmt.Errorf("sending request to Pushover API: %w", redact.URLError(err))
	}

	defer func() { _ = res.Body.Close() }()

	body, readErr := httpbody.Read(res.Body, httpbody.MaxSize)

	var response apiResponse

	decoded := readErr == nil && json.Unmarshal(body, &response, jsonopts.Default) == nil

	// The API accepts a request with a 200 status and a status of 1 in the
	// body. A body that is not JSON leaves the HTTP status to decide.
	if res.StatusCode == http.StatusOK && (!decoded || response.Status == statusAccepted) {
		return nil
	}

	if decoded && len(response.Errors) > 0 {
		return fmt.Errorf(
			"%w: %q, response status %q: %s",
			ErrSendFailed,
			device,
			res.Status,
			strings.Join(response.Errors, ", "),
		)
	}

	return fmt.Errorf("%w: %q, response status %q", ErrSendFailed, device, res.Status)
}
