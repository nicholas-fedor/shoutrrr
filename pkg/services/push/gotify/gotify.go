package gotify

import (
	"crypto/tls"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/nicholas-fedor/shoutrrr/pkg/format"
	"github.com/nicholas-fedor/shoutrrr/pkg/services/standard"
	"github.com/nicholas-fedor/shoutrrr/pkg/types"
	"github.com/nicholas-fedor/shoutrrr/pkg/util/jsonclient"
)

const (
	// HTTPTimeout defines the HTTP client timeout in seconds.
	HTTPTimeout = 10
	TokenLength = 15
	// TokenChars specifies the valid characters for a Gotify token.
	TokenChars = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789.-_"
)

// ErrInvalidToken indicates an invalid Gotify token format or content.
var ErrInvalidToken = errors.New("invalid gotify token")

// Service implements a Gotify notification service.
type Service struct {
	standard.Standard
	Config     *Config
	pkr        format.PropKeyResolver
	httpClient *http.Client
	client     jsonclient.Client
}

// Initialize configures the service with a URL and logger.
//
//nolint:gosec
func (service *Service) Initialize(configURL *url.URL, logger types.StdLogger) error {
	service.SetLogger(logger)
	service.Config = &Config{
		Title: "Shoutrrr notification",
	}
	service.pkr = format.NewPropKeyResolver(service.Config)

	err := service.Config.SetURL(configURL)
	if err != nil {
		return err
	}

	service.httpClient = &http.Client{
		Transport: &http.Transport{
			TLSClientConfig: &tls.Config{
				// InsecureSkipVerify disables TLS certificate verification when true.
				// This is set to Config.DisableTLS to support HTTP or self-signed certificate setups,
				// but it reduces security by allowing potential man-in-the-middle attacks.
				InsecureSkipVerify: service.Config.DisableTLS,
			},
		},
		Timeout: HTTPTimeout * time.Second,
	}
	if service.Config.DisableTLS {
		service.Log("Warning: TLS verification is disabled, making connections insecure")
	}

	service.client = jsonclient.NewWithHTTPClient(service.httpClient)

	return nil
}

// GetID returns the identifier for this service.
func (service *Service) GetID() string {
	return Scheme
}

// isTokenValid checks if a Gotify token meets length and character requirements.
// Rules are based on Gotify's token validation logic.
func isTokenValid(token string) bool {
	if len(token) != TokenLength || token[0] != 'A' {
		return false
	}

	for _, c := range token {
		if !strings.ContainsRune(TokenChars, c) {
			return false
		}
	}

	return true
}

// buildURL constructs the Gotify API URL with scheme, host, path, and token.
func buildURL(config *Config) (string, error) {
	token := config.Token
	if !isTokenValid(token) {
		return "", fmt.Errorf("%w: %q", ErrInvalidToken, token)
	}

	scheme := "https"
	if config.DisableTLS {
		scheme = "http" // Use HTTP if TLS is disabled
	}

	return fmt.Sprintf("%s://%s%s/message?token=%s", scheme, config.Host, config.Path, token), nil
}

// Send delivers a notification message to Gotify.
func (service *Service) Send(message string, params *types.Params) error {
	if params == nil {
		params = &types.Params{}
	}

	config := service.Config
	if err := service.pkr.UpdateConfigFromParams(config, params); err != nil {
		service.Logf("Failed to update params: %v", err)
	}

	postURL, err := buildURL(config)
	if err != nil {
		return err
	}

	request := &messageRequest{
		Message:  message,
		Title:    config.Title,
		Priority: config.Priority,
	}
	response := &messageResponse{}

	err = service.client.Post(postURL, request, response)
	if err != nil {
		errorRes := &responseError{}
		if service.client.ErrorResponse(err, errorRes) {
			return errorRes
		}

		return fmt.Errorf("failed to send notification to Gotify: %w", err)
	}

	return nil
}

// GetHTTPClient returns the HTTP client for testing purposes.
func (service *Service) GetHTTPClient() *http.Client {
	return service.httpClient
}
