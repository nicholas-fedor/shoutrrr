package signal

import (
	"fmt"
	"net"
	"net/url"
	"strconv"
	"strings"

	"github.com/nicholas-fedor/shoutrrr/pkg/format"
	"github.com/nicholas-fedor/shoutrrr/pkg/types"
	"github.com/nicholas-fedor/shoutrrr/pkg/util"
)

// Config holds settings for the Signal notification service.
type Config struct {
	// Host is the signal-cli-rest-api server host.
	Host string `default:"localhost" desc:"Signal REST API server hostname or IP" key:"host"`
	// User is the username for HTTP Basic authentication, such as for a
	// reverse proxy in front of the server.
	User string `desc:"Username for HTTP Basic Auth" key:"user" optional:""`
	// Password is the password for HTTP Basic authentication.
	Password string `desc:"Password for HTTP Basic Auth" key:"password" optional:"" sensitive:"true"`
	// Token is the API token for Bearer authentication. It takes precedence
	// over User and Password.
	Token string `desc:"API token for Bearer authentication" key:"token,apikey" optional:"" sensitive:"true"`
	// Source is the registered phone number the messages are sent from, with
	// its country code.
	Source string `desc:"Source phone number (with country code)" key:"source"`
	// Title is prepended to the message body.
	Title string `desc:"Optional title prepended to the message body" key:"title" optional:""`
	// Attachments holds comma-separated raw base64 attachments. A data: URI
	// value is one attachment.
	Attachments string `desc:"Comma-separated raw base64 attachments; a data: value is one URI" key:"attachments" optional:""`
	// Recipients lists the phone numbers, group IDs, and u: usernames to send
	// to.
	Recipients []string `desc:"Recipient phone numbers, group IDs, or u: usernames" key:"recipients,to"`
	// Port is the signal-cli-rest-api server port.
	Port int `default:"8080" desc:"Signal REST API server port" key:"port"`
	// TextMode is the message text mode. None leaves the server default.
	TextMode textMode `default:"None" desc:"Message text mode (None omits text_mode; Styled enables markup)" key:"textmode,text_mode"`
	// DisableTLS connects to the server over HTTP instead of HTTPS.
	DisableTLS bool `default:"No" desc:"Disable TLS for Signal REST API connection" key:"disabletls"`
	// SkipTLSVerify skips verification of the server's TLS certificate.
	SkipTLSVerify bool `default:"No" desc:"Skip TLS certificate verification" key:"skiptlsverify"`
	// NotifySelf notifies the sender's own linked devices of the message.
	NotifySelf bool `default:"Yes" desc:"Notify the sender's devices. When No, sends notify_self=false." key:"notifyself"`
}

// Service identity and URL defaults.
const (
	// Scheme identifies this service in configuration URLs.
	Scheme = "signal"
	// DefaultHost is the Signal REST API host a URL without a host uses.
	DefaultHost = "localhost"
	// minPathParts is the minimum number of path parts required (source + at least one recipient).
	minPathParts = 2
)

// urlPartKeys are the config keys that GetURL writes into the URL authority
// and path, so it leaves them out of the query.
var urlPartKeys = []string{"host", "port", "user", "password", "source", "recipients"}

// Enums returns the fields that use an EnumFormatter for their values.
//
// Returns:
//   - A map of config field names to their enum formatters.
func (*Config) Enums() map[string]types.EnumFormatter {
	return map[string]types.EnumFormatter{
		"TextMode": TextModes.Enum,
	}
}

// GetURL generates a URL from the current configuration values.
//
// Returns:
//   - *url.URL: the generated URL representing the configuration.
func (c *Config) GetURL() *url.URL {
	resolver := format.NewPropKeyResolver(c)

	return c.getURL(&resolver)
}

// SetURL updates the configuration from a URL representation.
//
// Parameters:
//   - serviceURL: the URL to parse configuration from
//
// Returns:
//   - error: if parsing fails, nil otherwise
func (c *Config) SetURL(serviceURL *url.URL) error {
	resolver := format.NewPropKeyResolver(c)

	return c.setURL(&resolver, serviceURL)
}

// getURL constructs a URL from the Config's fields using the provided resolver.
// The query holds only the settings that the authority and path do not carry.
//
// Parameters:
//   - resolver: the configuration query resolver for property resolution
//
// Returns:
//   - *url.URL: the constructed URL
func (c *Config) getURL(resolver types.ConfigQueryResolver) *url.URL {
	recipients := strings.Join(c.Recipients, "/")

	query := format.BuildQueryWithCustomFields(resolver, url.Values{})
	for _, key := range urlPartKeys {
		query.Del(key)
	}

	result := &url.URL{
		Scheme:   Scheme,
		Host:     net.JoinHostPort(c.Host, strconv.Itoa(c.Port)),
		Path:     fmt.Sprintf("/%s/%s", c.Source, recipients),
		RawQuery: query.Encode(),
	}

	if c.User != "" {
		if c.Password != "" {
			result.User = url.UserPassword(c.User, c.Password)
		} else {
			result.User = url.User(c.User)
		}
	}

	return result
}

// parseAuth extracts user and password from the URL.
//
// Parameters:
//   - serviceURL: the URL to extract authentication from
func (c *Config) parseAuth(serviceURL *url.URL) {
	if serviceURL.User != nil {
		c.User = serviceURL.User.Username()
		if password, ok := serviceURL.User.Password(); ok {
			c.Password = password
		}
	}
}

// parseHostPort extracts host and port from the URL. An IPv6 host must be in
// brackets.
//
// Parameters:
//   - serviceURL: the URL to extract host and port from
//
// Returns:
//   - error: [util.ErrInvalidHost] or [util.ErrInvalidPort] for an authority
//     that does not split into a hostname and port, or a parse error for a
//     port above 65535.
func (c *Config) parseHostPort(serviceURL *url.URL) error {
	host, portStr, err := util.SplitURLHost(serviceURL)
	if err != nil {
		return fmt.Errorf("parsing host: %w", err)
	}

	c.Host = host

	if portStr == "" {
		portStr = "8080"
	}

	port, err := strconv.ParseUint(portStr, 10, 16)
	if err != nil {
		return fmt.Errorf("parsing port: %w", err)
	}

	c.Port = int(port)

	return nil
}

// parsePath extracts source phone number and recipients from the URL path.
//
// Parameters:
//   - serviceURL: the URL to extract path from
//
// Returns:
//   - error: if parsing fails, nil otherwise
func (c *Config) parsePath(serviceURL *url.URL) error {
	pathParts := strings.Split(strings.Trim(serviceURL.Path, "/"), "/")
	if len(pathParts) < minPathParts {
		return ErrNoRecipients
	}

	source := pathParts[0]
	if !isValidPhoneNumber(source) {
		return fmt.Errorf("%w: %s", ErrInvalidPhoneNumber, source)
	}

	c.Source = source

	recipients, err := parseRecipients(pathParts[1:])
	if err != nil {
		return err
	}

	c.Recipients = recipients

	return nil
}

// parseQuery processes query parameters using the resolver.
//
// Parameters:
//   - resolver: the configuration query resolver
//   - serviceURL: the URL containing query parameters
//
// Returns:
//   - error: an error wrapping [util.ErrInvalidQuery] for a malformed query, or
//     a query parameter error
func (c *Config) parseQuery(resolver types.ConfigQueryResolver, serviceURL *url.URL) error {
	query, err := util.ParseURLQuery(serviceURL)
	if err != nil {
		return err
	}

	return format.ApplyQuery(resolver, query)
}

// setURL updates the Config from a URL using the provided resolver. A URL
// without a host, from its authority or the host query param, uses
// [DefaultHost]. The source and recipients are validated whether they come
// from the path or the query.
//
// Parameters:
//   - resolver: the configuration query resolver
//   - serviceURL: the URL to parse
//
// Returns:
//   - error: a host, path, or query error, [ErrInvalidPhoneNumber] for an
//     invalid source, [ErrNoRecipients] when the query clears the recipients,
//     or [ErrInvalidRecipient] for an invalid recipient
func (c *Config) setURL(resolver types.ConfigQueryResolver, serviceURL *url.URL) error {
	c.parseAuth(serviceURL)

	if err := c.parseHostPort(serviceURL); err != nil {
		return err
	}

	if err := c.parsePath(serviceURL); err != nil {
		return err
	}

	if err := c.parseQuery(resolver, serviceURL); err != nil {
		return err
	}

	// The host may come from the URL or the host query param, and neither is
	// required.
	if c.Host == "" {
		c.Host = DefaultHost
	}

	// The source and recipients query params replace the path's values, so the
	// final values are validated once the query is applied.
	if !isValidPhoneNumber(c.Source) {
		return fmt.Errorf("%w: %s", ErrInvalidPhoneNumber, c.Source)
	}

	if len(c.Recipients) == 0 {
		return ErrNoRecipients
	}

	return validateRecipients(c.Recipients)
}
