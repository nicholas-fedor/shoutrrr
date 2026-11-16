package xmpp

import (
	"fmt"
	"net"
	"net/url"
	"slices"
	"strconv"
	"strings"

	"golang.org/x/net/idna"

	"github.com/nicholas-fedor/shoutrrr/pkg/format"
	"github.com/nicholas-fedor/shoutrrr/pkg/services/standard"
	"github.com/nicholas-fedor/shoutrrr/pkg/types"
	"github.com/nicholas-fedor/shoutrrr/pkg/util"
)

// Config holds settings for the XMPP notification service.
type Config struct {
	// EnumlessConfig provides the empty enum set, since XMPP has no enum
	// fields.
	standard.EnumlessConfig

	// Host is the XMPP server hostname used for the TCP dial, or the domain
	// whose SRV records name the server.
	Host string `desc:"XMPP server hostname" url:"Host"`
	// User is the auth JID localpart or a full JID (local@domain).
	User string `desc:"Auth JID localpart or full JID" url:"User"`
	// Password is the SASL password.
	Password string `desc:"Auth password" sensitive:"true" url:"Pass"`
	// Nick is the MUC nickname. Empty uses the auth JID localpart.
	Nick string `desc:"MUC nickname" key:"nick" optional:""`
	// RoomPassword is sent when joining password-protected rooms.
	RoomPassword string `desc:"Password for protected MUC rooms" key:"roompassword" optional:"" sensitive:"true"`
	// Title is prepended to the message body when set.
	Title string `desc:"Optional title prepended to the message body" key:"title" optional:""`

	// scheme is the URL scheme used to build GetURL, either [Scheme] or [SchemeTLS].
	scheme string
	// To is the list of 1:1 chat recipient JIDs.
	To []string `desc:"Chat recipient JIDs" key:"to" optional:""`
	// Rooms is the list of MUC room JIDs.
	Rooms []string `desc:"MUC room JIDs" key:"rooms" optional:""`
	// Port is the XMPP server port. Zero leaves the port unset, so a send may
	// look the server up through SRV records.
	Port uint16 `desc:"XMPP server port" url:"Port"`
	// DisableTLS disables encryption. Allowed only on xmpp://.
	DisableTLS bool `default:"No" desc:"Disable TLS (xmpp:// only)" key:"disabletls"`
	// SkipTLSVerify skips TLS certificate verification.
	SkipTLSVerify bool `default:"No" desc:"Skip TLS certificate verification" key:"skiptlsverify"`
}

// URL schemes, default ports, and the session resource for XMPP.
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
//   - Config: a copy of the configuration.
func (c *Config) Clone() Config {
	clone := *c
	clone.To = slices.Clone(c.To)
	clone.Rooms = slices.Clone(c.Rooms)

	return clone
}

// GetURL returns a URL representation of the current field values.
//
// Returns:
//   - *url.URL: a configuration URL using the stored scheme.
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
//   - error: an error if the URL is invalid or required fields are missing.
func (c *Config) SetURL(serviceURL *url.URL) error {
	next := Config{}
	resolver := format.NewPropKeyResolver(&next)

	if err := resolver.SetDefaultProps(&next); err != nil {
		return fmt.Errorf("setting default props: %w", err)
	}

	if err := next.setURL(&resolver, serviceURL); err != nil {
		return err
	}

	*c = next

	return nil
}

// authJIDString returns the SASL auth JID.
//
// A userinfo value that already contains @ is used as-is. Otherwise the JID is
// localpart@host.
//
// Returns:
//   - string: the auth JID string.
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
//   - *url.URL: a configuration URL using the stored scheme, without a port
//     when the port is unset.
func (c *Config) getURL(resolver types.ConfigQueryResolver) *url.URL {
	scheme := c.scheme
	if scheme == "" {
		scheme = Scheme
	}

	result := &url.URL{
		Scheme:     scheme,
		Host:       util.JoinURLHostPort(c.Host, int(c.Port)),
		ForceQuery: true,
		RawQuery:   format.BuildQuery(resolver),
	}

	if c.User != "" {
		result.User = util.URLUserPassword(c.User, c.Password)
	}

	return result
}

// hostAddress returns the address of the host itself, at its port or, when
// the port is unset, at 5222 for xmpp:// and 5223 for xmpps://.
//
// Returns:
//   - string: the host and port.
func (c *Config) hostAddress() string {
	port := c.Port
	if port == 0 {
		port = DefaultPort
		if c.implicitTLS() {
			port = DefaultTLSPort
		}
	}

	return net.JoinHostPort(c.Host, strconv.FormatUint(uint64(port), 10))
}

// implicitTLS reports whether the URL uses xmpps:// implicit TLS.
//
// Returns:
//   - bool: true when the stored scheme is [SchemeTLS].
func (c *Config) implicitTLS() bool {
	return c.scheme == SchemeTLS
}

// mucNick returns the nickname used when joining MUC rooms.
//
// An explicit [Config.Nick] wins. Otherwise the auth JID localpart is used,
// falling back to [Resource] if the JID cannot be split.
//
// Returns:
//   - string: the MUC nickname.
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

// prepare normalizes the config and checks it.
//
// It drops the empty recipients and rooms, as reading a list setting does.
// Reading a service URL and each send run it, so a config built in code fails
// as its URL would.
//
// Returns:
//   - error: the error from the config's checks, such as [ErrMissingHost].
func (c *Config) prepare() error {
	c.To = util.NonEmpty(c.To)
	c.Rooms = util.NonEmpty(c.Rooms)

	return c.validate()
}

// setURL updates the configuration from a URL representation.
//
// Parameters:
//   - resolver: Resolver used to apply query keys onto the config.
//   - serviceURL: The XMPP configuration URL to parse.
//
// Returns:
//   - error: an error if the URL is invalid, including a port that is not a
//     number from 1 to 65535 ([util.ErrInvalidPort]) and a malformed query
//     ([util.ErrInvalidQuery]), or required fields are missing.
func (c *Config) setURL(resolver types.ConfigQueryResolver, serviceURL *url.URL) error {
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

	host, port, err := util.SplitURLHostPort(serviceURL)
	if err != nil {
		return fmt.Errorf("parsing host: %w", err)
	}

	c.Host = host
	c.Port = port

	query, err := util.ParseURLQuery(serviceURL)
	if err != nil {
		return err
	}

	if err := format.ApplyQuery(resolver, query); err != nil {
		return err
	}

	return c.prepare()
}

// srvService returns the SRV service that names the server for the URL's
// scheme. XEP-0368 gives direct TLS its own records.
//
// Returns:
//   - string: "xmpps-client" for xmpps://, otherwise "xmpp-client".
func (c *Config) srvService() string {
	if c.implicitTLS() {
		return "xmpps-client"
	}

	return "xmpp-client"
}

// useTLS reports whether the session should encrypt the stream.
//
// Returns:
//   - bool: false only when [Config.DisableTLS] is set.
func (c *Config) useTLS() bool {
	return !c.DisableTLS
}

// usesSRV reports whether a send looks the server up through SRV records.
//
// RFC 6120 section 3.2.1 resolves the auth JID's domain through its SRV
// records, and section 3.2.3 lets a configured host and port take their
// place. So a config looks the server up only without a port and when the
// host is the JID's domain. A trailing dot on either name does not count. An
// IP address has no SRV records. The two names are compared in their ASCII
// form, so an internationalized domain matches its A-label. The config must
// have passed its checks.
//
// Returns:
//   - bool: true for a config without a port whose host is a domain name and
//     the auth JID's domain.
func (c *Config) usesSRV() bool {
	if c.Port != 0 || net.ParseIP(c.Host) != nil {
		return false
	}

	_, domain, err := splitJID(c.authJIDString())

	return err == nil && strings.EqualFold(asciiDomain(c.Host), asciiDomain(domain))
}

// validate checks that required fields and JIDs are present and consistent.
//
// Returns:
//   - error: an error describing the first invalid field.
func (c *Config) validate() error {
	if c.Host == "" {
		return ErrMissingHost
	}

	if err := util.ValidateHostName(c.Host); err != nil {
		return fmt.Errorf("host: %w", err)
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

// asciiDomain returns a domain name in the ASCII form that DNS and
// certificates use, lowercase and without a trailing dot. IDNA maps other
// label separators, such as the ideographic full stop U+3002, to a dot, so
// a trailing one goes too. A name that IDNA rejects, such as an IP address in
// brackets, is returned without its trailing dot but otherwise as written.
//
// Parameters:
//   - name: the domain name, in Unicode or ASCII form.
//
// Returns:
//   - string: the domain name in ASCII form.
func asciiDomain(name string) string {
	name = strings.TrimSuffix(name, ".")

	if ascii, err := idna.Lookup.ToASCII(name); err == nil {
		return strings.TrimSuffix(ascii, ".")
	}

	return name
}

// splitJID splits value into localpart and domainpart.
//
// A resource, if present, is discarded. The domain must not contain another @.
//
// Parameters:
//   - value: A bare or full JID string.
//
// Returns:
//   - string: the localpart.
//   - string: the domainpart.
//   - error: an error if value is not a valid JID.
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
//   - error: an error if value cannot be split into localpart and domainpart.
func validateJID(value string) error {
	_, _, err := splitJID(value)

	return err
}
