package smtp

import (
	"crypto/tls"
	"fmt"
	"net/mail"
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
	// FromAddress is the sender e-mail address, such as ops@example.com or
	// Ops <ops@example.com>.
	//
	// It must be one RFC 5322 address. The SMTP envelope gets the bare address,
	// and the From header keeps a display name given here.
	FromAddress string `desc:"E-mail address that the mail are sent from, optionally as Name <address>" key:"fromaddress,from"`
	// FromName is the sender display name in the From header. When it is set,
	// it replaces a display name given in FromAddress.
	FromName string `desc:"Name of the sender" key:"fromname" optional:"yes"`
	// Subject is the e-mail subject. It is empty when omitted.
	Subject string `default:"" desc:"The subject of the sent mail" key:"subject,title"`
	// ClientHost is the hostname sent in the SMTP EHLO/HELO handshake.
	ClientHost string `default:"localhost" desc:"SMTP client hostname" key:"clienthost"`
	// ClientID is the OAuth2 client ID used to get the access token.
	ClientID string `desc:"OAuth2 client ID, for getting the access token" key:"clientid" optional:""`
	// ClientSecret is the OAuth2 client secret used to get the access token.
	// The client credentials grant needs one. Some providers do not need one
	// to refresh a token.
	ClientSecret string `desc:"OAuth2 client secret, for getting the access token" key:"clientsecret" optional:"" sensitive:"true"`
	// RefreshToken is the OAuth2 refresh token. When it is set, each send
	// gets a current access token from TokenURL, and Password is not used.
	RefreshToken string `desc:"OAuth2 refresh token, for refreshing the access token" key:"refreshtoken" optional:"" sensitive:"true"`
	// TokenURL is the provider's OAuth2 token endpoint, such as
	// https://oauth2.googleapis.com/token.
	TokenURL string `desc:"OAuth2 token endpoint, for getting the access token" key:"tokenurl" optional:""`
	// ToAddresses is the list of recipient e-mail addresses, such as
	// jane@example.com or Jane Doe <jane@example.com>.
	//
	// Each entry must be one RFC 5322 address without a comma, because the URL
	// key splits its value at commas. The SMTP envelope gets the bare address,
	// and the To header keeps a display name given here.
	ToAddresses []string `desc:"List of recipient e-mails, each optionally as Name <address>" key:"toaddresses,to"`
	// Scopes are the OAuth2 scopes of the client credentials grant, such as
	// https://outlook.office365.com/.default for Microsoft 365. When they are
	// set without a refresh token, each send gets a current access token from
	// TokenURL with the client ID and secret, and Password is not used.
	Scopes []string `desc:"OAuth2 scopes, for the client credentials grant" key:"scopes" optional:""`
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

// mailbox is one parsed sender or recipient, in the two forms a message
// needs.
type mailbox struct {
	// path is the address for MAIL FROM or RCPT TO, without angle brackets. A
	// local part that needs quoting keeps its quotes, as in "a;b"@example.com.
	path string
	// header is the whole mailbox for the From or To header, with its display
	// name quoted or RFC 2047 encoded where needed.
	header string
}

// mailboxes holds the parsed sender and recipients of a send, with the
// address settings they were parsed from.
type mailboxes struct {
	// fromAddress is the [Config.FromAddress] that from was parsed from.
	fromAddress string
	// fromName is the [Config.FromName] that from was parsed with.
	fromName string
	// toAddresses are the [Config.ToAddresses] that to was parsed from.
	toAddresses []string
	// from is the sender.
	from mailbox
	// to holds the recipients in the order of toAddresses.
	to []mailbox
}

// Scheme is the identifying part of this service's configuration URL.
const Scheme = "smtp"

// Clone returns a copy of the config.
//
// [Config.ToAddresses] and [Config.Scopes] are copied so later mutations do
// not affect the original.
//
// Returns:
//   - A copy of the configuration with independent recipient and scope lists.
func (c *Config) Clone() Config {
	clone := *c
	clone.ToAddresses = slices.Clone(c.ToAddresses)
	clone.Scopes = slices.Clone(c.Scopes)

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

// applyDefaults fills in the settings a config needs to connect.
//
// A zero port uses [DefaultSMTPPort] and an empty client host uses localhost.
// An unknown auth method is inferred as follows.
//   - [AuthOAuth2] when a refresh token or client credentials scopes are set.
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
		case c.grant() != grantNone:
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
// true, and the OAuth2 token settings only when set.
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
	// Include the OAuth2 token settings only when set, in a fixed order.
	for _, key := range []string{"clientid", "clientsecret", "refreshtoken", "scopes", "tokenurl"} {
		value, err := resolver.Get(key)
		if err != nil || value == "" {
			continue
		}

		queryParts = append(queryParts, fmt.Sprintf("%s=%s", key, url.QueryEscape(value)))
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

// parseMailboxes reads the sender and each recipient as one RFC 5322 address.
//
// [Config.FromName] replaces the display name of the sender when it is set. A
// recipient that holds a comma fails. The URL splits the recipient list at
// commas, so the config's own URL could not carry it.
//
// Returns:
//   - *mailboxes: the parsed sender and recipients.
//   - error: an error wrapping [ErrInvalidAddress]. It names the sender or the
//     recipient's position. It leaves out the address, which is personal data.
func (c *Config) parseMailboxes() (*mailboxes, error) {
	from, err := mail.ParseAddress(c.FromAddress)
	if err != nil {
		return nil, fmt.Errorf("sender: %w", ErrInvalidAddress)
	}

	if c.FromName != "" {
		from.Name = c.FromName
	}

	recipients := make([]mailbox, len(c.ToAddresses))
	for i, address := range c.ToAddresses {
		recipient, err := mail.ParseAddress(address)
		if err != nil || strings.Contains(address, ",") {
			return nil, fmt.Errorf("recipient %d: %w", i+1, ErrInvalidAddress)
		}

		recipients[i] = newMailbox(recipient)
	}

	return &mailboxes{
		fromAddress: c.FromAddress,
		fromName:    c.FromName,
		toAddresses: c.ToAddresses,
		from:        newMailbox(from),
		to:          recipients,
	}, nil
}

// prepare readies a config for a send.
//
// It trims the addresses and scopes and fills in the defaults. It then checks
// the result and parses the addresses. Reading a service URL and each send run
// it, so a config built in code fails before the token request or the dial.
//
// Parameters:
//   - previous: the mailboxes an earlier send parsed, or nil. They are used
//     again when the config's address settings still match them, so a send
//     does not parse unchanged addresses again.
//
// Returns:
//   - *mailboxes: the parsed sender and recipients, for the send.
//   - error: the error from the config's checks, such as [ErrHostMissing], or
//     an error wrapping [ErrInvalidAddress].
func (c *Config) prepare(previous *mailboxes) (*mailboxes, error) {
	c.trimAddresses()
	c.trimScopes()
	c.applyDefaults()

	if err := c.validate(); err != nil {
		return nil, err
	}

	if previous != nil && previous.parsedFrom(c) {
		return previous, nil
	}

	return c.parseMailboxes()
}

// restorePlusAddresses replaces the spaces that URL query decoding makes from
// plus signs in each address's addr-spec with '+'.
//
// Reading a URL turns plus signs into spaces, so this restores plus-tagged
// addresses such as user+tag@example.com. Display-name spaces, including
// those in mailbox syntax like `Name <user@example.com>`, are left unchanged.
// Only reading a URL runs it, because addresses set in code or by send params
// were never decoded.
func (c *Config) restorePlusAddresses() {
	c.FromAddress = restorePlusAddress(c.FromAddress)
	for i, adr := range c.ToAddresses {
		c.ToAddresses[i] = restorePlusAddress(adr)
	}
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
// plus-tagged addresses such as user+tag@example.com. The domain is left
// unchanged, and text without an @ is returned as it is.
//
// Parameters:
//   - spec: An addr-spec whose plus tags may have been decoded as spaces.
//
// Returns:
//   - The addr-spec with spaces in the local-part replaced by '+'.
func restorePlusInAddrSpec(spec string) string {
	local, domain, found := strings.Cut(spec, "@")
	if !found || !strings.Contains(local, " ") {
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

	c.restorePlusAddresses()

	_, err = c.prepare(nil)

	return err
}

// trimAddresses trims the sender and each recipient and drops the recipients
// left empty.
//
// Spaces around each address are list separators, as in "a@example.com,
// b@example.com". A sender of only spaces then counts as missing, and a list
// of blank recipients is empty.
func (c *Config) trimAddresses() {
	c.FromAddress = strings.TrimSpace(c.FromAddress)
	for i, address := range c.ToAddresses {
		c.ToAddresses[i] = strings.TrimSpace(address)
	}

	c.ToAddresses = util.NonEmpty(c.ToAddresses)
}

// trimScopes trims each scope and drops the ones left empty.
//
// A scope never holds a space (RFC 6749 section 3.3), so spaces around one
// come from the list separator, as in "scope-a, scope-b". A list of only blank
// scopes is empty, so it selects no client credentials grant.
func (c *Config) trimScopes() {
	for i, scope := range c.Scopes {
		c.Scopes[i] = strings.TrimSpace(scope)
	}

	c.Scopes = util.NonEmpty(c.Scopes)
}

// validate checks that the config has a server host, a sender, and a
// recipient.
//
// The authentication and encryption methods must be defined ones. It also
// checks that the OAuth2 token settings are complete.
//
// Returns:
//   - error: [ErrHostMissing], [ErrFromAddressMissing], [ErrToAddressMissing],
//     an error wrapping [util.ErrInvalidHost] for a host that holds a port or
//     brackets, an error wrapping [format.ErrInvalidEnumValue] for an
//     undefined method, [ErrInvalidOAuth2Settings] for invalid token
//     settings, or nil.
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

	if err := format.CheckEnumValue(AuthTypes.Enum, int(c.Auth)); err != nil {
		return fmt.Errorf("auth: %w", err)
	}

	if err := format.CheckEnumValue(EncMethods.Enum, int(c.Encryption)); err != nil {
		return fmt.Errorf("encryption: %w", err)
	}

	return validateTokenSettings(c)
}

// parsedFrom reports whether m was parsed from the address settings of c.
//
// Parameters:
//   - c: the config whose sender, sender name, and recipients to compare.
//
// Returns:
//   - bool: true when the settings match the ones m was parsed from.
func (m *mailboxes) parsedFrom(c *Config) bool {
	return m.fromAddress == c.FromAddress &&
		m.fromName == c.FromName &&
		slices.Equal(m.toAddresses, c.ToAddresses)
}

// newMailbox returns the SMTP path and the header form of a parsed address.
//
// The path is the address as [mail.Address.String] writes it without a name.
// A local part that needs quoting then keeps its quotes, as RFC 5321 section
// 4.1.2 requires.
//
// Parameters:
//   - address: the parsed address.
//
// Returns:
//   - mailbox: the path and the header form.
func newMailbox(address *mail.Address) mailbox {
	bare := (&mail.Address{Name: "", Address: address.Address}).String()

	header := bare
	if address.Name != "" {
		header = address.String()
	}

	return mailbox{path: bare[1 : len(bare)-1], header: header}
}

// newTLSConfig builds the [tls.Config] used for implicit TLS and STARTTLS from
// the shared helpers.
//
// It verifies the server's certificate against Host unless
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
