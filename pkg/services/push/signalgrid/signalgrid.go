package signalgrid

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/nicholas-fedor/shoutrrr/pkg/format"
	"github.com/nicholas-fedor/shoutrrr/pkg/services/standard"
	"github.com/nicholas-fedor/shoutrrr/pkg/types"
)

const (
	apiURL             = "https://api.signalgrid.co/v1/push"
	contentType        = "application/x-www-form-urlencoded"
	defaultHTTPTimeout = 10 * time.Second
)

type Service struct {
	standard.Standard

	Config     *Config
	pkr        format.PropKeyResolver
	Client     *http.Client
	httpClient types.HTTPClient
}

func (s *Service) GetID() string {
	return Scheme
}

func (s *Service) Initialize(
	serviceURL *url.URL,
	logger types.StdLogger,
) error {
	s.SetLogger(logger)

	s.Config = &Config{}

	s.pkr = format.NewPropKeyResolver(s.Config)

	if s.Client == nil {
		s.Client = &http.Client{
			Timeout: defaultHTTPTimeout,
		}
	}

	if err := s.Config.setURL(&s.pkr, serviceURL); err != nil {
		return err
	}

	return nil
}

func (s *Service) Send(
	message string,
	params *types.Params,
) error {
	config := s.Config

	if err := s.pkr.UpdateConfigFromParams(config, params); err != nil {
		return fmt.Errorf(
			"updating config from params: %w",
			err,
		)
	}

	return s.send(message, config)
}

func (s *Service) SetHTTPClient(client types.HTTPClient) {
	s.httpClient = client
}

func (s *Service) httpClientOrDefault() types.HTTPClient {
	if s.httpClient != nil {
		return s.httpClient
	}

	return s.Client
}

func (s *Service) send(
	message string,
	config *Config,
) error {
	data := url.Values{}

	data.Set("client_key", config.ClientKey)
	data.Set("channel", config.Channel)
	data.Set("body", message)

	notificationType := config.Type
	if notificationType == "" {
		notificationType = "INFO"
	}

	data.Set("type", notificationType)

	if config.Title != "" {
		data.Set("title", config.Title)
	} else {
		data.Set("title", "Shoutrrr")
	}

	if config.Critical {
		data.Set("critical", "true")
	}

	ctx, cancel := context.WithTimeout(
		context.Background(),
		defaultHTTPTimeout,
	)
	defer cancel()

	req, err := http.NewRequestWithContext(
		ctx,
		http.MethodPost,
		apiURL,
		strings.NewReader(data.Encode()),
	)
	if err != nil {
		return fmt.Errorf(
			"creating Signalgrid request: %w",
			err,
		)
	}

	req.Header.Set("Content-Type", contentType)
	req.Header.Set(
		"User-Agent",
		"Shoutrrr-Signalgrid/1.0",
	)

	client := s.httpClientOrDefault()

	res, err := client.Do(req)
	if err != nil {
		return fmt.Errorf(
			"sending request to Signalgrid API: %w",
			err,
		)
	}
	defer func() {
		_ = res.Body.Close()
	}()

	responseBody, _ := io.ReadAll(res.Body)

	if res.StatusCode < 200 || res.StatusCode >= 300 {
		return fmt.Errorf(
			"Signalgrid API returned %s: %s",
			res.Status,
			string(responseBody),
		)
	}

	return nil
}
