package join

import (
	"context"
	"encoding/json/jsontext"
	"encoding/json/v2"
	"fmt"
	"net/http"
	"net/url"
	"strings"

	"github.com/nicholas-fedor/shoutrrr/internal/httpbody"
	"github.com/nicholas-fedor/shoutrrr/internal/httpclient"
	"github.com/nicholas-fedor/shoutrrr/internal/jsonopts"
	"github.com/nicholas-fedor/shoutrrr/internal/redact"
	"github.com/nicholas-fedor/shoutrrr/pkg/format"
	"github.com/nicholas-fedor/shoutrrr/pkg/services/standard"
	"github.com/nicholas-fedor/shoutrrr/pkg/types"
)

// Service sends notifications to Join devices.
type Service struct {
	// Standard provides logging and the other shared service functionality.
	standard.Standard

	// client holds the injected HTTP client and the default client.
	client httpclient.Holder

	// Config holds the API key, the devices, and the message options.
	Config *Config
	// pkr reads and writes the config props by their query keys.
	pkr format.PropKeyResolver
}

// apiResponse holds the fields of a Join API reply that report the result of
// a push.
//
// Join answers a push it refuses with HTTP 200 and Success false.
type apiResponse struct {
	// ErrorMessage is Join's reason for refusing the push, such as "No device
	// to send message to" for a wrong API key.
	ErrorMessage string `json:"errorMessage"`
	// Success reports whether Join accepted the push.
	Success bool `json:"success"`
}

// Settings for requests to the Join API.
const (
	// hookURL defines the Join API endpoint for sending push notifications.
	hookURL = "https://joinjoaomgcd.appspot.com/_ah/api/messaging/v1/sendPush"
	// contentType is the Content-Type header of the request. The request has
	// an empty body and carries its fields in the query.
	contentType = "text/plain"
)

// Compile-time checks that Service implements the interfaces the router relies on.
var (
	_ types.Service          = (*Service)(nil)
	_ types.ContextSender    = (*Service)(nil)
	_ types.HTTPClientSetter = (*Service)(nil)
	_ types.ServiceTimeout   = (*Service)(nil)
)

// GetID returns the identifier for this service.
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
//   - error: the error from reading the URL, such as [ErrDevicesMissing] or
//     [ErrAPIKeyMissing].
func (s *Service) Initialize(serviceURL *url.URL, logger types.StdLogger) error {
	s.SetLogger(logger)
	s.Config = &Config{}
	s.pkr = format.NewPropKeyResolver(s.Config)

	if err := s.Config.setURL(&s.pkr, serviceURL); err != nil {
		return err
	}

	return nil
}

// Send delivers a notification message to Join devices.
//
// It delegates to [Service.SendContext] with [context.Background].
//
// Parameters:
//   - message: the message to send.
//   - params: optional title and icon overrides.
//
// Returns:
//   - error: [types.ErrServiceNotInitialized] before Initialize, the send
//     error.
func (s *Service) Send(message string, params *types.Params) error {
	return s.SendContext(context.Background(), message, params)
}

// SendContext delivers a notification message to Join devices in one request.
// A context without a deadline is bounded by the budget that
// [Service.ServiceTimeout] reports.
//
// Parameters:
//   - ctx: cancellation and deadline for the request.
//   - message: the message to send.
//   - params: optional overrides for the config keys, such as title, icon,
//     and devices. Any other key is an error.
//
// Returns:
//   - error: [types.ErrServiceNotInitialized] before Initialize. A params
//     error or the error from the config's checks, such as [ErrDevicesMissing]
//     when the params leave no devices, fails the send before any request.
//     [ErrSendFailed] when Join refuses the push or answers with a status
//     other than 200, with Join's reason when it gives one. Otherwise the
//     error sending the request, reading a 200 reply, or decoding a 200 JSON
//     reply. The error matches ctx's error when ctx ends the request.
func (s *Service) SendContext(ctx context.Context, message string, params *types.Params) error {
	if s.Config == nil {
		return types.ErrServiceNotInitialized
	}

	// Params apply to this send only, so they update a copy of the service
	// config.
	configCopy := *s.Config
	config := &configCopy

	resolver := format.NewPropKeyResolver(config)
	if err := resolver.UpdateConfigFromParams(config, params); err != nil {
		return fmt.Errorf("updating config from params: %w", err)
	}

	if err := config.prepare(); err != nil {
		return err
	}

	ctx, cancel := httpclient.BoundContext(ctx, s.ServiceTimeout(params))
	defer cancel()

	return s.sendToDevices(ctx, config, message)
}

// SetHTTPClient sets a custom HTTP client for the service.
//
// A nil client, including a nil *http.Client, selects the default client.
//
// Parameters:
//   - client: the HTTP client to use for requests, or nil for the default.
func (s *Service) SetHTTPClient(client types.HTTPClient) {
	s.client.Set(client)
}

// sendToDevices sends the message to the given devices through the Join API.
//
// Parameters:
//   - ctx: cancellation and deadline for the request.
//   - config: the config for this send, with the devices, API key, title, and
//     icon. An empty title or icon is left out.
//   - message: the message to send.
//
// Returns:
//   - error: the request error, or the error reading a 200 reply or
//     decoding a 200 JSON reply. [ErrSendFailed] for a status other than 200
//     or a reply that does not report success. The error carries Join's reason
//     when the reply gives one.
func (s *Service) sendToDevices(ctx context.Context, config *Config, message string) error {
	devices := strings.Join(config.Devices, ",")

	apiURL, err := url.Parse(hookURL)
	if err != nil {
		return fmt.Errorf("parsing Join API URL: %w", err)
	}

	data := url.Values{}
	data.Set("deviceIds", devices)
	data.Set("apikey", config.APIKey)
	data.Set("text", message)

	if config.Title != "" {
		data.Set("title", config.Title)
	}

	if config.Icon != "" {
		data.Set("icon", config.Icon)
	}

	apiURL.RawQuery = data.Encode()

	req, err := http.NewRequestWithContext(
		ctx,
		http.MethodPost,
		apiURL.String(),
		http.NoBody,
	)
	if err != nil {
		return fmt.Errorf("creating HTTP request: %w", redact.URLError(err))
	}

	req.Header.Set("Content-Type", contentType)

	res, err := s.client.Client().Do(req)
	if err != nil {
		return fmt.Errorf("sending HTTP request to Join: %w", redact.URLError(err))
	}

	defer func() { _ = res.Body.Close() }()

	body, readErr := httpbody.Read(res.Body, httpbody.MaxSize)

	// The unread part of a 200 reply could hold a refusal, so a read error
	// fails the send. An error status fails it below either way.
	if readErr != nil && res.StatusCode == http.StatusOK {
		return fmt.Errorf("reading response body: %w", readErr)
	}

	var response apiResponse

	// replyErr holds the error reading or decoding the reply. A read error
	// reaches here only with an error status.
	replyErr := readErr
	if replyErr == nil {
		replyErr = json.Unmarshal(body, &response, jsonopts.Default)
	}

	// Join accepts a push with a 200 status and success in the reply. A JSON
	// reply that does not decode cannot show that Join accepted the push. A
	// reply that is not JSON leaves the HTTP status to decide.
	if res.StatusCode == http.StatusOK {
		switch {
		case replyErr == nil:
			if response.Success {
				return nil
			}
		case jsontext.Value(body).IsValid(
			jsontext.AllowDuplicateNames(true),
			jsontext.AllowInvalidUTF8(true),
		):
			return fmt.Errorf("decoding response body: %w", replyErr)
		default:
			return nil
		}
	}

	if replyErr == nil && response.ErrorMessage != "" {
		return fmt.Errorf(
			"%w: %q, response status %q: %s",
			ErrSendFailed,
			devices,
			res.Status,
			response.ErrorMessage,
		)
	}

	return fmt.Errorf("%w: %q, response status %q", ErrSendFailed, devices, res.Status)
}
