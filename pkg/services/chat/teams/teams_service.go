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

// Service sends notifications to Microsoft Teams.
type Service struct {
	standard.Standard

	Config     *Config
	pkr        format.PropKeyResolver
	httpClient *http.Client
}

// defaultHTTPTimeout is the default timeout for HTTP requests.
const defaultHTTPTimeout = 30 * time.Second

// MaxSummaryLength defines the maximum length for a notification summary.
const MaxSummaryLength = 20

// TruncatedSummaryLen defines the length for a truncated summary.
const TruncatedSummaryLen = 21

// GetHTTPClient returns the service's HTTP client for testing purposes.
func (s *Service) GetHTTPClient() *http.Client {
	return s.httpClient
}

// GetID returns the service identifier.
func (s *Service) GetID() string {
	return Scheme
}

// GetServiceURLFromCustom converts a custom URL to a service URL.
func (s *Service) GetServiceURLFromCustom(customURL *url.URL) (*url.URL, error) {
	webhookURLStr := strings.TrimPrefix(customURL.String(), "teams+")

	tempURL, err := url.Parse(webhookURLStr)
	if err != nil {
		return nil, fmt.Errorf("parsing custom URL %q: %w", webhookURLStr, err)
	}

	webhookURL := &url.URL{
		Scheme: tempURL.Scheme,
		Host:   tempURL.Host,
		Path:   tempURL.Path,
	}

	config, err := ConfigFromWebhookURL(webhookURL)
	if err != nil {
		return nil, err
	}

	config.Color = ""
	config.Title = ""

	query := customURL.Query()
	for key, vals := range query {
		if vals[0] != "" {
			switch key {
			case "color":
				config.Color = vals[0]
			case "host":
				config.Host = vals[0]
			case "title":
				config.Title = vals[0]
			}
		}
	}

	return config.GetURL(), nil
}

// Initialize configures the service with a URL and logger.
func (s *Service) Initialize(serviceURL *url.URL, logger types.StdLogger) error {
	s.SetLogger(logger)
	s.Config = &Config{}
	s.pkr = format.NewPropKeyResolver(s.Config)

	s.httpClient = &http.Client{Timeout: defaultHTTPTimeout}

	return s.Config.SetURL(serviceURL)
}

// Send delivers a notification message to Microsoft Teams.
func (s *Service) Send(message string, params *types.Params) error {
	config := s.Config
	if err := s.pkr.UpdateConfigFromParams(config, params); err != nil {
		s.Logf("Failed to update params: %v", err)
	}

	return s.doSend(config, message)
}

// doSend sends the notification to Teams using the configured webhook URL.
func (s *Service) doSend(config *Config, message string) error {
	lines := strings.Split(message, "\n")
	sections := make([]section, 0, len(lines))

	for _, line := range lines {
		sections = append(sections, section{
			Text:             line,
			ActivityTitle:    "",
			ActivitySubtitle: "",
			ActivityImage:    "",
			Facts:            nil,
			Images:           nil,
			Actions:          nil,
			HeroImage:        nil,
		})
	}

	summary := config.Title
	if summary == "" && len(sections) > 0 {
		summary = sections[0].Text
		if len(summary) > MaxSummaryLength {
			summary = summary[:TruncatedSummaryLen]
		}
	}

	payload, err := json.Marshal(payload{
		CardType:   "MessageCard",
		Context:    "http://schema.org/extensions",
		Markdown:   true,
		Title:      config.Title,
		ThemeColor: config.Color,
		Summary:    summary,
		Sections:   sections,
	})
	if err != nil {
		return fmt.Errorf("marshaling payload to JSON: %w", err)
	}

	if config.Host == "" {
		return ErrMissingHost
	}

	postURL := BuildWebhookURL(
		config.Host,
		config.Group,
		config.Tenant,
		config.AltID,
		config.GroupOwner,
		config.ExtraID,
	)

	// Validate URL before sending
	if err := ValidateWebhookURL(postURL); err != nil {
		return err
	}

	res, err := s.postJSON(postURL, payload)
	if err != nil {
		return fmt.Errorf("%w: %s", ErrSendFailed, err.Error())
	}

	defer func() { _ = res.Body.Close() }() // Move defer after error check

	if res.StatusCode != http.StatusOK {
		return fmt.Errorf("%w: %s", ErrSendFailedStatus, res.Status)
	}

	return nil
}

// postJSON performs an HTTP POST with a pre-validated URL using the service's HTTP client.
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
