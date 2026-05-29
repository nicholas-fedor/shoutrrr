package teams

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/nicholas-fedor/shoutrrr/pkg/format"
	"github.com/nicholas-fedor/shoutrrr/pkg/services/standard"
	"github.com/nicholas-fedor/shoutrrr/pkg/types"
)

// HTTPClient defines the interface for HTTP operations.
type HTTPClient interface {
	Do(req *http.Request) (*http.Response, error)
}

// defaultHTTPClient implements HTTPClient using http.Client with a timeout.
type defaultHTTPClient struct {
	client *http.Client
}

// Service sends notifications to Microsoft Teams via Power Automate workflow webhooks.
type Service struct {
	standard.Standard

	Config     *Config
	pkr        format.PropKeyResolver
	httpClient HTTPClient
}

// defaultHTTPTimeout is the default timeout for HTTP requests.
const defaultHTTPTimeout = 30 * time.Second

// adaptiveCardVersion is the Adaptive Card schema version used in payloads.
const adaptiveCardVersion = "1.5"

// Do performs the HTTP request.
func (c *defaultHTTPClient) Do(req *http.Request) (*http.Response, error) {
	resp, err := c.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("performing HTTP request: %w", err)
	}

	return resp, nil
}

// GetHTTPClient returns the service's HTTP client for testing purposes.
func (s *Service) GetHTTPClient() HTTPClient {
	return s.httpClient
}

// GetID returns the service identifier.
func (s *Service) GetID() string {
	return Scheme
}

// Initialize configures the service with a URL and logger.
func (s *Service) Initialize(serviceURL *url.URL, logger types.StdLogger) error {
	s.SetLogger(logger)

	s.Config = &Config{}
	s.pkr = format.NewPropKeyResolver(s.Config)
	s.httpClient = &defaultHTTPClient{
		client: &http.Client{Timeout: defaultHTTPTimeout},
	}

	if err := s.pkr.SetDefaultProps(s.Config); err != nil {
		return fmt.Errorf("setting default properties: %w", err)
	}

	return s.Config.SetURL(serviceURL)
}

// Send delivers a notification message to Microsoft Teams.
func (s *Service) Send(message string, params *types.Params) error {
	if s.Config == nil {
		return ErrMissingHost
	}

	config := *s.Config
	if err := s.pkr.UpdateConfigFromParams(&config, params); err != nil {
		return fmt.Errorf("updating config from params: %w", err)
	}

	return s.doSend(&config, message)
}

// SetHTTPClient sets the HTTP client for testing purposes.
func (s *Service) SetHTTPClient(client HTTPClient) {
	s.httpClient = client
}

// doSend sends the notification to Teams as an Adaptive Card payload.
func (s *Service) doSend(config *Config, message string) error {
	if config.Host == "" {
		return ErrMissingHost
	}

	if err := ValidateWebhookURL(config.Host); err != nil {
		return err
	}

	lines := strings.Split(message, "\n")
	body := make([]adaptiveBlock, 0, len(lines)+1)

	if config.Title != "" {
		//nolint:exhaustruct // Weight, Size, Wrap are optional and default to zero values
		body = append(body, adaptiveBlock{
			Type:   "TextBlock",
			Text:   config.Title,
			Weight: "Bolder",
			Size:   "Medium",
		})
	}

	for _, line := range lines {
		//nolint:exhaustruct // Weight, Size are optional and default to zero values
		body = append(body, adaptiveBlock{
			Type: "TextBlock",
			Text: line,
			Wrap: true,
		})
	}

	payload := adaptivePayload{
		Type: "message",
		Attachments: []adaptiveAttachment{
			{
				ContentType: "application/vnd.microsoft.card.adaptive",
				ContentURL:  nil,
				Content: adaptiveCardContent{
					Schema:  "http://adaptivecards.io/schemas/adaptive-card.json",
					Type:    "AdaptiveCard",
					Version: adaptiveCardVersion,
					Body:    body,
				},
			},
		},
	}

	jsonBytes, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("marshaling payload to JSON: %w", err)
	}

	res, err := s.postJSON(config.Host, jsonBytes)
	if err != nil {
		return fmt.Errorf("%w: %s", ErrSendFailed, err.Error())
	}

	defer func() { _ = res.Body.Close() }()

	if res.StatusCode != http.StatusOK {
		return fmt.Errorf("%w: %s", ErrSendFailedStatus, res.Status)
	}

	return nil
}

// postJSON performs an HTTP POST with a JSON payload.
func (s *Service) postJSON(serviceURL string, payload []byte) (*http.Response, error) {
	ctx, cancel := context.WithTimeout(
		context.Background(),
		defaultHTTPTimeout,
	)
	defer cancel()

	req, err := http.NewRequestWithContext(
		ctx,
		http.MethodPost,
		serviceURL,
		bytes.NewBuffer(payload),
	)
	if err != nil {
		return nil, fmt.Errorf("creating request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")

	res, err := s.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("making HTTP POST request: %w", err)
	}

	return res, nil
}
