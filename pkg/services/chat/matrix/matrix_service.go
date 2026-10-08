package matrix

import (
	"context"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/nicholas-fedor/shoutrrr/pkg/format"
	"github.com/nicholas-fedor/shoutrrr/pkg/services/standard"
	"github.com/nicholas-fedor/shoutrrr/pkg/types"
)

// Service sends notifications via the Matrix protocol.
type Service struct {
	standard.Standard

	Config     *Config
	client     *client
	pkr        format.PropKeyResolver
	httpClient types.HTTPClient
	// loginGate holds one token while a send runs the password login, so only one
	// login runs at a time and waiting sends can give up when their context ends.
	loginGate chan struct{}
	// loggedIn reports whether the client holds an access token from a password login.
	loggedIn bool
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
func (s *Service) GetID() string {
	return Scheme
}

// Initialize configures the service with a URL and logger. It performs no network
// I/O: a password login runs on the first send, through the HTTP client and
// context in effect at that time.
func (s *Service) Initialize(serviceURL *url.URL, logger types.StdLogger) error {
	s.SetLogger(logger)
	s.Config = &Config{
		EnumlessConfig: standard.EnumlessConfig{},
		User:           "",
		Password:       "",
		DisableTLS:     false,
		Host:           "",
		Rooms:          nil,
		Title:          "",
	}
	s.pkr = format.NewPropKeyResolver(s.Config)

	if err := s.Config.setURL(&s.pkr, serviceURL); err != nil {
		return err
	}

	s.client = nil
	s.loggedIn = false
	s.loginGate = make(chan struct{}, 1)

	// The dummy.com placeholder URL, which config validation also accepts without
	// credentials, configures the service without a client.
	if serviceURL.Hostname() == "dummy.com" || serviceURL.Host == "" {
		return nil
	}

	s.client = newClient(s.Config.Host, s.Config.DisableTLS, logger)
	s.client.httpClient = adaptHTTPClient(s.httpClient)

	if s.Config.User == "" {
		s.client.useToken(s.Config.Password)
	}

	return nil
}

// Send delivers a notification message to Matrix rooms without a deadline of its own.
//
// Parameters:
//   - message: the message to send.
//   - params: per-send parameters, applied to this send only.
//
// Returns:
//   - error: the login or send failure, or nil on success.
func (s *Service) Send(message string, params *types.Params) error {
	return s.SendContext(context.Background(), message, params)
}

// SendContext delivers a notification message to Matrix rooms. The router calls it
// with its send deadline. The first send logs in when a user is configured.
//
// Parameters:
//   - ctx: bounds the login and the room requests.
//   - message: the message to send.
//   - params: per-send parameters, applied to this send only.
//
// Returns:
//   - error: the login or send failure, or nil on success.
func (s *Service) SendContext(ctx context.Context, message string, params *types.Params) error {
	if s.client == nil {
		return ErrClientNotInitialized
	}

	if err := s.ensureLogin(ctx); err != nil {
		return err
	}

	// Make a per-call copy of the config to avoid mutating the shared s.Config
	cfg := *s.Config

	if err := s.pkr.UpdateConfigFromParams(&cfg, params); err != nil {
		return fmt.Errorf("updating config from params: %w", err)
	}

	// Create message with title if provided
	fullMessage := createMessage(message, cfg.Title)

	sendErrors := s.client.sendMessage(ctx, fullMessage, cfg.Rooms)
	if len(sendErrors) > 0 {
		for _, err := range sendErrors {
			s.Logf("error sending message: %v", err)
		}

		return fmt.Errorf(
			"%v error(s) sending message, with initial error: %w",
			len(sendErrors),
			sendErrors[0],
		)
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

// ServiceTimeout reports the send budget the router gives Matrix. A send makes
// sequential requests, each bounded by the client's request timeout: two for the
// password login when a user is configured, then a join and a message for each
// configured room. Without configured rooms, the send looks up the joined rooms
// and is budgeted for one message, so sends to many joined rooms need configured
// rooms or a longer router timeout.
//
// Parameters:
//   - params: per-send parameters, which may set the rooms.
//
// Returns:
//   - time.Duration: the request timeout multiplied by the number of requests.
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

// SetHTTPClient sets a custom HTTP client for the service (propagated to internal client).
// A nil client restores the default client. An access token from an earlier login
// stays valid.
func (s *Service) SetHTTPClient(client types.HTTPClient) {
	if c, ok := client.(*http.Client); ok && c == nil {
		client = nil
	}

	s.httpClient = client
	if s.client != nil {
		s.client.httpClient = adaptHTTPClient(client)
	}
}

// ensureLogin logs in with the configured user and password once, on the first
// send. A failed login is retried on the next send. A send that waits for another
// send's login stops waiting when ctx ends.
//
// Parameters:
//   - ctx: cancellation for waiting on and running the login.
//
// Returns:
//   - error: the login failure or ctx's error, or nil when no login is needed.
func (s *Service) ensureLogin(ctx context.Context) error {
	if s.Config.User == "" {
		return nil
	}

	select {
	case s.loginGate <- struct{}{}:
	case <-ctx.Done():
		return fmt.Errorf("waiting for Matrix login: %w", ctx.Err())
	}

	defer func() { <-s.loginGate }()

	if s.loggedIn {
		return nil
	}

	if err := s.client.login(ctx, s.Config.User, s.Config.Password); err != nil {
		return fmt.Errorf("logging in to Matrix: %w", err)
	}

	s.loggedIn = true

	return nil
}

// createMessage creates the full message body by prepending the title if provided.
// Format: If title is "Alert" and message is "Hello", output is "Alert\n\nHello".
func createMessage(message, title string) string {
	trimmedTitle := strings.TrimSpace(title)
	if trimmedTitle == "" {
		return message
	}

	return trimmedTitle + "\n\n" + message
}
