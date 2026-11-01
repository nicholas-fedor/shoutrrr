package telegram

import (
	"errors"
	"fmt"
	"net/url"
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
	// Title is prepended to the message in bold. It needs the HTML parse mode,
	// which a message without a parse mode switches to.
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

// Service identity.
const (
	// Scheme identifies this service in configuration URLs.
	Scheme = "telegram"
)

// Errors returned when reading the configuration.
var (
	// ErrInvalidToken indicates an invalid Telegram token format or content.
	ErrInvalidToken = errors.New("invalid telegram token")
	// ErrNoChannelsDefined indicates a service URL without chats.
	ErrNoChannelsDefined = errors.New("no channels defined in config URL")
	// ErrInvalidAPIURL indicates an API URL that is not an http or https URL
	// with a host, or that carries userinfo, a query, or a fragment.
	ErrInvalidAPIURL = errors.New("invalid telegram API URL")
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
	// A checked API URL starts with its scheme, so the prefix names it without
	// parsing the URL again.
	return len(c.APIURL) >= len("http:") && strings.EqualFold(c.APIURL[:len("http:")], "http:")
}

// sendBudget returns the send budget that [Service.ServiceTimeout] describes,
// for a config with the send's params applied.
//
// Returns:
//   - time.Duration: the budget for the send.
func (c *Config) sendBudget() time.Duration {
	return defaultHTTPTimeout * time.Duration(max(1, len(c.Chats)))
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
//     [ErrNoChannelsDefined] when no chats are set, or [ErrInvalidAPIURL].
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

	if err := read.validate(); err != nil {
		return err
	}

	c.Token = token

	return nil
}

// validate checks the bot token, the chats, and the API URL. Reading a service
// URL and each send run it, so a config built in code fails before a request.
//
// Returns:
//   - error: [ErrInvalidToken] for an invalid bot token, [ErrNoChannelsDefined]
//     when no chats are set, or [ErrInvalidAPIURL]. Otherwise nil.
func (c *Config) validate() error {
	if !IsTokenValid(c.Token) {
		return ErrInvalidToken
	}

	if len(c.Chats) < 1 {
		return ErrNoChannelsDefined
	}

	return validateAPIURL(c.APIURL)
}

// validateAPIURL checks that an API URL is an http or https URL with a host
// name, not only a port. The bot token and method are appended to its path, so
// it cannot carry a query or a fragment, even an empty one, and credentials
// belong in the token, not in userinfo.
//
// Parameters:
//   - rawURL: the API URL, or empty for the default.
//
// Returns:
//   - error: [ErrInvalidAPIURL] with the reason, or nil for a valid or empty URL.
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
		return nil
	}
}
