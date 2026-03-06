package lark

import (
	"bytes"
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"time"

	"github.com/nicholas-fedor/shoutrrr/pkg/format"
	"github.com/nicholas-fedor/shoutrrr/pkg/services/standard"
	"github.com/nicholas-fedor/shoutrrr/pkg/types"
)

// Service sends notifications to Lark.
type Service struct {
	standard.Standard

	Config *Config
	pkr    format.PropKeyResolver
}

// Constants for the Lark service configuration and limits.
const (
	apiFormat   = "https://%s/open-apis/bot/v2/hook/%s" // API endpoint format
	maxLength   = 4096                                  // Maximum message length in bytes
	defaultTime = 30 * time.Second                      // Default HTTP client timeout
)

const (
	larkHost   = "open.larksuite.com"
	feishuHost = "open.feishu.cn"
)

// Error variables for the Lark service.
var (
	ErrInvalidHost = errors.New("invalid host, use 'open.larksuite.com' or 'open.feishu.cn'")
	ErrNoPath      = errors.New(
		"no path, path like 'xxx' in 'https://open.larksuite.com/open-apis/bot/v2/hook/xxx'",
	)
	ErrLargeMessage     = errors.New("message exceeds the max length")
	ErrMissingHost      = errors.New("host is required but not specified in the configuration")
	ErrSendFailed       = errors.New("failed to send notification to Lark")
	ErrInvalidSignature = errors.New("failed to generate valid signature")
)

// httpClient is configured with a default timeout.
//

var httpClient = &http.Client{Timeout: defaultTime}

// GetID returns the service identifier.
func (s *Service) GetID() string {
	return Scheme
}

// Initialize configures the service with a URL and logger.
//

func (s *Service) Initialize(configURL *url.URL, logger types.StdLogger) error {
	s.SetLogger(logger)
	s.Config = &Config{}
	s.pkr = format.NewPropKeyResolver(s.Config)

	return s.Config.SetURL(configURL)
}

// Send delivers a notification message to Lark.
func (s *Service) Send(message string, params *types.Params) error {
	if len(message) > maxLength {
		return ErrLargeMessage
	}

	config := *s.Config
	if err := s.pkr.UpdateConfigFromParams(&config, params); err != nil {
		return fmt.Errorf("updating params: %w", err)
	}

	if config.Host != larkHost && config.Host != feishuHost {
		return ErrInvalidHost
	}

	if config.Path == "" {
		return ErrNoPath
	}

	return s.doSend(&config, message, params)
}

// doSend sends the notification to Lark using the configured API URL.
func (s *Service) doSend(config *Config, message string, params *types.Params) error {
	if config.Host == "" {
		return ErrMissingHost
	}

	postURL := fmt.Sprintf(apiFormat, config.Host, config.Path)

	payload, err := s.preparePayload(message, config, params)
	if err != nil {
		return err
	}

	return s.sendRequest(postURL, payload)
}

// genSign generates a signature for the request using the secret and timestamp.
func (s *Service) genSign(secret string, timestamp int64) (string, error) {
	stringToSign := fmt.Sprintf("%v\n%s", timestamp, secret)

	h := hmac.New(sha256.New, []byte(stringToSign))
	if _, err := h.Write([]byte{}); err != nil {
		return "", fmt.Errorf("%w: computing HMAC: %w", ErrInvalidSignature, err)
	}

	return base64.StdEncoding.EncodeToString(h.Sum(nil)), nil
}

// getRequestBody constructs the request body for the Lark API, supporting rich content via params.
func (s *Service) getRequestBody(
	message, title, secret string,
	params *types.Params,
) *RequestBody {
	body := &RequestBody{}

	if secret != "" {
		ts := time.Now().Unix()
		body.Timestamp = strconv.FormatInt(ts, 10)

		sign, err := s.genSign(secret, ts)
		if err != nil {
			sign = "" // Fallback to empty string on error
		}

		body.Sign = sign
	}

	if title == "" {
		body.MsgType = MsgTypeText
		body.Content.Text = message
	} else {
		body.MsgType = MsgTypePost
		//nolint:exhaustruct // Item fields are populated inline; Link is optional
		content := [][]Item{{{Tag: TagValueTextContent, Text: message}}}

		if params != nil {
			if link, ok := (*params)["link"]; ok && link != "" {
				content = append(
					content,

					[]Item{{Tag: TagValueLink, Text: "More Info", Link: link}},
				)
			}
		}

		//nolint:exhaustruct // Post fields are populated inline; Zh is optional
		body.Content.Post = &Post{
			En: &Message{
				Title:   title,
				Content: content,
			},
		}
	}

	return body
}

// handleResponse processes the API response and checks for errors.
func (s *Service) handleResponse(resp *http.Response) error {
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("%w: unexpected status %s", ErrSendFailed, resp.Status)
	}

	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("reading response body: %w", err)
	}

	var response Response
	if err := json.Unmarshal(data, &response); err != nil {
		return fmt.Errorf("unmarshaling response: %w", err)
	}

	if response.Code != 0 {
		return fmt.Errorf(
			"%w: server returned code %d: %s",
			ErrSendFailed,
			response.Code,
			response.Msg,
		)
	}

	s.Logf(
		"Notification sent successfully to %s/%s",
		s.Config.Host,
		s.Config.Path,
	)

	return nil
}

// preparePayload constructs and marshals the request payload for the Lark API.
func (s *Service) preparePayload(
	message string,
	config *Config,
	params *types.Params,
) ([]byte, error) {
	body := s.getRequestBody(message, config.Title, config.Secret, params)

	data, err := json.Marshal(body)
	if err != nil {
		return nil, fmt.Errorf("marshaling payload to JSON: %w", err)
	}

	s.Logf("Lark Request Body: %s", string(data))

	return data, nil
}

// sendRequest performs the HTTP POST request to the Lark API and handles the response.
//
//nolint:gosec // G704: URL is validated through config validation before calling this function
func (s *Service) sendRequest(postURL string, payload []byte) error {
	req, err := http.NewRequestWithContext(
		context.Background(),
		http.MethodPost,
		postURL,
		bytes.NewReader(payload),
	)
	if err != nil {
		return fmt.Errorf("creating HTTP request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")

	resp, err := httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("%w: making HTTP request: %w", ErrSendFailed, err)
	}

	defer func() { _ = resp.Body.Close() }()

	return s.handleResponse(resp)
}
