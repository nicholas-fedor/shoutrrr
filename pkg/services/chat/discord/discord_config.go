package discord

import (
	"fmt"
	"net/url"
	"strings"

	"github.com/nicholas-fedor/shoutrrr/pkg/format"
	"github.com/nicholas-fedor/shoutrrr/pkg/services/standard"
	"github.com/nicholas-fedor/shoutrrr/pkg/types"
)

// Scheme defines the protocol identifier for this service's configuration URL.
const Scheme = "discord"

// Config holds the settings required for sending Discord notifications.
type Config struct {
	standard.EnumlessConfig

	WebhookID  string `url:"host"`
	Token      string `url:"user"`
	Title      string `           default:""         key:"title"`
	Username   string `           default:""         key:"username"         desc:"Override the webhook default username"`
	Avatar     string `           default:""         key:"avatar,avatarurl" desc:"Override the webhook default avatar with specified URL"`
	Color      uint   `           default:"0x50D9ff" key:"color"            desc:"The color of the left border for plain messages"                                                  base:"16"`
	ColorError uint   `           default:"0xd60510" key:"colorError"       desc:"The color of the left border for error messages"                                                  base:"16"`
	ColorWarn  uint   `           default:"0xffc441" key:"colorWarn"        desc:"The color of the left border for warning messages"                                                base:"16"`
	ColorInfo  uint   `           default:"0x2488ff" key:"colorInfo"        desc:"The color of the left border for info messages"                                                   base:"16"`
	ColorDebug uint   `           default:"0x7b00ab" key:"colorDebug"       desc:"The color of the left border for debug messages"                                                  base:"16"`
	SplitLines bool   `           default:"Yes"      key:"splitLines"       desc:"Whether to send each line as a separate embedded item"`
	JSON       bool   `           default:"No"       key:"json"             desc:"Whether to send the whole message as the JSON payload instead of using it as the 'content' field"`
	ThreadID   string `           default:""         key:"thread_id"        desc:"The thread ID to send the message to"`
}

// LevelColors returns an array of colors indexed by MessageLevel.
func (config *Config) LevelColors() [types.MessageLevelCount]uint {
	var colors [types.MessageLevelCount]uint

	colors[types.Unknown] = config.Color
	colors[types.Error] = config.ColorError
	colors[types.Warning] = config.ColorWarn
	colors[types.Info] = config.ColorInfo
	colors[types.Debug] = config.ColorDebug

	return colors
}

// GetURL generates a URL from the current configuration values.
func (config *Config) GetURL() *url.URL {
	resolver := format.NewPropKeyResolver(config)

	return config.getURL(&resolver)
}

// SetURL updates the configuration from a URL representation.
func (config *Config) SetURL(url *url.URL) error {
	resolver := format.NewPropKeyResolver(config)

	return config.setURL(&resolver, url)
}

// getURL constructs a URL from configuration using the provided resolver.
func (config *Config) getURL(resolver types.ConfigQueryResolver) *url.URL {
	url := &url.URL{
		User:       url.User(config.Token),
		Host:       config.WebhookID,
		Scheme:     Scheme,
		RawQuery:   format.BuildQuery(resolver),
		ForceQuery: false,
	}

	if config.JSON {
		url.Path = "/raw"
	}

	return url
}

// setURL updates the configuration from a URL using the provided resolver.
func (config *Config) setURL(resolver types.ConfigQueryResolver, url *url.URL) error {
	config.WebhookID = url.Host
	config.Token = url.User.Username()

	if len(url.Path) > 0 {
		switch url.Path {
		case "/raw":
			config.JSON = true
		default:
			return ErrIllegalURLArgument
		}
	}

	if config.WebhookID == "" {
		return ErrMissingWebhookID
	}

	if len(config.Token) < 1 {
		return ErrMissingToken
	}

	for key, vals := range url.Query() {
		if key == "thread_id" {
			// Trim whitespace from thread_id
			config.ThreadID = strings.TrimSpace(vals[0])

			continue
		}

		if err := resolver.Set(key, vals[0]); err != nil {
			return fmt.Errorf("setting config value for key %s: %w", key, err)
		}
	}

	return nil
}

// CreateAPIURLFromConfig builds a POST URL from the Discord configuration.
func CreateAPIURLFromConfig(config *Config) string {
	if config.WebhookID == "" || config.Token == "" {
		return "" // Invalid cases are caught in doSend
	}
	// Trim whitespace to prevent malformed URLs
	webhookID := strings.TrimSpace(config.WebhookID)
	token := strings.TrimSpace(config.Token)

	baseURL := fmt.Sprintf("%s/%s/%s", HooksBaseURL, webhookID, token)

	query := url.Values{}

	if config.ThreadID != "" {
		// Append thread_id as a query parameter
		query.Set("thread_id", strings.TrimSpace(config.ThreadID))
	}

	if len(query) > 0 {
		return baseURL + "?" + query.Encode()
	}

	return baseURL
}
