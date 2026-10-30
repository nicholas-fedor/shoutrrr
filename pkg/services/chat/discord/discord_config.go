package discord

import (
	"fmt"
	"net/url"
	"strings"

	"github.com/nicholas-fedor/shoutrrr/pkg/format"
	"github.com/nicholas-fedor/shoutrrr/pkg/services/standard"
	"github.com/nicholas-fedor/shoutrrr/pkg/types"
	"github.com/nicholas-fedor/shoutrrr/pkg/util"
)

// Config holds the settings required for sending Discord notifications.
type Config struct {
	// EnumlessConfig provides the empty enum set, since Discord has no enum
	// fields.
	standard.EnumlessConfig

	// WebhookID is the webhook ID, written as the host of the service URL.
	WebhookID string `url:"host"`
	// Token is the webhook token, written as the userinfo of the service URL.
	Token string `sensitive:"true" url:"user"`
	// Title is the title of the first embed in each request. A message with a
	// title is always sent as embeds.
	Title string `default:"" desc:"The title of the first embedded item" key:"title"`
	// Username replaces the webhook's default username when set.
	Username string `default:"" desc:"Override the webhook default username" key:"username"`
	// Avatar is the URL of an avatar that replaces the webhook's default
	// avatar when set.
	Avatar string `default:"" desc:"Override the webhook default avatar with specified URL" key:"avatar,avatarurl"`
	// ThreadID is the thread to post the message in. It is sent as the
	// thread_id query parameter of the webhook URL.
	ThreadID string `default:"" desc:"The thread ID to send the message to" key:"thread_id"`
	// Color is the embed color for messages without a level.
	Color uint `base:"16" default:"0x50D9ff" desc:"The color of the left border for plain messages" key:"color"`
	// ColorError is the embed color for error messages.
	ColorError uint `base:"16" default:"0xd60510" desc:"The color of the left border for error messages" key:"colorError"`
	// ColorWarn is the embed color for warning messages.
	ColorWarn uint `base:"16" default:"0xffc441" desc:"The color of the left border for warning messages" key:"colorWarn"`
	// ColorInfo is the embed color for info messages.
	ColorInfo uint `base:"16" default:"0x2488ff" desc:"The color of the left border for info messages" key:"colorInfo"`
	// ColorDebug is the embed color for debug messages.
	ColorDebug uint `base:"16" default:"0x7b00ab" desc:"The color of the left border for debug messages" key:"colorDebug"`
	// SplitLines sends each line of a plain message as its own embed.
	SplitLines bool `default:"Yes" desc:"Whether to send each line as a separate embedded item" key:"splitLines"`
	// JSON sends the message as the raw webhook payload instead of as text.
	// The service URL marks it with the /raw path.
	JSON bool `default:"No" desc:"Whether to send the whole message as the JSON payload instead of using it as the 'content' field" key:"json"`
}

// Scheme defines the protocol identifier for this service's configuration URL.
const Scheme = "discord"

// GetURL generates a URL from the current configuration values.
//
// Returns:
//   - *url.URL: the service URL.
func (c *Config) GetURL() *url.URL {
	resolver := format.NewPropKeyResolver(c)

	return c.getURL(&resolver)
}

// LevelColors returns an array of colors indexed by MessageLevel.
//
// Returns:
//   - [types.MessageLevelCount]uint: the embed color for each message level.
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
//
// Parameters:
//   - serviceURL: the service URL.
//
// Returns:
//   - error: the error from reading the URL, such as [ErrMissingToken].
func (c *Config) SetURL(serviceURL *url.URL) error {
	resolver := format.NewPropKeyResolver(c)

	return c.setURL(&resolver, serviceURL)
}

// getURL constructs a URL from configuration using the provided resolver.
//
// Parameters:
//   - resolver: the resolver that builds the query from the config props.
//
// Returns:
//   - *url.URL: the service URL, with the /raw path when JSON is set.
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
//
// Parameters:
//   - resolver: the resolver that sets the config props from the query.
//   - serviceURL: the service URL.
//
// Returns:
//   - error: [ErrIllegalURLArgument] for a path other than /raw,
//     [ErrMissingWebhookID] or [ErrMissingToken] for a missing webhook ID or
//     token, an error wrapping [util.ErrInvalidQuery] for a malformed query, or
//     a query prop error.
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

	query, err := util.ParseURLQuery(serviceURL)
	if err != nil {
		return err
	}

	if err := format.ApplyQuery(resolver, query); err != nil {
		return err
	}

	// A thread ID copied from a client can carry surrounding whitespace.
	c.ThreadID = strings.TrimSpace(c.ThreadID)

	return nil
}

// CreatePostURLFromConfig builds a POST URL from the Discord configuration.
// The webhook ID, token, and thread ID are trimmed of surrounding whitespace.
//
// Parameters:
//   - config: the configuration with the webhook ID, token, and thread ID.
//
// Returns:
//   - string: the webhook URL under [HooksBaseURL], with a thread_id query
//     parameter when the config sets a thread ID. It is empty when the config
//     lacks a webhook ID or a token.
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
