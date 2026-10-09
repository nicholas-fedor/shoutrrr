package discord

import (
	"fmt"
	"net/url"
	"strings"

	"github.com/nicholas-fedor/shoutrrr/pkg/format"
	"github.com/nicholas-fedor/shoutrrr/pkg/services/standard"
	"github.com/nicholas-fedor/shoutrrr/pkg/types"
)

// Config holds the settings required for sending Discord notifications.
type Config struct {
	standard.EnumlessConfig

	WebhookID  string `url:"host"`
	Token      string `url:"user"`
	Title      string `default:""    key:"title"`
	Username   string `default:""    desc:"Override the webhook default username"                                                            key:"username"`
	Avatar     string `default:""    desc:"Override the webhook default avatar with specified URL"                                           key:"avatar,avatarurl"`
	Color      uint   `base:"16"     default:"0x50D9ff"                                                                                      desc:"The color of the left border for plain messages"   key:"color"`
	ColorError uint   `base:"16"     default:"0xd60510"                                                                                      desc:"The color of the left border for error messages"   key:"colorError"`
	ColorWarn  uint   `base:"16"     default:"0xffc441"                                                                                      desc:"The color of the left border for warning messages" key:"colorWarn"`
	ColorInfo  uint   `base:"16"     default:"0x2488ff"                                                                                      desc:"The color of the left border for info messages"    key:"colorInfo"`
	ColorDebug uint   `base:"16"     default:"0x7b00ab"                                                                                      desc:"The color of the left border for debug messages"   key:"colorDebug"`
	SplitLines bool   `default:"Yes" desc:"Whether to send each line as a separate embedded item"                                            key:"splitLines"`
	JSON       bool   `default:"No"  desc:"Whether to send the whole message as the JSON payload instead of using it as the 'content' field" key:"json"`
	ThreadID   string `default:""    desc:"The thread ID to send the message to"                                                             key:"thread_id"`
}

// Scheme defines the protocol identifier for this service's configuration URL.
const Scheme = "discord"

// GetURL generates a URL from the current configuration values.
func (c *Config) GetURL() *url.URL {
	resolver := format.NewPropKeyResolver(c)

	return c.getURL(&resolver)
}

// LevelColors returns an array of colors indexed by MessageLevel.
func (c *Config) LevelColors() [types.MessageLevelCount]uint {
	var colors [types.MessageLevelCount]uint

	colors[types.Unknown] = c.Color
	colors[types.Error] = c.ColorError
	colors[types.Warning] = c.ColorWarn
	colors[types.Info] = c.ColorInfo
	colors[types.Debug] = c.ColorDebug

	return colors
}

// SetURL updates the configuration from a URL representation.
func (c *Config) SetURL(serviceURL *url.URL) error {
	resolver := format.NewPropKeyResolver(c)

	return c.setURL(&resolver, serviceURL)
}

// getURL constructs a URL from configuration using the provided resolver.
func (c *Config) getURL(resolver types.ConfigQueryResolver) *url.URL {
	result := &url.URL{
		User:       url.User(c.Token),
		Host:       c.WebhookID,
		Scheme:     Scheme,
		RawQuery:   format.BuildQuery(resolver),
		ForceQuery: false,
	}

	if c.JSON {
		result.Path = "/raw"
	}

	return result
}

// setURL updates the configuration from a URL using the provided resolver.
func (c *Config) setURL(resolver types.ConfigQueryResolver, serviceURL *url.URL) error {
	c.WebhookID = serviceURL.Host
	c.Token = serviceURL.User.Username()

	if serviceURL.Path != "" {
		switch serviceURL.Path {
		case "/raw":
			c.JSON = true
		default:
			return ErrIllegalURLArgument
		}
	}

	if c.WebhookID == "" {
		return ErrMissingWebhookID
	}

	if len(c.Token) < 1 {
		return ErrMissingToken
	}

	for key, vals := range serviceURL.Query() {
		if key == "thread_id" {
			// Trim whitespace from thread_id
			c.ThreadID = strings.TrimSpace(vals[0])

			continue
		}

		if err := resolver.Set(key, vals[0]); err != nil {
			return fmt.Errorf("setting config value for key %s: %w", key, err)
		}
	}

	return nil
}

// CreatePostURLFromConfig builds a POST URL from the Discord configuration.
func CreatePostURLFromConfig(config *Config) string {
	if config.WebhookID == "" || config.Token == "" {
		return "" // Invalid cases are caught in doSend
	}
	// Trim whitespace to prevent malformed URLs
	webhookID := strings.TrimSpace(config.WebhookID)
	token := strings.TrimSpace(config.Token)

	postURL := fmt.Sprintf("%s/%s/%s", HooksBaseURL, webhookID, token)

	query := url.Values{}

	if config.ThreadID != "" {
		// Append thread_id as a query parameter
		query.Set("thread_id", strings.TrimSpace(config.ThreadID))
	}

	if len(query) > 0 {
		return postURL + "?" + query.Encode()
	}

	return postURL
}
