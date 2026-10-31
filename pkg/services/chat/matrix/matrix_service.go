package matrix

import (
	"context"
	"fmt"
	"log/slog"
	"net/url"
	"strings"
	"time"

	"github.com/nicholas-fedor/shoutrrr/internal/httpclient"
	"github.com/nicholas-fedor/shoutrrr/internal/logging"
	"github.com/nicholas-fedor/shoutrrr/pkg/format"
	"github.com/nicholas-fedor/shoutrrr/pkg/services/standard"
	"github.com/nicholas-fedor/shoutrrr/pkg/types"
)

// Service sends notifications via the Matrix protocol.
type Service struct {
	// Standard provides logging and the other shared service functionality.
	standard.Standard

	// httpClient holds the injected HTTP client and the default clients. Each
	// send picks one for its login and room requests.
	httpClient httpclient.Holder
	// warnings logs each warning about the service's settings once.
	warnings logging.Once

	// Config holds the homeserver, the credentials, and the rooms.
	Config *Config
	// accessToken authenticates the room requests. It is the configured
	// access token, or the token of the password login once a send has
	// logged in.
	accessToken string
	// loginGate holds one token while a send runs the password login, so only one
	// login runs at a time and waiting sends can give up when their context ends.
	loginGate chan struct{}
	// pkr reads and writes the config props by their query keys.
	pkr format.PropKeyResolver
	// loggedIn reports whether accessToken holds the token of a password login.
	loggedIn bool
	// ready reports whether the last Initialize succeeded, so a homeserver is
	// configured to send to.
	ready bool
}

// Scheme identifies this service in configuration URLs.
const Scheme = "matrix"

// Request counts used to budget a send.
const (
	// loginRequests is the number of requests a password login makes: the login
	// flows lookup and the login itself.
	loginRequests = 2
	// requestsPerRoom is the number of requests a send makes for a configured room:
	// joining it by alias and sending the message.
	requestsPerRoom = 2
	// minRequestsPerSend is the number of requests a send makes without
	// configured rooms: the joined rooms lookup and one message.
	minRequestsPerSend = 2
)

// Compile-time checks that Service implements the interfaces the router relies on.
var (
	_ types.Service          = (*Service)(nil)
	_ types.HTTPClientSetter = (*Service)(nil)
	_ types.ContextSender    = (*Service)(nil)
	_ types.ServiceTimeout   = (*Service)(nil)
)

// GetID returns the identifier for this service.
//
// Returns:
//   - string: the service scheme, [Scheme].
func (s *Service) GetID() string {
	return Scheme
}

// Initialize configures the service with a URL and logger. It performs no network
// I/O: a password login runs on the first send, through the HTTP client and
// context in effect at that time.
//
// Parameters:
//   - serviceURL: the service URL.
//   - logger: the logger for service output.
//
// Returns:
//   - error: the error from reading the URL, such as [ErrMissingCredentials]
//     or [ErrMissingHost].
func (s *Service) Initialize(serviceURL *url.URL, logger types.StdLogger) error {
	s.SetLogger(logger)
	s.warnings.Reset()
	s.Config = &Config{
		EnumlessConfig: standard.EnumlessConfig{},
		User:           "",
		Password:       "",
		DisableTLS:     false,
		SkipTLSVerify:  false,
		Host:           "",
		Rooms:          nil,
		Title:          "",
	}
	s.pkr = format.NewPropKeyResolver(s.Config)

	// The earlier session ends before the URL is read, so a URL that fails to
	// read leaves no session to send with.
	s.accessToken = ""
	s.loggedIn = false
	s.ready = false
	s.loginGate = make(chan struct{}, 1)

	if err := s.Config.setURL(&s.pkr, serviceURL); err != nil {
		return err
	}

	s.ready = true

	if s.Config.User == "" {
		s.accessToken = s.Config.Password
	}

	s.LogAttrs(context.Background(), slog.LevelDebug, "using server", slog.String(logging.KeyServer, s.Config.Host))

	return nil
}

// Send delivers a notification message to Matrix rooms.
//
// Parameters:
//   - message: the message to send.
//   - params: per-send parameters, applied to this send only.
//
// Returns:
//   - error: [types.ErrServiceNotInitialized] before Initialize, the login or
//     send failure, or nil on success.
func (s *Service) Send(message string, params *types.Params) error {
	return s.SendContext(context.Background(), message, params)
}

// SendContext delivers a notification message to Matrix rooms. The router calls it
// with its send deadline, and a context without a deadline is bounded by the
// budget that [Service.ServiceTimeout] reports. The first send logs in when a
// user is configured. The first send without TLS logs a warning, and so does
// the first send that skips certificate verification through the default
// client.
//
// Parameters:
//   - ctx: bounds the login and the room requests.
//   - message: the message to send.
//   - params: per-send parameters, applied to this send only.
//
// Returns:
//   - error: [types.ErrServiceNotInitialized] before Initialize, the login or
//     send failure, or nil on success.
func (s *Service) SendContext(ctx context.Context, message string, params *types.Params) error {
	if s.Config == nil {
		return types.ErrServiceNotInitialized
	}

	if !s.ready {
		return ErrClientNotInitialized
	}

	ctx, cancel := httpclient.BoundContext(ctx, s.ServiceTimeout(params))
	defer cancel()

	// Make a per-call copy of the config to avoid mutating the shared s.Config
	cfg := *s.Config

	if err := s.pkr.UpdateConfigFromParams(&cfg, params); err != nil {
		return fmt.Errorf("updating config from params: %w", err)
	}

	// The send's config decides the protocol, so an injected client is warned
	// about too.
	if cfg.DisableTLS {
		s.warnings.Plaintext(ctx, s)
	}

	// One API client carries the send's protocol and HTTP client to the login
	// and the room requests.
	api := newClient(cfg.Host, cfg.apiScheme(), s.sendClient(ctx, &cfg), &s.Logger)

	if err := s.ensureLogin(ctx, api); err != nil {
		return err
	}

	// Create message with title if provided
	fullMessage := createMessage(message, cfg.Title)

	if err := api.sendMessage(ctx, fullMessage, cfg.Rooms); err != nil {
		return fmt.Errorf("sending message: %w", err)
	}

	return nil
}

// SendWithContext delivers a notification message with the provided context.
//
// Parameters:
//   - ctx: bounds the send.
//   - message: the message to send.
//   - params: per-send parameters.
//
// Returns:
//   - error: the send failure, or nil on success.
//
// Deprecated: Use [Service.SendContext], which the router calls with its send deadline.
//
//go:fix inline
func (s *Service) SendWithContext(ctx context.Context, message string, params *types.Params) error {
	return s.SendContext(ctx, message, params)
}

// ServiceTimeout reports the send budget of Matrix. A send makes sequential
// requests, and the budget allows [defaultHTTPTimeout] for each: two for the
// password login when a user is configured, then a join and a message for each
// configured room. Without configured rooms, the send looks up the joined rooms
// and is budgeted for one message, so sends to many joined rooms need configured
// rooms or a longer router timeout. Waits for rate limits count toward the
// budget, so a request is retried only while the budget has room for the wait.
//
// Parameters:
//   - params: per-send parameters, which may set the rooms.
//
// Returns:
//   - time.Duration: [defaultHTTPTimeout] multiplied by the number of requests.
func (s *Service) ServiceTimeout(params *types.Params) time.Duration {
	if s.Config == nil {
		return defaultHTTPTimeout * minRequestsPerSend
	}

	config := *s.Config
	if params != nil && len(*params) > 0 {
		if err := s.pkr.UpdateConfigFromParams(&config, params); err != nil {
			// An invalid param fails the send itself, so the stored config sets the budget.
			config = *s.Config
		}
	}

	requests := minRequestsPerSend
	if len(config.Rooms) > 0 {
		requests = requestsPerRoom * len(config.Rooms)
	}

	if config.User != "" {
		requests += loginRequests
	}

	return defaultHTTPTimeout * time.Duration(requests)
}

// SetHTTPClient sets a custom HTTP client for the service, which the API client
// then sends through. A nil client, including a nil *http.Client, restores the
// default client. An access token from an earlier login stays valid.
//
// Parameters:
//   - client: the HTTP client to use for requests, or nil for the default.
func (s *Service) SetHTTPClient(client types.HTTPClient) {
	s.httpClient.Set(client)
}

// ensureLogin gives api the access token for the send. With a user
// configured, the first send logs in with the user and password through api. A
// failed login is retried on the next send. A send that waits for another
// send's login stops waiting when ctx ends.
//
// Parameters:
//   - ctx: cancellation for waiting on and running the login.
//   - api: the API client of the send, which the login uses too.
//
// Returns:
//   - error: the login failure or ctx's error, or nil when api holds the token.
func (s *Service) ensureLogin(ctx context.Context, api *client) error {
	if s.Config.User == "" {
		api.accessToken = s.accessToken

		return nil
	}

	select {
	case s.loginGate <- struct{}{}:
	case <-ctx.Done():
		return fmt.Errorf("waiting for Matrix login: %w", ctx.Err())
	}

	defer func() { <-s.loginGate }()

	if s.loggedIn {
		api.accessToken = s.accessToken

		return nil
	}

	if err := api.login(ctx, s.Config.User, s.Config.Password); err != nil {
		return fmt.Errorf("logging in to Matrix: %w", err)
	}

	s.accessToken = api.accessToken
	s.loggedIn = true

	return nil
}

// sendClient returns the HTTP client for a send's requests, the login
// included. A send that skips certificate verification uses the insecure
// default client, and the first such send logs the shared warning. An injected
// client is used unchanged and never warned about, because its owner controls
// its TLS settings.
//
// Parameters:
//   - ctx: the context of the send, passed to its warnings.
//   - config: the config for this send.
//
// Returns:
//   - types.HTTPClient: the client to send through.
func (s *Service) sendClient(ctx context.Context, config *Config) types.HTTPClient {
	if config.SkipTLSVerify {
		return s.httpClient.InsecureClient(func() { s.warnings.Insecure(ctx, s) })
	}

	return s.httpClient.Client()
}

// createMessage creates the full message body by prepending the title if provided.
// Format: If title is "Alert" and message is "Hello", output is "Alert\n\nHello".
//
// Parameters:
//   - message: the message.
//   - title: the title, trimmed of spaces, or empty for none.
//
// Returns:
//   - string: the message body.
func createMessage(message, title string) string {
	trimmedTitle := strings.TrimSpace(title)
	if trimmedTitle == "" {
		return message
	}

	return trimmedTitle + "\n\n" + message
}
