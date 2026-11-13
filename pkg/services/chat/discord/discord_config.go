package discord

import (
	"fmt"
	"net/url"
	"slices"
	"strconv"
	"strings"
	"unicode/utf8"

	"github.com/nicholas-fedor/shoutrrr/pkg/format"
	"github.com/nicholas-fedor/shoutrrr/pkg/services/standard"
	"github.com/nicholas-fedor/shoutrrr/pkg/types"
	"github.com/nicholas-fedor/shoutrrr/pkg/util"
)

// Config holds the settings required for sending Discord notifications.
//
// Reading a service URL checks the webhook ID, token, thread ID, thread
// name, and mentions. Each send checks them again after applying its params. A Config built in code or
// a param with an invalid value fails before any request with an error such as
// [ErrInvalidWebhookID].
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
	// ThreadName names a thread that each send creates in a forum or media
	// channel, of up to 100 characters. A send that takes several requests
	// creates the thread with the first and sends the rest into it. It cannot
	// be set with ThreadID. JSON mode sends its payload as it is and ignores
	// it.
	ThreadName string `default:"" desc:"The name of a thread to create for each message in a forum or media channel" key:"thread_name"`
	// Mentions lists who each send pings: everyone, here, user:ID, or
	// role:ID. Shoutrrr adds them to the message content of the send's first
	// request, the only place Discord notifies them from, and allows only
	// them. A mention already in the message pings only when the list allows
	// it, and Discord counts @everyone and @here as one kind, so listing
	// either allows both. JSON mode sends its payload as it is and ignores
	// them.
	Mentions []string `desc:"Mentions to add to each message, the only ones that ping: everyone, here, user:ID, or role:ID" key:"mentions" optional:""`
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

// mentions reads the config's mentions into the text that goes in front of
// the message content and the allowed mentions that let exactly them ping.
//
// Each mention is everyone, here, user:ID, or role:ID, in any case, with a
// snowflake as the ID. They become @everyone, @here, <@ID>, and <@&ID> in the
// text, in the order given. Blank and repeated mentions are left out.
//
// Returns:
//   - mentionSet: the mentions, empty when the config has none.
//   - error: an error wrapping [ErrInvalidMention] for a mention that cannot
//     be read, or for more than 100 user or role mentions.
func (c *Config) mentions() (mentionSet, error) {
	var (
		set    mentionSet
		tokens []string
	)

	for _, mention := range c.Mentions {
		mention = strings.TrimSpace(mention)
		if mention == "" {
			continue
		}

		token, err := set.allowed.add(mention)
		if err != nil {
			return mentionSet{}, err
		}

		if !slices.Contains(tokens, token) {
			tokens = append(tokens, token)
		}
	}

	if len(set.allowed.Users) > maxMentionIDs || len(set.allowed.Roles) > maxMentionIDs {
		return mentionSet{}, fmt.Errorf("%w: more than %d user or role mentions", ErrInvalidMention, maxMentionIDs)
	}

	set.text = strings.Join(tokens, " ")

	return set, nil
}

// postURL builds the webhook URL of a config that passes the checks described
// on [Config], as [CreatePostURLFromConfig] describes it.
//
// Returns:
//   - string: the webhook URL.
func (c *Config) postURL() string {
	// A valid webhook ID and thread ID are all digits, so only the token needs
	// escaping.
	webhookID := strings.TrimSpace(c.WebhookID)
	token := url.PathEscape(strings.TrimSpace(c.Token))

	postURL := HooksBaseURL + "/" + webhookID + "/" + token + "?"
	if threadID := strings.TrimSpace(c.ThreadID); threadID != "" {
		postURL += "thread_id=" + threadID + "&"
	}

	// wait=true makes Discord confirm the message was saved, or answer with
	// an error. Without it, Discord answers 204 before saving.
	return postURL + "wait=true"
}

// setURL updates the configuration from a URL using the provided resolver.
//
// Parameters:
//   - resolver: the resolver that sets the config props from the query.
//   - serviceURL: the service URL.
//
// Returns:
//   - error: [ErrIllegalURLArgument] for a path other than /raw. The checks
//     described on [Config] give an error such as [ErrInvalidWebhookID]. A
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

	query, err := util.ParseURLQuery(serviceURL)
	if err != nil {
		return err
	}

	if err := format.ApplyQuery(resolver, query); err != nil {
		return err
	}

	// A thread ID copied from a client can carry surrounding whitespace.
	c.ThreadID = strings.TrimSpace(c.ThreadID)

	return c.validate()
}

// validate checks the webhook ID, token, thread ID, thread name, and mentions.
//
// Surrounding whitespace is ignored. The webhook ID is a Discord snowflake, a
// number of up to 64 bits. The token is one segment of the webhook URL's path,
// so it holds no slash and is not "." or "..". The thread ID is empty or a
// snowflake. The thread name has at most 100 characters, and is empty when
// the thread ID is set.
//
// Returns:
//   - error: [ErrMissingWebhookID] or [ErrMissingToken] for a blank webhook
//     ID or token. [ErrInvalidWebhookID] or [ErrInvalidThreadID] for a webhook
//     ID or thread ID that is not a snowflake. An error wrapping
//     [util.ErrInvalidPathSegment] for a token that is not one path segment.
//     [ErrThreadIDAndName] for a thread ID and a thread name together, and
//     [ErrInvalidThreadName] for a thread name over 100 characters. An error
//     wrapping [ErrInvalidMention] for a mention that cannot be read.
//     Otherwise nil.
func (c *Config) validate() error {
	webhookID := strings.TrimSpace(c.WebhookID)
	if webhookID == "" {
		return ErrMissingWebhookID
	}

	if !isSnowflake(webhookID) {
		return ErrInvalidWebhookID
	}

	token := strings.TrimSpace(c.Token)
	if token == "" {
		return ErrMissingToken
	}

	if err := util.ValidatePathSegment(token); err != nil {
		return fmt.Errorf("token: %w", err)
	}

	threadID := strings.TrimSpace(c.ThreadID)
	if threadID != "" && !isSnowflake(threadID) {
		return ErrInvalidThreadID
	}

	threadName := strings.TrimSpace(c.ThreadName)
	if threadName != "" && threadID != "" {
		return ErrThreadIDAndName
	}

	if utf8.RuneCountInString(threadName) > maxThreadName {
		return ErrInvalidThreadName
	}

	if _, err := c.mentions(); err != nil {
		return err
	}

	return nil
}

// CreatePostURLFromConfig builds a POST URL from the Discord configuration.
//
// The webhook ID, token, and thread ID are trimmed of surrounding whitespace.
// The token is path-escaped, so it cannot change the URL's path or start its
// query.
//
// Parameters:
//   - config: the configuration with the webhook ID, token, and thread ID.
//
// Returns:
//   - string: the webhook URL under [HooksBaseURL] with wait=true, plus a
//     thread_id query parameter when the config sets a thread ID that is not
//     blank. It is empty for a config that fails the checks described on
//     [Config].
func CreatePostURLFromConfig(config *Config) string {
	if config.validate() != nil {
		return ""
	}

	return config.postURL()
}

// isSnowflake reports whether id is a Discord snowflake, a number of up to 64
// bits written in decimal.
//
// Parameters:
//   - id: the ID, without surrounding whitespace.
//
// Returns:
//   - bool: whether id is a snowflake.
func isSnowflake(id string) bool {
	_, err := strconv.ParseUint(id, 10, 64)

	return err == nil
}
