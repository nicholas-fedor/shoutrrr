package smtp

import (
	"crypto/tls"
	"fmt"
	"net/url"
	"slices"
	"strings"
	"time"

	"github.com/nicholas-fedor/shoutrrr/internal/transport"
	"github.com/nicholas-fedor/shoutrrr/pkg/format"
	"github.com/nicholas-fedor/shoutrrr/pkg/types"
	"github.com/nicholas-fedor/shoutrrr/pkg/util"
)

// Config is the configuration needed to send e-mail notifications over SMTP.
type Config struct {
	// Host is the SMTP server hostname or IP address.
	Host string `desc:"SMTP server hostname or IP address" url:"Host"`
	// Username is the SMTP server username.
	Username string `default:"" desc:"SMTP server username" url:"User"`
	// Password is the SMTP server password, or an OAuth2 access token when Auth is [AuthOAuth2].
	Password string `default:"" desc:"SMTP server password, or the OAuth2 access token" sensitive:"true" url:"Pass"`
	// FromAddress is the sender e-mail address.
	FromAddress string `desc:"E-mail address that the mail are sent from" key:"fromaddress,from"`
	// FromName is the sender display name.
	FromName string `desc:"Name of the sender" key:"fromname" optional:"yes"`
	// Subject is the e-mail subject. It is empty when omitted.
	Subject string `default:"" desc:"The subject of the sent mail" key:"subject,title"`
	// ClientHost is the hostname sent in the SMTP EHLO/HELO handshake.
	ClientHost string `default:"localhost" desc:"SMTP client hostname" key:"clienthost"`
	// ClientID is the OAuth2 client ID used to refresh the access token.
	ClientID string `desc:"OAuth2 client ID, for refreshing the access token" key:"clientid" optional:""`
	// ClientSecret is the OAuth2 client secret used to refresh the access
	// token. Some providers do not need one.
	ClientSecret string `desc:"OAuth2 client secret, for refreshing the access token" key:"clientsecret" optional:"" sensitive:"true"`
	// RefreshToken is the OAuth2 refresh token. When it is set, each send
	// gets a current access token from TokenURL, and Password is not used.
	RefreshToken string `desc:"OAuth2 refresh token, for refreshing the access token" key:"refreshtoken" optional:"" sensitive:"true"`
	// TokenURL is the provider's OAuth2 token endpoint, such as
	// https://oauth2.googleapis.com/token.
	TokenURL string `desc:"OAuth2 token endpoint, for refreshing the access token" key:"tokenurl" optional:""`
	// ToAddresses is the list of recipient e-mail addresses.
	ToAddresses []string `desc:"List of recipient e-mails" key:"toaddresses,to"`
	// Auth is the SMTP authentication method. See [AuthTypes].
	Auth authType `default:"Unknown" desc:"SMTP authentication method" key:"auth"`
	// Encryption is the SMTP transport encryption method. See [EncMethods].
	Encryption encMethod `default:"Auto" desc:"Encryption method" key:"encryption"`
	// Timeout is the timeout for the SMTP connection and session.
	Timeout time.Duration `default:"10s" desc:"Timeout for the SMTP connection and session" key:"timeout"`
	// Port is the SMTP server port. Common values are 25, 465, 587, and 2525.
	Port uint16 `default:"25" desc:"SMTP server port, common ones are 25, 465, 587 or 2525" url:"Port"`
	// UseStartTLS controls whether STARTTLS is attempted on non-implicit-TLS connections.
	//
	// A URL without the usestarttls key turns it on, but its zero value turns
	// STARTTLS off. A config built in code sets it to use STARTTLS.
	UseStartTLS bool `default:"Yes" desc:"Whether to use StartTLS encryption" key:"usestarttls,starttls"`
	// UseHTML sends the message as multipart/alternative with an HTML part when true.
	UseHTML bool `default:"No" desc:"Whether the message being sent is in HTML" key:"usehtml"`
	// RequireStartTLS fails the send when STARTTLS is enabled but unsupported.
	RequireStartTLS bool `default:"No" desc:"Fail if StartTLS is enabled but unsupported" key:"requirestarttls"`
	// SkipTLSVerify skips TLS certificate verification when true.
	SkipTLSVerify bool `default:"No" desc:"Whether to skip TLS certificate verification" key:"skiptlsverify"`
}

// Scheme is the identifying part of this service's configuration URL.
const Scheme = "smtp"

// Clone returns a copy of the config.
//
// [Config.ToAddresses] is copied so later mutations do not affect the original.
//
// Returns:
//   - A copy of the configuration with an independent recipient list.
func (c *Config) Clone() Config {
	clone := *c
	clone.ToAddresses = slices.Clone(c.ToAddresses)

	return clone
}

// Enums returns the fields that should use a corresponding EnumFormatter to Print/Parse their values.
//
// Returns:
//   - A map from field names to their enum formatters for [Config.Auth] and [Config.Encryption].
func (c *Config) Enums() map[string]types.EnumFormatter {
	return map[string]types.EnumFormatter{
		"Auth":       AuthTypes.Enum,
		"Encryption": EncMethods.Enum,
	}
}

// GetURL returns a URL representation of its current field values.
//
// Returns:
//   - A smtp:// URL encoding the current configuration.
func (c *Config) GetURL() *url.URL {
	resolver := format.NewPropKeyResolver(c)

	return c.getURL(&resolver)
}

// SetURL updates a [Config] from a URL representation of its field values.
//
// Parameters:
//   - serviceURL: The SMTP configuration URL to parse.
//
// Returns:
//   - An error if a query parameter is invalid or required addresses are missing.
func (c *Config) SetURL(serviceURL *url.URL) error {
	resolver := format.NewPropKeyResolver(c)

	return c.setURL(&resolver, serviceURL)
}

// applyDefaults fills in the settings a config needs to connect. A zero port
// uses [DefaultSMTPPort] and an empty client host uses localhost. An unknown
// auth method is inferred as follows.
//   - [AuthOAuth2] when a refresh token is set.
//   - [AuthPlain] when a username is present.
//   - [AuthNone] otherwise.
func (c *Config) applyDefaults() {
	if c.Port == 0 {
		c.Port = DefaultSMTPPort
	}

	if c.ClientHost == "" {
		c.ClientHost = defaultClientHost
	}

	if c.Auth == AuthTypes.Unknown {
		switch {
		case c.usesTokenRefresh():
			c.Auth = AuthTypes.OAuth2
		case c.Username != "":
			c.Auth = AuthTypes.Plain
		default:
			c.Auth = AuthTypes.None
		}
	}
}

// getURL constructs a URL from the Config's fields using the provided resolver.
//
// [Config.RequireStartTLS] and [Config.SkipTLSVerify] are included only when
// true, and the OAuth2 refresh settings only when set.
//
// Parameters:
//   - resolver: Resolver used to serialize configuration query keys.
//
// Returns:
//   - A smtp:// URL encoding the current configuration.
func (c *Config) getURL(resolver types.ConfigQueryResolver) *url.URL {
	// A zero port selects the default port. The URL leaves it out, and a URL
	// without a port reads back as port 0.
	serviceURL := &url.URL{
		User:       util.URLUserPassword(c.Username, c.Password),
		Host:       util.JoinURLHostPort(c.Host, int(c.Port)),
		Path:       "/",
		Scheme:     Scheme,
		ForceQuery: true,
	}
	// Define primary keys in the exact order matching urlWithAllProps.
	primaryKeys := []string{
		"auth",
		"clienthost",
		"encryption",
		"fromaddress",
		"fromname",
		"subject",
		"toaddresses",
		"usehtml",
		"usestarttls",
		"timeout",
	}

	queryParts := make([]string, 0, len(primaryKeys)+1)
	for _, key := range primaryKeys {
		value, err := resolver.Get(key)
		if err != nil {
			continue
		}

		queryParts = append(queryParts, fmt.Sprintf("%s=%s", key, url.QueryEscape(value)))
	}
	// Include the OAuth2 refresh settings only when set, in a fixed order.
	for _, prop := range [][2]string{
		{"clientid", c.ClientID},
		{"clientsecret", c.ClientSecret},
		{"refreshtoken", c.RefreshToken},
		{"tokenurl", c.TokenURL},
	} {
		if prop[1] != "" {
			queryParts = append(queryParts, fmt.Sprintf("%s=%s", prop[0], url.QueryEscape(prop[1])))
		}
	}
	// Only include requirestarttls if explicitly set to true.
	if c.RequireStartTLS {
		queryParts = append(queryParts, "requirestarttls=Yes")
	}
	// Only include skiptlsverify if explicitly set to true.
	if c.SkipTLSVerify {
		queryParts = append(queryParts, "skiptlsverify=Yes")
	}

	serviceURL.RawQuery = strings.Join(queryParts, "&")

	return serviceURL
}

// prepare readies a config for a send. It normalizes the addresses and fills
// in the defaults, then checks the result. Reading a service URL and each send
// run it, so a config built in code fails before the token request or the dial.
//
// Returns:
//   - error: the error from the config's checks, such as [ErrHostMissing].
func (c *Config) prepare() error {
	c.restorePlusAddresses()
	c.applyDefaults()

	return c.validate()
}

// restorePlusAddresses trims each address and replaces parsed spaces in e-mail
// addr-spec local parts with '+'.
//
// URL query parsing turns plus signs into spaces, so this restores plus-tagged
// addresses such as user+tag@example.com. Spaces around each address are list
// separators and are removed, and recipient entries left blank are dropped, so
// a list without addresses is empty. Display-name spaces, including those in
// mailbox syntax like `Name <user@example.com>`, are left unchanged.
func (c *Config) restorePlusAddresses() {
	c.FromAddress = restorePlusAddress(c.FromAddress)
	for i, adr := range c.ToAddresses {
		c.ToAddresses[i] = restorePlusAddress(adr)
	}

	c.ToAddresses = util.NonEmpty(c.ToAddresses)
}

// restorePlusAddress restores plus tags in a mailbox string.
//
// Whitespace around the mailbox and around an angle-bracket addr-spec is
// trimmed first, because it comes from the list separator (as in
// "a@example.com, b@example.com") rather than from a decoded plus sign. If addr
// uses angle-bracket mailbox syntax, only the addr-spec between '<' and '>' is
// rewritten so display-name spaces are preserved.
//
// Parameters:
//   - addr: A mailbox string, either a bare addr-spec or display-name <addr-spec>.
//
// Returns:
//   - The trimmed mailbox string with plus tags restored in the addr-spec local-part.
func restorePlusAddress(addr string) string {
	addr = strings.TrimSpace(addr)
	start := strings.LastIndex(addr, "<")

	end := strings.LastIndex(addr, ">")
	if start >= 0 && end > start {
		spec := strings.TrimSpace(addr[start+1 : end])

		return addr[:start+1] + restorePlusInAddrSpec(spec) + addr[end:]
	}

	return restorePlusInAddrSpec(addr)
}

// restorePlusInAddrSpec restores plus tags in an RFC 5322 addr-spec.
//
// Spaces in the local-part are replaced with '+' to undo URL query decoding of
// plus-tagged addresses such as user+tag@example.com. The domain is left unchanged.
//
// Parameters:
//   - spec: An addr-spec whose plus tags may have been decoded as spaces.
//
// Returns:
//   - The addr-spec with spaces in the local-part replaced by '+'.
func restorePlusInAddrSpec(spec string) string {
	local, domain, found := strings.Cut(spec, "@")
	if !found {
		return strings.ReplaceAll(spec, " ", "+")
	}

	// An addr-spec without a space in its local part is returned as it is, so
	// a send does not build a copy of each address.
	if !strings.Contains(local, " ") {
		return spec
	}

	return strings.ReplaceAll(local, " ", "+") + "@" + domain
}

// setURL updates the Config from a URL using the provided resolver.
//
// [Config.FromAddress] and [Config.ToAddresses] are required.
// Addresses are trimmed first, so a sender that is only whitespace counts as missing.
//
// Parameters:
//   - resolver: Resolver used to apply query parameters to configuration fields.
//   - serviceURL: The SMTP configuration URL to parse.
//
// Returns:
//   - An error wrapping [util.ErrInvalidHost] for a host that is not a
//     hostname or a bracketed IPv6 address. A port outside 1 to 65535 returns
//     an error wrapping [util.ErrInvalidPort]. A malformed query returns
//     [util.ErrInvalidQuery], and an invalid query parameter returns its
//     error. A config that fails its checks returns their error, such as
//     [ErrHostMissing] for a missing host.
func (c *Config) setURL(resolver types.ConfigQueryResolver, serviceURL *url.URL) error {
	password, _ := serviceURL.User.Password()
	c.Username = serviceURL.User.Username()
	c.Password = password

	// A URL without a port reads as port 0, which the defaults turn into the
	// default port.
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

// validate checks that the config has a server host, a sender, and a
// recipient.
//
// No address may hold a CR or LF, which would break the SMTP commands and
// headers. The authentication and encryption methods must be defined ones. It
// also checks that the OAuth2 refresh settings are complete.
//
// Returns:
//   - error: [ErrHostMissing], [ErrFromAddressMissing], [ErrToAddressMissing],
//     an error wrapping [util.ErrInvalidHost] for a host that holds a port or
//     brackets, an error for an address with a line break, an error wrapping
//     [format.ErrInvalidEnumValue] for an undefined method,
//     [ErrInvalidOAuth2Refresh] for invalid refresh settings, or nil.
func (c *Config) validate() error {
	// An SMTP server has no default host.
	if c.Host == "" {
		return ErrHostMissing
	}

	if err := util.ValidateHostName(c.Host); err != nil {
		return fmt.Errorf("host: %w", err)
	}

	if c.FromAddress == "" {
		return ErrFromAddressMissing
	}

	if len(c.ToAddresses) == 0 {
		return ErrToAddressMissing
	}

	if strings.ContainsAny(c.FromAddress, "\r\n") {
		return fmt.Errorf("sender: %w", errHeaderBreak)
	}

	for _, address := range c.ToAddresses {
		if strings.ContainsAny(address, "\r\n") {
			return fmt.Errorf("recipient: %w", errHeaderBreak)
		}
	}

	if err := format.CheckEnumValue(AuthTypes.Enum, int(c.Auth)); err != nil {
		return fmt.Errorf("auth: %w", err)
	}

	if err := format.CheckEnumValue(EncMethods.Enum, int(c.Encryption)); err != nil {
		return fmt.Errorf("encryption: %w", err)
	}

	return validateOAuth2Refresh(c)
}

// newTLSConfig builds the [tls.Config] used for implicit TLS and STARTTLS from
// the shared helpers. It verifies the server's certificate against Host unless
// [Config.SkipTLSVerify] is set.
//
// Parameters:
//   - config: SMTP configuration providing the server name and [Config.SkipTLSVerify] flag.
//
// Returns:
//   - A TLS config with TLS 1.2 as the minimum version. The maximum version is
//     left unset so crypto/tls can negotiate the highest mutually supported version.
func newTLSConfig(config *Config) *tls.Config {
	if config.SkipTLSVerify {
		return transport.InsecureTLSConfig(config.Host)
	}

	return transport.TLSConfig(config.Host)
}
