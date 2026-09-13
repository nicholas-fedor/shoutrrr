package xmpp

import (
	"fmt"
	"net"
	"net/url"
	"slices"
	"strconv"
	"strings"

	"github.com/nicholas-fedor/shoutrrr/pkg/format"
	"github.com/nicholas-fedor/shoutrrr/pkg/services/standard"
	"github.com/nicholas-fedor/shoutrrr/pkg/types"
	"github.com/nicholas-fedor/shoutrrr/pkg/util"
)

// Config holds settings for the XMPP notification service.
type Config struct {
	standard.EnumlessConfig

	// Host is the XMPP server hostname used for the TCP dial.
	Host string `desc:"XMPP server hostname" url:"Host"`
	// Port is the XMPP server port.
	Port uint16 `desc:"XMPP server port" url:"Port"`
	// User is the auth JID localpart or a full JID (local@domain).
	User string `desc:"Auth JID localpart or full JID" url:"User"`
	// Password is the SASL password.
	Password string `desc:"Auth password" sensitive:"true" url:"Pass"`
	// To is the list of 1:1 chat recipient JIDs.
	To []string `desc:"Chat recipient JIDs" key:"to" optional:""`
	// Rooms is the list of MUC room JIDs.
	Rooms []string `desc:"MUC room JIDs" key:"rooms" optional:""`
	// Nick is the MUC nickname. Empty uses the auth JID localpart.
	Nick string `desc:"MUC nickname" key:"nick" optional:""`
	// RoomPassword is sent when joining password-protected rooms.
	RoomPassword string `desc:"Password for protected MUC rooms" key:"roompassword" optional:"" sensitive:"true"`
	// Title is prepended to the message body when set.
	Title string `desc:"Optional title prepended to the message body" key:"title" optional:""`
	// DisableTLS disables encryption. Allowed only on xmpp://.
	DisableTLS bool `default:"No" desc:"Disable TLS (xmpp:// only)" key:"disabletls"`
	// SkipTLSVerify skips TLS certificate verification.
	SkipTLSVerify bool `default:"No" desc:"Skip TLS certificate verification" key:"skiptlsverify"`

	// scheme is the URL scheme used to build GetURL, either [Scheme] or [SchemeTLS].
	scheme string
}

const (
	// Scheme identifies STARTTLS XMPP URLs.
	Scheme = "xmpp"
	// SchemeTLS identifies implicit-TLS XMPP URLs.
	SchemeTLS = "xmpps"
	// DefaultPort is the standard XMPP c2s port.
	DefaultPort uint16 = 5222
	// DefaultTLSPort is the standard XMPP implicit-TLS c2s port.
	DefaultTLSPort uint16 = 5223
	// Resource is the resource bound on the XMPP session.
	Resource = "shoutrrr"
)

// Clone returns a copy of the config with independent target lists.
//
// Returns:
//   - A copy of the configuration.
func (c *Config) Clone() Config {
	clone := *c
	clone.To = slices.Clone(c.To)
	clone.Rooms = slices.Clone(c.Rooms)

	return clone
}

// GetURL returns a URL representation of the current field values.
//
// Returns:
//   - A configuration URL using the stored scheme.
func (c *Config) GetURL() *url.URL {
	resolver := format.NewPropKeyResolver(c)

	return c.getURL(&resolver)
}

// SetURL updates the configuration from a URL representation.
//
// Parameters:
//   - serviceURL: The XMPP configuration URL to parse.
//
// Returns:
//   - An error if the URL is invalid or required fields are missing.
func (c *Config) SetURL(serviceURL *url.URL) error {
	resolver := format.NewPropKeyResolver(c)

	return c.setURL(&resolver, serviceURL)
}

// authJIDString returns the SASL auth JID.
//
// A userinfo value that already contains @ is used as-is. Otherwise the JID is
// localpart@host.
//
// Returns:
//   - The auth JID string.
func (c *Config) authJIDString() string {
	if strings.Contains(c.User, "@") {
		return c.User
	}

	return c.User + "@" + c.Host
}

// getURL constructs a URL from the Config fields using the provided resolver.
//
// Parameters:
//   - resolver: Resolver used to serialize configuration query keys.
//
// Returns:
//   - A configuration URL using the stored scheme and default port when unset.
func (c *Config) getURL(resolver types.ConfigQueryResolver) *url.URL {
	scheme := c.scheme
	if scheme == "" {
		scheme = Scheme
	}

	port := c.Port
	if port == 0 {
		port = defaultPort(scheme)
	}

	result := &url.URL{
		Scheme:     scheme,
		Host:       net.JoinHostPort(c.Host, strconv.FormatUint(uint64(port), 10)),
		ForceQuery: true,
		RawQuery:   format.BuildQuery(resolver),
	}

	if c.User != "" {
		result.User = util.URLUserPassword(c.User, c.Password)
	}

	return result
}

// implicitTLS reports whether the URL uses xmpps:// implicit TLS.
//
// Returns:
//   - true when the stored scheme is [SchemeTLS].
func (c *Config) implicitTLS() bool {
	return c.scheme == SchemeTLS
}

// mucNick returns the nickname used when joining MUC rooms.
//
// An explicit [Config.Nick] wins. Otherwise the auth JID localpart is used,
// falling back to [Resource] if the JID cannot be split.
//
// Returns:
//   - The MUC nickname.
func (c *Config) mucNick() string {
	if c.Nick != "" {
		return c.Nick
	}

	local, _, err := splitJID(c.authJIDString())
	if err != nil {
		return Resource
	}

	return local
}

// setURL updates the configuration from a URL representation.
//
// Dummy docs URLs (host dummy.com) skip credential and target validation so
// generated documentation can initialize the service.
//
// Parameters:
//   - resolver: Resolver used to apply query keys onto the config.
//   - serviceURL: The XMPP configuration URL to parse.
//
// Returns:
//   - An error if the URL is invalid or required fields are missing.
func (c *Config) setURL(resolver types.ConfigQueryResolver, serviceURL *url.URL) error {
	dummy := isDummyURL(serviceURL)

	c.scheme = serviceURL.Scheme
	if c.scheme == "" {
		c.scheme = Scheme
	}

	if c.scheme != Scheme && c.scheme != SchemeTLS {
		return fmt.Errorf("%w: %s", ErrUnsupportedScheme, c.scheme)
	}

	if serviceURL.User != nil {
		c.User = serviceURL.User.Username()
		if password, ok := serviceURL.User.Password(); ok {
			c.Password = password
		}
	}

	c.Host = serviceURL.Hostname()

	if port := serviceURL.Port(); port != "" {
		parsed, err := strconv.ParseUint(port, 10, 16)
		if err != nil {
			return fmt.Errorf("parsing port: %w", err)
		}

		c.Port = uint16(parsed)
	} else {
		c.Port = defaultPort(c.scheme)
	}

	for key, vals := range serviceURL.Query() {
		if err := resolver.Set(key, vals[0]); err != nil {
			return fmt.Errorf("setting query parameter %q to %q: %w", key, vals[0], err)
		}
	}

	c.To = compactStrings(c.To)
	c.Rooms = compactStrings(c.Rooms)

	if dummy {
		return nil
	}

	return c.validate()
}

// useTLS reports whether the session should encrypt the stream.
//
// Returns:
//   - false only when [Config.DisableTLS] is set.
func (c *Config) useTLS() bool {
	return !c.DisableTLS
}

// validate checks that required fields and JIDs are present and consistent.
//
// Returns:
//   - An error describing the first invalid field.
func (c *Config) validate() error {
	if c.Host == "" {
		return ErrMissingHost
	}

	if c.User == "" {
		return ErrMissingUser
	}

	if c.Password == "" {
		return ErrMissingPassword
	}

	if c.DisableTLS && c.scheme == SchemeTLS {
		return ErrDisableTLSOnXMPPS
	}

	if len(c.To) == 0 && len(c.Rooms) == 0 {
		return ErrMissingTargets
	}

	if strings.Contains(c.User, "@") {
		if err := validateJID(c.User); err != nil {
			return fmt.Errorf("%w: %s", ErrInvalidUserJID, c.User)
		}
	}

	for _, to := range c.To {
		if err := validateJID(to); err != nil {
			return fmt.Errorf("%w: %s", ErrInvalidRecipientJID, to)
		}
	}

	for _, room := range c.Rooms {
		if err := validateJID(room); err != nil {
			return fmt.Errorf("%w: %s", ErrInvalidRoomJID, room)
		}
	}

	return nil
}

// compactStrings removes empty entries from values.
//
// Parameters:
//   - values: The slice to compact.
//
// Returns:
//   - A slice with empty strings removed, or nil when none remain.
func compactStrings(values []string) []string {
	values = slices.DeleteFunc(values, isEmpty)
	if len(values) == 0 {
		return nil
	}

	return values
}

// composeMessage prepends title to message when title is set.
//
// Parameters:
//   - title: Optional heading prepended to the body.
//   - message: The notification body.
//
// Returns:
//   - The combined text sent as the XMPP body.
func composeMessage(title, message string) string {
	if title == "" {
		return message
	}

	if message == "" {
		return title
	}

	return title + "\n" + message
}

// defaultPort returns the default c2s port for scheme.
//
// Parameters:
//   - scheme: [Scheme] or [SchemeTLS].
//
// Returns:
//   - [DefaultTLSPort] for xmpps, otherwise [DefaultPort].
func defaultPort(scheme string) uint16 {
	if scheme == SchemeTLS {
		return DefaultTLSPort
	}

	return DefaultPort
}

// isDummyURL reports whether serviceURL is the docs-generator placeholder.
//
// Parameters:
//   - serviceURL: The URL to inspect.
//
// Returns:
//   - true when the hostname is dummy.com.
func isDummyURL(serviceURL *url.URL) bool {
	return serviceURL.Hostname() == "dummy.com"
}

// isEmpty reports whether value is the empty string.
//
// Parameters:
//   - value: The string to inspect.
//
// Returns:
//   - true if value has length zero.
func isEmpty(value string) bool {
	return value == ""
}

// splitJID splits value into localpart and domainpart.
//
// A resource, if present, is discarded. The domain must not contain another @.
//
// Parameters:
//   - value: A bare or full JID string.
//
// Returns:
//   - The localpart.
//   - The domainpart.
//   - An error if value is not a valid JID.
func splitJID(value string) (string, string, error) {
	local, rest, ok := strings.Cut(value, "@")
	if !ok || local == "" || rest == "" {
		return "", "", fmt.Errorf("%w: %s", ErrInvalidUserJID, value)
	}

	domain, _, _ := strings.Cut(rest, "/")
	if domain == "" || strings.Contains(domain, "@") {
		return "", "", fmt.Errorf("%w: %s", ErrInvalidUserJID, value)
	}

	return local, domain, nil
}

// validateJID reports whether value is a usable JID.
//
// Parameters:
//   - value: A candidate JID string.
//
// Returns:
//   - An error if value cannot be split into localpart and domainpart.
func validateJID(value string) error {
	_, _, err := splitJID(value)

	return err
}
