package discord

import (
	"net/url"
	"strconv"
	"strings"

	"github.com/nicholas-fedor/shoutrrr/pkg/format"
	"github.com/nicholas-fedor/shoutrrr/pkg/services/standard"
	"github.com/nicholas-fedor/shoutrrr/pkg/types"
	"github.com/nicholas-fedor/shoutrrr/pkg/util"
)

// Config holds the settings required for sending Discord notifications.
//
// Reading a service URL checks the webhook ID and token. Each send checks
// them again, so a Config built in code with an invalid webhook ID or token
// fails before any request, with an error such as [ErrInvalidWebhookID].
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
//   - error: [ErrIllegalURLArgument] for a path other than /raw. An invalid
//     webhook ID or token gives an error such as [ErrInvalidWebhookID]. A
//     malformed query gives an error wrapping [util.ErrInvalidQuery], and an
//     invalid query value gives a query prop error.
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

	if err := c.validate(); err != nil {
		return err
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

// validate checks the webhook ID and token, with surrounding whitespace
// ignored. The webhook ID is a Discord snowflake, a number of up to 64 bits.
// The token can be any text that is not blank.
//
// Returns:
//   - error: [ErrMissingWebhookID] or [ErrMissingToken] for a blank webhook
//     ID or token. [ErrInvalidWebhookID] for a webhook ID that is not a
//     snowflake. Otherwise nil.
func (c *Config) validate() error {
	webhookID := strings.TrimSpace(c.WebhookID)
	if webhookID == "" {
		return ErrMissingWebhookID
	}

	if _, err := strconv.ParseUint(webhookID, 10, 64); err != nil {
		return ErrInvalidWebhookID
	}

	if strings.TrimSpace(c.Token) == "" {
		return ErrMissingToken
	}

	return nil
}

// CreatePostURLFromConfig builds a POST URL from the Discord configuration.
// The webhook ID, token, and thread ID are trimmed of surrounding whitespace.
// The token is path-escaped, so it cannot change the URL's path or start its
// query.
//
// Parameters:
//   - config: the configuration with the webhook ID, token, and thread ID.
//
// Returns:
//   - string: the webhook URL under [HooksBaseURL], with a thread_id query
//     parameter when the config sets a thread ID that is not blank. It is
//     empty when the webhook ID or token is invalid, such as a webhook ID that
//     is not a number.
func CreatePostURLFromConfig(config *Config) string {
	if config.validate() != nil {
		return ""
	}

	// A valid webhook ID is all digits, so only the token needs escaping.
	webhookID := strings.TrimSpace(config.WebhookID)
	token := url.PathEscape(strings.TrimSpace(config.Token))

	threadID := strings.TrimSpace(config.ThreadID)
	if threadID == "" {
		return HooksBaseURL + "/" + webhookID + "/" + token
	}

	return HooksBaseURL + "/" + webhookID + "/" + token + "?thread_id=" + url.QueryEscape(threadID)
}
