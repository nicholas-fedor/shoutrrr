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
var httpClient = &http.Client{Timeout: defaultTime}

// Service sends notifications to Lark.
type Service struct {
	standard.Standard
	Config *Config
	pkr    format.PropKeyResolver
}

// Send delivers a notification message to Lark.
func (service *Service) Send(message string, params *types.Params) error {
	if len(message) > maxLength {
		return ErrLargeMessage
	}

	config := *service.Config
	if err := service.pkr.UpdateConfigFromParams(&config, params); err != nil {
		return fmt.Errorf("updating params: %w", err)
	}

	if config.Host != larkHost && config.Host != feishuHost {
		return ErrInvalidHost
	}

	if config.Path == "" {
		return ErrNoPath
	}

	return service.doSend(config, message, params)
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

// doSend sends the notification to Lark using the configured API URL.
func (service *Service) doSend(config Config, message string, params *types.Params) error {
	if config.Host == "" {
		return ErrMissingHost
	}

	postURL := fmt.Sprintf(apiFormat, config.Host, config.Path)

	payload, err := service.preparePayload(message, config, params)
	if err != nil {
		return err
	}

	return service.sendRequest(postURL, payload)
}

// preparePayload constructs and marshals the request payload for the Lark API.
func (service *Service) preparePayload(
	message string,
	config Config,
	params *types.Params,
) ([]byte, error) {
	body := service.getRequestBody(message, config.Title, config.Secret, params)

	data, err := json.Marshal(body)
	if err != nil {
		return nil, fmt.Errorf("marshaling payload to JSON: %w", err)
	}

	service.Logf("Lark Request Body: %s", string(data))

	return data, nil
}

// sendRequest performs the HTTP POST request to the Lark API and handles the response.
func (service *Service) sendRequest(postURL string, payload []byte) error {
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

	return service.handleResponse(resp)
}

// handleResponse processes the API response and checks for errors.
func (service *Service) handleResponse(resp *http.Response) error {
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

	service.Logf(
		"Notification sent successfully to %s/%s",
		service.Config.Host,
		service.Config.Path,
	)

	return nil
}

// genSign generates a signature for the request using the secret and timestamp.
func (service *Service) genSign(secret string, timestamp int64) (string, error) {
	stringToSign := fmt.Sprintf("%v\n%s", timestamp, secret)

	h := hmac.New(sha256.New, []byte(stringToSign))
	if _, err := h.Write([]byte{}); err != nil {
		return "", fmt.Errorf("%w: computing HMAC: %w", ErrInvalidSignature, err)
	}

	return base64.StdEncoding.EncodeToString(h.Sum(nil)), nil
}

// getRequestBody constructs the request body for the Lark API, supporting rich content via params.
func (service *Service) getRequestBody(
	message, title, secret string,
	params *types.Params,
) *RequestBody {
	body := &RequestBody{}

	if secret != "" {
		ts := time.Now().Unix()
		body.Timestamp = strconv.FormatInt(ts, 10)

		sign, err := service.genSign(secret, ts)
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
		content := [][]Item{{{Tag: TagValueText, Text: message}}}

		if params != nil {
			if link, ok := (*params)["link"]; ok && link != "" {
				content = append(content, []Item{{Tag: TagValueLink, Text: "More Info", Link: link}})
			}
		}

		body.Content.Post = &Post{
			En: &Message{
				Title:   title,
				Content: content,
			},
		}
	}

	return body
}
