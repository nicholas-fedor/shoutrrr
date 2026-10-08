package matrix

import (
	"context"
	"fmt"
	"net/http"
	"net/url"
	"strings"

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

// Send delivers a notification message to Matrix rooms.
func (s *Service) Send(message string, params *types.Params) error {
	return s.SendWithContext(context.Background(), message, params)
}

// SendWithContext delivers a notification message to Matrix rooms with the provided context.
func (s *Service) SendWithContext(ctx context.Context, message string, params *types.Params) error {
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
