package telegram

import (
	"errors"
	"fmt"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/nicholas-fedor/shoutrrr/pkg/format"
	"github.com/nicholas-fedor/shoutrrr/pkg/types"
	"github.com/nicholas-fedor/shoutrrr/pkg/util"
)

// Config holds settings for the Telegram notification service.
type Config struct {
	// Token is the bot token, written as the userinfo of the service URL.
	Token string `sensitive:"true" url:"user"`
	// Title is sent in bold on a line before the message, escaped for the
	// parse mode. A message without a parse mode switches to HTML for it.
	Title string `default:"" desc:"Notification title, optionally set by the sender" key:"title"`
	// APIURL is the base URL of the Bot API server, such as a self-hosted
	// server or a proxy. Empty selects https://api.telegram.org.
	APIURL string `desc:"Bot API server base URL (default: https://api.telegram.org)" key:"apiurl" optional:""`
	// Chats lists the chat IDs or @channel names to send to. An entry written as
	// <chat ID>:<thread ID> posts in that forum topic.
	Chats []string `desc:"Chat IDs or Channel names (using @channel-name)" key:"chats,channels"`
	// ParseMode is how Telegram formats the message.
	ParseMode parseMode `default:"None" desc:"How the text Message should be parsed" key:"parsemode"`
	// Preview shows link previews for URLs in the message.
	Preview bool `default:"Yes" desc:"If disabled, no web page preview will be displayed for URLs" key:"preview"`
	// Notification delivers the message with a sound.
	Notification bool `default:"Yes" desc:"If disabled, sends Message silently" key:"notification"`
	// SkipTLSVerify skips verification of the Bot API server's TLS
	// certificate, such as the certificate of a proxy in front of a
	// self-hosted server.
	SkipTLSVerify bool `default:"No" desc:"Skip TLS certificate verification" key:"skiptlsverify"`
}

// chatTarget is a configured chat, read into the fields of a sendMessage
// request.
type chatTarget struct {
	// threadID is the forum topic to post in, or nil for the chat's main
	// thread.
	threadID *int
	// chat is the chat as configured, which a failed send names.
	chat string
	// id is the chat ID or @channel.
	id string
}

// Service identity.
const (
	// Scheme identifies this service in configuration URLs.
	Scheme = "telegram"
)

// Errors returned when reading the configuration.
var (
	// ErrInvalidToken indicates an invalid Telegram token format or content.
	ErrInvalidToken = errors.New("invalid telegram token")
	// ErrNoChannelsDefined indicates a config without chats.
	ErrNoChannelsDefined = errors.New("no channels defined in config")
	// ErrInvalidAPIURL indicates an API URL that is not an http or https URL
	// with a host, or that carries userinfo, a query, or a fragment.
	ErrInvalidAPIURL = errors.New("invalid telegram API URL")
	// ErrInvalidThreadID indicates a chat whose thread ID, after the colon, is
	// not a positive integer.
	ErrInvalidThreadID = errors.New("invalid telegram thread ID")
)

// Enums returns the fields that use an EnumFormatter for their values.
//
// Returns:
//   - map[string]types.EnumFormatter: the parse mode formatter by field name.
func (c *Config) Enums() map[string]types.EnumFormatter {
	return map[string]types.EnumFormatter{
		"ParseMode": ParseModes.Enum,
	}
}

// GetURL generates a URL from the current configuration values.
//
// Returns:
//   - *url.URL: the service URL.
func (c *Config) GetURL() *url.URL {
	resolver := format.NewPropKeyResolver(c)

	return c.getURL(&resolver)
}

// SetURL updates the configuration from a URL representation.
//
// Parameters:
//   - serviceURL: the service URL.
//
// Returns:
//   - error: the error from reading the URL, such as [ErrInvalidToken].
func (c *Config) SetURL(serviceURL *url.URL) error {
	resolver := format.NewPropKeyResolver(c)

	return c.setURL(&resolver, serviceURL)
}

// getURL constructs a URL from the Config's fields using the provided resolver.
//
// Parameters:
//   - resolver: the resolver that builds the query from the config props.
//
// Returns:
//   - *url.URL: the service URL, with the bot ID and secret of the token as its
//     userinfo. A token without a colon, such as a masked one, is the username.
func (c *Config) getURL(resolver types.ConfigQueryResolver) *url.URL {
	user := url.User(c.Token)
	if botID, secret, found := strings.Cut(c.Token, ":"); found {
		user = url.UserPassword(botID, secret)
	}

	return &url.URL{
		User:       user,
		Host:       Scheme,
		Scheme:     Scheme,
		ForceQuery: true,
		RawQuery:   format.BuildQuery(resolver),
	}
}

// plaintextAPI reports whether the requests go to the Bot API server over
// plain HTTP. The bot token is in every request path, so it then travels
// unencrypted. It reads an API URL that has passed the config's checks.
//
// Returns:
//   - bool: true for an API URL with the http scheme in any case, and false
//     for an https or empty API URL.
func (c *Config) plaintextAPI() bool {
	// A checked API URL starts with its scheme, so the text before the first
	// colon names it without parsing the URL again.
	scheme, _, found := strings.Cut(c.APIURL, ":")

	return found && strings.EqualFold(scheme, "http")
}

// prepare drops the empty chats, as reading a list setting does, then checks
// the bot token, the chats and their thread IDs, the parse mode, and the API
// URL.
//
// Reading a service URL and each send run it, so a config built in code sends
// to the chats its URL would, and fails before a request as its URL would.
//
// Returns:
//   - []chatTarget: the chats, each read into its chat ID and thread ID.
//   - error: [ErrInvalidToken] for an invalid bot token, [ErrNoChannelsDefined]
//     when no chats are set, [ErrInvalidThreadID] for a thread ID that is not
//     a positive integer, an error wrapping [format.ErrInvalidEnumValue] for a
//     parse mode that is not one of its values, or [ErrInvalidAPIURL].
//     Otherwise nil.
func (c *Config) prepare() ([]chatTarget, error) {
	c.Chats = util.NonEmpty(c.Chats)

	if !IsTokenValid(c.Token) {
		return nil, ErrInvalidToken
	}

	if len(c.Chats) < 1 {
		return nil, ErrNoChannelsDefined
	}

	targets := make([]chatTarget, len(c.Chats))

	for i, chat := range c.Chats {
		target, err := parseChat(chat)
		if err != nil {
			return nil, err
		}

		targets[i] = target
	}

	if err := format.CheckEnumValue(ParseModes.Enum, int(c.ParseMode)); err != nil {
		return nil, fmt.Errorf("parse mode: %w", err)
	}

	if err := validateAPIURL(c.APIURL); err != nil {
		return nil, err
	}

	return targets, nil
}

// sendBudget returns the send budget that [Service.ServiceTimeout] describes,
// for a config with the send's params applied.
//
// Returns:
//   - time.Duration: the budget for the send.
func (c *Config) sendBudget() time.Duration {
	return defaultHTTPTimeout * time.Duration(max(1, len(util.NonEmpty(c.Chats))))
}

// setURL updates the Config from a URL using the provided resolver.
//
// Parameters:
//   - resolver: the resolver that sets the config props from the query.
//   - serviceURL: the service URL.
//
// Returns:
//   - error: [ErrInvalidToken] for an invalid bot token, an error wrapping
//     [util.ErrInvalidQuery] for a malformed query, a query prop error,
//     [ErrNoChannelsDefined] when no chats are set, [ErrInvalidThreadID], or
//     [ErrInvalidAPIURL].
func (c *Config) setURL(resolver types.ConfigQueryResolver, serviceURL *url.URL) error {
	password, _ := serviceURL.User.Password()

	// The token is checked before the query is applied, so a URL with an
	// invalid token leaves the config unchanged.
	token := serviceURL.User.Username() + ":" + password
	if !IsTokenValid(token) {
		return ErrInvalidToken
	}

	query, err := util.ParseURLQuery(serviceURL)
	if err != nil {
		return err
	}

	if err := format.ApplyQuery(resolver, query); err != nil {
		return err
	}

	// The token is kept only once the whole config passes, so a rejected URL
	// leaves no token behind.
	read := *c
	read.Token = token

	if _, err := read.prepare(); err != nil {
		return err
	}

	*c = read

	return nil
}

// parseChat reads a chat into its chat ID and the forum topic it names.
//
// Parameters:
//   - chat: the chat ID or @channel, optionally followed by a colon and a
//     thread ID.
//
// Returns:
//   - chatTarget: the chat with its chat ID, and its thread ID when it names
//     one.
//   - error: [ErrInvalidThreadID] for a thread ID that is not a positive
//     integer.
func parseChat(chat string) (chatTarget, error) {
	chatID, thread, ok := strings.Cut(chat, ":")
	if !ok {
		return chatTarget{threadID: nil, chat: chat, id: chat}, nil
	}

	threadID, err := strconv.Atoi(thread)
	if err != nil || threadID < 1 {
		return chatTarget{}, fmt.Errorf("%w: chat %q", ErrInvalidThreadID, chat)
	}

	return chatTarget{threadID: &threadID, chat: chat, id: chatID}, nil
}

// validateAPIURL checks that an API URL is an http or https URL with a host
// name.
//
// A port must be a number from 1 to 65535. The bot token and method are
// appended to the URL's path, so it cannot carry a query or a fragment, even an
// empty one. Credentials belong in the token, not in userinfo.
//
// Parameters:
//   - rawURL: the API URL, or empty for the default.
//
// Returns:
//   - error: [ErrInvalidAPIURL] with the reason, also wrapping
//     [util.ErrInvalidPort] for a port outside 1 to 65535, or nil for a valid
//     or empty URL.
func validateAPIURL(rawURL string) error {
	if rawURL == "" {
		return nil
	}

	apiURL, err := url.Parse(rawURL)

	switch {
	case err != nil:
		return fmt.Errorf("%w: the URL does not parse", ErrInvalidAPIURL)
	case apiURL.Scheme != "http" && apiURL.Scheme != "https":
		return fmt.Errorf("%w: the scheme must be http or https", ErrInvalidAPIURL)
	case apiURL.Hostname() == "":
		return fmt.Errorf("%w: the URL has no host", ErrInvalidAPIURL)
	case apiURL.User != nil:
		return fmt.Errorf("%w: the URL cannot hold userinfo", ErrInvalidAPIURL)
	// An empty fragment leaves no trace in the parsed URL, so the raw URL is
	// checked for its "#".
	case apiURL.RawQuery != "" || apiURL.ForceQuery || strings.Contains(rawURL, "#"):
		return fmt.Errorf("%w: the URL cannot hold a query or a fragment", ErrInvalidAPIURL)
	default:
		if _, _, err := util.SplitURLHost(apiURL); err != nil {
			return fmt.Errorf("%w: %w", ErrInvalidAPIURL, err)
		}

		return nil
	}
}
