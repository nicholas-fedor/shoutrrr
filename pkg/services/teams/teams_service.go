package teams

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strings"

	"github.com/nicholas-fedor/shoutrrr/pkg/format"
	"github.com/nicholas-fedor/shoutrrr/pkg/services/standard"
	"github.com/nicholas-fedor/shoutrrr/pkg/types"
)

// MaxSummaryLength defines the maximum length for a notification summary.
const MaxSummaryLength = 20

// TruncatedSummaryLen defines the length for a truncated summary.
const TruncatedSummaryLen = 21

// Service sends notifications to Microsoft Teams.
type Service struct {
	standard.Standard
	Config *Config
	pkr    format.PropKeyResolver
}

// Send delivers a notification message to Microsoft Teams.
func (service *Service) Send(message string, params *types.Params) error {
	config := service.Config
	if err := service.pkr.UpdateConfigFromParams(config, params); err != nil {
		service.Logf("Failed to update params: %v", err)
	}

	return service.doSend(config, message)
}

// Initialize configures the service with a URL and logger.
func (service *Service) Initialize(configURL *url.URL, logger types.StdLogger) error {
	service.SetLogger(logger)
	service.Config = &Config{}
	service.pkr = format.NewPropKeyResolver(service.Config)

	return service.Config.SetURL(configURL)
}

// GetID returns the service identifier.
func (service *Service) GetID() string {
	return Scheme
}

// GetConfigURLFromCustom converts a custom URL to a service URL.
func (service *Service) GetConfigURLFromCustom(customURL *url.URL) (*url.URL, error) {
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

	config, err := ConfigFromWebhookURL(*webhookURL)
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

// doSend sends the notification to Teams using the configured webhook URL.
func (service *Service) doSend(config *Config, message string) error {
	lines := strings.Split(message, "\n")
	sections := make([]section, 0, len(lines))

	for _, line := range lines {
		sections = append(sections, section{Text: line})
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

	res, err := safePost(postURL, payload)
	if err != nil {
		return fmt.Errorf("%w: %s", ErrSendFailed, err.Error())
	}
	defer res.Body.Close() // Move defer after error check

	if res.StatusCode != http.StatusOK {
		return fmt.Errorf("%w: %s", ErrSendFailedStatus, res.Status)
	}

	return nil
}

// safePost performs an HTTP POST with a pre-validated URL.
// Validation is already done; this wrapper isolates the call.
//
//nolint:gosec,noctx // Ignoring G107: Potential HTTP request made with variable url
func safePost(url string, payload []byte) (*http.Response, error) {
	res, err := http.Post(url, "application/json", bytes.NewBuffer(payload))
	if err != nil {
		return nil, fmt.Errorf("making HTTP POST request: %w", err)
	}

	return res, nil
}
