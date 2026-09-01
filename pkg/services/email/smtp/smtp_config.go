package smtp

import (
	"crypto/tls"
	"fmt"
	"net"
	"net/url"
	"slices"
	"strconv"
	"strings"
	"time"

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
	Password string `default:"" desc:"SMTP server password or hash (for OAuth2)" url:"Pass"`
	// Port is the SMTP server port. Common values are 25, 465, 587, and 2525.
	Port uint16 `default:"25" desc:"SMTP server port, common ones are 25, 465, 587 or 2525" url:"Port"`
	// FromAddress is the sender e-mail address.
	FromAddress string `desc:"E-mail address that the mail are sent from" key:"fromaddress,from"`
	// FromName is the sender display name.
	FromName string `desc:"Name of the sender" key:"fromname" optional:"yes"`
	// ToAddresses is the list of recipient e-mail addresses.
	ToAddresses []string `desc:"List of recipient e-mails" key:"toaddresses,to"`
	// Subject is the e-mail subject. It is empty when omitted.
	Subject string `default:"" desc:"The subject of the sent mail" key:"subject,title"`
	// Auth is the SMTP authentication method. See [AuthTypes].
	Auth authType `default:"Unknown" desc:"SMTP authentication method" key:"auth"`
	// Encryption is the SMTP transport encryption method. See [EncMethods].
	Encryption encMethod `default:"Auto" desc:"Encryption method" key:"encryption"`
	// UseStartTLS controls whether STARTTLS is attempted on non-implicit-TLS connections.
	UseStartTLS bool `default:"Yes" desc:"Whether to use StartTLS encryption" key:"usestarttls,starttls"`
	// UseHTML sends the message as multipart/alternative with an HTML part when true.
	UseHTML bool `default:"No" desc:"Whether the message being sent is in HTML" key:"usehtml"`
	// ClientHost is the hostname sent in the SMTP EHLO/HELO handshake.
	ClientHost string `default:"localhost" desc:"SMTP client hostname" key:"clienthost"`
	// RequireStartTLS fails the send when STARTTLS is enabled but unsupported.
	RequireStartTLS bool `default:"No" desc:"Fail if StartTLS is enabled but unsupported" key:"requirestarttls"`
	// SkipTLSVerify skips TLS certificate verification when true.
	SkipTLSVerify bool `default:"No" desc:"Whether to skip TLS certificate verification" key:"skiptlsverify"`
	// Timeout is the timeout for the SMTP connection and session.
	Timeout time.Duration `default:"10s" desc:"Timeout for the SMTP connection and session" key:"timeout"`
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

// getURL constructs a URL from the Config's fields using the provided resolver.
//
// [Config.RequireStartTLS] and [Config.SkipTLSVerify] are included only when true.
//
// Parameters:
//   - resolver: Resolver used to serialize configuration query keys.
//
// Returns:
//   - A smtp:// URL encoding the current configuration.
func (c *Config) getURL(resolver types.ConfigQueryResolver) *url.URL {
	serviceURL := &url.URL{
		User:       util.URLUserPassword(c.Username, c.Password),
		Host:       net.JoinHostPort(c.Host, strconv.FormatUint(uint64(c.Port), 10)),
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

// restorePlusAddresses replaces parsed spaces in e-mail addr-spec local parts with '+'.
//
// URL query parsing turns plus signs into spaces, so this restores plus-tagged
// addresses such as user+tag@example.com. Display-name spaces, including those
// in mailbox syntax like `Name <user@example.com>`, are left unchanged.
func (c *Config) restorePlusAddresses() {
	c.FromAddress = restorePlusAddress(c.FromAddress)
	for i, adr := range c.ToAddresses {
		c.ToAddresses[i] = restorePlusAddress(adr)
	}
}

// restorePlusAddress restores plus tags in a mailbox string.
//
// If addr uses angle-bracket mailbox syntax, only the addr-spec between '<'
// and '>' is rewritten so display-name spaces are preserved.
//
// Parameters:
//   - addr: A mailbox string, either a bare addr-spec or display-name <addr-spec>.
//
// Returns:
//   - The mailbox string with plus tags restored in the addr-spec local-part.
func restorePlusAddress(addr string) string {
	start := strings.LastIndex(addr, "<")

	end := strings.LastIndex(addr, ">")
	if start >= 0 && end > start {
		return addr[:start+1] + restorePlusInAddrSpec(addr[start+1:end]) + addr[end:]
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

	return strings.ReplaceAll(local, " ", "+") + "@" + domain
}

// setURL updates the Config from a URL using the provided resolver.
//
// [Config.FromAddress] and [Config.ToAddresses] are required except for the dummy URL used in tests.
//
// Parameters:
//   - resolver: Resolver used to apply query parameters to configuration fields.
//   - serviceURL: The SMTP configuration URL to parse.
//
// Returns:
//   - An error if a query parameter is invalid or required addresses are missing.
func (c *Config) setURL(resolver types.ConfigQueryResolver, serviceURL *url.URL) error {
	password, _ := serviceURL.User.Password()
	c.Username = serviceURL.User.Username()
	c.Password = password
	c.Host = serviceURL.Hostname()

	if port, err := strconv.ParseUint(serviceURL.Port(), 10, 16); err == nil {
		c.Port = uint16(port)
	}

	for key, vals := range serviceURL.Query() {
		if err := resolver.Set(key, vals[0]); err != nil {
			return fmt.Errorf("setting query parameter %q to %q: %w", key, vals[0], err)
		}
	}

	if serviceURL.String() != "smtp://dummy@dummy.com" {
		if len(c.FromAddress) < 1 {
			return ErrFromAddressMissing
		}

		if len(c.ToAddresses) < 1 {
			return ErrToAddressMissing
		}
	}

	c.restorePlusAddresses()

	return nil
}

// newTLSConfig builds the [tls.Config] used for implicit TLS and STARTTLS.
//
// Parameters:
//   - config: SMTP configuration providing the server name and [Config.SkipTLSVerify] flag.
//
// Returns:
//   - A TLS config with TLS 1.2 as the minimum version. The maximum version is
//     left unset so crypto/tls can negotiate the highest mutually supported version.
func newTLSConfig(config *Config) *tls.Config {
	return &tls.Config{
		ServerName:         config.Host,
		MinVersion:         tls.VersionTLS12,
		InsecureSkipVerify: config.SkipTLSVerify,
	}
}
