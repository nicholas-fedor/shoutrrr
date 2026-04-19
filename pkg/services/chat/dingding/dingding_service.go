package dingding

import (
	"bytes"
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"time"

	"github.com/nicholas-fedor/shoutrrr/pkg/format"
	"github.com/nicholas-fedor/shoutrrr/pkg/services/standard"
	"github.com/nicholas-fedor/shoutrrr/pkg/types"
)

// HTTPClient defines the interface for HTTP operations.
type HTTPClient interface {
	Do(req *http.Request) (*http.Response, error)
}

// DefaultHTTPClient is the default implementation using http.DefaultClient with timeout.
type DefaultHTTPClient struct {
	client *http.Client
}

// Service sends notifications to a pre-configured Dingding channel or user.
type Service struct {
	standard.Standard

	Config     *Config
	pkr        format.PropKeyResolver
	HTTPClient HTTPClient

	apiToken       string
	apiTokenExpiry time.Time
}

// defaultHTTPTimeout is the default timeout for HTTP requests to Dingding.
const defaultHTTPTimeout = 30 * time.Second

// NewDefaultHTTPClient creates a new default HTTP client with a reasonable timeout.
func NewDefaultHTTPClient() *DefaultHTTPClient {
	return &DefaultHTTPClient{
		client: &http.Client{
			Timeout: defaultHTTPTimeout,
		},
	}
}

// Do performs the HTTP request.
func (c *DefaultHTTPClient) Do(req *http.Request) (*http.Response, error) {
	resp, err := c.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("performing HTTP request: %w", err)
	}

	return resp, nil
}

// GetID returns the identifier for this service.
func (s *Service) GetID() string {
	return Scheme
}

// Initialize configures the service with a URL and logger.
func (s *Service) Initialize(serviceURL *url.URL, logger types.StdLogger) error {
	s.SetLogger(logger)
	s.Config = &Config{}
	s.pkr = format.NewPropKeyResolver(s.Config)
	s.HTTPClient = NewDefaultHTTPClient()
	s.apiToken = ""
	s.apiTokenExpiry = time.Time{}

	if err := s.pkr.SetDefaultProps(s.Config); err != nil {
		return fmt.Errorf("setting default properties: %w", err)
	}

	if err := s.Config.SetURL(serviceURL); err != nil {
		return err
	}

	return nil
}

// Send delivers a notification message to Dingding.
func (s *Service) Send(message string, params *types.Params) error {
	return s.SendWithContext(context.Background(), message, params)
}

// SendWithContext delivers a notification message to Dingding with context support.
func (s *Service) SendWithContext(ctx context.Context, message string, params *types.Params) error {
	// Clone the config to avoid modifying the original for this send operation.
	config := s.Config.Clone()

	// Use PropKeyResolver for consistent param handling
	if err := s.pkr.UpdateConfigFromParams(config, params); err != nil {
		return fmt.Errorf("updating config from params: %w", err)
	}

	return s.doSend(ctx, config, message)
}

// 自定义机器人的脱裤子放屁式签名
func (s *Service) makeCustomBotSignature() (string, string) {
	unixMillis := time.Now().UnixMilli()
	toSign := fmt.Sprintf("%d\n%s", unixMillis, s.Config.Secret)
	h := hmac.New(sha256.New, []byte(s.Config.Secret))
	h.Write([]byte(toSign))
	signatureBytes := h.Sum(nil)
	signatureBase64ed := base64.StdEncoding.EncodeToString(signatureBytes)

	return fmt.Sprintf("%d", unixMillis), signatureBase64ed
}

const defaultAPIEndpoint = "api.dingtalk.com"

func (s *Service) getOpenAPIToken(ctx context.Context) (string, error) {
	if s.apiToken != "" && time.Until(s.apiTokenExpiry) > 1*time.Minute {
		return s.apiToken, nil
	}

	apiEndpoint := defaultAPIEndpoint
	if s.Config.APIEndpoint != "" {
		apiEndpoint = s.Config.APIEndpoint
	}

	payloadJSON, err := json.Marshal(map[string]string{
		"appKey":    s.Config.AccessToken,
		"appSecret": s.Config.Secret,
	})
	if err != nil {
		return "", fmt.Errorf("marshaling auth payload to JSON: %w", err)
	}

	payload := bytes.NewReader(payloadJSON)
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, "https://"+apiEndpoint+"/v1.0/oauth2/accessToken", payload)
	if err != nil {
		return "", fmt.Errorf("creating auth request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")

	res, err := s.HTTPClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("performing auth request: %w", err)
	}
	defer res.Body.Close()

	if res.StatusCode != http.StatusOK {
		resBytes, _ := io.ReadAll(res.Body)
		var resp struct {
			Code    string `json:"code"`
			Message string `json:"message"`
		}
		_ = json.Unmarshal(resBytes, &resp)

		return "", fmt.Errorf("received non-successful response status code from Dingding API during auth: %d: %s: %s", res.StatusCode, resp.Code, resp.Message)
	}

	var authResp struct {
		Code        string `json:"code"`
		RequestID   string `json:"requestid"`
		Message     string `json:"message"`
		ExpireIn    int    `json:"expireIn"`
		AccessToken string `json:"accessToken"`
	}

	if err := json.NewDecoder(res.Body).Decode(&authResp); err != nil {
		return "", fmt.Errorf("decoding auth response body: %w", err)
	}

	if authResp.Code != "" || authResp.AccessToken == "" {
		return "", fmt.Errorf("Dingding API auth error: %s: %s", authResp.Code, authResp.Message)
	}

	s.apiToken = authResp.AccessToken
	s.apiTokenExpiry = time.Now().Add(time.Duration(authResp.ExpireIn) * time.Second)

	return authResp.AccessToken, nil
}

type dingdingCustomBotResp struct {
	ErrCode int    `json:"errcode"`
	ErrMsg  string `json:"errmsg"`
}

type dingdingWorkNoticeResp struct {
	RequestID                 string   `json:"requestid"`
	Code                      string   `json:"code"`
	Message                   string   `json:"message"`
	FlowControlledStaffIdList []string `json:"flowControlledStaffIdList"`
	InvalidStaffIdList        []string `json:"invalidStaffIdList"`
	ProcessQueryKey           string   `json:"processQueryKey"`
}

// doSend sends the notification to Dingding using the configured API URL.
func (s *Service) doSend(ctx context.Context, config *Config, message string) error {
	var req *http.Request
	switch config.Kind {
	case "custombot":
		query := url.Values{}
		query.Set("access_token", config.AccessToken)
		if config.Secret != "" {
			timestamp, sign := s.makeCustomBotSignature()
			query.Set("timestamp", timestamp)
			query.Set("sign", sign)
		}

		apiURL := "https://oapi.dingtalk.com/robot/send?" + query.Encode()

		payload, err := config.createPayload(config.Title, message)
		if err != nil {
			return err
		}

		req, err = http.NewRequestWithContext(
			ctx,
			http.MethodPost,
			apiURL,
			bytes.NewReader(payload),
		)
		if err != nil {
			return fmt.Errorf("creating request: %w", err)
		}

		req.Header.Set("Content-Type", "application/json")
	case "worknotice":
		token, err := s.getOpenAPIToken(ctx)
		if err != nil {
			return fmt.Errorf("getting API token: %w", err)
		}

		payload, err := config.createPayload(config.Title, message)
		if err != nil {
			return err
		}

		apiEndpoint := defaultAPIEndpoint
		if config.APIEndpoint != "" {
			apiEndpoint = config.APIEndpoint
		}

		req, err = http.NewRequestWithContext(
			ctx,
			http.MethodPost,
			"https://"+apiEndpoint+"/v1.0/robot/oToMessages/batchSend",
			bytes.NewReader(payload),
		)
		if err != nil {
			return fmt.Errorf("creating request: %w", err)
		}

		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("x-acs-dingtalk-access-token", token)
	default:
		return ErrInvalidKind
	}

	res, err := s.HTTPClient.Do(req)
	if err != nil {
		return fmt.Errorf("making HTTP POST request: %w", err)
	}

	defer res.Body.Close()

	switch config.Kind {
	case "custombot":
		// always 200, no need to check status code
		var resp dingdingCustomBotResp
		if err := json.NewDecoder(res.Body).Decode(&resp); err != nil {
			return fmt.Errorf("decoding response body: %w", err)
		}

		if resp.ErrCode != 0 {
			return fmt.Errorf("dingding API error: %d: %s", resp.ErrCode, resp.ErrMsg)
		}
	case "worknotice":
		var resp dingdingWorkNoticeResp
		bodyBytes, err := io.ReadAll(res.Body)
		if err != nil {
			return fmt.Errorf("reading response body: %w", err)
		}

		if err := json.Unmarshal(bodyBytes, &resp); err != nil {
			return fmt.Errorf("decoding response body: %w", err)
		}

		if res.StatusCode != http.StatusOK {
			return fmt.Errorf("failed to send dingding message: %s: %s", resp.Code, resp.Message)
		}
	default:
		return ErrInvalidKind
	}
	return nil
}
